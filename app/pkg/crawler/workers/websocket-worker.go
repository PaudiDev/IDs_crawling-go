package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"crawler/app/pkg/assert"
	wtypes "crawler/app/pkg/crawler/workers/workers-types"
	ctypes "crawler/app/pkg/custom-types"
	safews "crawler/app/pkg/safe-ws"

	"github.com/gorilla/websocket"
)

// WebsocketWorker is a worker that sends the contents received from ContentsChan
// to one websocket client out of the Conns parameter set.
// The websocket client is selected sequentially in a cyclic way from the conns set.
//
// This struct does not implement the Worker interface.
// However, the directory is the same for structural organization purposes.
type WebsocketWorker struct {
	ID  int
	Ctx context.Context

	// ContentsChan is used to receive the contents to send to the websocket clients.
	// The received type is a WsContentElement object that contains
	// the result JSON object (represented with a map[string]interface{})
	// along with the content ID (for logging).
	ContentsChan <-chan *wtypes.ContentElement

	// Thread safe connections to the websocket clients.
	Conns []*safews.SafeConn

	Fatal error
}

func (wsWk *WebsocketWorker) Run() {
	logChan := make(chan ctypes.LogData, 1000)
	defer close(logChan)
	go wsWk.log(logChan)

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				wsWk.Fatal = err
			} else {
				wsWk.Fatal = fmt.Errorf("recover panic: %v", r)
			}
			panic(r)
		} else {
			assert.NotNil(
				wsWk.Fatal,
				"at this point worker must have a done ctx error. an unexpected error occurred",
				assert.AssertData{"WorkerID": wsWk.ID},
			)
			logChan <- ctypes.LogData{
				Level: slog.LevelError,
				Msg:   "Worker finished due to context done",
			}
		}
	}()

	var currentConnIdx int = 0
	var connsAmount int = len(wsWk.Conns)

	for {
		contentEl := <-wsWk.ContentsChan

		go func() {
			jsonResponse, err := json.Marshal(contentEl.Content)
			if err != nil {
				logChan <- ctypes.LogData{
					Level: slog.LevelError,
					Msg: fmt.Sprintf(
						"error marshalling item response to json, "+
							"impossible sending to websocket (ID %d): %s",
						contentEl.ContentID, err.Error(),
					),
				}
				return
			}

			err = wsWk.Conns[currentConnIdx].WriteMessage(websocket.TextMessage, jsonResponse)
			if err != nil {
				logChan <- ctypes.LogData{
					Level: slog.LevelError,
					Msg: fmt.Sprintf(
						"error sending item to websocket (ID %d): %s",
						contentEl.ContentID, err.Error(),
					),
				}
			}
		}()

		currentConnIdx = (currentConnIdx + 1) % connsAmount
	}
}

func (wsWk *WebsocketWorker) log(logChan <-chan ctypes.LogData) {
	for {
		select {
		case <-wsWk.Ctx.Done():
			return
		case data, ok := <-logChan:
			if !ok {
				return
			}
			slog.Log(wsWk.Ctx, data.Level, wsWk.logFormat(data.Msg))
		}
	}
}

func (wsWk *WebsocketWorker) logFormat(text string) string {
	return fmt.Sprintf("(WebsocketWorker WS%d): %s", wsWk.ID, text)
}
