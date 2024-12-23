package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"crawler/app/pkg/assert"
	assetsHandler "crawler/app/pkg/assets-handler"
	"crawler/app/pkg/crawler"
	safews "crawler/app/pkg/safe-ws"
	"crawler/app/pkg/shutdown"
	"crawler/app/pkg/utils/httpx"
	"crawler/app/pkg/utils/mapx"
	"crawler/app/pkg/utils/pathx"

	"github.com/gorilla/websocket"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go shutdown.HandleSIGTERM(cancel)
	assert.LoadCtxCancel(cancel)

	slogHandler := slog.NewTextHandler(
		os.Stdout,
		&slog.HandlerOptions{Level: slog.LevelDebug},
	)
	slog.SetDefault(slog.New(slogHandler))

	statusLogFile, err := os.OpenFile(
		pathx.FromCwd(os.Getenv("STATUS_LOG_FILE")),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_TRUNC, 0o666,
	)
	assert.NoError(err, "status log file must be created to start the crawler")
	defer statusLogFile.Close()

	config := assetsHandler.GetConfigFromFile(pathx.FromCwd(os.Getenv("CONFIG_FILE")))
	proxies := assetsHandler.GetProxiesFromFile(pathx.FromCwd(os.Getenv("PROXIES_FILE")))
	httpAssets := assetsHandler.HttpAssets{
		UserAgents: assetsHandler.GetUAsFromFile(pathx.FromCwd(os.Getenv("USER_AGENTS_FILE"))),
	}

	assert.NoError(
		httpx.LoadProxies(proxies),
		"no proxies found in file",
	)
	assert.NoError(
		httpx.LoadUserAgents(httpAssets.UserAgents),
		"no user agents found in file",
	)

	dialer := websocket.Dialer{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	safeConns := make([]*safews.SafeConn, len(config.Standard.WebSocket.WsUrls))
	validFormatHeaders := mapx.StringToStringsList(config.Standard.WebSocket.WsHeaders)
	for idx, wsUrl := range config.Standard.WebSocket.WsUrls {
		conn, _, err := dialer.Dial(
			wsUrl,
			validFormatHeaders,
		)
		assert.NoError(err, "error connecting to websocket")
		defer conn.Close()
		conn.SetReadDeadline(time.Time{})
		slog.Info(fmt.Sprintf("connected to websocket with url: %s", wsUrl))
		safeConns[idx] = safews.NewSafeConn(conn)
	}

	crawler.Start(ctx, &config, safeConns, statusLogFile)
}
