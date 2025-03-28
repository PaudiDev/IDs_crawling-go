package network

import (
	"errors"
	"math/rand"
	"net/http/cookiejar"
	"net/url"

	wtypes "crawler/app/pkg/crawler/workers/workers-types"
)

var (
	proxiesPool    []*url.URL
	userAgentsPool []string
	profilesPool   []*Profile

	// must be exported in order to make the crawler assign each cookie jar to a refresh worker
	CookieJarSessionsPool []*wtypes.CookieJarSession
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

func InitCookieJars(amount uint16) error {
	if amount == 0 {
		return errors.New("tried to initialize cookie jars pool with 0 elements")
	}

	CookieJarSessionsPool = make([]*wtypes.CookieJarSession, amount)
	for i := uint16(0); i < amount; i++ {
		cookieJar, err := cookiejar.New(nil)
		if err != nil {
			return err
		}

		CookieJarSessionsPool[i] = &wtypes.CookieJarSession{
			CookieJar:   cookieJar,
			RefreshChan: make(chan struct{}, 10000),
		}
	}
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

func PickRandomCookieJarSession(randGen *rand.Rand) *wtypes.CookieJarSession {
	return CookieJarSessionsPool[randGen.Intn(len(CookieJarSessionsPool))]
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
