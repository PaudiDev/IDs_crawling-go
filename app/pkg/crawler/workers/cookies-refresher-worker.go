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

// This struct does not implement the Worker interface.
// However, the directory is the same for structural organization purposes.
type CookiesRefreshWorker struct {
	ID  int
	Ctx context.Context

	// A session that contains the actual cookie jar this worker manages, along with
	// a refresh channel that other workers can use to request a force refresh of the jar.
	CookieJarSession *wtypes.CookieJarSession

	// A function that will be called from the refresh worker once the first fetch
	// of the jar session has been completed.
	// This is useful to notify the Run caller that the session is ready to be used.
	OnSessionReady func()

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
		assert.NoError(err, "error fetching first cookie", assert.AssertData{"CWorkerID": cWk.ID})
	}

	cWk.OnSessionReady()

	logChan <- ctypes.LogData{
		Level: slog.LevelInfo,
		Msg:   "First cookie fetched",
	}

	go network.FetchCookieLoop(cWk.Ctx, cfg, &cWk.CookieJarSession.CookieJar, targetCookieNames, cWk.Rand, logChan)

	for {
		select {
		case <-cWk.Ctx.Done():
			cWk.Fatal = fmt.Errorf("worker %v ctx done", cWk.ID)
			return
		case <-cWk.CookieJarSession.RefreshChan:
			network.FetchCookie(cWk.Ctx, cfg, &cWk.CookieJarSession.CookieJar, targetCookieNames, cWk.Rand)

			// discard all other refresh requests that might have come in while fetching the cookie
		L:
			for {
				select {
				case <-cWk.Ctx.Done():
					cWk.Fatal = fmt.Errorf("worker %v ctx done", cWk.ID)
					return
				case <-cWk.CookieJarSession.RefreshChan:
				default:
					break L
				}
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
