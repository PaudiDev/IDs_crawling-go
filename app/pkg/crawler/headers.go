package crawler

import (
	"net/http"

	"crawler/app/pkg/assert"
)

func createBaseHeaders(userAgent string) map[string]string {
	headers := map[string]string{
		"User-Agent": userAgent,
	}

	return headers
}

func createScrapingHeaders(userAgent string) map[string]string {
	headers := map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":           "fr-FR,fr;q=0.9,en-US;q=0.8,en;q=0.7",
		"Sec-Ch-Ua":                 `"Google Chrome";v="123", "Not:A-Brand";v="8", "Chromium";v="123"`,
		"Sec-Ch-Ua-Mobile":          "?0",
		"Sec-Ch-Ua-Platform":        `"Windows"`,
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
	}

	for k, v := range createBaseHeaders(userAgent) {
		headers[k] = v
	}

	return headers
}

func setHeaders(req *http.Request, headers map[string]string) {
	assert.NotNil(req, "nil pointer to request must never happen")
	assert.Assert(len(headers) > 0, "empty headers map")

	for k, v := range headers {
		req.Header.Set(k, v)
	}
}
