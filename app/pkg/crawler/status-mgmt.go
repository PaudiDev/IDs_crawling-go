package crawler

import (
	"fmt"
	"os"
	"time"

	assetshandler "crawler/app/pkg/assets-handler"
	crawltypes "crawler/app/pkg/crawler/crawl-types"
	"crawler/app/pkg/utils/slicex"
)

// TODO: All this mutex locking/unlocking might be cleaned up with safe get functions or similar

func logAndResetVarsLoop(
	core *Core,
	state *State,
	outcome *Outcome,
	seconds int,
	logFile *os.File,
) {
	for {
		time.Sleep((time.Duration)(seconds) * time.Second)

		outcome.Mu.Lock()
		var totalRequests int = outcome.Successes + outcome.NotFounds +
			outcome.RateLimits + outcome.OtherErrs
		var successRate float32
		if totalRequests > 0 {
			successRate = (float32)(outcome.Successes) / (float32)(totalRequests) * 100
		}
		outcome.Mu.Unlock()

		core.Mu.Lock()
		var avgConcurrency float32
		if count := len(core.Concurrencies); count > 0 {
			avgConcurrency = (float32)(slicex.Sum(core.Concurrencies)) / (float32)(count)
		}
		var avgStep float32
		if count := len(core.Steps); count > 0 {
			avgStep = (float32)(slicex.Sum(core.Steps)) / (float32)(count)
		}
		core.Mu.Unlock()

		state.Mu.Lock()
		var avgDelay float32
		if count := len(state.Delays); count > 0 {
			avgDelay = (float32)(slicex.Sum(state.Delays)) / (float32)(count)
		}
		state.Mu.Unlock()

		logFile.WriteString(
			func() string {
				core.Mu.Lock()
				state.Mu.Lock()
				outcome.Mu.Lock()
				defer outcome.Mu.Unlock()
				defer state.Mu.Unlock()
				defer core.Mu.Unlock()
				return fmt.Sprintf(
					time.Now().Format("2006-01-02 15:04:05.0")+" CURRENT STATUS INFO\n"+
						"Requests per second: %d, Success rate: %.2f%%\n"+
						"Rate limits per second (429): %d, Not found per second (404): %d, "+
						"Other errors per sec: %d\n"+
						"Current ID: %d, Most recent ID: %d, Time since last published: %d\n"+
						"Concurrency: %d, Step: %d\n"+
						"Average Concurrency: %.2f, Average step: %.2f, "+
						"Average delay: %.2f"+
						"\n\n",
					totalRequests, successRate,
					outcome.RateLimits, outcome.NotFounds, outcome.OtherErrs,
					state.CurrentID, state.MostRecentID, state.DelayNewest,
					core.Concurrency, core.Step,
					avgConcurrency, avgStep, avgDelay,
				)
			}(),
		)

		outcome.Mu.Lock()
		outcome.RateLimits = 0
		outcome.NotFounds = 0
		outcome.OtherErrs = 0
		outcome.Successes = 0
		outcome.Mu.Unlock()

		core.Mu.Lock()
		core.Concurrencies = core.Concurrencies[:0]
		core.Steps = core.Steps[:0]
		core.Mu.Unlock()

		state.Mu.Lock()
		state.Delays = state.Delays[:0]
		state.Mu.Unlock()
	}
}

