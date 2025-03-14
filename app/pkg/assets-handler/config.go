package assetshandler

import (
	"os"

	"crawler/app/pkg/assert"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Core     core     `yaml:"core"`
	Http     http     `yaml:"http"`
	Standard standard `yaml:"standard"`
}

type core struct {
	MaxConcurrency     int `yaml:"max_concurrency"`
	InitialConcurrency int `yaml:"initial_concurrency"`
	InitialStep        int `yaml:"initial_step"`
}

type http struct {
	Timeout                int             `yaml:"requests_timeout_seconds"`
	CookiesRefreshDelay    int             `yaml:"cookies_refresh_delay"`
	MaxRetriesPerItem      int16           `yaml:"max_retries_per_item"`
	DelayBetweenRetries    uint64          `yaml:"delay_between_retries_milli"`
	MaxRateLimitsPerSecond int             `yaml:"max_rate_limits_per_second"`
	RateLimitWait          int             `yaml:"rate_limit_wait_seconds"`
	StepData               stepData        `yaml:"step_data"`
	ConcurrencyData        concurrencyData `yaml:"concurrency_data"`
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

type stepData struct {
	NewAdjustmentCfg     newAdjustmentCfg `yaml:"new_adjustment_config"`
	MaxErrorDeviation    int              `yaml:"max_error_deviation"`
	MaxConsecutiveErrors int              `yaml:"max_consecutive_errors"`
	MaxRetries           int              `yaml:"max_retries"`
	MaxTime              int              `yaml:"max_time"`
	AggressiveTime       int              `yaml:"aggressive_time"`
	MediumAggressiveTime int              `yaml:"medium_aggressive_time"`
	MediumTime           int              `yaml:"medium_time"`
	MinTime              int              `yaml:"min_time"`
	RetryTime            int              `yaml:"retry_time"`
	LastDelayOffset      int              `yaml:"last_delay_offset"`
}

type concurrencyData struct {
	NewAdjustmentCfg     newAdjustmentCfg `yaml:"new_adjustment_config"`
	MaxErrorDeviation    int              `yaml:"max_error_deviation"`
	MaxConsecutiveErrors int              `yaml:"max_consecutive_errors"`
	MinConcurrency       int              `yaml:"min_concurrency"`
	MaxTime              int              `yaml:"max_time"`
	MediumTime           int              `yaml:"medium_time"`
	MinTime              int              `yaml:"min_time"`
}

type newAdjustmentCfg struct {
	MinChangeTimeHighDelay int `yaml:"min_time_since_last_adjustment_high_delay_milli"`
	MinChangeTimeLowDelay  int `yaml:"min_time_since_last_adjustment_low_delay_milli"`
	HighDelayThreshold     int `yaml:"high_delay_threshold"`
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
