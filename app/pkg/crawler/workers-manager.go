package crawler

import (
	"math/rand"
	"slices"

	wtypes "crawler/app/pkg/crawler/workers/workers-types"
	"crawler/app/pkg/thresholds"
)

type workersManager struct {
	// A thresholds controller used to manage the IDs thresholds.
	thresholdsController *thresholds.ThresholdsController

	// The offset to keep between each ID threshold.
	offset uint16

	rand *rand.Rand
}

func (wkM *workersManager) run(
	thresholdsWkIDsChan chan<- *wtypes.ItemFromBatchPacket,
	thresholdsWkResultsChan <-chan *wtypes.ThresholdsWorkerResult,
	subordinateWkChan chan<- *wtypes.ItemFromBatchPacket,
	successfulItemsChan chan<- *wtypes.ContentElement,
	state *wtypes.State,
) {
	var result *wtypes.ThresholdsWorkerResult
	var highestThresholdID int = state.HighestID
	var initialOffset uint16 = wkM.offset

	var batchID uint16 = 1

	for {
		lastSuccID := highestThresholdID
		thresholdsAmount := wkM.thresholdsController.GetThresholdsAmount()
		results := make(map[int]*wtypes.ThresholdsWorkerResult, thresholdsAmount)
		wkM.offset += uint16(wkM.rand.Intn(3) - 1) // -1, 0, or 1

		// update state for logging
		state.Mu.Lock()
		state.ThresholdsAmounts = append(state.ThresholdsAmounts, thresholdsAmount)
		state.ThresholdsOffsets = append(state.ThresholdsOffsets, wkM.offset)
		state.Mu.Unlock()

		// avoid too big or too small / negative offsets
		if wkM.offset >= 2*initialOffset || wkM.offset <= uint16(0.5*float32(initialOffset)) {
			wkM.offset = initialOffset
		}

		for i := uint16(0); i < thresholdsAmount; i++ {
			highestThresholdID += int(wkM.offset)
			thresholdsWkIDsChan <- &wtypes.ItemFromBatchPacket{
				ItemID:  highestThresholdID,
				BatchID: batchID,
			}
		}

	getResults: // use a label to allow breaking the loop without an additional flag
		for thresholdsAmount > 0 {
			result = <-thresholdsWkResultsChan

			// discard previous batch results that keep coming
			if result.ItemID < lastSuccID {
				continue
			}

			results[result.ItemID] = result

			if result.Success {
				successfulItemsChan <- &wtypes.ContentElement{
					Content:   result.Item,
					ContentID: result.ItemID,
				}
			}

			var ok bool
			for thresholdsAmount > 0 {
				if result, ok = results[highestThresholdID]; !ok {
					break // highest threshold ID item still not fetched
				}
				if result.Success {
					break getResults
				}

				// decrease the highest threshold level by 1
				thresholdsAmount--
				highestThresholdID -= int(wkM.offset)
			}
		}

		var timestamp uint32
		if thresholdsAmount <= 0 {
			timestamp = 0
		} else {
			timestamp = result.Timestamp

			// let thresholdsIDs be the set of IDs successfully fetched by the thresholds workers.
			//
			// send to the subordinate workers all IDs within the range
			// [lastSuccID+1, highestThresholdID] \ thresholdsIDs to fill the IDs gaps.
			succThresholdIDs := make([]int, 0, thresholdsAmount)
			for k, v := range results {
				if v.Success {
					succThresholdIDs = append(succThresholdIDs, k)
				}
			}
			slices.Sort(succThresholdIDs)

			succThresholdIDsLen := len(succThresholdIDs)
			for count := 0; count < succThresholdIDsLen; count++ {
				interruptID := succThresholdIDs[count]
				for id := lastSuccID + 1; id < interruptID; id++ {
					subordinateWkChan <- &wtypes.ItemFromBatchPacket{
						ItemID:  id,
						BatchID: batchID,
					}
				}
				lastSuccID = interruptID
			}
		}

		// update state for logging
		state.Mu.Lock()
		state.BatchID = batchID
		state.HighestID = highestThresholdID
		state.HitThresholdLevels = append(state.HitThresholdLevels, thresholdsAmount)
		state.Mu.Unlock()
		batchID++

		wkM.thresholdsController.Update(
			&thresholds.ThresholdsControllerInput{
				ThresholdLevel: thresholdsAmount,
				Timestamp:      timestamp,
			},
		)
	}
}
