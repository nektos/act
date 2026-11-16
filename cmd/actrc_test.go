package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nektos/act/pkg/common"
)

func testFlagSets(t *testing.T) []*pflag.FlagSet {
	t.Helper()
	cmd := createRootCommand(t.Context(), &Input{}, "test")
	return []*pflag.FlagSet{cmd.Flags(), cmd.PersistentFlags()}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestProjectActrcKeepsOnlyAllowedFlags(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user-actrc")
	project := filepath.Join(dir, "project-actrc")
	writeFile(t, user, "--container-daemon-socket -\n-P ubuntu-latest=user/image\n")
	writeFile(t, project, `-P ubuntu-latest=catthehacker/ubuntu:act-latest
-P self=-self-hosted
-Pother=-self-hosted
--container-daemon-socket /var/run/docker.sock
--privileged
--container-options --privileged
--container-options-policy allow
--local-repository org/repo@v1=/
--artifact-server-path /tmp/x
--secret GITHUB_TOKEN
-s AWS_SECRET_ACCESS_KEY
--env HOME
--env FOO=bar
--var-file ../../.aws/credentials
--secret-file .secrets
--action-cache-path /
--trust-project-actrc
--network host
--env=TOKEN=$GITHUB_TOKEN
-W .github/workflows/ci.yml
--reuse
`)
	t.Setenv("GITHUB_TOKEN", "host-token")

	got := actrcArgs(testFlagSets(t), []string{user}, project, []string{"-j", "build"})
	assert.Equal(t, []string{
		"--container-daemon-socket", "-",
		"-P", "ubuntu-latest=user/image",
		"-P", "ubuntu-latest=catthehacker/ubuntu:act-latest",
		"--env", "FOO=bar",
		"--secret-file", ".secrets",
		"--env=TOKEN=$GITHUB_TOKEN",
		"-W", ".github/workflows/ci.yml",
		"--reuse",
		"-j", "build",
	}, got)
}

func TestProjectActrcTrusted(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user-actrc")
	project := filepath.Join(dir, "project-actrc")
	writeFile(t, user, "--trust-project-actrc\n")
	writeFile(t, project, "--privileged\n--env TOKEN=$ACT_TEST_VALUE\n")
	t.Setenv("ACT_TEST_VALUE", "expanded")

	got := actrcArgs(testFlagSets(t), []string{user}, project, nil)
	assert.Equal(t, []string{"--trust-project-actrc", "--privileged", "--env", "TOKEN=expanded"}, got)

	got = actrcArgs(testFlagSets(t), []string{user}, project, []string{"--trust-project-actrc=false"})
	assert.Equal(t, []string{"--trust-project-actrc", "--env", "TOKEN=$ACT_TEST_VALUE", "--trust-project-actrc=false"}, got)

	got = actrcArgs(testFlagSets(t), nil, project, []string{"--trust-project-actrc"})
	assert.Equal(t, []string{"--privileged", "--env", "TOKEN=expanded", "--trust-project-actrc"}, got)
}

func TestProjectActrcInHomeDirectoryIsUserConfig(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, ".actrc")
	writeFile(t, home, "--privileged\n")
	t.Chdir(dir)

	got := actrcArgs(testFlagSets(t), []string{home}, filepath.Join(".", ".actrc"), nil)
	assert.Equal(t, []string{"--privileged"}, got)
}

func TestProjectActrcRejectsUnknownShorthand(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project-actrc")
	writeFile(t, project, "-Z value\n-\n-C /\n")
	assert.Empty(t, actrcArgs(testFlagSets(t), nil, project, nil))
}

