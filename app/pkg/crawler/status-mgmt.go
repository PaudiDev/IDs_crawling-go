package crawler

import (
	"time"

	assetshandler "crawler/app/pkg/assets-handler"
	crawltypes "crawler/app/pkg/crawler/crawl-types"
)

func adjustStep(
	handler *crawltypes.StepHandler,
	cfg *assetshandler.Config,
	core *Core,
	state *State,
	outcome *Outcome,
) int {
	now := (int)(time.Now().UnixMilli())
	if now-handler.UpdateTime < cfg.Http.StepData.MinChangeTime {
		return core.Step
	}
	handler.UpdateTime = now

	if outcome.ConsecutiveErrs > cfg.Http.StepData.MaxConsecutiveErrors {
		return -2
	}

	var maxStep int
	var step int = core.Step
	var errors int = outcome.NotFounds + outcome.NotFounds

	step = max(step, 1)
	if errors-outcome.Successes > cfg.Http.StepData.MaxErrorDeviation {
		if state.DelayNewest < cfg.Http.StepData.RetryTime &&
			handler.Retries < cfg.Http.StepData.MaxRetries {
			step = 0
			handler.Retries++
		} else {
			handler.Retries = 0
		}
	} else {
		switch {
		case state.DelayNewest > cfg.Http.StepData.MaxTime:
			maxStep = 20
			step *= 2
		case state.DelayNewest > cfg.Http.StepData.AggressiveTime:
			maxStep = 10
			step += 3
		case state.DelayNewest > cfg.Http.StepData.MediumTime:
			if core.Concurrency == cfg.Core.MaxConcurrency {
				maxStep = 3
				step++
			} else {
				maxStep = 1
			}
		default:
			if state.DelayNewest > handler.LastDelay+cfg.Http.StepData.LastDelayOffset {
				step--
			} else if state.DelayNewest < handler.LastDelay-cfg.Http.StepData.LastDelayOffset {
				step++
			}

			if state.DelayNewest > cfg.Http.StepData.MinTime &&
				core.Concurrency == cfg.Core.MaxConcurrency {
				maxStep = 2
			} else {
				maxStep = 1
			}

			step = max(step, 1)
		}

		step = min(step, maxStep)

		handler.LastDelay = state.DelayNewest
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
	if now-handler.UpdateTime < cfg.Http.ConcurrencyData.MinChangeTime {
		return core.Concurrency
	}
	handler.UpdateTime = now

	var concurrency int = core.Concurrency
	var maxConcurrency int = cfg.Core.MaxConcurrency
	var minConcurrency int = cfg.Http.ConcurrencyData.MinConcurrency
	var errors int = outcome.NotFounds + outcome.NotFounds

	if errors-outcome.Successes > cfg.Http.ConcurrencyData.MaxErrorDeviation {
		switch {
		case state.DelayNewest > cfg.Http.ConcurrencyData.MediumTime:
			concurrency = max(concurrency-1, minConcurrency)
		default:
			concurrency = max(concurrency-3, minConcurrency)
		}
	} else {
		switch {
		case state.DelayNewest > cfg.Http.ConcurrencyData.MaxTime:
			concurrency = min(concurrency+3, maxConcurrency)
		case state.DelayNewest > cfg.Http.ConcurrencyData.MinTime:
			concurrency = min(concurrency+1, maxConcurrency)
		default:
			concurrency = max(concurrency-1, minConcurrency)
		}
	}

	return concurrency
}
