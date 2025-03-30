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
)

func Start(ctx context.Context, cfg *assetshandler.Config, conns []*safews.SafeConn, statusLogFile *os.File) {
	slog.Info("Crawler Started...")

	var core *wtypes.Core = wtypes.NewCore(cfg)
	var state *wtypes.State = wtypes.NewState(cfg)
	var outcome *wtypes.Outcome = new(wtypes.Outcome)

	var crawlWorkersAmount int = cfg.Core.MaxConcurrency
	var maxRetriesPerItem uint8 = cfg.Http.MaxRetriesPerItem
	var delayBetweenRetries uint64 = cfg.Http.DelayBetweenRetries

	// Let's rename "crawlWorkersAmount" to N, "maxRetriesPerItem" to M and
	// "delayBetweenRetries" to D.
	//
	// Hypothesize all requests fail and each one takes the same amount of T ms.
	// In this situation, each crawl worker will produce a backup packet every T ms,
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
	// To take care of this, the channel size is doubled.
	//
	// The amount of backup workers could also be doubled instead but, since
	// the hypothesis of all requests failing is very pessimistic, it would only
	// waste double the resources without any actual benefit.
	var backupWorkersAmount int
	if maxRetriesPerItem < 2 {
		backupWorkersAmount = crawlWorkersAmount
	} else {
		backupWorkersAmount = crawlWorkersAmount * (int)(maxRetriesPerItem)
	}
	backupWorkersAmount *= 1 + int(math.Ceil((float64)(delayBetweenRetries)/1000))
	backupChan := make(chan *wtypes.BackupPacket, backupWorkersAmount*2)

	// this channel is used to send results by subordinate Wks and backup Wks.
	// since the backup workers are the ones in majority, the channel size is
	// set to their amount.
	wsChan := make(chan *wtypes.ContentElement, backupWorkersAmount)

	var handlers *wtypes.Handlers = wtypes.NewHandlers()

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

	wsWk := &workers.WebsocketWorker{
		ID:           1,
		Ctx:          ctx,
		ContentsChan: wsChan,
		Conns:        conns,
	}

	go wsWk.Run()

	mainRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	// wait for the cookies refresher workers to fetch all the cookies for the first time
	wg.Wait()
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

	for i := 1; i <= crawlWorkersAmount; i++ {
		cWk := &workers.CrawlWorker{
			ID:          i,
			Ctx:         ctx,
			ResultsChan: wsChan,
			BackupChan:  backupChan,
			Rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		go cWk.Run(cfg, core, state, outcome, handlers)
	}

	for j := 1; j <= backupWorkersAmount; j++ {
		bWk := &workers.BackupWorker{
			ID:          j,
			Ctx:         ctx,
			ItemsChan:   backupChan,
			ResultsChan: wsChan,
			MaxRetries:  int16(maxRetriesPerItem) - 1,
			Delay:       delayBetweenRetries,
			Rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		go bWk.Run(cfg, core, state, outcome, handlers)
	}

	slog.Info(fmt.Sprintf("%d crawl workers and %d backup workers Started...", crawlWorkersAmount, backupWorkersAmount))

	logSeconds := 1
	workers.LogAndResetVarsLoop(core, state, outcome, logSeconds, statusLogFile)
}