func adjustStep(
	handler *crawltypes.StepHandler,
	cfg *assetshandler.Config,
	core *Core,
	state *State,
	outcome *Outcome,
) int {
	now := (int)(time.Now().UnixMilli())
	handler.Mu.Lock()
	if now-handler.UpdateTime < cfg.Http.StepData.MinChangeTime ||
		func() int {
			state.Mu.Lock()
			defer state.Mu.Unlock()
			return state.DelayNewest
		}() == cfg.Standard.InitialDelay {
		core.Mu.Lock()
		defer core.Mu.Unlock()
		handler.Mu.Unlock()
		return core.Step
	}
	handler.UpdateTime = now
	handler.Mu.Unlock()

	if func() int {
		outcome.Mu.Lock()
		defer outcome.Mu.Unlock()
		return outcome.ConsecutiveErrs
	}() > cfg.Http.StepData.MaxConsecutiveErrors {
		return -2
	}

	var maxStep int
	core.Mu.Lock()
	var step int = core.Step
	core.Mu.Unlock()
	outcome.Mu.Lock()
	var errors int = outcome.NotFounds + outcome.OtherErrs
	outcome.Mu.Unlock()

	step = max(step, 1)
	if errors-func() int {
		outcome.Mu.Lock()
		defer outcome.Mu.Unlock()
		return outcome.Successes
	}() > cfg.Http.StepData.MaxErrorDeviation {
		handler.Mu.Lock()
		if func() int {
			state.Mu.Lock()
			defer state.Mu.Unlock()
			return state.DelayNewest
		}() < cfg.Http.StepData.RetryTime &&
			handler.Retries < cfg.Http.StepData.MaxRetries {
			step = 0
			handler.Retries++
		} else {
			handler.Retries = 0
		}
		handler.Mu.Unlock()
	} else {
		state.Mu.Lock()
		delay := state.DelayNewest
		state.Mu.Unlock()
		switch {
		case delay > cfg.Http.StepData.MaxTime:
			maxStep = 20
			step *= 2
		case delay > cfg.Http.StepData.AggressiveTime:
			maxStep = 10
			step += 2
		case delay > cfg.Http.StepData.MediumTime:
			if func() int {
				core.Mu.Lock()
				defer core.Mu.Unlock()
				return core.Concurrency
			}() == cfg.Core.MaxConcurrency {
				if delay > cfg.Http.StepData.MediumAggressiveTime {
					maxStep = 5
				} else {
					maxStep = 3
				}
				step++
			} else {
				maxStep = 1
			}
		default:
			handler.Mu.Lock()
			if delay > handler.LastDelay+cfg.Http.StepData.LastDelayOffset {
				step--
			} else if delay < handler.LastDelay-cfg.Http.StepData.LastDelayOffset {
				step++
			}
			handler.Mu.Unlock()

			if delay > cfg.Http.StepData.MinTime &&
				func() int {
					core.Mu.Lock()
					defer core.Mu.Unlock()
					return core.Concurrency
				}() == cfg.Core.MaxConcurrency {
				maxStep = 2
			} else {
				maxStep = 1
			}

			step = max(step, 1)
		}

		step = min(step, maxStep)

		handler.Mu.Lock()
		handler.LastDelay = delay
		handler.Mu.Unlock()
	}

	return step
}

func adjustConcurrency(
	handler *crawltypes.ConcurrencyHandler,
	cfg *assetshandler.Config,
	core *Core,
	state *State,
	outcome *Outcome,
) int {
	now := (int)(time.Now().UnixMilli())
	handler.Mu.Lock()
	if now-handler.UpdateTime < cfg.Http.ConcurrencyData.MinChangeTime ||
		func() int {
			state.Mu.Lock()
			defer state.Mu.Unlock()
			return state.DelayNewest
		}() == cfg.Standard.InitialDelay {
		core.Mu.Lock()
		defer core.Mu.Unlock()
		handler.Mu.Unlock()
		return core.Concurrency
	}
	handler.UpdateTime = now
	handler.Mu.Unlock()

	var concurrency int = core.Concurrency
	var maxConcurrency int = cfg.Core.MaxConcurrency
	var minConcurrency int = cfg.Http.ConcurrencyData.MinConcurrency

	if func() int {
		outcome.Mu.Lock()
		defer outcome.Mu.Unlock()
		return outcome.ConsecutiveErrs
	}() > cfg.Http.ConcurrencyData.MaxConsecutiveErrors {
		return max(concurrency/2, minConcurrency)
	}

	outcome.Mu.Lock()
	var errors int = outcome.NotFounds + outcome.OtherErrs
	outcome.Mu.Unlock()

	if errors-func() int {
		outcome.Mu.Lock()
		defer outcome.Mu.Unlock()
		return outcome.Successes
	}() > cfg.Http.ConcurrencyData.MaxErrorDeviation {
		switch {
		case func() int {
			state.Mu.Lock()
			defer state.Mu.Unlock()
			return state.DelayNewest
		}() > cfg.Http.ConcurrencyData.MediumTime:
			concurrency = max(concurrency-1, minConcurrency)
		default:
			concurrency = max(concurrency-3, minConcurrency)
		}
	} else {
		state.Mu.Lock()
		delay := state.DelayNewest
		state.Mu.Unlock()
		switch {
		case delay > cfg.Http.ConcurrencyData.MaxTime:
			concurrency = min(concurrency+1, maxConcurrency)
		case delay > cfg.Http.ConcurrencyData.MediumTime:
		case delay > cfg.Http.ConcurrencyData.MinTime:
			concurrency = max(concurrency-1, minConcurrency)
		default:
			concurrency = max(concurrency-2, minConcurrency)
		}
	}

	return concurrency
}
