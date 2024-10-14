package main

import (
	"context"
	"log/slog"
	"os"

	"crawler/app/pkg/assert"
	assetsHandler "crawler/app/pkg/assets-handler"
	"crawler/app/pkg/crawler"
	"crawler/app/pkg/shutdown"
	"crawler/app/pkg/utils/pathx"
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

	config := assetsHandler.GetConfigFromFile(pathx.FromCwd(os.Getenv("CONFIG_FILE")))
	proxies := assetsHandler.GetProxiesFromFile(pathx.FromCwd(os.Getenv("PROXIES_FILE")))

	crawler.Start(ctx, &config, proxies)
}
