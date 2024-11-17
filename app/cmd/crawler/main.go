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
	httpAssets := assetsHandler.HttpAssets{
		UserAgents: assetsHandler.GetUAsFromFile(pathx.FromCwd(os.Getenv("USER_AGENTS_FILE"))),
	}

	assert.NoError(
		crawler.LoadProxies(proxies),
		"no proxies found in file",
	)
	assert.NoError(
		crawler.LoadUserAgents(httpAssets.UserAgents),
		"no user agents found in file",
	)

	crawler.Start(ctx, &config)
}
