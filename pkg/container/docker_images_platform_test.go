//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"
	"errors"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageExistsLocallyPlatformInspect(t *testing.T) {
	arm64Image := client.ImageInspectResult{InspectResponse: image.InspectResponse{Os: "linux", Architecture: "arm64"}}
	amd64Image := client.ImageInspectResult{InspectResponse: image.InspectResponse{Os: "linux", Architecture: "amd64"}}
	// moby client.requiresVersion text for ImageInspectWithPlatform on API < 1.49.
	unsupported := errors.New(`"platform" requires API version 1.49, but the Docker daemon API version is 1.48`)
	daemonErr := errors.New("daemon exploded")

	tests := []struct {
		name     string
		platform string
		steps    []fakeInspectStep
		want     bool
		wantErr  error
	}{
		{
			name:     "platform inspect finds the requested architecture",
			platform: "linux/arm64",
			steps:    []fakeInspectStep{{opts: 1, res: arm64Image}},
			want:     true,
		},
		{
			name:     "platform inspect reports a different architecture",
			platform: "linux/arm64",
			steps:    []fakeInspectStep{{opts: 1, res: amd64Image}},
			want:     false,
		},
		{
			name:     "platform inspect miss is not found",
			platform: "linux/arm64",
			steps:    []fakeInspectStep{{opts: 1, err: cerrdefs.ErrNotFound}},
			want:     false,
		},
		{
			name:     "api older than 1.49 falls back to unscoped inspect",
			platform: "linux/arm64",
			steps: []fakeInspectStep{
				{opts: 1, err: unsupported},
				{opts: 0, res: arm64Image},
			},
			want: true,
		},
		{
			name:     "fallback keeps the unscoped platform comparison",
			platform: "linux/arm64",
			steps: []fakeInspectStep{
				{opts: 1, err: unsupported},
				{opts: 0, res: amd64Image},
			},
			want: false,
		},
		{
			name:     "empty inspect platform still mismatches",
			platform: "linux/arm64",
			steps:    []fakeInspectStep{{opts: 1, res: client.ImageInspectResult{}}},
			want:     false,
		},
		{
			name:     "other inspect errors are returned",
			platform: "linux/arm64",
			steps:    []fakeInspectStep{{opts: 1, err: daemonErr}},
			wantErr:  daemonErr,
		},
		{
			name:     "any does not filter by platform",
			platform: "any",
			steps:    []fakeInspectStep{{opts: 0, res: amd64Image}},
			want:     true,
		},
		{
			name:     "empty platform does not filter",
			platform: "",
			steps:    []fakeInspectStep{{opts: 0, res: amd64Image}},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &fakeImageAPI{t: t, steps: tt.steps}
			got, err := imageExistsLocally(context.Background(), cli, "alpine:3.21", tt.platform)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, len(tt.steps), cli.call)
		})
	}
}

func TestExtractFromImageEnvUsesRequestedPlatform(t *testing.T) {
	cli := &fakeImageAPI{
		t: t,
		steps: []fakeInspectStep{{
			opts: 1,
			res: client.ImageInspectResult{InspectResponse: image.InspectResponse{
				Os:           "linux",
				Architecture: "arm64",
			}},
		}},
	}
	cr := &containerReference{
		cli: cli,
		input: &NewContainerInput{
			Image:    "node:24-bookworm-slim",
			Platform: "linux/arm64",
		},
	}
	env := map[string]string{"PATH": "/usr/bin"}

	err := cr.extractFromImageEnv(&env)(context.Background())

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"PATH": "/usr/bin"}, env)
	assert.Equal(t, 1, cli.call)
}

type fakeInspectStep struct {
	opts int
	res  client.ImageInspectResult
	err  error
}

type fakeImageAPI struct {
	client.APIClient
	t     *testing.T
	steps []fakeInspectStep
	call  int
}

func (f *fakeImageAPI) ImageInspect(_ context.Context, _ string, opts ...client.ImageInspectOption) (client.ImageInspectResult, error) {
	f.t.Helper()
	if f.call >= len(f.steps) {
		f.t.Fatalf("unexpected ImageInspect call %d", f.call+1)
	}
	step := f.steps[f.call]
	f.call++
	if len(opts) != step.opts {
		f.t.Fatalf("ImageInspect call %d: got %d options, want %d", f.call, len(opts), step.opts)
	}
	return step.res, step.err
}
