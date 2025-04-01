package workers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"time"

	"crawler/app/pkg/assert"
	assetshandler "crawler/app/pkg/assets-handler"
	"crawler/app/pkg/crawler/network"
	wtypes "crawler/app/pkg/crawler/workers/workers-types"
	ctypes "crawler/app/pkg/custom-types"
	customerrors "crawler/app/pkg/custom-types/custom-errors"
)

type BackupWorker struct {
	ID  int
	Ctx context.Context

	// ItemsBackupPacketChan is filled overtime (by the main worker(s)) with the BackupPackets
	// of the items (identified with their IDs) that need to be fetched by the backup worker.
	// The packet also specifies if the url suffix has been appeneded in the original
	// request.
	ItemsBackupPacketChan <-chan *wtypes.BackupPacket

	// ResultsChan is used to send successful fetches results to something that processes them.
	ResultsChan chan<- *wtypes.ContentElement

	// MaxRetries specifies the maximum amount of retries the backup worker can do
	// on each item before skipping it and labeling it as lost / non existing.
	MaxRetries int16

	// Delay specifies the amount of time in milliseconds the backup worker
	// will wait between each request of the same item.
	Delay uint64

	Rand  *rand.Rand
	Fatal error
}

func (bWk *BackupWorker) Run(
	cfg *assetshandler.Config,
	state *wtypes.State,
	outcome *wtypes.Outcome,
) {
	logChan := make(chan ctypes.LogData, 1000)
	defer close(logChan)
	go bWk.log(logChan)

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				bWk.Fatal = err
			} else {
				bWk.Fatal = fmt.Errorf("recover panic: %v", r)
			}
			panic(r)
		} else {
			assert.NotNil(
				bWk.Fatal,
				"at this point worker must have a done ctx error. an unexpected error occurred",
				assert.AssertData{"WorkerID": bWk.ID},
			)
			logChan <- ctypes.LogData{
				Level: slog.LevelError,
				Msg:   "Worker finished due to context done",
			}
		}
	}()

	for {
		select {
		case <-bWk.Ctx.Done():
			bWk.Fatal = fmt.Errorf("worker %v ctx done", bWk.ID)
			return
		case itemPacket := <-bWk.ItemsBackupPacketChan:
			if func() int {
				outcome.Mu.Lock()
				defer outcome.Mu.Unlock()
				return outcome.RateLimits
			}() > cfg.Http.MaxRateLimitsPerSecond {
				time.Sleep((time.Duration)(cfg.Http.RateLimitWait) * time.Second)
			}

			var itemID int = itemPacket.ItemID
			var appendedSuffix bool = itemPacket.AppendSuffix

			var url string = cfg.Standard.Urls.ItemUrl + strconv.Itoa(itemID)

			if itemPacket.AppendSuffix {
				url += cfg.Standard.Urls.ItemUrlAfterID
			}

			var retriesAmount int16
			var s401, s429, s404, sOther uint8
			for {
				if retriesAmount > bWk.MaxRetries {
					var retrySingPlur string
					if retriesAmount > 0 {
						retrySingPlur = "retries"
					} else {
						retrySingPlur = "retry"
					}
					logChan <- ctypes.LogData{
						Level: slog.LevelWarn,
						Msg: fmt.Sprintf("item (ID %v) skipped after %d failed %s (%d 401s, %d 404s, %d 429s, %d unknowns)",
							itemID, retriesAmount, retrySingPlur, s401, s404, s429, sOther),
					}

					outcome.Mu.Lock()
					outcome.Lost++
					outcome.Mu.Unlock()

					break
				}

				if retriesAmount > 0 {
					time.Sleep((time.Duration)(bWk.Delay) * time.Millisecond)
				}

				cookieJarSession := network.PickRandomCookieJarSession(bWk.Rand)

				decodedResp, err := network.FetchDirectJSONUrl(bWk.Ctx, url, cookieJarSession.CookieJar, cfg.Http.Timeout, bWk.Rand)
				if err != nil {
					switch {
					case errors.Is(err, customerrors.ErrorUnauthorized):
						select {
						case cookieJarSession.RefreshChan <- struct{}{}:
						default: // channel is full, the refresher is already working on this
						}
						s401++
					case errors.Is(err, customerrors.ErrorRateLimit):
						s429++
					case errors.Is(err, customerrors.ErrorNotFound):
						s404++
					default:
						sOther++
					}
					retriesAmount++
					continue
				}

				bWk.ResultsChan <- &wtypes.ContentElement{
					Content:   decodedResp,
					ContentID: itemID,
				}

				outcome.Mu.Lock()
				outcome.Recovered++
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

				// it can happen that a server displays some items with a timestamp
				// in the future for internal sync issues, so we make sure to keep
				// the delay positive
				delay := uint32(max(int(time.Since(parsedTs).Milliseconds()), 0))

				state.Mu.Lock()
				state.Delays = append(state.Delays, delay)
				state.Mu.Unlock()

				// XXX: In production this can be removed for increased performance
				var retrySingPlur string
				if retriesAmount > 0 {
					retrySingPlur = "retries"
				} else {
					retrySingPlur = "retry"
				}
				logChan <- ctypes.LogData{
					Level: slog.LevelDebug,
					Msg: fmt.Sprintf("recovered item (ID %v) after %d %s (%d 401s, %d 404s, %d 429s, %d unknowns) ----- %v",
						itemID, retriesAmount+1, retrySingPlur, s401, s404, s429, sOther, delay),
				}

				break
			}
		}
	}
}

func (bWk *BackupWorker) log(logChan <-chan ctypes.LogData) {
	for {
		select {
		case <-bWk.Ctx.Done():
			return
		case data, ok := <-logChan:
			if !ok {
				return
			}
			slog.Log(bWk.Ctx, data.Level, bWk.logFormat(data.Msg))
		}
	}
}

func (bWk *BackupWorker) logFormat(text string) string {
	return fmt.Sprintf("(BackupWorker B%d): %s", bWk.ID, text)
}
