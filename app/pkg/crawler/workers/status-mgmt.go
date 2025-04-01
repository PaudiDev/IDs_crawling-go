package workers

import (
	"fmt"
	"os"
	"time"

	wtypes "crawler/app/pkg/crawler/workers/workers-types"
	"crawler/app/pkg/utils/slicex"
)

func LogAndResetVarsLoop(
	state *wtypes.State,
	outcome *wtypes.Outcome,
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

		state.Mu.Lock()
		var avgThreshAmount, avgThreshOffset, avgHitThreshLevel, avgDelay float32
		if count := len(state.ThresholdsAmounts); count > 0 {
			avgThreshAmount = (float32)(slicex.Sum(state.ThresholdsAmounts)) / (float32)(count)
		}
		if count := len(state.ThresholdsOffsets); count > 0 {
			avgThreshOffset = (float32)(slicex.Sum(state.ThresholdsOffsets)) / (float32)(count)
		}
		if count := len(state.HitThresholdLevels); count > 0 {
			avgHitThreshLevel = (float32)(slicex.Sum(state.HitThresholdLevels)) / (float32)(count)
		}
		if count := len(state.Delays); count > 0 {
			avgDelay = (float32)(slicex.Sum(state.Delays)) / (float32)(count)
		}
		state.Mu.Unlock()

		logFile.WriteString(
			func() string {
				state.Mu.Lock()
				outcome.Mu.Lock()
				defer outcome.Mu.Unlock()
				defer state.Mu.Unlock()
				return fmt.Sprintf(
					time.Now().Format("2006-01-02 15:04:05.0")+" STATUS\n"+
						"Reqs: %d, Success: %.2f%%\n"+
						"RateLimits (429): %d, NotFounds (404): %d, "+
						"OtherErrs: %d\n"+
						"Recovered from backup: %d, Lost from backup: %d\n"+
						"BatchID: %d, HighestID: %d\n"+
						"AvgThreshAmount: %.2f, AvgThreshOffset: %.2f\n"+
						"AvgHitThreshLevel: %.2f, AvgDelay: %.2f"+
						"\n\n",
					totalRequests, successRate,
					outcome.RateLimits, outcome.NotFounds, outcome.OtherErrs,
					outcome.Recovered, outcome.Lost,
					state.BatchID, state.HighestID,
					avgThreshAmount, avgThreshOffset,
					avgHitThreshLevel, avgDelay,
				)
			}(),
		)

		outcome.Mu.Lock()
		outcome.RateLimits = 0
		outcome.NotFounds = 0
		outcome.OtherErrs = 0
		outcome.Successes = 0
		outcome.Recovered = 0
		outcome.Lost = 0
		outcome.Mu.Unlock()

		state.Mu.Lock()
		state.ThresholdsAmounts = state.ThresholdsAmounts[:0]
		state.ThresholdsOffsets = state.ThresholdsOffsets[:0]
		state.HitThresholdLevels = state.HitThresholdLevels[:0]
		state.Delays = state.Delays[:0]
		state.Mu.Unlock()
	}
}
