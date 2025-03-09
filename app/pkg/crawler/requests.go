package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
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
	jar *http.CookieJar,
	targetCookieNames []string,
	headers map[string]string,
	randGen *rand.Rand,
) error {
	parsedUrl, err := url.Parse(cfg.Standard.Urls.BaseUrl)
	if err != nil {
		return fmt.Errorf("could not parse the base url: %w", err)
	}

	var cookies []*http.Cookie = (*jar).Cookies(parsedUrl)

	// using tmp cookies to avoid modifying the original jar,
	// ensuring other goroutines that use the jar to keep their cookies
	// until this function is completed
	tmpCookies := make([]*http.Cookie, len(cookies))
	idx := 0
	var isTarget bool

	// remove target cookies from the tmp cookies
	for _, cookie := range cookies {
		isTarget = false
		for _, targetCookieName := range targetCookieNames {
			if cookie.Name == targetCookieName {
				isTarget = true
				break
			}
		}
		if !isTarget {
			tmpCookies[idx] = cookie
			idx++
		}
	}
	// at this point tmpCookies[idx:] contains nil values, so we must remove them
	tmpCookies = tmpCookies[:idx]

	tmpJar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("could not create a new cookie jar: %w", err)
	}
	tmpJar.SetCookies(parsedUrl, tmpCookies)

	req, err := httpx.BuildRequest(ctx, "GET", cfg.Standard.Urls.BaseUrl, nil, headers)
	if err != nil {
		return err
	}

	response, err := httpx.MakeRequestWithProxy(req, tmpJar, cfg.Http.Timeout, randGen)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return customerrors.InferHttpError(response.StatusCode)
	}

	newCookies := tmpJar.Cookies(parsedUrl)
	if newCookies == nil {
		return fmt.Errorf("no cookies found in response")
	}

	targetCookiesAmount := len(targetCookieNames)
	for _, newCookie := range newCookies {
		for _, targetCookieName := range targetCookieNames {
			if newCookie.Name == targetCookieName {
				targetCookiesAmount--
				break
			}
		}

		if targetCookiesAmount == 0 {
			*jar = tmpJar
			return nil
		}
	}

	if targetCookiesAmount > 0 {
		var cookieSingPlur string
		if targetCookiesAmount == 1 {
			cookieSingPlur = "cookie"
		} else {
			cookieSingPlur = "cookies"
		}
		return fmt.Errorf("%v target %s not found in response\nFound Cookies: %+v", targetCookiesAmount, cookieSingPlur, newCookies)
	}

	return nil
}

func fetchCookieLoop(
	ctx context.Context,
	cfg *assetshandler.Config,
	jar *http.CookieJar,
	targetCookieNames []string,
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
			err := fetchCookie(ctx, cfg, jar, targetCookieNames, headers, randGen)
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
) (map[string]interface{}, bool, error) {
	var appendedSuffix bool
	url := cfg.Standard.Urls.ItemUrl + strconv.Itoa(itemID)

	// This randomization is not the fastest but it is the simplest
	// Caching a rand.Source.Int63n value and shifting it by 1 until it is 0 would be faster
	// If each url has different rate limits, the best would be to switch
	// on each request based on the proxy, but this would increase the coupling
	if !cfg.Standard.Urls.RandomizeItemUrlSuffix || randGen.Intn(2) == 1 {
		url += cfg.Standard.Urls.ItemUrlAfterID
		appendedSuffix = true
	} else {
		appendedSuffix = false
	}

	req, err := httpx.BuildRequest(ctx, "GET", url, nil, headers)
	if err != nil {
		return nil, appendedSuffix, err
	}

	response, err := httpx.MakeRequestWithProxy(req, jar, cfg.Http.Timeout, randGen)
	if err != nil {
		return nil, appendedSuffix, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, appendedSuffix, customerrors.InferHttpError(response.StatusCode)
	}

	var decodedResp map[string]interface{}
	err = json.NewDecoder(response.Body).Decode(&decodedResp)
	if err != nil {
		return nil, appendedSuffix, err
	}

	return decodedResp, appendedSuffix, nil
}
