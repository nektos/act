//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/go-connections/nat"
	mobycontainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

type servicePortDockerClient struct {
	client.APIClient
	inspectResult client.ContainerInspectResult
	inspectErr    error
	removeResult  client.ContainerRemoveResult
	removeErr     error
	removeCalls   int
}

func (c *servicePortDockerClient) ContainerInspect(
	context.Context,
	string,
	client.ContainerInspectOptions,
) (client.ContainerInspectResult, error) {
	return c.inspectResult, c.inspectErr
}

func (c *servicePortDockerClient) ContainerRemove(
	context.Context,
	string,
	client.ContainerRemoveOptions,
) (client.ContainerRemoveResult, error) {
	c.removeCalls++
	return c.removeResult, c.removeErr
}

func TestConvertDockerPortMap(t *testing.T) {
	port, err := network.ParsePort("5432/tcp")
	require.NoError(t, err)

	got := convertDockerPortMap(network.PortMap{
		port: {
			{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "49153"},
			{HostIP: netip.MustParseAddr("::1"), HostPort: "49153"},
			{HostPort: "49154"},
		},
	})

	require.Equal(t, nat.PortMap{
		nat.Port("5432/tcp"): {
			{HostIP: "127.0.0.1", HostPort: "49153"},
			{HostIP: "::1", HostPort: "49153"},
			{HostPort: "49154"},
		},
	}, got)
}

func TestConvertDockerPortMapNil(t *testing.T) {
	require.Nil(t, convertDockerPortMap(nil))
}

func TestGetPortBindingsRequiresStartedContainer(t *testing.T) {
	for _, cr := range []*containerReference{
		{},
		{cli: &servicePortDockerClient{}},
	} {
		_, err := cr.GetPortBindings(context.Background())
		require.ErrorContains(t, err, "must be started")
	}
}

func TestGetPortBindingsReportsInspectError(t *testing.T) {
	inspectErr := errors.New("inspect failed")
	cr := &containerReference{
		cli: &servicePortDockerClient{inspectErr: inspectErr},
		id:  "service-container-id",
	}
	_, err := cr.GetPortBindings(context.Background())
	require.ErrorIs(t, err, inspectErr)
	require.ErrorContains(t, err, "inspect container port bindings")
}

func TestGetPortBindingsRequiresNetworkSettings(t *testing.T) {
	cr := &containerReference{
		cli: &servicePortDockerClient{
			inspectResult: client.ContainerInspectResult{
				Container: mobycontainer.InspectResponse{},
			},
		},
		id: "service-container-id",
	}
	_, err := cr.GetPortBindings(context.Background())
	require.ErrorContains(t, err, "missing network settings")
}

func TestGetPortBindingsConvertsInspectResponse(t *testing.T) {
	port, err := network.ParsePort("5432/tcp")
	require.NoError(t, err)
	cr := &containerReference{
		cli: &servicePortDockerClient{
			inspectResult: client.ContainerInspectResult{
				Container: mobycontainer.InspectResponse{
					NetworkSettings: &mobycontainer.NetworkSettings{
						Ports: network.PortMap{
							port: {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "49153"}},
						},
					},
				},
			},
		},
		id: "service-container-id",
	}

	got, err := cr.GetPortBindings(context.Background())
	require.NoError(t, err)
	require.Equal(t, nat.PortMap{
		nat.Port("5432/tcp"): {{HostIP: "127.0.0.1", HostPort: "49153"}},
	}, got)
}

func TestGetContainerNetworkReflectsOptionsOverride(t *testing.T) {
	const nominalNetwork = "act-job-network"
	cr := &containerReference{
		input: &NewContainerInput{
			NetworkMode: nominalNetwork,
			Options:     "--network host",
		},
	}
	_, mergedHostConfig, err := cr.mergeContainerConfigs(
		context.Background(),
		&mobycontainer.Config{},
		&mobycontainer.HostConfig{NetworkMode: mobycontainer.NetworkMode(nominalNetwork)},
	)
	require.NoError(t, err)
	require.Equal(t, mobycontainer.NetworkMode("host"), mergedHostConfig.NetworkMode)

	cr.cli = &servicePortDockerClient{
		inspectResult: client.ContainerInspectResult{
			Container: mobycontainer.InspectResponse{HostConfig: mergedHostConfig},
		},
	}
	cr.id = "job-container-id"

	got, err := cr.GetContainerNetwork(context.Background())
	require.NoError(t, err)
	require.Equal(t, "host", got)
	require.Equal(t, nominalNetwork, cr.input.NetworkMode)
}

func TestRemoveRetainsContainerIDOnFailure(t *testing.T) {
	removeErr := errors.New("remove failed")
	cli := &servicePortDockerClient{removeErr: removeErr}
	cr := &containerReference{cli: cli, id: "service-container-id"}

	err := cr.remove()(context.Background())
	require.ErrorIs(t, err, removeErr)
	require.Equal(t, "service-container-id", cr.id)
	require.Equal(t, 1, cli.removeCalls)
}

func TestRemoveClearsContainerIDWhenGone(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "removed"},
		{name: "already absent", err: cerrdefs.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := &servicePortDockerClient{removeErr: tt.err}
			cr := &containerReference{cli: cli, id: "service-container-id"}
			require.NoError(t, cr.remove()(context.Background()))
			require.Empty(t, cr.id)
			require.Equal(t, 1, cli.removeCalls)
		})
	}
}
