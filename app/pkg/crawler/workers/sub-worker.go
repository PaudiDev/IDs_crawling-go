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

type SubordinateWorker struct {
	ID  int
	Ctx context.Context

	// ItemsIDsChan will be used to receive the IDs of the items to fetch.
	// It also contains the ID of the batch the item belongs to, which is used for logging purposes.
	ItemsIDsChan <-chan *wtypes.ItemFromBatchPacket

	// ResultsChan is used to send successful fetches results to something that processes them.
	ResultsChan chan<- *wtypes.ContentElement

	// BackupChan is used to signal to the backup worker(s)
	// that it has to fetch an item due to a miss, indicated by its ID.
	// It also specifies if the url suffix has been appended in the original
	// request.
	BackupChan chan<- *wtypes.BackupPacket

	Rand  *rand.Rand
	Fatal error
}

func (sWk *SubordinateWorker) Run(
	cfg *assetshandler.Config,
	core *wtypes.Core,
	state *wtypes.State,
	outcome *wtypes.Outcome,
	handlers *wtypes.Handlers,
) {
	logChan := make(chan ctypes.LogData, 1000)
	defer close(logChan)
	go sWk.log(logChan)

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				sWk.Fatal = err
			} else {
				sWk.Fatal = fmt.Errorf("recover panic: %v", r)
			}
			panic(r)
		} else {
			assert.NotNil(
				sWk.Fatal,
				"at this point worker must have a done ctx error. an unexpected error occurred",
				assert.AssertData{"WorkerID": sWk.ID},
			)
			logChan <- ctypes.LogData{
				Level: slog.LevelError,
				Msg:   "Worker finished due to context done",
			}
		}
	}()

	for {
		select {
		case <-sWk.Ctx.Done():
			sWk.Fatal = fmt.Errorf("worker %v ctx done", sWk.ID)
			return
		case itemRequest := <-sWk.ItemsIDsChan:
			itemID := itemRequest.ItemID
			if func() int {
				outcome.Mu.Lock()
				defer outcome.Mu.Unlock()
				return outcome.RateLimits
			}() > cfg.Http.MaxRateLimitsPerSecond {
				time.Sleep((time.Duration)(cfg.Http.RateLimitWait) * time.Second)
			}

			cookieJarSession := network.PickRandomCookieJarSession(sWk.Rand)

			decodedResp, appendedSuffix, err := network.FetchItem(sWk.Ctx, cfg, cookieJarSession.CookieJar, itemID, sWk.Rand)
			if err != nil {
				sWk.BackupChan <- &wtypes.BackupPacket{
					ItemID:       itemID,
					AppendSuffix: appendedSuffix,
				}
				outcome.Mu.Lock()
				outcome.SentToBackup++
				outcome.Mu.Unlock()

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
						"got an error fetching item (ID %d, B %d). %s",
						itemID, itemRequest.BatchID, err.Error(),
					),
				}
				continue
			}

			outcome.Mu.Lock()
			outcome.Successes++
			outcome.ConsecutiveErrs = 0
			outcome.Mu.Unlock()

			sWk.ResultsChan <- &wtypes.ContentElement{
				Content:   decodedResp,
				ContentID: itemID,
			}

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

			// it can happen that a server displays some items with a timestamp
			// in the future for internal sync issues, so we make sure to keep
			// the delay positive
			delay := max(int(time.Since(parsedTs).Milliseconds()), 0)

			state.Mu.Lock()
			state.DelayNewest = delay
			state.Delays = append(state.Delays, delay)
			state.Mu.Unlock()

			// XXX: In production this can be removed for increased performance
			logChan <- ctypes.LogData{
				Level: slog.LevelDebug,
				Msg:   fmt.Sprintf("item (ID %d, B %d) fetched ----- %d", itemID, itemRequest.BatchID, delay),
			}
		}
	}
}

func (sWk *SubordinateWorker) log(logChan <-chan ctypes.LogData) {
	for {
		select {
		case <-sWk.Ctx.Done():
			return
		case data, ok := <-logChan:
			if !ok {
				return
			}
			slog.Log(sWk.Ctx, data.Level, sWk.logFormat(data.Msg))
		}
	}
}

func (sWk *SubordinateWorker) logFormat(text string) string {
	return fmt.Sprintf("(SubordinateWorker S%d): %s", sWk.ID, text)
}
