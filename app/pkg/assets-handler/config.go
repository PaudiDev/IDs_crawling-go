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
	Timeout                int `yaml:"requests_timeout_seconds"`
	CookiesRefreshDelay    int `yaml:"cookies_refresh_delay"`
	MaxRateLimitsPerSecond int `yaml:"max_rate_limits_per_second"`
	RateLimitWait          int `yaml:"rate_limit_wait_seconds"`
}

type standard struct {
	BaseUrl           string `yaml:"base_url"`
	SessionCookieName string `yaml:"session_cookie_name"`
	TimestampFormat   string `yaml:"timestamp_format"`
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
