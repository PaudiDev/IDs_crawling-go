package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"time"

	"crawler/app/pkg/assert"
	assetshandler "crawler/app/pkg/assets-handler"
	"crawler/app/pkg/crawler/network"
	"crawler/app/pkg/crawler/workers"
	wtypes "crawler/app/pkg/crawler/workers/workers-types"
	safews "crawler/app/pkg/safe-ws"
	"crawler/app/pkg/thresholds"
)

func Start(ctx context.Context, cfg *assetshandler.Config, conns []*safews.SafeConn, statusLogFile *os.File) {
	slog.Info("Crawler Started...")

	//
	// Setup workers related variables
	//

	var core *wtypes.Core = wtypes.NewCore(cfg)
	var state *wtypes.State = wtypes.NewState(cfg)
	var outcome *wtypes.Outcome = new(wtypes.Outcome)

	var maxRetriesPerItem uint8 = cfg.Http.MaxRetriesPerItem
	var delayBetweenRetries uint64 = cfg.Http.DelayBetweenRetries

	// (2^16-1)/500 ~= 131 should be enough. (the thresholds variables are uint16).
	// if this amount of thresholds is exceeded the system will slow down.
	//
	// TODO: maybe in future handle this by dynamically spawning workers
	// or putting them in a pool, this ideal limit is not the best for every purpose.
	idealMaxThresholdsAmount := math.MaxUint16 / 500
	thresholdsWkIDsChan := make(chan int, idealMaxThresholdsAmount)
	thresholdsWkResultsChan := make(chan *wtypes.ThresholdsWorkerResult, idealMaxThresholdsAmount)

	var subWorkersAmount int = 25 * idealMaxThresholdsAmount / 2 // TODO: replace 25 with cfg.Thresholds.Offset

	// same reasoning as the backup workers amount.
	subordinateWkIDsChannel := make(chan int, subWorkersAmount*3)
	fmt.Println("Initial size:", subWorkersAmount*3)

	// Let's rename "subWorkersAmount" to N, "maxRetriesPerItem" to M and
	// "delayBetweenRetries" to D.
	//
	// Hypothesize all requests fail and each one takes the same amount of T ms.
	// In this situation, each subordinate worker will produce a backup packet every T ms,
	// while each backup worker will take M*(T+D) ms to consume it.
	//
	// Being X the amount of backup workers needed to exactly match the
	// production and consumption rates, X will need to satisfy the following equation:
	// X/[M*(T+D)] = N/T ==> X = N*M*(T+D)/T = N*M*(1+D/T).
	// In the hypothesized scenario, with this amount of backup workers,
	// a backup channel size of N*M*(1+D/T) is never exceeded.
	//
	// Since a GET request ideally takes less than 1 second to complete, assuming
	// T = 1000 ms is a good approximation.
	// In this case, the amount of backup workers needed is N*M*(1+D/1000).
	//
	// In practice, each request takes a different amount of time to complete
	// and little delays are introduced all the time.
	// To take care of this, the channel size is tripled.
	//
	// The amount of backup workers could also be tripled instead but, since
	// the hypothesis of all requests failing is very pessimistic, it would only
	// waste many resources without any actual benefit.
	var backupWorkersAmount int
	if maxRetriesPerItem < 2 {
		backupWorkersAmount = subWorkersAmount
	} else {
		backupWorkersAmount = subWorkersAmount * (int)(maxRetriesPerItem)
	}
	backupWorkersAmount *= 1 + int(math.Ceil((float64)(delayBetweenRetries)/1000))
	backupChan := make(chan wtypes.BackupPacket, backupWorkersAmount*3)

	// this channel is used to send results by subordinate Wks, backup Wks and
	// the workers manager.
	// since the backup workers are the ones in majority, the channel size is
	// set to their amount.
	wsChan := make(chan *wtypes.WsContentElement, backupWorkersAmount)

	var handlers *wtypes.Handlers = wtypes.NewHandlers()

	// traceFn := func(uniqId string) context.Context {
	// 	trace := &httptrace.ClientTrace{
	// 		GetConn: func(hostPort string) {
	// 			fmt.Println("GetConn id:", uniqId, hostPort)
	// 		},
	// 		GotConn: func(connInfo httptrace.GotConnInfo) {
	// 			fmt.Println("GotConn id:", uniqId, connInfo.Conn.LocalAddr())
	// 		},

	// 		ConnectStart: func(network, addr string) {
	// 			fmt.Println("ConnectStart id:", uniqId, network, addr)
	// 		},
	// 		ConnectDone: func(network, addr string, err error) {
	// 			fmt.Println("ConnectDone id:", uniqId, network, addr, err)
	// 		},
	// 	}
	// 	return httptrace.WithClientTrace(ctx, trace)
	// }

	//
	// Start the cookies refresher workers
	//

	cookieJarSessionsPool := network.CookieJarSessionsPool
	for i, cookieJarSession := range cookieJarSessionsPool {
		cWk := &workers.CookiesRefreshWorker{
			ID:               i,
			Ctx:              ctx, // traceFn("C" + strconv.Itoa(i)),
			CookieJarSession: cookieJarSession,
			Rand:             rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		go cWk.Run(cfg, cfg.Standard.SessionCookieNames)
	}

	//
	// Start the main workers
	//

	for i := 1; i <= subWorkersAmount; i++ {
		sWk := &workers.SubordinateWorker{
			ID:           i,
			Ctx:          ctx, // traceFn("S" + strconv.Itoa(i)),
			ItemsIDsChan: subordinateWkIDsChannel,
			ResultsChan:  wsChan,
			BackupChan:   backupChan,
			Rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		go sWk.Run(cfg, core, state, outcome, handlers)
	}

	for j := 1; j <= backupWorkersAmount; j++ {
		bWk := &workers.BackupWorker{
			ID:           j,
			Ctx:          ctx, // traceFn("B" + strconv.Itoa(j)),
			ItemsIDsChan: backupChan,
			ResultsChan:  wsChan,
			MaxRetries:   int16(maxRetriesPerItem) - 1,
			Delay:        delayBetweenRetries,
			Rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		go bWk.Run(cfg, core, state, outcome, handlers)
	}

	for k := 1; k <= idealMaxThresholdsAmount; k++ {
		tWk := &workers.ThresholdsWorker{
			ID:           k,
			Ctx:          ctx, // traceFn("T" + strconv.Itoa(k)),
			ItemsIDsChan: thresholdsWkIDsChan,
			ResultsChan:  thresholdsWkResultsChan,
			Rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		go tWk.Run(cfg, core, state, outcome, handlers)
	}

	slog.Info(
		fmt.Sprintf(
			"%d thresholds workers, %d subordinate workers and %d backup workers Started...",
			idealMaxThresholdsAmount, subWorkersAmount, backupWorkersAmount,
		),
	)

	//
	// Start the websocket worker
	//

	wsWk := &workers.WebsocketWorker{
		ID:           1,
		Ctx:          ctx,
		ContentsChan: wsChan,
		Conns:        conns,
	}

	go wsWk.Run()

	//
	// Setup workers manager related variables
	//

	mainRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	// wait a bit for the cookies refresher workers to fetch the cookies for the first time
	time.Sleep(5 * time.Second)
	cookieJarSession := network.PickRandomCookieJarSession(mainRand)

	var err error
	state.CurrentID, err = network.FetchHighestID(ctx, cfg, cookieJarSession.CookieJar, mainRand)
	assert.NoError(
		err, "highest id fetch must be successful to start the crawler",
		assert.AssertData{
			"CookieJar": cookieJarSession.CookieJar,
		},
	)
	state.MostRecentID = state.CurrentID

	thresholdsControllerCfg := &thresholds.ThresholdsControllerConfig{
		InitialThresholdsAmount:      4, // cfg.Thresholds.InitialAmount,
		ThresholdsAdjustmentPolicies: makeThresholdsAdjustmentPolicies(),
	}
	thresholdsController, err := thresholds.NewThresholdsController(thresholdsControllerCfg)
	assert.NoError(err, "thresholds controller must be created successfully")

	//
	// Start the status handler
	//

	logSeconds := 1
	go workers.LogAndResetVarsLoop(core, state, outcome, logSeconds, statusLogFile)

	//
	// Start the workers manager
	//

	var wksManager workersManager = workersManager{
		thresholdsController: thresholdsController,
		offset:               25, // cfg.Thresholds.Offset,
		rand:                 rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	wksManager.run(
		thresholdsWkIDsChan,
		thresholdsWkResultsChan,
		subordinateWkIDsChannel,
		wsChan,
		state.CurrentID,
	)
}

func makeThresholdsAdjustmentPolicies() []thresholds.ThresholdsAdjustmentPolicy {
	// return []thresholds.ThresholdsAdjustmentPolicy{
	// 	{
	// 		Percentage: 0,
	// 		ComputeIncrement: func(currentTimestamp, newTimestamp uint32, thresholdsAmount uint16) int32 {
	// 			return 0
	// 		},
	// 	},
	// }
	return []thresholds.ThresholdsAdjustmentPolicy{
		{
			Percentage: 0.9,
			ComputeIncrement: func(currentTimestamp, newTimestamp uint32, thresholdsAmount uint16) int32 {
				if newTimestamp >= currentTimestamp+500 {
					return 2
				}
				return 1
			},
		},
		{
			Percentage: 0.5,
			ComputeIncrement: func(currentTimestamp, newTimestamp uint32, thresholdsAmount uint16) int32 {
				return 0
			},
		},
		{
			Percentage: 0.3,
			ComputeIncrement: func(currentTimestamp, newTimestamp uint32, thresholdsAmount uint16) int32 {
				return -1
			},
		},
		{
			Percentage: 0,
			ComputeIncrement: func(currentTimestamp, newTimestamp uint32, thresholdsAmount uint16) int32 {
				return int32(-0.25 * float32(thresholdsAmount))
			},
		},
	}
}
