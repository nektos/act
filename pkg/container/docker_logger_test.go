//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"io"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogDockerResponseErrorDetail(t *testing.T) {
	logger := log.New()
	logger.SetOutput(io.Discard)

	body := io.NopCloser(strings.NewReader("{\"errorDetail\":{\"message\":\"NotFound: content digest sha256:abc: not found\"}}\n"))
	err := logDockerResponse(logger, body, true)

	require.Error(t, err)
	assert.True(t, isMissingImageContent(err))
}
