package workers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"crawler/app/pkg/assert"
	assetshandler "crawler/app/pkg/assets-handler"
	"crawler/app/pkg/crawler/network"
	wtypes "crawler/app/pkg/crawler/workers/workers-types"
	ctypes "crawler/app/pkg/custom-types"
	customerrors "crawler/app/pkg/custom-types/custom-errors"
)

type ThresholdsWorker struct {
	ID  int
	Ctx context.Context

	// ItemsIDsChan will be used to receive the IDs of the items to fetch.
	ItemsIDsChan <-chan wtypes.BChan

	// ResultsChan is used to send all fetches results (both successful and failed)
	// to something that processes them.
	// The sent type is a ThresholdsWorkerResult object that contains
	// the result JSON object (represented with a map[string]interface{}) along with
	// the associated metadata and the hit threshold level.
	ResultsChan chan<- *wtypes.ThresholdsWorkerResult

	Rand  *rand.Rand
	Fatal error
}

func (tWk *ThresholdsWorker) Run(
	cfg *assetshandler.Config,
	core *wtypes.Core,
	state *wtypes.State,
	outcome *wtypes.Outcome,
	handlers *wtypes.Handlers,
) {
	logChan := make(chan ctypes.LogData, 1000)
	defer close(logChan)
	go tWk.log(logChan)

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				tWk.Fatal = err
			} else {
				tWk.Fatal = fmt.Errorf("recover panic: %v", r)
			}
			panic(r)
		} else {
			assert.NotNil(
				tWk.Fatal,
				"at this point worker must have a done ctx error. an unexpected error occurred",
				assert.AssertData{"WorkerID": tWk.ID},
			)
			logChan <- ctypes.LogData{
				Level: slog.LevelError,
				Msg:   "Worker finished due to context done",
			}
		}
	}()

	for {
		select {
		case <-tWk.Ctx.Done():
			tWk.Fatal = fmt.Errorf("worker %v ctx done", tWk.ID)
			return
		case itemB := <-tWk.ItemsIDsChan:
			if func() int {
				outcome.Mu.Lock()
				defer outcome.Mu.Unlock()
				return outcome.RateLimits
			}() > cfg.Http.MaxRateLimitsPerSecond {
				time.Sleep((time.Duration)(cfg.Http.RateLimitWait) * time.Second)
			}

			itemID := itemB.ID

			cookieJarSession := network.PickRandomCookieJarSession(tWk.Rand)

			decodedResp, appendedSuffix, err := network.FetchItem(tWk.Ctx, cfg, cookieJarSession.CookieJar, itemID, tWk.Rand)
			if err != nil {
				switch {
				case errors.Is(err, customerrors.ErrorUnauthorized):
					cookieJarSession.RefreshChan <- struct{}{}
					outcome.Mu.Lock()
					outcome.OtherErrs++
					outcome.Mu.Unlock()
				case errors.Is(err, customerrors.ErrorRateLimit):
					outcome.Mu.Lock()
					outcome.RateLimits++
					outcome.Mu.Unlock()
				case errors.Is(err, customerrors.ErrorNotFound):
					outcome.Mu.Lock()
					outcome.NotFounds++
					outcome.ConsecutiveErrs++
					outcome.Mu.Unlock()
				default:
					outcome.Mu.Lock()
					outcome.OtherErrs++
					outcome.Mu.Unlock()
				}
				logChan <- ctypes.LogData{
					Level: slog.LevelWarn,
					Msg: fmt.Sprintf(
						"got an error fetching threshold item (ID %d, B %d). %s",
						itemID, itemB.BatchID, err.Error(),
					),
				}

				tWk.ResultsChan <- &wtypes.ThresholdsWorkerResult{
					Item:      nil,
					ItemID:    itemID,
					Success:   false,
					Timestamp: 0,
				}

				continue
			}

			// no outcome.ConsecutiveErrs = 0 to avoid interfering with the sub workers
			outcome.Mu.Lock()
			outcome.Successes++
			outcome.Mu.Unlock()

			var tsKey string
			if appendedSuffix {
				tsKey = cfg.Standard.ItemResponse.TimestampSuffix
			} else {
				tsKey = cfg.Standard.ItemResponse.Timestamp
			}

			item := decodedResp[cfg.Standard.ItemResponse.Item].(map[string]interface{})
			rawTs := item[tsKey].(string)
			parsedTs, err := time.Parse(cfg.Standard.TimestampFormat, rawTs)
			assert.NoError(err, "timestamp must be parsed successfully")
			tmp_d := (uint32)(time.Since(parsedTs).Milliseconds())

			tWk.ResultsChan <- &wtypes.ThresholdsWorkerResult{
				Item:      decodedResp,
				ItemID:    itemID,
				Success:   err == nil,
				Timestamp: tmp_d,
			}

			state.Mu.Lock()
			state.DelayNewest = int(tmp_d)
			state.Delays = append(state.Delays, int(tmp_d))
			state.Mu.Unlock()

			// XXX: In production this can be removed for increased performance
			logChan <- ctypes.LogData{
				Level: slog.LevelDebug,
				Msg:   fmt.Sprintf("threshold item (ID %d B %d) fetched ----- %d", itemID, itemB.BatchID, tmp_d),
			}
		}
	}
}

func (tWk *ThresholdsWorker) log(logChan <-chan ctypes.LogData) {
	for {
		select {
		case <-tWk.Ctx.Done():
			return
		case data, ok := <-logChan:
			if !ok {
				return
			}
			slog.Log(tWk.Ctx, data.Level, tWk.logFormat(data.Msg))
		}
	}
}

func (tWk *ThresholdsWorker) logFormat(text string) string {
	return fmt.Sprintf("(ThresholdsWorker T%d): %s", tWk.ID, text)
}
