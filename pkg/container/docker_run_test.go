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

	"github.com/moby/moby/client"
	"github.com/nektos/act/pkg/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDocker(t *testing.T) {
	ctx := context.Background()
	cli, err := GetDockerClient(ctx)
	assert.NoError(t, err)
	defer cli.Close()

	dockerBuild := NewDockerBuildExecutor(NewDockerBuildExecutorInput{
		ContextDir: "testdata",
		ImageTag:   "envmergetest",
	})

	err = dockerBuild(ctx)
	assert.NoError(t, err)

	cr := &containerReference{
		cli: cli,
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

func (m *mockDockerClient) ExecCreate(ctx context.Context, id string, opts client.ExecCreateOptions) (client.ExecCreateResult, error) {
	args := m.Called(ctx, id, opts)
	return args.Get(0).(client.ExecCreateResult), args.Error(1)
}

func (m *mockDockerClient) ExecAttach(ctx context.Context, id string, opts client.ExecAttachOptions) (client.ExecAttachResult, error) {
	args := m.Called(ctx, id, opts)
	return args.Get(0).(client.ExecAttachResult), args.Error(1)
}

func (m *mockDockerClient) ExecInspect(ctx context.Context, execID string, opts client.ExecInspectOptions) (client.ExecInspectResult, error) {
	args := m.Called(ctx, execID, opts)
	return args.Get(0).(client.ExecInspectResult), args.Error(1)
}

func (m *mockDockerClient) CopyToContainer(ctx context.Context, id string, options client.CopyToContainerOptions) (client.CopyToContainerResult, error) {
	args := m.Called(ctx, id, options.DestinationPath, options.Content, options)
	return client.CopyToContainerResult{}, args.Error(0)
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

	cli := &mockDockerClient{}
	cli.On("ExecCreate", ctx, "123", mock.AnythingOfType("client.ExecCreateOptions")).Return(client.ExecCreateResult{ID: "id"}, nil)
	cli.On("ExecAttach", ctx, "id", mock.AnythingOfType("client.ExecAttachOptions")).Return(client.ExecAttachResult{
		HijackedResponse: client.HijackedResponse{
			Conn:   conn,
			Reader: bufio.NewReader(endlessReader{}),
		},
	}, nil)

	cr := &containerReference{
		id:  "123",
		cli: cli,
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
	cli.AssertExpectations(t)
}

func TestDockerExecFailure(t *testing.T) {
	ctx := context.Background()

	conn := &mockConn{}

	cli := &mockDockerClient{}
	cli.On("ExecCreate", ctx, "123", mock.AnythingOfType("client.ExecCreateOptions")).Return(client.ExecCreateResult{ID: "id"}, nil)
	cli.On("ExecAttach", ctx, "id", mock.AnythingOfType("client.ExecAttachOptions")).Return(client.ExecAttachResult{
		HijackedResponse: client.HijackedResponse{
			Conn:   conn,
			Reader: bufio.NewReader(strings.NewReader("output")),
		},
	}, nil)
	cli.On("ExecInspect", ctx, "id", mock.AnythingOfType("client.ExecInspectOptions")).Return(client.ExecInspectResult{
		ExitCode: 1,
	}, nil)

	cr := &containerReference{
		id:  "123",
		cli: cli,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	err := cr.exec([]string{""}, map[string]string{}, "user", "workdir")(ctx)
	assert.Error(t, err, "exit with `FAILURE`: 1")

	conn.AssertExpectations(t)
	cli.AssertExpectations(t)
}

func TestDockerCopyTarStream(t *testing.T) {
	ctx := context.Background()

	conn := &mockConn{}

	cli := &mockDockerClient{}
	cli.On("CopyToContainer", ctx, "123", "/", mock.Anything, mock.AnythingOfType("client.CopyToContainerOptions")).Return(nil)
	cli.On("CopyToContainer", ctx, "123", "/var/run/act", mock.Anything, mock.AnythingOfType("client.CopyToContainerOptions")).Return(nil)
	cr := &containerReference{
		id:  "123",
		cli: cli,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	_ = cr.CopyTarStream(ctx, "/var/run/act", &bytes.Buffer{})

	conn.AssertExpectations(t)
	cli.AssertExpectations(t)
}

func TestDockerCopyTarStreamDryRun(t *testing.T) {
	ctx := common.WithDryrun(context.Background(), true)

	conn := &mockConn{}

	cli := &mockDockerClient{}
	cli.AssertNotCalled(t, "CopyToContainer", ctx, "123", "/", mock.Anything, mock.AnythingOfType("client.CopyToContainerOptions"))
	cli.AssertNotCalled(t, "CopyToContainer", ctx, "123", "/var/run/act", mock.Anything, mock.AnythingOfType("client.CopyToContainerOptions"))
	cr := &containerReference{
		id:  "123",
		cli: cli,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	_ = cr.CopyTarStream(ctx, "/var/run/act", &bytes.Buffer{})

	conn.AssertExpectations(t)
	cli.AssertExpectations(t)
}

func TestDockerCopyTarStreamErrorInCopyFiles(t *testing.T) {
	ctx := context.Background()

	conn := &mockConn{}

	merr := fmt.Errorf("Failure")

	cli := &mockDockerClient{}
	cli.On("CopyToContainer", ctx, "123", "/", mock.Anything, mock.AnythingOfType("client.CopyToContainerOptions")).Return(merr)
	cli.On("CopyToContainer", ctx, "123", "/", mock.Anything, mock.AnythingOfType("client.CopyToContainerOptions")).Return(merr)
	cr := &containerReference{
		id:  "123",
		cli: cli,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	err := cr.CopyTarStream(ctx, "/var/run/act", &bytes.Buffer{})
	assert.ErrorIs(t, err, merr)

	conn.AssertExpectations(t)
	cli.AssertExpectations(t)
}

func TestDockerCopyTarStreamErrorInMkdir(t *testing.T) {
	ctx := context.Background()

	conn := &mockConn{}

	merr := fmt.Errorf("Failure")

	cli := &mockDockerClient{}
	cli.On("CopyToContainer", ctx, "123", "/", mock.Anything, mock.AnythingOfType("client.CopyToContainerOptions")).Return(nil)
	cli.On("CopyToContainer", ctx, "123", "/var/run/act", mock.Anything, mock.AnythingOfType("client.CopyToContainerOptions")).Return(merr)
	cr := &containerReference{
		id:  "123",
		cli: cli,
		input: &NewContainerInput{
			Image: "image",
		},
	}

	err := cr.CopyTarStream(ctx, "/var/run/act", &bytes.Buffer{})
	assert.ErrorIs(t, err, merr)

	conn.AssertExpectations(t)
	cli.AssertExpectations(t)
}

// Type assert containerReference implements ExecutionsEnvironment
var _ ExecutionsEnvironment = &containerReference{}
