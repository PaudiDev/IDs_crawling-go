package pathx

import (
	"path/filepath"

	"crawler/app/pkg/assert"
)

func FromCwd(path string) string {
	connectedPath, err := filepath.Abs(filepath.FromSlash(path))
	assert.NoError(
		err, "not finding a path should never happen",
		assert.AssertData{"path": path},
	)

	return connectedPath
}
