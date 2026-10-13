package container

import (
	"context"
	"fmt"
	"io"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
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

	// Test if image exists with specific tag
	invalidImageTag, err := ImageExistsLocally(ctx, "library/alpine:this-random-tag-will-never-exist", "linux/amd64")
	assert.Nil(t, err)
	assert.Equal(t, false, invalidImageTag)

	// Test if image exists with specific architecture (image platform)
	invalidImagePlatform, err := ImageExistsLocally(ctx, "alpine:latest", "windows/amd64")
	assert.Nil(t, err)
	assert.Equal(t, false, invalidImagePlatform)

	// pull an image
	cli, err := client.New(client.FromEnv)
	assert.Nil(t, err)

	// Chose alpine latest because it's so small
	// maybe we should build an image instead so that tests aren't reliable on dockerhub
	readerDefault, err := cli.ImagePull(ctx, "node:16-buster-slim", client.ImagePullOptions{
		Platforms: []specs.Platform{{OS: "linux", Architecture: "amd64"}},
	})
	assert.Nil(t, err)
	defer readerDefault.Close()
	_, err = io.ReadAll(readerDefault)
	assert.Nil(t, err)

	imageDefaultArchExists, err := ImageExistsLocally(ctx, "node:16-buster-slim", "linux/amd64")
	assert.Nil(t, err)
	assert.Equal(t, true, imageDefaultArchExists)

	// Validate if another architecture platform can be pulled
	readerArm64, err := cli.ImagePull(ctx, "node:16-buster-slim", client.ImagePullOptions{
		Platforms: []specs.Platform{{OS: "linux", Architecture: "arm64"}},
	})
	assert.Nil(t, err)
	defer readerArm64.Close()
	_, err = io.ReadAll(readerArm64)
	assert.Nil(t, err)

	imageArm64Exists, err := ImageExistsLocally(ctx, "node:16-buster-slim", "linux/arm64")
	assert.Nil(t, err)
	assert.Equal(t, true, imageArm64Exists)
}

type fakeImageAPIClient struct {
	client.APIClient
	apiVersion    string
	versionErr    error
	inspectResult client.ImageInspectResult
	inspectErr    error
	inspectOpts   int
}

func (f *fakeImageAPIClient) ServerVersion(_ context.Context, _ client.ServerVersionOptions) (client.ServerVersionResult, error) {
	return client.ServerVersionResult{APIVersion: f.apiVersion}, f.versionErr
}

func (f *fakeImageAPIClient) ImageInspect(_ context.Context, _ string, opts ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	f.inspectOpts = len(opts)
	return f.inspectResult, f.inspectErr
}

func inspectResult(os, arch, variant string) client.ImageInspectResult {
	var r client.ImageInspectResult
	r.Os = os
	r.Architecture = arch
	r.Variant = variant
	return r
}

func Test_imageExistsLocally(t *testing.T) {
	ctx := context.Background()
	notFound := fmt.Errorf("not found: %w", cerrdefs.ErrNotFound)

	tests := []struct {
		name       string
		apiVersion string
		versionErr error
		imageName  string
		platform   string
		inspect    client.ImageInspectResult
		inspectErr error
		want       bool
		wantOpts   bool
	}{
		{
			name:       "api 1.49 passes platform option and returns true",
			apiVersion: "1.49",
			imageName:  "alpine:latest",
			platform:   "linux/arm64",
			want:       true,
			wantOpts:   true,
		},
		{
			name:       "api 1.49 platform-scoped not found",
			apiVersion: "1.49",
			imageName:  "alpine:latest",
			platform:   "linux/arm64",
			inspectErr: notFound,
			want:       false,
			wantOpts:   true,
		},
		{
			name:       "old api platform match",
			apiVersion: "1.48",
			imageName:  "alpine:latest",
			platform:   "linux/amd64",
			inspect:    inspectResult("linux", "amd64", ""),
			want:       true,
			wantOpts:   false,
		},
		{
			name:       "old api platform mismatch",
			apiVersion: "1.48",
			imageName:  "alpine:latest",
			platform:   "linux/arm64",
			inspect:    inspectResult("linux", "amd64", ""),
			want:       false,
			wantOpts:   false,
		},
		{
			name:       "old api variant match",
			apiVersion: "1.48",
			imageName:  "alpine:latest",
			platform:   "linux/arm64/v8",
			inspect:    inspectResult("linux", "arm64", "v8"),
			want:       true,
			wantOpts:   false,
		},
		{
			name:       "old api variant mismatch",
			apiVersion: "1.48",
			imageName:  "alpine:latest",
			platform:   "linux/arm64/v7",
			inspect:    inspectResult("linux", "arm64", "v8"),
			want:       false,
			wantOpts:   false,
		},
		{
			name:       "old api request without variant matches any variant",
			apiVersion: "1.48",
			imageName:  "alpine:latest",
			platform:   "linux/arm64",
			inspect:    inspectResult("linux", "arm64", "v8"),
			want:       true,
			wantOpts:   false,
		},
		{
			name:       "server version error falls back to comparison",
			versionErr: fmt.Errorf("connection refused"),
			imageName:  "alpine:latest",
			platform:   "linux/amd64",
			inspect:    inspectResult("linux", "amd64", ""),
			want:       true,
			wantOpts:   false,
		},
		{
			name:       "any platform constraint",
			apiVersion: "1.49",
			imageName:  "alpine:latest",
			platform:   "any",
			inspect:    inspectResult("linux", "amd64", ""),
			want:       true,
			wantOpts:   false,
		},
		{
			name:       "empty platform constraint",
			apiVersion: "1.49",
			imageName:  "alpine:latest",
			platform:   "",
			inspect:    inspectResult("linux", "amd64", ""),
			want:       true,
			wantOpts:   false,
		},
		{
			name:       "image not found",
			apiVersion: "1.48",
			imageName:  "missing:latest",
			platform:   "linux/amd64",
			inspectErr: notFound,
			want:       false,
			wantOpts:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &fakeImageAPIClient{
				apiVersion:    tt.apiVersion,
				versionErr:    tt.versionErr,
				inspectResult: tt.inspect,
				inspectErr:    tt.inspectErr,
			}
			got, err := imageExistsLocally(ctx, cli, tt.imageName, tt.platform)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantOpts, cli.inspectOpts > 0)
		})
	}
}

func Test_parsePlatform(t *testing.T) {
	assert.Nil(t, parsePlatform(""))
	assert.Nil(t, parsePlatform("any"))
	assert.Equal(t, &specs.Platform{OS: "linux", Architecture: "amd64"}, parsePlatform("linux/amd64"))
	assert.Equal(t, &specs.Platform{OS: "linux", Architecture: "arm", Variant: "v7"}, parsePlatform("linux/arm/v7"))
}
