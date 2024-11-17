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
	crawltypes "crawler/app/pkg/crawler/crawl-types"
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

	var itemsContainer struct {
		Items []crawltypes.Item
	}
	err = json.NewDecoder(response.Body).Decode(&itemsContainer)
	if err != nil {
		return 0, err
	}

	var items []crawltypes.Item = itemsContainer.Items
	var highestID int = items[0].ID
	for _, item := range items[1:] {
		if item.ID > highestID {
			highestID = item.ID
		}
	}

	return highestID, nil
}

func fetchItem(
	ctx context.Context,
	cfg *assetshandler.Config,
	jar http.CookieJar,
	itemID int,
	headers map[string]string,
	randGen *rand.Rand,
) (crawltypes.Item, error) {
	url := cfg.Standard.Urls.ItemUrl + strconv.Itoa(itemID)

	req, err := httpx.BuildRequest(ctx, "GET", url, nil, headers)
	if err != nil {
		return crawltypes.Item{}, err
	}

	response, err := httpx.MakeRequestWithProxy(req, jar, cfg.Http.Timeout, randGen)
	if err != nil {
		return crawltypes.Item{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return crawltypes.Item{}, customerrors.InferHttpError(response.StatusCode)
	}

	var itemContainer struct {
		Item crawltypes.Item
	}
	err = json.NewDecoder(response.Body).Decode(&itemContainer)
	if err != nil {
		return crawltypes.Item{}, err
	}

	return itemContainer.Item, nil
}
