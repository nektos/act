package container

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDocker(t *testing.T) {
	ctx := context.Background()
	client, err := GetDockerClient(ctx)
	assert.NoError(t, err)
	defer client.Close()

	dockerBuild := NewDockerBuildExecutor(NewDockerBuildExecutorInput{
		ContextDir: "testdata",
		ImageTag:   "envmergetest",
	})

	err = dockerBuild(ctx)
	assert.NoError(t, err)

	cr := &containerReference{
		cli: client,
		input: &NewContainerInput{
			Image: "envmergetest",
		},
	}
	env := map[string]string{
		"PATH":         "/usr/local/bin:/usr/bin:/usr/sbin:/bin:/sbin",
		"RANDOM_VAR":   "WITH_VALUE",
		"ANOTHER_VAR":  "",
		"CONFLICT_VAR": "I_EXIST_IN_MULTIPLE_PLACES",
	}

	envExecutor := cr.extractFromImageEnv(&env)
	err = envExecutor(ctx)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"PATH":            "/usr/local/bin:/usr/bin:/usr/sbin:/bin:/sbin:/this/path/does/not/exists/anywhere:/this/either",
		"RANDOM_VAR":      "WITH_VALUE",
		"ANOTHER_VAR":     "",
		"SOME_RANDOM_VAR": "",
		"ANOTHER_ONE":     "BUT_I_HAVE_VALUE",
		"CONFLICT_VAR":    "I_EXIST_IN_MULTIPLE_PLACES",
	}, env)
}

type mockDockerClient struct {
	client.APIClient
	mock.Mock
}

func (m *mockDockerClient) ContainerExecCreate(ctx context.Context, id string, opts container.ExecOptions) (types.IDResponse, error) {
	args := m.Called(ctx, id, opts)
	return args.Get(0).(types.IDResponse), args.Error(1)
}

func (m *mockDockerClient) ContainerExecAttach(ctx context.Context, id string, opts container.ExecStartOptions) (types.HijackedResponse, error) {
	args := m.Called(ctx, id, opts)
	return args.Get(0).(types.HijackedResponse), args.Error(1)
}

func (m *mockDockerClient) ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error) {
	args := m.Called(ctx, execID)
	return args.Get(0).(container.ExecInspect), args.Error(1)
}

func (m *mockDockerClient) CopyToContainer(ctx context.Context, id string, path string, content io.Reader, options container.CopyToContainerOptions) error {
	args := m.Called(ctx, id, path, content, options)
	return args.Error(0)
}

type endlessReader struct {
	io.Reader
}

func (r endlessReader) Read(_ []byte) (n int, err error) {
	return 1, nil
}

type mockConn struct {
	net.Conn
	mock.Mock
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *mockConn) Close() (err error) {
	return nil
}

func TestDockerExecAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	conn := &mockConn{}
	conn.On("Write", mock.AnythingOfType("[]uint8")).Return(1, nil)

	client := &mockDockerClient{}
	client.On("ContainerExecCreate", ctx, "123", mock.AnythingOfType("container.ExecOptions")).Return(types.IDResponse{ID: "id"}, nil)
	client.On("ContainerExecAttach", ctx, "id", mock.AnythingOfType("container.ExecStartOptions")).Return(types.HijackedResponse{
		Conn:   conn,
		Reader: bufio.NewReader(endlessReader{}),
	}, nil)

	cr := &containerReference{
		id:  "123",
		cli: client,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	channel := make(chan error)

	go func() {
		channel <- cr.exec([]string{""}, map[string]string{}, "user", "workdir")(ctx)
	}()

	time.Sleep(500 * time.Millisecond)

	cancel()

	err := <-channel
	assert.ErrorIs(t, err, context.Canceled)

	conn.AssertExpectations(t)
	client.AssertExpectations(t)
}

func TestDockerExecFailure(t *testing.T) {
	ctx := context.Background()

	conn := &mockConn{}

	client := &mockDockerClient{}
	client.On("ContainerExecCreate", ctx, "123", mock.AnythingOfType("container.ExecOptions")).Return(types.IDResponse{ID: "id"}, nil)
	client.On("ContainerExecAttach", ctx, "id", mock.AnythingOfType("container.ExecStartOptions")).Return(types.HijackedResponse{
		Conn:   conn,
		Reader: bufio.NewReader(strings.NewReader("output")),
	}, nil)
	client.On("ContainerExecInspect", ctx, "id").Return(container.ExecInspect{
		ExitCode: 1,
	}, nil)

	cr := &containerReference{
		id:  "123",
		cli: client,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	err := cr.exec([]string{""}, map[string]string{}, "user", "workdir")(ctx)
	assert.Error(t, err, "exit with `FAILURE`: 1")

	conn.AssertExpectations(t)
	client.AssertExpectations(t)
}

func TestDockerCopyTarStream(t *testing.T) {
	ctx := context.Background()

	conn := &mockConn{}

	client := &mockDockerClient{}
	client.On("CopyToContainer", ctx, "123", "/", mock.Anything, mock.AnythingOfType("container.CopyToContainerOptions")).Return(nil)
	client.On("CopyToContainer", ctx, "123", "/var/run/act", mock.Anything, mock.AnythingOfType("container.CopyToContainerOptions")).Return(nil)
	cr := &containerReference{
		id:  "123",
		cli: client,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	_ = cr.CopyTarStream(ctx, "/var/run/act", &bytes.Buffer{})

	conn.AssertExpectations(t)
	client.AssertExpectations(t)
}

func TestDockerCopyTarStreamErrorInCopyFiles(t *testing.T) {
	ctx := context.Background()

	conn := &mockConn{}

	merr := fmt.Errorf("Failure")

	client := &mockDockerClient{}
	client.On("CopyToContainer", ctx, "123", "/", mock.Anything, mock.AnythingOfType("container.CopyToContainerOptions")).Return(merr)
	client.On("CopyToContainer", ctx, "123", "/", mock.Anything, mock.AnythingOfType("container.CopyToContainerOptions")).Return(merr)
	cr := &containerReference{
		id:  "123",
		cli: client,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	err := cr.CopyTarStream(ctx, "/var/run/act", &bytes.Buffer{})
	assert.ErrorIs(t, err, merr)

	conn.AssertExpectations(t)
	client.AssertExpectations(t)
}

func TestDockerCopyTarStreamErrorInMkdir(t *testing.T) {
	ctx := context.Background()

	conn := &mockConn{}

	merr := fmt.Errorf("Failure")

	client := &mockDockerClient{}
	client.On("CopyToContainer", ctx, "123", "/", mock.Anything, mock.AnythingOfType("container.CopyToContainerOptions")).Return(nil)
	client.On("CopyToContainer", ctx, "123", "/var/run/act", mock.Anything, mock.AnythingOfType("container.CopyToContainerOptions")).Return(merr)
	cr := &containerReference{
		id:  "123",
		cli: client,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	err := cr.CopyTarStream(ctx, "/var/run/act", &bytes.Buffer{})
	assert.ErrorIs(t, err, merr)

	conn.AssertExpectations(t)
	client.AssertExpectations(t)
}

// Type assert containerReference implements ExecutionsEnvironment
var _ ExecutionsEnvironment = &containerReference{}
