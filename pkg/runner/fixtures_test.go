package runner

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/assert"
)

const (
	hostEnvFixtureTag = "act-docker-action-host-env-action-dockeraction:latest"
	bookwormCurlTag   = "act-ci/bookworm-curl:latest"
)

func TestMain(m *testing.M) {
	flag.Parse()
	// Build when the daemon runs Linux containers, including Docker Desktop on
	// macOS. A Windows daemon cannot pull debian:bookworm-slim.
	if !testing.Short() && dockerDaemonOS() == "linux" {
		ensureCIFixtures()
	}
	os.Exit(m.Run())
}

func dockerDaemonOS() string {
	if _, err := exec.LookPath("docker"); err != nil {
		return ""
	}
	cmd := exec.Command("docker", "info", "--format", "{{.OSType}}")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func TestHostEnvFixtureTagMatchesAct(t *testing.T) {
	actionName := "./docker-action-host-env/action"
	image := fmt.Sprintf("%s-dockeraction:%s", regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(actionName, "-"), "latest")
	image = fmt.Sprintf("act-%s", strings.TrimLeft(image, "-"))
	image = strings.ToLower(image)
	assert.Equal(t, hostEnvFixtureTag, image)
}

func ensureCIFixtures() {
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Fprintln(os.Stderr, "docker not found; skipping fixture image build")
		return
	}
	root := repoRoot()
	fixtures := []struct {
		tag        string
		dockerfile string
		context    string
	}{
		{
			tag:        hostEnvFixtureTag,
			dockerfile: filepath.Join(root, "pkg/runner/testdata/docker-action-host-env/action/Dockerfile"),
			context:    filepath.Join(root, "pkg/runner/testdata/docker-action-host-env/action"),
		},
		{
			tag:        bookwormCurlTag,
			dockerfile: filepath.Join(root, ".github/ci/fixtures/bookworm-curl.Dockerfile"),
			context:    filepath.Join(root, ".github/ci/fixtures"),
		},
	}
	for _, fixture := range fixtures {
		if dockerImagePresent(fixture.tag) {
			continue
		}
		fmt.Fprintf(os.Stderr, "building fixture image %s\n", fixture.tag)
		//nolint:gosec // G204: tag and paths are fixed fixture constants, not user input
		cmd := exec.Command("docker", "build", "-t", fixture.tag, "-f", fixture.dockerfile, fixture.context)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "fixture image %s: %v\n", fixture.tag, err)
			if os.Getenv("GITHUB_ACTIONS") == "true" {
				os.Exit(1)
			}
		}
	}
}

func dockerImagePresent(tag string) bool {
	cmd := exec.Command("docker", "image", "inspect", "--format", "{{.Id}}", tag)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
