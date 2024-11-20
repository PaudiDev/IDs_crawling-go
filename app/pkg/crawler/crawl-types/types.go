package crawltypes

import "sync"

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
