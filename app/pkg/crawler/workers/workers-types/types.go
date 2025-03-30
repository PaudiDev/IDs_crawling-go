package workerstypes

import (
	"net/http"
	"sync"

	assetshandler "crawler/app/pkg/assets-handler"
)

// Must be initialized with the "NewCore" function
type Core struct {
	Concurrency   int
	Step          int
	Concurrencies []int
	Steps         []int
	Mu            sync.Mutex
}

// Must be initialized with the "NewState" function
type State struct {
	CurrentID    int
	MostRecentID int
	DelayNewest  int
	Delays       []int
	Mu           sync.Mutex
}

type Outcome struct {
	RateLimits      int
	NotFounds       int
	OtherErrs       int
	ConsecutiveErrs int
	Successes       int
	SentToBackup    int
	Recovered       int
	Lost            int
	Mu              sync.Mutex
}

func NewCore(cfg *assetshandler.Config) *Core {
	return &Core{
		Concurrency: cfg.Core.InitialConcurrency,
		Step:        cfg.Core.InitialStep,
	}
}

func NewState(cfg *assetshandler.Config) *State {
	return &State{
		CurrentID:    0,
		MostRecentID: 0,
		DelayNewest:  cfg.Standard.InitialDelay,
	}
}

type Handlers struct {
	SHandler StepHandler
	CHandler ConcurrencyHandler
}

type StepHandler struct {
	UpdateTime int
	LastDelay  int
	Retries    int
	Mu         sync.Mutex
}

type ConcurrencyHandler struct {
	UpdateTime int
	Mu         sync.Mutex
}

func NewHandlers() *Handlers {
	return &Handlers{
		SHandler: StepHandler{
			LastDelay: 30000,
		},
	}
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