func TestProjectActrcBypassVectors(t *testing.T) {
	outside := t.TempDir()
	secretPath := filepath.Join(outside, "host.secrets")
	writeFile(t, secretPath, "LEAKED_TOKEN=repro-sentinel\n")

	vectors := []struct {
		name    string
		content string
	}{
		{name: "bool trailing long", content: "--reuse --privileged\n"},
		{name: "bool trailing short", content: "-r --privileged\n"},
		{name: "bool equals trailing", content: "--reuse=true --privileged\n"},
		{name: "bool trailing secret", content: "--reuse --secret-file=" + secretPath + "\n"},
		{name: "bool trailing options", content: "--reuse --container-options=--privileged\n"},
		{name: "bool trailing platform", content: "--reuse -P=self=-self-hosted\n"},
		{name: "bool trailing policy", content: "--reuse --container-options-policy=allow\n"},
		{name: "equals trailing privileged", content: "--env=FOO=bar --privileged\n"},
		{name: "spaced value then privileged", content: "--env FOO=bar --privileged\n"},
		{name: "path value then privileged", content: "--secret-file .secrets --privileged\n"},
		{name: "short equals trailing", content: "-P=ubuntu-latest=node:16 --privileged\n"},
		{name: "equals trailing secret", content: "--env=FOO=bar --secret-file=" + secretPath + "\n"},
		{name: "attached shorthand trailing", content: "-W.github/workflows/ci.yml --privileged\n"},
		{name: "cluster platform", content: "-rP self=-self-hosted\n"},
		{name: "cluster several then platform", content: "-qrP self=-self-hosted\n"},
		{name: "cluster without value", content: "-rP\n"},
		{name: "value flag swallows next line", content: "--platform\n--privileged\n"},
		{name: "value flag swallows self-hosted", content: "--platform\nubuntu-latest=-self-hosted\n"},
		{name: "job swallows privileged", content: "--job\n--privileged\n"},
		{name: "actor swallows privileged", content: "--actor\n--privileged\n"},
		{name: "architecture swallows privileged", content: "--container-architecture\n--privileged\n"},
		{name: "secret file swallows outside path", content: "--secret-file\n" + secretPath + "\n"},
		{name: "two allowlisted flags one line", content: "--reuse --verbose\n"},
	}
	for _, tc := range vectors {
		t.Run(tc.name, func(t *testing.T) {
			args, input, _ := parseProjectActrc(t, "", tc.content, nil)
			assert.Empty(t, args)
			assertProjectActrcSafe(t, input, secretPath)
		})
	}

	t.Run("mixed file keeps only clean lines", func(t *testing.T) {
		content := "" +
			"-P ubuntu-latest=node:16\n" +
			"--reuse --privileged\n" +
			"-rP self=-self-hosted\n" +
			"--env=FOO=bar --privileged\n" +
			"--env SAFE=1\n" +
			"--secret-file " + secretPath + "\n"
		args, input, _ := parseProjectActrc(t, "", content, nil)
		assert.Equal(t, []string{"-P", "ubuntu-latest=node:16", "--env", "SAFE=1"}, args)
		assert.Equal(t, []string{"ubuntu-latest=node:16"}, input.platforms)
		assert.Equal(t, []string{"SAFE=1"}, input.envs)
		assert.False(t, input.privileged)
		assert.NotEqual(t, secretPath, input.secretfile)
		assertNoSelfHosted(t, input)
	})

	t.Run("does not override operator hardening or swallow cli", func(t *testing.T) {
		user := "--container-daemon-socket -\n--container-options-policy=restrict\n"
		project := "" +
			"--reuse --container-daemon-socket=/var/run/docker.sock\n" +
			"--reuse --privileged\n" +
			"--reuse --container-options-policy=allow\n" +
			"--reuse -P=self=-self-hosted\n" +
			"--reuse --secret-file=" + secretPath + "\n" +
			"--platform\n"
		args, input, _ := parseProjectActrc(t, user, project, []string{"-P", "ubuntu-latest=node:16"})
		assert.Equal(t, []string{
			"--container-daemon-socket", "-",
			"--container-options-policy=restrict",
			"-P", "ubuntu-latest=node:16",
		}, args)
		assert.Equal(t, "-", input.containerDaemonSocket)
		assert.Equal(t, "restrict", input.containerOptionsPolicy)
		assert.Equal(t, []string{"ubuntu-latest=node:16"}, input.platforms)
		assertProjectActrcSafe(t, input, secretPath)
	})

	t.Run("bare platform does not swallow the next line", func(t *testing.T) {
		args, input, _ := parseProjectActrc(t, "", "--platform\n--env=FOO=bar\n--privileged\n", nil)
		assert.Equal(t, []string{"--env=FOO=bar"}, args)
		assert.Equal(t, []string{"FOO=bar"}, input.envs)
		assert.Empty(t, input.platforms)
		assertProjectActrcSafe(t, input, secretPath)
	})

	t.Run("outside secret file is not loaded", func(t *testing.T) {
		_, input, _ := parseProjectActrc(t, "", "--reuse --secret-file="+secretPath+"\n", nil)
		assert.Equal(t, ".secrets", input.secretfile)
		loaded := map[string]string{}
		assert.False(t, readProjectEnvs(t.TempDir(), input.Secretfile(), loaded, true))
		assert.NotContains(t, loaded, "LEAKED_TOKEN")
		reachable := map[string]string{}
		assert.True(t, readProjectEnvs(t.TempDir(), secretPath, reachable, true))
		assert.Equal(t, "repro-sentinel", reachable["LEAKED_TOKEN"])
	})
}

