package crawler

import (
	"fmt"
	"math/rand"
	"slices"

	wtypes "crawler/app/pkg/crawler/workers/workers-types"
	"crawler/app/pkg/thresholds"
)

type workersManager struct {
	// A thresholds controller used to manage the IDs thresholds.
	thresholdsController *thresholds.ThresholdsController

	// The offset to keep between each ID threshold.
	offset int

	rand *rand.Rand
}

func (wkM *workersManager) run(
	thresholdsWkIDsChan chan<- int,
	thresholdsWkResultsChan <-chan *wtypes.ThresholdsWorkerResult,
	subordinateWkChan chan<- int,
	successfulItemsChan chan<- *wtypes.WsContentElement,
	initialID int,
) {
	var result *wtypes.ThresholdsWorkerResult
	var highestThresholdID int = initialID

	for {
		lastSuccID := highestThresholdID
		thresholdsAmount := wkM.thresholdsController.GetThresholdsAmount()
		results := make(map[int]*wtypes.ThresholdsWorkerResult, thresholdsAmount)
		wkM.offset += wkM.rand.Intn(3) - 1
		// TODO: remove this newsource from here, no sense

		for i := uint16(0); i < thresholdsAmount; i++ {
			highestThresholdID += wkM.offset
			thresholdsWkIDsChan <- highestThresholdID
		}

	getResults: // use a label to allow breaking the loop without an additional flag
		for thresholdsAmount > 0 {
			result = <-thresholdsWkResultsChan
			results[result.ItemID] = result

			if result.Success {
				successfulItemsChan <- &wtypes.WsContentElement{
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
				highestThresholdID -= wkM.offset
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
					subordinateWkChan <- id
				}
				lastSuccID = interruptID
			}
		}
		fmt.Println("current wsChan size:", len(successfulItemsChan))
		fmt.Println("current subWKChan size:", len(subordinateWkChan))
		fmt.Println("hit threshold level:", thresholdsAmount)
		fmt.Println("thresholds amount:", wkM.thresholdsController.GetThresholdsAmount())

		wkM.thresholdsController.Update(
			&thresholds.ThresholdsControllerInput{
				ThresholdLevel: thresholdsAmount,
				Timestamp:      timestamp,
			},
		)
	}
}
