package crawler

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

var (
	proxiesPool    []*url.URL
	userAgentsPool []string
)

func LoadProxies(proxies []*url.URL) error {
	if len(proxies) == 0 {
		return errors.New("tried to load pool with an empty proxies slice")
	}

	proxiesPool = proxies
	return nil
}

func LoadUserAgents(userAgents []string) error {
	if len(userAgents) == 0 {
		return errors.New("tried to load pool with an empty user agents slice")
	}

	userAgentsPool = userAgents
	return nil
}

// XXX: The two pickRandom functions are not assert checked (len(proxiesPool) > 0)
// to increase performances. This is unsafe and might be changed in future
func pickRandomProxy(randGen *rand.Rand) *url.URL {
	return proxiesPool[randGen.Intn(len(proxiesPool))]
}

func pickRandomUserAgent(randGen *rand.Rand) string {
	return userAgentsPool[randGen.Intn(len(userAgentsPool))]
}

func buildRequest(
	ctx context.Context,
	method string,
	url string,
	body io.Reader,
	headers map[string]string,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	setHeaders(req, headers)

	return req, nil
}

func makeRequestWithProxy(
	req *http.Request,
	cookieJar http.CookieJar,
	timeout int,
	randGen *rand.Rand,
) (*http.Response, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(pickRandomProxy(randGen)),
		},
		Jar:     cookieJar,
		Timeout: (time.Duration)(timeout) * time.Second,
	}

	return client.Do(req)
}
