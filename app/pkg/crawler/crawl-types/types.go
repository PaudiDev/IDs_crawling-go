package crawltypes

type Item struct {
	ID        int    `json:"id"`
	Timestamp string `json:"created_at_ts"`
}

type Handlers struct {
	SHandler StepHandler
	CHandler ConcurrencyHandler
}

type StepHandler struct {
	UpdateTime int
	LastDelay  int
	Retries    int
}

type ConcurrencyHandler struct {
	UpdateTime int
}

func NewHandlers() *Handlers {
	return &Handlers{
		SHandler: StepHandler{
			LastDelay: 30000,
		},
	}
}
