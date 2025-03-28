package workers

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"

	"crawler/app/pkg/assert"
	assetshandler "crawler/app/pkg/assets-handler"
	"crawler/app/pkg/crawler/network"
	wtypes "crawler/app/pkg/crawler/workers/workers-types"
	ctypes "crawler/app/pkg/custom-types"
)

type CookiesRefreshWorker struct {
	ID  int
	Ctx context.Context

	CookieJarSession *wtypes.CookieJarSession

	Rand  *rand.Rand
	Fatal error
}

func (cWk *CookiesRefreshWorker) Run(cfg *assetshandler.Config, targetCookieNames []string) {
	logChan := make(chan ctypes.LogData, 1000)
	defer close(logChan)
	go cWk.log(logChan)

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				cWk.Fatal = err
			} else {
				cWk.Fatal = fmt.Errorf("recover panic: %v", r)
			}
			panic(r)
		} else {
			assert.NotNil(
				cWk.Fatal,
				"at this point worker must have a done ctx error. an unexpected error occurred",
				assert.AssertData{"WorkerID": cWk.ID},
			)
			logChan <- ctypes.LogData{
				Level: slog.LevelError,
				Msg:   "Worker finished due to context done",
			}
		}
	}()

	err := network.FetchCookie(cWk.Ctx, cfg, &cWk.CookieJarSession.CookieJar, targetCookieNames, cWk.Rand)
	if cfg.Http.CrashOnFirstCookieFetchError {
		assert.Nil(err, "error fetching first cookie", assert.AssertData{"CWorkerID": cWk.ID})
	}

	logChan <- ctypes.LogData{
		Level: slog.LevelInfo,
		Msg:   "First cookie fetched",
	}

	go network.FetchCookieLoop(cWk.Ctx, cfg, &cWk.CookieJarSession.CookieJar, targetCookieNames, cWk.Rand, logChan)

	for {
		<-cWk.CookieJarSession.RefreshChan
		network.FetchCookie(cWk.Ctx, cfg, &cWk.CookieJarSession.CookieJar, targetCookieNames, cWk.Rand)

		// discard all other refresh requests that might have come in while fetching the cookie
	L:
		for {
			select {
			// TODO: might want to check for err in case of close channel, in all receives actually
			case <-cWk.CookieJarSession.RefreshChan:
			default:
				break L
			}
		}
	}
}

func (cWk *CookiesRefreshWorker) log(logChan <-chan ctypes.LogData) {
	for {
		select {
		case <-cWk.Ctx.Done():
			return
		case data, ok := <-logChan:
			if !ok {
				return
			}
			slog.Log(cWk.Ctx, data.Level, cWk.logFormat(data.Msg))
		}
	}
}

func (cWk *CookiesRefreshWorker) logFormat(text string) string {
	return fmt.Sprintf("(CookiesRefreshWorker C%d): %s", cWk.ID, text)
}