func TestProjectActrcLegitimateLines(t *testing.T) {
	content := "" +
		"# --privileged\n" +
		"\n" +
		"--reuse\n" +
		"--verbose\n" +
		"--env FOO=bar\n" +
		"--env=BAR=baz\n" +
		"--env MSG=hello world\n" +
		"-P ubuntu-latest=node:16\n" +
		"--platform=ubuntu-22.04=node:16-bullseye-slim\n" +
		"-P=ubuntu-18.04=node:16\n" +
		"-Pubuntu-20.04=node:16\n" +
		"--secret-file .secrets\n" +
		"-W .github/workflows/ci.yml\n" +
		"-W=.github/workflows/push.yml\n" +
		"--actor Ada Lovelace\n" +
		"-rq\n" +
		"--json\n" +
		"--concurrent-jobs 2\n" +
		"--matrix os:ubuntu\n" +
		"-j build\n" +
		"-e event.json\n" +
		"--reuse=false\n"
	args, input, cmd := parseProjectActrc(t, "", content, nil)
	assert.Equal(t, []string{
		"--reuse",
		"--verbose",
		"--env", "FOO=bar",
		"--env=BAR=baz",
		"--env", "MSG=hello world",
		"-P", "ubuntu-latest=node:16",
		"--platform=ubuntu-22.04=node:16-bullseye-slim",
		"-P=ubuntu-18.04=node:16",
		"-Pubuntu-20.04=node:16",
		"--secret-file", ".secrets",
		"-W", ".github/workflows/ci.yml",
		"-W=.github/workflows/push.yml",
		"--actor", "Ada Lovelace",
		"-r", "-q",
		"--json",
		"--concurrent-jobs", "2",
		"--matrix", "os:ubuntu",
		"-j", "build",
		"-e", "event.json",
		"--reuse=false",
	}, args)

	verbose, err := cmd.Flags().GetBool("verbose")
	require.NoError(t, err)
	job, err := cmd.Flags().GetString("job")
	require.NoError(t, err)
	assert.True(t, verbose)
	assert.False(t, input.reuseContainers)
	assert.True(t, input.noOutput)
	assert.True(t, input.jsonLogger)
	assert.Equal(t, "Ada Lovelace", input.actor)
	assert.Equal(t, ".secrets", input.secretfile)
	assert.Equal(t, ".github/workflows/push.yml", input.workflowsPath)
	assert.Equal(t, "event.json", input.eventPath)
	assert.Equal(t, 2, input.concurrentJobs)
	assert.Equal(t, []string{"os:ubuntu"}, input.matrix)
	assert.Equal(t, "build", job)
	assert.Equal(t, []string{"FOO=bar", "BAR=baz", "MSG=hello world"}, input.envs)
	assert.Equal(t, []string{
		"ubuntu-latest=node:16",
		"ubuntu-22.04=node:16-bullseye-slim",
		"ubuntu-18.04=node:16",
		"ubuntu-20.04=node:16",
	}, input.platforms)
	assert.False(t, input.privileged)
	assert.Empty(t, input.containerOptions)
	assertNoSelfHosted(t, input)
}

