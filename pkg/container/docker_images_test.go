package container

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestImageExistsLocally(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()
	// to help make this test reliable and not flaky, we need to have
	// an image that will exist, and onew that won't exist

	exists, err := ImageExistsLocally(ctx, "library/alpine:this-random-tag-will-never-exist")
	assert.Nil(t, err)
	assert.Equal(t, false, exists)

	// pull an image
	cli, err := client.NewClientWithOpts(client.FromEnv)
	assert.Nil(t, err)
	cli.NegotiateAPIVersion(context.Background())

	// Chose alpine latest because it's so small
	// maybe we should build an image instead so that tests aren't reliable on dockerhub
	reader, err := cli.ImagePull(ctx, "alpine:latest", types.ImagePullOptions{})
	assert.Nil(t, err)
	defer reader.Close()
	_, err = ioutil.ReadAll(reader)
	assert.Nil(t, err)

	exists, err = ImageExistsLocally(ctx, "alpine:latest")
	assert.Nil(t, err)
	assert.Equal(t, true, exists)
}
