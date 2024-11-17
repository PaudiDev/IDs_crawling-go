package assetshandler

import (
	"bufio"
	"os"

	"crawler/app/pkg/assert"
)

type HttpAssets struct {
	UserAgents []string
}

func GetUAsFromFile(path string) []string {
	assert.Assert(path != "", "user agents file path cannot be empty", assert.AssertData{"path": path})

	uaFile, err := os.Open(path)
	assert.NoError(err, "error opening user agents file", assert.AssertData{"path": path})
	defer func() {
		assert.NoError(uaFile.Close(), "error closing user agents file", assert.AssertData{"path": path})
	}()

	scanner := bufio.NewScanner(uaFile)
	var userAgents []string

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			userAgents = append(userAgents, line)
		}
	}
	assert.NoError(scanner.Err(), "error reading user agents file", assert.AssertData{"path": path})

	return userAgents
}
