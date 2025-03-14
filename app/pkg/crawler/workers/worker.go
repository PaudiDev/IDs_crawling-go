package workers

import (
	assetshandler "crawler/app/pkg/assets-handler"
	wtypes "crawler/app/pkg/crawler/workers/workers-types"
	ctypes "crawler/app/pkg/custom-types"
	safews "crawler/app/pkg/safe-ws"
)

type Worker interface {
	Run(cfg *assetshandler.Config,
		core *wtypes.Core,
		state *wtypes.State,
		outcome *wtypes.Outcome,
		handlers *wtypes.Handlers,
		conns []*safews.SafeConn)

	log(logChan <-chan ctypes.LogData)

	logFormat(text string) string
}
