package crawler

import (
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
