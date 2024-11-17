package customtypes

import "log/slog"

type LogData struct {
	Level slog.Level
	Msg   string
}
