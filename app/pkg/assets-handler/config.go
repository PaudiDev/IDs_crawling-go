package assetshandler

import (
	"os"

	"crawler/app/pkg/assert"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Core     core                     `yaml:"core"`
	Http     http                     `yaml:"http"`
	Standard standard                 `yaml:"standard"`
	Policies []ThresholdsAdjPolicyCfg `yaml:"thresholds_adjustment_policies"`
}

type core struct {
	ThresholdsInitialAmount uint8       `yaml:"thresholds_initial_amount(max_255)"`
	ExpMaxThresholdsAmount  uint8       `yaml:"expected_max_thresholds_amount(max_255)"`
	ThresholdsOffset        uint8       `yaml:"thresholds_offset(max_255)"`
	BatchLimits             BatchLimits `yaml:"batch_limits"`
}

type http struct {
	Timeout                      int    `yaml:"requests_timeout_seconds"`
	CookiesSessionsAmount        uint16 `yaml:"cookies_sessions_amount"`
	CookiesRefreshDelay          int    `yaml:"cookies_refresh_delay"`
	CrashOnFirstCookieFetchError bool   `yaml:"crash_on_first_cookie_fetch_error"`
	MaxRetriesPerItem            uint8  `yaml:"max_retries_per_item"`
	DelayBetweenRetries          uint64 `yaml:"delay_between_retries_milli"`
	MaxRateLimitsPerSecond       int    `yaml:"max_rate_limits_per_second"`
	RateLimitWait                int    `yaml:"rate_limit_wait_seconds"`
}

type standard struct {
	Urls               urls          `yaml:"urls"`
	ItemsResponse      itemsResponse `yaml:"items_response"`
	ItemResponse       itemResponse  `yaml:"item_response"`
	WebSocket          websocket     `yaml:"websocket"`
	SessionCookieNames []string      `yaml:"session_cookie_names"`
	TimestampFormat    string        `yaml:"timestamp_format"`
	InitialDelay       int           `yaml:"initial_delay"`
}

type ThresholdsAdjPolicyCfg struct {
	Percentage           float32 `yaml:"percentage"`
	ComputeIncrementExpr string  `yaml:"compute_increment"`
}

type BatchLimits struct {
	EnableBatchLimits bool   `yaml:"enable_batch_limits"`
	MaxBatchSize      uint16 `yaml:"max_batch_size"`
}

type urls struct {
	BaseUrl                string `yaml:"base_url"`
	ItemsUrl               string `yaml:"items_url"`
	ItemUrl                string `yaml:"item_url"`
	ItemUrlAfterID         string `yaml:"item_url_after_id"`
	RandomizeItemUrlSuffix bool   `yaml:"randomize_item_url_addition"`
}

type itemsResponse struct {
	Items string `yaml:"items"`
	ID    string `yaml:"id"`
}

type itemResponse struct {
	Item            string `yaml:"item"`
	Timestamp       string `yaml:"timestamp"`
	ItemSuffix      string `yaml:"item_when_url_suffix"`
	TimestampSuffix string `yaml:"timestamp_when_url_suffix"`
}

type websocket struct {
	WsUrls    []string               `yaml:"ws_urls"`
	WsHeaders map[string]interface{} `yaml:"ws_headers"`
}

func GetConfigFromFile(path string) Config {
	assert.Assert(path != "", "config file path cannot be empty", assert.AssertData{"path": path})

	configBytes, err := os.ReadFile(path)
	assert.NoError(err, "error reading config file", assert.AssertData{"path": path})

	var config Config

	assert.NoError(
		yaml.Unmarshal(configBytes, &config), "error unmarshalling config file",
		assert.AssertData{"path": path},
	)

	return config
}
