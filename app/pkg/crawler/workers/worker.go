package workers

import (
	assetshandler "crawler/app/pkg/assets-handler"
	wtypes "crawler/app/pkg/crawler/workers/workers-types"
	ctypes "crawler/app/pkg/custom-types"
)

// cookies refresh and websocket workers don't implement this interface but it makes
// sense to have them here since they are workers but not of the same type of the main ones
// (thresholds, subordinates and backup)
type Worker interface {
	Run(cfg *assetshandler.Config,
		core *wtypes.Core,
		state *wtypes.State,
		outcome *wtypes.Outcome,
		handlers *wtypes.Handlers,
	)

	log(logChan <-chan ctypes.LogData)

	logFormat(text string) string
}
