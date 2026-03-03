package cmd

import (
	"context"
	"path"
	"testing"

	"github.com/spf13/cobra"
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

func TestSanitizeFlagDefaultsForDocs(t *testing.T) {
	root := &cobra.Command{Use: "act"}
	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)
	root.PersistentFlags().String("artifact-server-addr", "192.168.0.10:34567", "addr")
	root.PersistentFlags().String("cache-server-address", "10.0.0.5", "addr")
	child.Flags().String("custom-addr", "172.16.0.2:1234", "addr")
	root.PersistentFlags().String("actor", "nektos/act", "user that triggered the event")
	sanitizeFlagDefaultsForDocs(root)
	const docAddrPlaceholder = "[auto-detected IP]"
	assert.Equal(t, docAddrPlaceholder, root.PersistentFlags().Lookup("artifact-server-addr").DefValue)
	assert.Equal(t, docAddrPlaceholder, root.PersistentFlags().Lookup("cache-server-address").DefValue)
	assert.Equal(t, docAddrPlaceholder, child.Flags().Lookup("custom-addr").DefValue)
	assert.Equal(t, "nektos/act", root.PersistentFlags().Lookup("actor").DefValue)
}
