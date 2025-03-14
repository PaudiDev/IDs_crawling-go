package network

import (
	"errors"
	"math/rand"
	"net/url"
)

var (
	proxiesPool    []*url.URL
	userAgentsPool []string
	profilesPool   []*Profile
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

// This function should only be called after LoadUserAgents
func GenerateAndLoadProfiles() (profilesAmount int) {
	profilesPool = generateProfiles(userAgentsPool)
	return len(profilesPool)
}

// XXX: The three pickRandom functions are not assert checked (len(pool) > 0)
// to increase performances. This is unsafe and might be changed in future
func pickRandomProxy(randGen *rand.Rand) *url.URL {
	return proxiesPool[randGen.Intn(len(proxiesPool))]
}

func PickRandomUserAgent(randGen *rand.Rand) string {
	return userAgentsPool[randGen.Intn(len(userAgentsPool))]
}

func pickRandomProfile(randGen *rand.Rand) *Profile {
	return profilesPool[randGen.Intn(len(profilesPool))]
}

type implements struct {
	DeviceMemory bool
	SecFetchUser bool
	SecCh        bool
}

type Headers struct {
	UserAgent       string
	Referer         string
	AcceptLanguage  string
	AcceptEncoding  string
	Accept          string
	SecChUa         string
	SecChUaMobile   string
	SecChUaPlatform string
	DeviceMemory    string
	ViewportWidth   string
	ReferrerPolicy  string
	Implements      implements
}
