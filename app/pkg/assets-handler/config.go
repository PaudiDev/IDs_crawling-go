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
	MaxRateLimitsPerSecond int             `yaml:"max_rate_limits_per_second"`
	RateLimitWait          int             `yaml:"rate_limit_wait_seconds"`
	StepData               stepData        `yaml:"step_data"`
	ConcurrencyData        concurrencyData `yaml:"concurrency_data"`
}

type standard struct {
	Urls              urls   `yaml:"urls"`
	SessionCookieName string `yaml:"session_cookie_name"`
	TimestampFormat   string `yaml:"timestamp_format"`
	InitialDelay      int    `yaml:"initial_delay"`
}

type stepData struct {
	MinChangeTime        int `yaml:"min_time_since_last_adjustment_milli"`
	MaxErrorDeviation    int `yaml:"max_error_deviation"`
	MaxConsecutiveErrors int `yaml:"max_consecutive_errors"`
	MaxRetries           int `yaml:"max_retries"`
	MaxTime              int `yaml:"max_time"`
	AggressiveTime       int `yaml:"aggressive_time"`
	MediumTime           int `yaml:"medium_time"`
	MinTime              int `yaml:"min_time"`
	RetryTime            int `yaml:"retry_time"`
	LastDelayOffset      int `yaml:"last_delay_offset"`
}

type concurrencyData struct {
	MinChangeTime     int `yaml:"min_time_since_last_adjustment_milli"`
	MaxErrorDeviation int `yaml:"max_error_deviation"`
	MinConcurrency    int `yaml:"min_concurrency"`
	MaxTime           int `yaml:"max_time"`
	MediumTime        int `yaml:"medium_time"`
	MinTime           int `yaml:"min_time"`
}

type urls struct {
	BaseUrl                string `yaml:"base_url"`
	ItemsUrl               string `yaml:"items_url"`
	ItemUrl                string `yaml:"item_url"`
	ItemUrlAfterID         string `yaml:"item_url_after_id"`
	RandomizeItemUrlSuffix bool   `yaml:"randomize_item_url_addition"`
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
