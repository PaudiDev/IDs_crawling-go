package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	assetshandler "crawler/app/pkg/assets-handler"
	ctypes "crawler/app/pkg/custom-types"
	customerrors "crawler/app/pkg/custom-types/custom-errors"
	"crawler/app/pkg/utils/httpx"
)

func fetchCookie(
	ctx context.Context,
	cfg *assetshandler.Config,
	jar http.CookieJar,
	targetCookieName string,
	headers map[string]string,
	randGen *rand.Rand,
) error {
	req, err := httpx.BuildRequest(ctx, "GET", cfg.Standard.Urls.BaseUrl, nil, headers)
	if err != nil {
		return err
	}

	response, err := httpx.MakeRequestWithProxy(req, jar, cfg.Http.Timeout, randGen)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return customerrors.InferHttpError(response.StatusCode)
	}

	// if this returns an error it is already handled by buildRequest
	parsedUrl, _ := url.Parse(cfg.Standard.Urls.BaseUrl)
	var cookies []*http.Cookie = jar.Cookies(parsedUrl)
	if cookies == nil {
		return fmt.Errorf("no cookies found in response")
	}

	for _, cookie := range cookies {
		if cookie.Name == targetCookieName {
			return nil
		}
	}

	return fmt.Errorf("target cookie not found in response\nCookies: %+v", cookies)
}

func fetchCookieLoop(
	ctx context.Context,
	cfg *assetshandler.Config,
	jar http.CookieJar,
	targetCookieName string,
	headers map[string]string,
	randGen *rand.Rand,
	logChan chan<- ctypes.LogData,
) {
	for {
		time.Sleep(time.Duration(cfg.Http.CookiesRefreshDelay) * time.Second)

		select {
		case <-ctx.Done():
			return
		default:
			err := fetchCookie(ctx, cfg, jar, targetCookieName, headers, randGen)
			if err != nil {
				logChan <- ctypes.LogData{
					Level: slog.LevelError,
					Msg:   fmt.Sprintf("error fetching cookie: %v", err),
				}
			}
		}
	}
}

func fetchHighestID(
	ctx context.Context,
	cfg *assetshandler.Config,
	jar http.CookieJar,
	headers map[string]string,
	randGen *rand.Rand,
) (int, error) {
	req, err := httpx.BuildRequest(ctx, "GET", cfg.Standard.Urls.ItemsUrl, nil, headers)
	if err != nil {
		return 0, err
	}

	response, err := httpx.MakeRequestWithProxy(req, jar, cfg.Http.Timeout, randGen)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return 0, customerrors.InferHttpError(response.StatusCode)
	}

	var decodedResp map[string]interface{}
	err = json.NewDecoder(response.Body).Decode(&decodedResp)
	if err != nil {
		return 0, err
	}

	items := decodedResp[cfg.Standard.ItemsResponse.Items].([]interface{})
	var highestID float64 = 0
	for _, item := range items {
		if itemID := item.(map[string]interface{})[cfg.Standard.ItemsResponse.ID].(float64); itemID > highestID {
			highestID = itemID
		}
	}

	return int(highestID), nil
}

func fetchItem(
	ctx context.Context,
	cfg *assetshandler.Config,
	jar http.CookieJar,
	itemID int,
	headers map[string]string,
	randGen *rand.Rand,
) (map[string]interface{}, error) {
	url := cfg.Standard.Urls.ItemUrl + strconv.Itoa(itemID)

	// This randomization is not the fastest but it is the simplest
	// Caching a rand.Source.Int63n value and shifting it by 1 until it is 0 would be faster
	// If each url has different rate limits, the best would be to switch
	// on each request based on the proxy, but this would increase the coupling
	if !cfg.Standard.Urls.RandomizeItemUrlSuffix || randGen.Intn(2) == 1 {
		url += cfg.Standard.Urls.ItemUrlAfterID
	}

	req, err := httpx.BuildRequest(ctx, "GET", url, nil, headers)
	if err != nil {
		return nil, err
	}

	response, err := httpx.MakeRequestWithProxy(req, jar, cfg.Http.Timeout, randGen)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, customerrors.InferHttpError(response.StatusCode)
	}

	var decodedResp map[string]interface{}
	err = json.NewDecoder(response.Body).Decode(&decodedResp)
	if err != nil {
		return nil, err
	}

	return decodedResp, nil
}
