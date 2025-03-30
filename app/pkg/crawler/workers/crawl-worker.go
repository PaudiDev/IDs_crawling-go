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

type CrawlWorker struct {
	ID  int
	Ctx context.Context

	// BackupChan is used to signal to the backup worker(s)
	// that it has to fetch an item due to a miss, indicated by its ID.
	// It also specifies if the url suffix has been appeneded in the original
	// request.
	BackupChan chan<- *wtypes.BackupPacket

	// ResultsChan is used to send successful fetches results to something that processes them.
	ResultsChan chan<- *wtypes.ContentElement

	Rand  *rand.Rand
	Fatal error
}

func (cWk *CrawlWorker) Run(
	cfg *assetshandler.Config,
	core *wtypes.Core,
	state *wtypes.State,
	outcome *wtypes.Outcome,
	handlers *wtypes.Handlers,
) {
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

	var nonExistingOffset int = 500
	var selectedItemID int
	for {
		select {
		case <-cWk.Ctx.Done():
			cWk.Fatal = fmt.Errorf("worker %v ctx done", cWk.ID)
			return
		default:
			if func() int {
				outcome.Mu.Lock()
				defer outcome.Mu.Unlock()
				return outcome.RateLimits
			}() > cfg.Http.MaxRateLimitsPerSecond {
				time.Sleep((time.Duration)(cfg.Http.RateLimitWait) * time.Second)
			}

			state.Mu.Lock()
			core.Mu.Lock()
			onNonExistingItem := state.CurrentID > state.MostRecentID+nonExistingOffset
			onOldItems := core.Step < 0 && state.CurrentID < state.MostRecentID
			core.Mu.Unlock()
			state.Mu.Unlock()

			var tmp_c int
			if onNonExistingItem {
				tmp_c = 1
			} else {
				tmp_c = adjustConcurrency(&handlers.CHandler, cfg, core, state, outcome)
			}
			core.Mu.Lock()
			core.Concurrency = tmp_c
			core.Concurrencies = append(core.Concurrencies, core.Concurrency)
			core.Mu.Unlock()

			if cWk.ID > func() int {
				core.Mu.Lock()
				defer core.Mu.Unlock()
				return core.Concurrency
			}() {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			var tmp_s int
			if onNonExistingItem || onOldItems {
				tmp_s = 1
				state.Mu.Lock()
				state.CurrentID = state.MostRecentID
				state.Mu.Unlock()
			} else {
				tmp_s = adjustStep(&handlers.SHandler, cfg, core, state, outcome)
			}
			core.Mu.Lock()
			core.Step = tmp_s
			core.Steps = append(core.Steps, core.Step)
			core.Mu.Unlock()
			state.Mu.Lock()
			state.CurrentID += tmp_s
			selectedItemID = state.CurrentID
			state.Mu.Unlock()

			cookieJarSession := network.PickRandomCookieJarSession(cWk.Rand)

			decodedResp, appendedSuffix, err := network.FetchItem(cWk.Ctx, cfg, cookieJarSession.CookieJar, selectedItemID, cWk.Rand)
			if err != nil {
				cWk.BackupChan <- &wtypes.BackupPacket{
					ItemID:       selectedItemID,
					AppendSuffix: appendedSuffix,
				}
				outcome.Mu.Lock()
				outcome.SentToBackup++
				outcome.Mu.Unlock()

				switch {
				case errors.Is(err, customerrors.ErrorUnauthorized):
					select {
					case cookieJarSession.RefreshChan <- struct{}{}:
					default: // channel is full, the refresher is already working on this
					}
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
					Msg: fmt.Sprintf("got an error fetching item (ID %v). "+
						"%s ----- %v ----- %v", selectedItemID, err.Error(), tmp_s, tmp_c),
				}
				continue
			}

			outcome.Mu.Lock()
			outcome.Successes++
			outcome.ConsecutiveErrs = 0
			outcome.Mu.Unlock()

			if selectedItemID > func() int {
				state.Mu.Lock()
				defer state.Mu.Unlock()
				return state.MostRecentID
			}() {
				cWk.ResultsChan <- &wtypes.ContentElement{
					Content:   decodedResp,
					ContentID: selectedItemID,
				}

				state.Mu.Lock()
				state.MostRecentID = selectedItemID
				state.Mu.Unlock()

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
				tmp_d := (int)(time.Since(parsedTs).Milliseconds())

				state.Mu.Lock()
				state.DelayNewest = tmp_d
				state.Delays = append(state.Delays, tmp_d)
				state.Mu.Unlock()

				// XXX: In production this can be removed for increased performance
				logChan <- ctypes.LogData{
					Level: slog.LevelDebug,
					Msg: fmt.Sprintf("%v ----- %v ----- %v ----- %v",
						selectedItemID, tmp_d, tmp_s, tmp_c),
				}
			}
		}
	}
}

func (cWk *CrawlWorker) log(logChan <-chan ctypes.LogData) {
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

func (cWk *CrawlWorker) logFormat(text string) string {
	return fmt.Sprintf("(CrawlWorker C%v): %s", cWk.ID, text)
}
