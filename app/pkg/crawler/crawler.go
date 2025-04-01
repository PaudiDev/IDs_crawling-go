package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"sync"
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

	var state *wtypes.State = new(wtypes.State)
	var outcome *wtypes.Outcome = new(wtypes.Outcome)

	var maxRetriesPerItem uint8 = cfg.Http.MaxRetriesPerItem
	var delayBetweenRetries uint64 = cfg.Http.DelayBetweenRetries

	// if this amount of thresholds is exceeded the system will slow down.
	//
	// TODO: maybe in future handle this by dynamically spawning workers
	// as the user might not have any idea about what value to put here.
	var idealMaxThresholdsAmount uint8 = cfg.Core.ExpMaxThresholdsAmount
	thresholdsWkIDsChan := make(chan *wtypes.ItemFromBatchPacket, idealMaxThresholdsAmount)
	thresholdsWkResultsChan := make(chan *wtypes.ThresholdsWorkerResult, idealMaxThresholdsAmount)

	var subWorkersAmount uint16 = uint16(cfg.Core.ThresholdsOffset) * uint16(idealMaxThresholdsAmount)

	// Each request takes a different amount of time to complete
	// and little delays are introduced all the time.
	// To take care of this, the channel size is tripled.
	subordinateWkIDsChannel := make(chan *wtypes.ItemFromBatchPacket, subWorkersAmount*3)

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
	//
	// backupWorkersAmount is a uint32 because, even letting N and M being their highest possible
	// values (2^16-1 and 2^8-1 respectively), the result of N*M*(1+D/1000) exceeds
	// the maximum value of a uint32 (2^32-1) only if D is greater than 4.2 minutes.
	// This is a very unlikely scenario, so we can safely use uint32 instead of uint64.
	var backupWorkersAmount uint32
	if maxRetriesPerItem < 2 {
		backupWorkersAmount = uint32(subWorkersAmount)
	} else {
		backupWorkersAmount = uint32(subWorkersAmount) * (uint32)(maxRetriesPerItem)
	}
	backupWorkersAmount *= 1 + uint32(math.Ceil((float64)(delayBetweenRetries)/1000))
	backupChan := make(chan *wtypes.BackupPacket, backupWorkersAmount*3)

	// this channel is used to send results by subordinate Wks, backup Wks and
	// the workers manager.
	// since the backup workers are the ones in majority, the channel size is
	// set to their amount.
	wsChan := make(chan *wtypes.ContentElement, backupWorkersAmount)

	var wg sync.WaitGroup

	for i, cookieJarSession := range network.CookieJarSessionsPool {
		wg.Add(1)
		cWk := &workers.CookiesRefreshWorker{
			ID:               i,
			Ctx:              ctx,
			CookieJarSession: cookieJarSession,
			OnSessionReady:   wg.Done,
			Rand:             rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		go cWk.Run(cfg, cfg.Standard.SessionCookieNames)
	}

	//
	// Start the main workers
	//

	// TODO: do not use one rand source per worker, instead implement a thread safe one

	for i := uint16(1); i <= subWorkersAmount; i++ {
		sWk := &workers.SubordinateWorker{
			ID:           int(i),
			Ctx:          ctx,
			ItemsIDsChan: subordinateWkIDsChannel,
			ResultsChan:  wsChan,
			BackupChan:   backupChan,
			Rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		go sWk.Run(cfg, state, outcome)
	}

	for j := uint32(1); j <= backupWorkersAmount; j++ {
		bWk := &workers.BackupWorker{
			ID:                    int(j),
			Ctx:                   ctx,
			ItemsBackupPacketChan: backupChan,
			ResultsChan:           wsChan,
			MaxRetries:            int16(maxRetriesPerItem) - 1,
			Delay:                 delayBetweenRetries,
			Rand:                  rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		go bWk.Run(cfg, state, outcome)
	}

	for k := uint8(1); k <= idealMaxThresholdsAmount; k++ {
		tWk := &workers.ThresholdsWorker{
			ID:           int(k),
			Ctx:          ctx,
			ItemsIDsChan: thresholdsWkIDsChan,
			ResultsChan:  thresholdsWkResultsChan,
			Rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		go tWk.Run(cfg, state, outcome)
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

	compiledThresholdsAdjPolicies, err := compilePolicies(cfg.Policies)
	assert.NoError(err, "all thresholds adjustment policies must be compiled successfully")

	thresholdsControllerCfg := &thresholds.ThresholdsControllerConfig{
		InitialThresholdsAmount:      uint16(cfg.Core.ThresholdsInitialAmount),
		ThresholdsAdjustmentPolicies: compiledThresholdsAdjPolicies,
	}
	thresholdsController, err := thresholds.NewThresholdsController(thresholdsControllerCfg)
	assert.NoError(err, "thresholds controller must be created successfully")

	mainRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	// wait for the cookies refresher workers to fetch all the cookies for the first time
	wg.Wait()
	cookieJarSession := network.PickRandomCookieJarSession(mainRand)

	state.HighestID, err = network.FetchHighestID(ctx, cfg, cookieJarSession.CookieJar, mainRand)
	assert.NoError(
		err, "highest id fetch must be successful to start the crawler",
		assert.AssertData{
			"CookieJar": cookieJarSession.CookieJar,
		},
	)

	//
	// Start the status logger
	//

	logSeconds := 1
	go workers.LogAndResetVarsLoop(state, outcome, logSeconds, statusLogFile)

	//
	// Start the workers manager
	//

	var wksManager workersManager = workersManager{
		thresholdsController: thresholdsController,
		offset:               uint16(cfg.Core.ThresholdsOffset),
		rand:                 rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	wksManager.run(
		thresholdsWkIDsChan,
		thresholdsWkResultsChan,
		subordinateWkIDsChannel,
		wsChan,
		state,
		&cfg.Core.BatchLimits,
	)
}
