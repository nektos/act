package cmd

import (
	"context"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadSecrets(t *testing.T) {
	secrets := map[string]string{}
	ret := readEnvsEx(path.Join("testdata", "secrets.yml"), secrets, true)
	assert.True(t, ret)
	assert.Equal(t, `line1
line2
line3
`, secrets["MYSECRET"])
}

func TestReadEnv(t *testing.T) {
	secrets := map[string]string{}
	ret := readEnvs(path.Join("testdata", "secrets.yml"), secrets)
	assert.True(t, ret)
	assert.Equal(t, `line1
line2
line3
`, secrets["mysecret"])
}

func TestListOptions(t *testing.T) {
	rootCmd := createRootCommand(context.Background(), &Input{}, "")
	err := newRunCommand(context.Background(), &Input{
		listOptions: true,
	})(rootCmd, []string{})
	assert.NoError(t, err)
}

func TestRun(t *testing.T) {
	rootCmd := createRootCommand(context.Background(), &Input{}, "")
	err := newRunCommand(context.Background(), &Input{
		platforms:     []string{"ubuntu-latest=node:16-buster-slim"},
		workdir:       "../pkg/runner/testdata/",
		workflowsPath: "./basic/push.yml",
	})(rootCmd, []string{})
	assert.NoError(t, err)
}

func TestRunPush(t *testing.T) {
	rootCmd := createRootCommand(context.Background(), &Input{}, "")
	err := newRunCommand(context.Background(), &Input{
		platforms:     []string{"ubuntu-latest=node:16-buster-slim"},
		workdir:       "../pkg/runner/testdata/",
		workflowsPath: "./basic/push.yml",
	})(rootCmd, []string{"push"})
	assert.NoError(t, err)
}

func TestRunPushJsonLogger(t *testing.T) {
	rootCmd := createRootCommand(context.Background(), &Input{}, "")
	err := newRunCommand(context.Background(), &Input{
		platforms:     []string{"ubuntu-latest=node:16-buster-slim"},
		workdir:       "../pkg/runner/testdata/",
		workflowsPath: "./basic/push.yml",
		jsonLogger:    true,
	})(rootCmd, []string{"push"})
	assert.NoError(t, err)
}

func TestFlags(t *testing.T) {
	for _, f := range []string{"graph", "list", "bug-report", "man-page"} {
		t.Run("TestFlag-"+f, func(t *testing.T) {
			rootCmd := createRootCommand(context.Background(), &Input{}, "")
			err := rootCmd.Flags().Set(f, "true")
			assert.NoError(t, err)
			err = newRunCommand(context.Background(), &Input{
				platforms:     []string{"ubuntu-latest=node:16-buster-slim"},
				workdir:       "../pkg/runner/testdata/",
				workflowsPath: "./basic/push.yml",
			})(rootCmd, []string{})
			assert.NoError(t, err)
		})
	}
}

func TestValidateDockerDaemonServiceFlags(t *testing.T) {
	cases := []struct {
		name              string
		dockerDaemon      bool
		socketIn          string
		socketFlagChanged bool
		wantErr           bool
		wantErrContains   string
		wantSocketOut     string
	}{
		{
			// dind off, socket untouched: no-op.
			name:          "dind off / socket unset -> passthrough",
			wantSocketOut: "",
		},
		{
			// dind off, user picked a socket: leave it alone, no error.
			name:              "dind off / socket set to path -> passthrough",
			socketIn:          "unix:///var/run/docker.sock",
			socketFlagChanged: true,
			wantSocketOut:     "unix:///var/run/docker.sock",
		},
		{
			// dind off, user disabled the bind: leave it alone.
			name:              "dind off / socket set to '-' -> passthrough",
			socketIn:          "-",
			socketFlagChanged: true,
			wantSocketOut:     "-",
		},
		{
			// Common convenience case: --docker-daemon-service alone
			// auto-forces the socket to "-" so no host bind is added.
			name:          "dind on / socket unset -> auto '-'",
			dockerDaemon:  true,
			wantSocketOut: "-",
		},
		{
			// dind on, socket left at default "" but explicitly by the
			// user (shell-quoting mishap, --container-daemon-socket=).
			// Empty is not "-", so this is a conflict.
			name:              "dind on / socket explicitly empty -> error",
			dockerDaemon:      true,
			socketIn:          "",
			socketFlagChanged: true,
			wantErr:           true,
			wantErrContains:   `--container-daemon-socket=""`,
			wantSocketOut:     "",
		},
		{
			// dind on, explicit "-": accepted, no rewrite.
			name:              "dind on / socket explicitly '-' -> accepted",
			dockerDaemon:      true,
			socketIn:          "-",
			socketFlagChanged: true,
			wantSocketOut:     "-",
		},
		{
			// dind on with a URI: hard error, value quoted in message.
			name:              "dind on / socket set to unix URI -> error",
			dockerDaemon:      true,
			socketIn:          "unix:///var/run/docker.sock",
			socketFlagChanged: true,
			wantErr:           true,
			wantErrContains:   `--container-daemon-socket="unix:///var/run/docker.sock"`,
			wantSocketOut:     "unix:///var/run/docker.sock",
		},
		{
			// dind on with tcp URI: same hard error.
			name:              "dind on / socket set to tcp URI -> error",
			dockerDaemon:      true,
			socketIn:          "tcp://127.0.0.1:2375",
			socketFlagChanged: true,
			wantErr:           true,
			wantErrContains:   `--container-daemon-socket="tcp://127.0.0.1:2375"`,
			wantSocketOut:     "tcp://127.0.0.1:2375",
		},
		{
			// dind on with a bare filesystem path: same hard error.
			name:              "dind on / socket set to bare path -> error",
			dockerDaemon:      true,
			socketIn:          "/var/run/docker.sock",
			socketFlagChanged: true,
			wantErr:           true,
			wantErrContains:   `--container-daemon-socket="/var/run/docker.sock"`,
			wantSocketOut:     "/var/run/docker.sock",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := &Input{
				dockerDaemonService:   tc.dockerDaemon,
				containerDaemonSocket: tc.socketIn,
			}
			err := validateDockerDaemonServiceFlags(in, tc.socketFlagChanged)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrContains != "" && err != nil {
					assert.Contains(t, err.Error(), tc.wantErrContains)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.wantSocketOut, in.containerDaemonSocket,
				"containerDaemonSocket after validation")
		})
	}
}

func TestReadArgsFile(t *testing.T) {
	tables := []struct {
		path  string
		split bool
		args  []string
		env   map[string]string
	}{
		{
			path:  path.Join("testdata", "simple.actrc"),
			split: true,
			args:  []string{"--container-architecture=linux/amd64", "--action-offline-mode"},
		},
		{
			path:  path.Join("testdata", "env.actrc"),
			split: true,
			env: map[string]string{
				"FAKEPWD": "/fake/test/pwd",
				"FOO":     "foo",
			},
			args: []string{
				"--artifact-server-path", "/fake/test/pwd/.artifacts",
				"--env", "FOO=prefix/foo/suffix",
			},
		},
		{
			path:  path.Join("testdata", "split.actrc"),
			split: true,
			args:  []string{"--container-options", "--volume /foo:/bar --volume /baz:/qux --volume /tmp:/tmp"},
		},
	}
	for _, table := range tables {
		t.Run(table.path, func(t *testing.T) {
			for k, v := range table.env {
				t.Setenv(k, v)
			}
			args := readArgsFile(table.path, table.split)
			assert.Equal(t, table.args, args)
		})
	}
}
