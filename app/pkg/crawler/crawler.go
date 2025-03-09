package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http/cookiejar"
	"os"
	"time"

	"crawler/app/pkg/assert"
	assetshandler "crawler/app/pkg/assets-handler"
	crawltypes "crawler/app/pkg/crawler/crawl-types"
	ctypes "crawler/app/pkg/custom-types"
	customerrors "crawler/app/pkg/custom-types/custom-errors"
	safews "crawler/app/pkg/safe-ws"
	"crawler/app/pkg/utils/fmtx"
	"crawler/app/pkg/utils/httpx"

	"github.com/gorilla/websocket"
)

type worker struct {
	ID    int
	Ctx   context.Context
	Rand  *rand.Rand
	Fatal error
}

func (wk *worker) run(
	cfg *assetshandler.Config,
	core *Core,
	state *State,
	outcome *Outcome,
	handlers *crawltypes.Handlers,
	conns []*safews.SafeConn,
) {
	logChan := make(chan ctypes.LogData, 1000)
	defer close(logChan)
	go wk.Log(logChan)

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				wk.Fatal = err
			} else {
				wk.Fatal = fmt.Errorf("recover panic: %v", r)
			}
			panic(r)
		} else {
			assert.NotNil(
				wk.Fatal,
				"at this point worker must have a done ctx error. an unexpected error occurred",
				assert.AssertData{"WorkerID": wk.ID},
			)
			logChan <- ctypes.LogData{
				Level: slog.LevelError,
				Msg:   "Worker finished due to context done",
			}
		}
	}()

	var currentConnIdx int = 0
	var connsAmount int = len(conns)

	jar, err := cookiejar.New(nil)
	assert.NoError(err, fmtx.Worker("cookie jar must be created to start the worker", wk.ID))

	headers := createBaseHeaders(httpx.PickRandomUserAgent(wk.Rand))

	fetchCookie(wk.Ctx, cfg, jar, cfg.Standard.SessionCookieNames, headers, wk.Rand)
	go fetchCookieLoop(wk.Ctx, cfg, jar, cfg.Standard.SessionCookieNames, headers, wk.Rand, logChan)

	var nonExistingOffset int = 500
	var selectedItemID int
	for {
		select {
		case <-wk.Ctx.Done():
			wk.Fatal = fmt.Errorf("worker %v ctx done", wk.ID)
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

			if wk.ID > func() int {
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

			for k, v := range createBaseHeaders(httpx.PickRandomUserAgent(wk.Rand)) {
				headers[k] = v
			}

			decodedResp, appendedSuffix, err := fetchItem(wk.Ctx, cfg, jar, selectedItemID, headers, wk.Rand)
			if err != nil {
				switch {
				case errors.Is(err, customerrors.ErrorUnauthorized):
					fetchCookie(wk.Ctx, cfg, jar, cfg.Standard.SessionCookieNames, headers, wk.Rand)
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
					Level: slog.LevelError,
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
				go func() {
					jsonResponse, err := json.Marshal(decodedResp)
					if err != nil {
						logChan <- ctypes.LogData{
							Level: slog.LevelError,
							Msg: fmt.Sprintf(
								"error marshalling item response to json, "+
									"impossible sending to websocket (ID %v): %v",
								selectedItemID, err,
							),
						}
						return
					}

					err = conns[currentConnIdx].WriteMessage(websocket.TextMessage, jsonResponse)
					if err != nil {
						logChan <- ctypes.LogData{
							Level: slog.LevelError,
							Msg: fmt.Sprintf(
								"error sending item to websocket (ID %v): %v",
								selectedItemID, err,
							),
						}
					}
					currentConnIdx = (currentConnIdx + 1) % connsAmount
				}()

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

func (wk *worker) Log(logChan <-chan ctypes.LogData) {
	for {
		select {
		case <-wk.Ctx.Done():
			return
		case data, ok := <-logChan:
			if !ok {
				return
			}
			slog.Log(wk.Ctx, data.Level, fmtx.Worker(data.Msg, wk.ID))
		}
	}
}

func Start(ctx context.Context, cfg *assetshandler.Config, conns []*safews.SafeConn, statusLogFile *os.File) {
	slog.Info("Crawler Started...")

	var mainRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	headers := createScrapingHeaders(httpx.PickRandomUserAgent(mainRand))
	jar, err := cookiejar.New(nil)
	assert.NoError(err, "cookie jar must be created to start the crawler")

	err = fetchCookie(ctx, cfg, jar, cfg.Standard.SessionCookieNames, headers, mainRand)
	assert.NoError(err, "first cookie fetch must be successful to start the crawler")

	var core *Core = NewCore(cfg)
	var state *State = NewState(cfg)
	var outcome *Outcome = new(Outcome)

	state.CurrentID, err = fetchHighestID(ctx, cfg, jar, headers, mainRand)
	assert.NoError(
		err, "highest id fetch must be successful to start the crawler",
		assert.AssertData{
			"CookieJar": jar,
			"Headers":   headers,
		},
	)
	state.MostRecentID = state.CurrentID

	var handlers *crawltypes.Handlers = crawltypes.NewHandlers()

	for i := 0; i < cfg.Core.MaxConcurrency; i++ {
		wk := &worker{
			ID: i + 1, Ctx: ctx,
			Rand: rand.New(rand.NewSource(time.Now().UnixNano())),
		}

		go wk.run(cfg, core, state, outcome, handlers, conns)
	}

	slog.Info(fmt.Sprintf("%v workers Started...", cfg.Core.MaxConcurrency))

	logSeconds := 1
	logAndResetVarsLoop(core, state, outcome, logSeconds, statusLogFile)
}