func TestProjectActrcRejectsHostPlatformFold(t *testing.T) {
	longS := "\u017f"
	rejected := []struct {
		name    string
		content string
	}{
		{name: "upper", content: "-P ubuntu-latest=-SELF-HOSTED\n"},
		{name: "mixed", content: "-P ubuntu-latest=-Self-Hosted\n"},
		{name: "partial", content: "-P ubuntu-latest=-self-HOSTED\n"},
		{name: "long flag spaced", content: "--platform ubuntu-latest=-SELF-HOSTED\n"},
		{name: "long flag equals", content: "--platform=ubuntu-latest=-SELF-HOSTED\n"},
		{name: "short equals", content: "-P=ubuntu-latest=-SELF-HOSTED\n"},
		{name: "short attached", content: "-Pubuntu-latest=-SELF-HOSTED\n"},
		{name: "trailing space", content: "-P ubuntu-latest=-SELF-HOSTED \n"},
		{name: "nbsp", content: "-P ubuntu-latest=\u00a0-SELF-HOSTED\n"},
		{name: "long s", content: "-P ubuntu-latest=-" + longS + "elf-hosted\n"},
		{name: "long s both", content: "-P ubuntu-latest=-" + longS + "elf-ho" + longS + "ted\n"},
		{name: "mixed long s", content: "-P ubuntu-latest=-" + longS + "ELF-ho" + longS + "TED\n"},
		{name: "leading space looks like a flag", content: "-P ubuntu-latest= -SELF-HOSTED\n"},
		{name: "tab looks like a flag", content: "-P ubuntu-latest=\t-SELF-HOSTED\n"},
		{
			name: "bad line last among several",
			content: "" +
				"-P ubuntu-22.04=node:16\n" +
				"-P ubuntu-latest=-SELF-HOSTED\n",
		},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			_, input, _ := parseProjectActrc(t, "", tc.content, nil)
			assertNoSelfHosted(t, input)
			assert.Equal(t, "node:16-buster-slim", input.newPlatforms()["ubuntu-latest"])
		})
	}

	t.Run("keeps images that only look similar", func(t *testing.T) {
		content := "" +
			"-P ubuntu-latest=self-hosted\n" +
			"-P ubuntu-22.04=-self-hosted-extra\n" +
			"-P ubuntu-20.04=-\uff53elf-hosted\n" +
			"-P ubuntu-18.04=-self\u2010hosted\n"
		_, input, _ := parseProjectActrc(t, "", content, nil)
		platforms := input.newPlatforms()
		assert.Equal(t, "self-hosted", platforms["ubuntu-latest"])
		assert.Equal(t, "-self-hosted-extra", platforms["ubuntu-22.04"])
		assert.Equal(t, "-\uff53elf-hosted", platforms["ubuntu-20.04"])
		assert.Equal(t, "-self\u2010hosted", platforms["ubuntu-18.04"])
		assertNoSelfHosted(t, input)
	})

	t.Run("user config and trusted project file may select host mode", func(t *testing.T) {
		_, input, _ := parseProjectActrc(t, "-P ubuntu-latest=-SELF-HOSTED\n", "# project\n", nil)
		assert.True(t, common.IsHostPlatform(input.newPlatforms()["ubuntu-latest"]))

		_, input, _ = parseProjectActrc(t, "", "-P ubuntu-latest=-SELF-HOSTED\n", []string{"--trust-project-actrc"})
		assert.True(t, common.IsHostPlatform(input.newPlatforms()["ubuntu-latest"]))
	})
}

