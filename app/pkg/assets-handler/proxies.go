package assetshandler

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"

	"crawler/app/pkg/assert"
)

func GetProxiesFromFile(path string) []*url.URL {
	assert.Assert(path != "", "proxies file path cannot be empty", assert.AssertData{"path": path})

	pFile, err := os.Open(path)
	assert.NoError(err, "error opening proxies file", assert.AssertData{"path": path})
	defer func() {
		assert.NoError(pFile.Close(), "error closing proxies file", assert.AssertData{"path": path})
	}()

	scanner := bufio.NewScanner(pFile)
	var proxies []*url.URL
	var proxy string

	for scanner.Scan() {
		line := scanner.Text()
		proxyDataSlice := strings.Split(line, ":")

		switch len(proxyDataSlice) {
		case 2:
			proxyData := map[string]string{
				"protocol": "http",
				"ip":       proxyDataSlice[0],
				"port":     proxyDataSlice[1],
			}
			proxy = fmt.Sprintf(
				"%s://%s:%s",
				proxyData["protocol"],
				proxyData["ip"],
				proxyData["port"],
			)
		case 4:
			proxyData := map[string]string{
				"protocol": "http",
				"ip":       proxyDataSlice[0],
				"port":     proxyDataSlice[1],
				"username": proxyDataSlice[2],
				"password": proxyDataSlice[3],
			}
			proxy = fmt.Sprintf(
				"%s://%s:%s@%s:%s",
				proxyData["protocol"],
				proxyData["username"],
				proxyData["password"],
				proxyData["ip"],
				proxyData["port"],
			)
		default:
			assert.Never(
				"invalid proxy format. Should be ip:port or ip:port:username:password",
				assert.AssertData{"proxy": line, "path": path},
			)
		}
		proxyURL, err := url.Parse(proxy)
		assert.NoError(
			err, "error parsing proxy URL",
			assert.AssertData{"proxy": proxy, "path": path},
		)
		proxies = append(proxies, proxyURL)
	}
	assert.NoError(scanner.Err(), "error reading proxies file", assert.AssertData{"path": path})

	return proxies
}
