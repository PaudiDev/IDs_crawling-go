package workerstypes

import (
	"net/http"
	"sync"
)

type State struct {
	BatchID            uint16
	HighestID          int
	ThresholdsAmounts  []uint16
	ThresholdsOffsets  []uint16
	HitThresholdLevels []uint16
	Delays             []uint32
	Mu                 sync.Mutex
}

type Outcome struct {
	RateLimits int
	NotFounds  int
	OtherErrs  int
	Successes  int
	Recovered  int
	Lost       int
	Mu         sync.Mutex
}

type ThresholdsWorkerResult struct {
	Item   map[string]interface{}
	ItemID int

	// a flag indicating whether the item was fetched successfully or not
	Success bool

	// the timestamp of the item
	Timestamp uint32
}

type BackupPacket struct {
	ItemID       int
	AppendSuffix bool
}

type CookieJarSession struct {
	CookieJar   http.CookieJar
	RefreshChan chan struct{}
}

type ContentElement struct {
	// the actual content to process.
	// it is a map[string]interface{} that represents the JSON object.
	Content map[string]interface{}

	ContentID int
}

// ItemFromBatchPacket is used to pass the ID of the item to fetch along with
// the ID of the batch of generated item IDs it belongs to.
// This is useful for debugging purposes.
type ItemFromBatchPacket struct {
	ItemID  int
	BatchID uint16
}