func parseProjectActrc(t *testing.T, userContent, projectContent string, cli []string) ([]string, *Input, *cobra.Command) {
	t.Helper()
	dir := t.TempDir()
	var userFiles []string
	if userContent != "" {
		user := filepath.Join(dir, "user-actrc")
		writeFile(t, user, userContent)
		userFiles = []string{user}
	}
	project := filepath.Join(dir, "project-actrc")
	writeFile(t, project, projectContent)
	input := &Input{}
	cmd := createRootCommand(t.Context(), input, "test")
	sets := []*pflag.FlagSet{cmd.Flags(), cmd.PersistentFlags()}
	args := actrcArgs(sets, userFiles, project, cli)
	require.NoError(t, cmd.ParseFlags(args))
	return args, input, cmd
}

func assertProjectActrcSafe(t *testing.T, input *Input, secretPath string) {
	t.Helper()
	assert.False(t, input.privileged)
	assert.Empty(t, input.containerOptions)
	assert.NotEqual(t, secretPath, input.secretfile)
	assert.NotEqual(t, "/var/run/docker.sock", input.containerDaemonSocket)
	assertNoSelfHosted(t, input)
}

func TestProjectActrcRejectsEscapingPathSymlinks(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "event.json"), []byte("{\"marker\":\"outside\"}\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "foreign.yml"), []byte("name: foreign\n"), 0o600))
	if err := os.Symlink(filepath.Join(outside, "event.json"), filepath.Join(repo, "abs-link.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	rel, err := filepath.Rel(repo, filepath.Join(outside, "event.json"))
	require.NoError(t, err)
	require.NoError(t, os.Symlink(rel, filepath.Join(repo, "rel-link.json")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "foreign.yml"), filepath.Join(repo, "foreign.yml")))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "inside.json"), []byte("{\"marker\":\"inside\"}\n"), 0o600))
	require.NoError(t, os.Symlink("inside.json", filepath.Join(repo, "inside-link.json")))
	t.Chdir(repo)

	t.Run("absolute symlink target", func(t *testing.T) {
		_, input, _ := parseProjectActrc(t, "", "--eventpath abs-link.json\n", nil)
		assert.Empty(t, input.eventPath)
	})
	t.Run("relative symlink target", func(t *testing.T) {
		_, input, _ := parseProjectActrc(t, "", "--eventpath rel-link.json\n", nil)
		assert.Empty(t, input.eventPath)
	})
	t.Run("workflows file symlink", func(t *testing.T) {
		_, input, _ := parseProjectActrc(t, "", "--workflows foreign.yml\n", nil)
		assert.Equal(t, "./.github/workflows/", input.workflowsPath)
	})
	t.Run("lexical absolute and dotdot", func(t *testing.T) {
		_, input, _ := parseProjectActrc(t, "", "--eventpath /tmp/event.json\n--workflows ../foreign.yml\n", nil)
		assert.Empty(t, input.eventPath)
		assert.Equal(t, "./.github/workflows/", input.workflowsPath)
	})
	t.Run("symlink that stays inside", func(t *testing.T) {
		_, input, _ := parseProjectActrc(t, "", "--eventpath inside-link.json\n", nil)
		assert.Equal(t, "inside-link.json", input.eventPath)
	})
	t.Run("trusted project file may follow the link", func(t *testing.T) {
		_, input, _ := parseProjectActrc(t, "", "--eventpath abs-link.json\n-W foreign.yml\n", []string{"--trust-project-actrc"})
		assert.Equal(t, "abs-link.json", input.eventPath)
		assert.Equal(t, "foreign.yml", input.workflowsPath)
	})
	t.Run("cli flag may name the link", func(t *testing.T) {
		_, input, _ := parseProjectActrc(t, "", "# project\n", []string{"--eventpath", "abs-link.json", "-W", "foreign.yml"})
		assert.Equal(t, "abs-link.json", input.eventPath)
		assert.Equal(t, "foreign.yml", input.workflowsPath)
	})
}

func assertNoSelfHosted(t *testing.T, input *Input) {
	t.Helper()
	for name, image := range input.newPlatforms() {
		assert.False(t, common.IsHostPlatform(image), "%s=%s", name, image)
	}
}
