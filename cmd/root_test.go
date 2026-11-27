package cmd

import (
	"context"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nektos/act/pkg/model"
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
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	rootCmd := createRootCommand(context.Background(), &Input{}, "")
	err := newRunCommand(context.Background(), &Input{
		platforms:     []string{"ubuntu-latest=node:16-buster-slim"},
		workdir:       "../pkg/runner/testdata/",
		workflowsPath: "./basic/push.yml",
	})(rootCmd, []string{})
	assert.NoError(t, err)
}

func TestRunPush(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	rootCmd := createRootCommand(context.Background(), &Input{}, "")
	err := newRunCommand(context.Background(), &Input{
		platforms:     []string{"ubuntu-latest=node:16-buster-slim"},
		workdir:       "../pkg/runner/testdata/",
		workflowsPath: "./basic/push.yml",
	})(rootCmd, []string{"push"})
	assert.NoError(t, err)
}

func TestRunPushJsonLogger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
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
			if testing.Short() && f == "bug-report" {
				t.Skip("skipping integration test")
			}
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

func TestArtifactServerAddrDefaultIsLoopback(t *testing.T) {
	rootCmd := createRootCommand(context.Background(), &Input{}, "")
	flag := rootCmd.PersistentFlags().Lookup("artifact-server-addr")
	if assert.NotNil(t, flag) {
		assert.Equal(t, "127.0.0.1", flag.DefValue)
	}
}

func TestPrepareArtifactServerUsesLoopbackOnLinuxHostNetwork(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("loopback is selected for host networking on linux")
	}
	rootCmd := createRootCommand(context.Background(), &Input{}, "")
	addr, key, err := prepareArtifactServer(rootCmd, &Input{
		artifactServerPath: t.TempDir(),
		networkName:        "host",
	}, nil)
	assert.NoError(t, err)
	assert.Equal(t, "127.0.0.1", addr)
	assert.Len(t, key, 32)

	assert.NoError(t, rootCmd.ParseFlags([]string{"--artifact-server-addr=10.9.8.7"}))
	// The flag is bound to the Input passed to createRootCommand, not this one.
	addr, _, err = prepareArtifactServer(rootCmd, &Input{
		artifactServerPath: t.TempDir(),
		artifactServerAddr: "10.9.8.7",
		networkName:        "bridge",
	}, nil)
	assert.NoError(t, err)
	assert.Equal(t, "10.9.8.7", addr)
}

func TestPlanHasServiceContainers(t *testing.T) {
	assert.False(t, planHasServiceContainers(nil))
	assert.False(t, planHasServiceContainers(&model.Plan{}))

	withServices := &model.Plan{Stages: []*model.Stage{{
		Runs: []*model.Run{{
			Workflow: &model.Workflow{Jobs: map[string]*model.Job{
				"build": {Services: map[string]*model.ContainerSpec{"db": {}}},
			}},
			JobID: "build",
		}},
	}}}
	assert.True(t, planHasServiceContainers(withServices))

	without := &model.Plan{Stages: []*model.Stage{{
		Runs: []*model.Run{{
			Workflow: &model.Workflow{Jobs: map[string]*model.Job{
				"build": {},
			}},
			JobID: "build",
		}},
	}}}
	assert.False(t, planHasServiceContainers(without))
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

// TestGetDefaultNetworkStack checks the default follows the WSL_DISTRO_NAME environment variable.
func TestGetDefaultNetworkStack(t *testing.T) {
	tests := []struct {
		name     string
		wslEnv   string
		expected string
	}{
		{
			name:     "WSL2 environment",
			wslEnv:   "Ubuntu",
			expected: "tcp4",
		},
		{
			name:     "Non-WSL environment",
			wslEnv:   "",
			expected: "tcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original the value
			oldVal, wasSet := os.LookupEnv("WSL_DISTRO_NAME")

			if tt.wslEnv != "" {
				os.Setenv("WSL_DISTRO_NAME", tt.wslEnv)
			} else {
				os.Unsetenv("WSL_DISTRO_NAME")
			}

			// Cleanup: restore the original state
			t.Cleanup(func() {
				if wasSet {
					os.Setenv("WSL_DISTRO_NAME", oldVal)
				} else {
					os.Unsetenv("WSL_DISTRO_NAME")
				}
			})

			result := getDefaultNetworkStack()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestValidateNetworkStack checks only tcp, tcp4 and tcp6 are accepted.
func TestValidateNetworkStack(t *testing.T) {
	tests := []struct {
		name      string
		network   string
		wantError bool
	}{
		{
			name:      "valid tcp",
			network:   "tcp",
			wantError: false,
		},
		{
			name:      "valid tcp4",
			network:   "tcp4",
			wantError: false,
		},
		{
			name:      "valid tcp6",
			network:   "tcp6",
			wantError: false,
		},
		{
			name:      "invalid udp",
			network:   "udp",
			wantError: true,
		},
		{
			name:      "invalid empty",
			network:   "",
			wantError: true,
		},
		{
			name:      "invalid random",
			network:   "invalid",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNetworkStack(tt.network)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
