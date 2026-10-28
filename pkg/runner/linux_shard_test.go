package runner

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/assert"
)

// linuxArchShard is the test-linux matrix leg that installs QEMU.
// TestRunDifferentArchitecture is pinned to this leg.
const linuxArchShard = 0

// linuxShardTotal is the test-linux matrix width. The workflow sets
// ACT_TEST_LINUX_SHARD to "<index>/<linuxShardTotal>".
const linuxShardTotal = 4

// linuxShardOverride pins slow tests to a leg. Weights are estimates from the
// 2026-10-28 test-linux run (pkg/runner 6m44s) plus known fixture costs
// (pwsh, mysql, qemu, remote clones). Hashing covers anything not listed.
var linuxShardOverride = map[string]int{
	"TestRunDifferentArchitecture":                              linuxArchShard,
	"TestRunEvent/uses-workflow":                                0,
	"TestRunEvent/remote-action-js-node-user":                   0,
	"TestRunEvent/docker-action-host-env":                       0,
	"TestRunEvent/shells/bash":                                  0,
	"TestRunEvent/local-action-via-composite-dockerfile":        0,
	"TestRunEvent/evalmatrixneeds2":                             0,
	"TestRunEvent/matrix":                                       0,
	"TestRunEvent/outputs":                                      0,
	"TestRunEvent/workflow_call_inputs":                         0,
	"TestRunEvent/container-volumes":                            0,
	"TestRunEventPullRequest":                                   0,
	"TestRunEvent/uses-composite":                               0,
	"TestRunEvent/act-composite-env-test":                       0,
	"TestRunEvent/issue-1195":                                   0,
	"TestRunEvent/commands":                                     0,
	"TestRunEvent/issue-597":                                    0,
	"TestRunEvent/environment-files":                            0,
	"TestRunEvent/steps-context/conclusion":                     0,
	"TestRunEvent/evalenv":                                      0,
	"TestRunEvent/workflow_dispatch-scalar":                     0,
	"TestRunEvent/set-env-step-env-override":                    0,
	"TestRunEvent/services-empty-image":                         0,
	"TestRunEventHostEnvironment":                               1,
	"TestRunEvent/checkout":                                     1,
	"TestRunEvent/remote-action-composite-js-pre-with-defaults": 1,
	"TestPullAndPostStepFailureIsJobFailure":                    1,
	"TestRunEvent/remote-action-docker-new-cache":               1,
	"TestRunEvent/local-remote-action-overrides":                1,
	"TestDryrunEvent":                                           1,
	"TestRunEvent/uses-nested-composite":                        1,
	"TestRunEvent/basic":                                        1,
	"TestRunEvent/matrix-include-exclude":                       1,
	"TestRunEvent/uses-action-with-pre-and-post-step":           1,
	"TestRunEvent/workflow_dispatch-scalar-composite-action":    1,
	"TestMaskValues":                                            1,
	"TestRunMatrixWithUserDefinedInclusions":                    1,
	"TestRunEvent/uses-composite-with-error":                    1,
	"TestRunEvent/evalmatrix":                                   1,
	"TestRunEvent/fail":                                         1,
	"TestRunEvent/workdir":                                      1,
	"TestRunEvent/issue-598":                                    1,
	"TestRunEvent/GITHUB_STATE":                                 1,
	"TestRunEvent/steps-context/outcome":                        1,
	"TestRunEvent/GITHUB_ENV-use-in-env-ctx":                    1,
	"TestRunEvent/job-needs-context-contains-result":            1,
	"TestRunEvent/set-env-new-env-file-per-step":                1,
	"TestFetchFailureIsJobFailure":                              1,
	"TestRunEvent/mysql-service-container-with-health-check":    2,
	"TestRunEvent/shells/python":                                2,
	"TestActionCache":                                           2,
	"TestActionCacheFailures":                                   2,
	"TestRunEvent/remote-action-composite-action-ref":           2,
	"TestRunEvent/remote-action-docker":                         2,
	"TestRunEvent/services-with-container":                      2,
	"TestRunEvent/services":                                     2,
	"TestRunEvent/shells/sh":                                    2,
	"TestRunEvent/uses-docker-url":                              2,
	"TestRunEvent/job-container":                                2,
	"TestRunEvent/matrix-exitcode":                              2,
	"TestRunEvent/docker-action-custom-path":                    2,
	"TestRunEvent/uses-workflow-defaults":                       2,
	"TestRunEventSecrets":                                       2,
	"TestRunEvent/local-action-docker-url":                      2,
	"TestRunEvent/uses-composite-check-for-input-collision":     2,
	"TestRunEvent/evalmatrix-merge-map":                         2,
	"TestRunEvent/runs-on":                                      2,
	"TestRunEvent/defaults-run":                                 2,
	"TestRunEvent/if-env-act":                                   2,
	"TestRunEvent/environment-files-parser-bug":                 2,
	"TestRunEvent/job-status-check":                             2,
	"TestRunEvent/workflow_dispatch":                            2,
	"TestRunEvent/path-handling":                                2,
	"TestRunEvent/no-panic-on-invalid-composite-action":         2,
	"TestRunEvent/shells/pwsh":                                  3,
	"TestDockerActionForcePullForceRebuild":                     3,
	"TestRunEvent/job-container-non-root":                       3,
	"TestRunEvent/remote-action-js":                             3,
	"TestRunEvent/uses-workflow#01":                             3,
	"TestRunEvent/services-host-network":                        3,
	"TestRunEvent/actions-environment-and-context-tests":        3,
	"TestRunEvent/shells/defaults":                              3,
	"TestRunEvent/local-action-dockerfile":                      3,
	"TestRunEvent/evalmatrixneeds":                              3,
	"TestRunEvent/container-hostname":                           3,
	"TestRunEvent/non-existent-action":                          3,
	"TestRunEvent/ensure-post-steps":                            3,
	"TestRunEvent/strategy":                                     3,
	"TestRunActionInputs":                                       3,
	"TestRunEvent/local-action-js":                              3,
	"TestRunEvent/uses-composite-check-for-input-shadowing":     3,
	"TestRunEvent/evalmatrix-merge-array":                       3,
	"TestRunEvent/job-container-invalid-credentials":            3,
	"TestRunEvent/composite-fail-with-output":                   3,
	"TestRunEvent/env-and-path":                                 3,
	"TestRunEvent/networking":                                   3,
	"TestRunEvent/if-expressions":                               3,
	"TestRunEvent/workflow_dispatch_no_inputs_mapping":          3,
	"TestRunEvent/do-not-leak-step-env-in-composite":            3,
	"TestRunEvent/stepsummary":                                  3,
}

func shardSkip(t *testing.T) {
	t.Helper()
	idx, n, enabled, err := linuxShardFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		return
	}
	if !linuxShardOwns(t.Name(), idx, n) {
		t.Skipf("assigned to a different test-linux shard (%d/%d)", idx, n)
	}
}

func linuxShardFromEnv() (idx int, n int, enabled bool, err error) {
	v := os.Getenv("ACT_TEST_LINUX_SHARD")
	if v == "" {
		return 0, 0, false, nil
	}
	left, right, ok := strings.Cut(v, "/")
	if !ok {
		return 0, 0, false, fmt.Errorf("ACT_TEST_LINUX_SHARD=%q, want INDEX/TOTAL", v)
	}
	idx, err = strconv.Atoi(left)
	if err != nil {
		return 0, 0, false, fmt.Errorf("ACT_TEST_LINUX_SHARD index: %w", err)
	}
	n, err = strconv.Atoi(right)
	if err != nil {
		return 0, 0, false, fmt.Errorf("ACT_TEST_LINUX_SHARD total: %w", err)
	}
	if n != linuxShardTotal || idx < 0 || idx >= n {
		return 0, 0, false, fmt.Errorf("ACT_TEST_LINUX_SHARD=%q, want 0..%d/%d", v, linuxShardTotal-1, linuxShardTotal)
	}
	return idx, n, true, nil
}

func linuxShardOwns(name string, idx int, n int) bool {
	if shard, ok := linuxShardOverride[name]; ok {
		return shard == idx
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int(h.Sum32()%uint32(n)) == idx
}

func TestLinuxShardOverridesCoverRunEvent(t *testing.T) {
	paths := runEventWorkflowPaths(t)
	for _, path := range paths {
		name := "TestRunEvent/" + path
		_, ok := linuxShardOverride[name]
		assert.Truef(t, ok, "missing shard override for %s", name)
	}
	counts := make([]int, linuxShardTotal)
	for name, shard := range linuxShardOverride {
		assert.GreaterOrEqualf(t, shard, 0, name)
		assert.Lessf(t, shard, linuxShardTotal, name)
		counts[shard]++
	}
	for i, n := range counts {
		assert.GreaterOrEqualf(t, n, 15, "shard %d has too few tests", i)
	}
	assert.Equal(t, linuxArchShard, linuxShardOverride["TestRunDifferentArchitecture"])
}

func TestLinuxShardHashAssignsEachTestOnce(t *testing.T) {
	const name = "TestGraphEvent"
	owned := 0
	for i := 0; i < linuxShardTotal; i++ {
		if linuxShardOwns(name, i, linuxShardTotal) {
			owned++
		}
	}
	assert.Equal(t, 1, owned)
}

func TestLinuxShardFromEnv(t *testing.T) {
	// CI sets ACT_TEST_LINUX_SHARD on the job, so the disabled case must clear it.
	t.Setenv("ACT_TEST_LINUX_SHARD", "")
	idx, n, enabled, err := linuxShardFromEnv()
	assert.NoError(t, err)
	assert.False(t, enabled)
	assert.Equal(t, 0, idx)
	assert.Equal(t, 0, n)

	t.Setenv("ACT_TEST_LINUX_SHARD", "2/4")
	idx, n, enabled, err = linuxShardFromEnv()
	assert.NoError(t, err)
	assert.True(t, enabled)
	assert.Equal(t, 2, idx)
	assert.Equal(t, 4, n)

	t.Setenv("ACT_TEST_LINUX_SHARD", "4/4")
	_, _, _, err = linuxShardFromEnv()
	assert.Error(t, err)
}

func runEventWorkflowPaths(t *testing.T) []string {
	t.Helper()
	f, err := os.Open("runner_test.go")
	assert.NoError(t, err)
	defer f.Close()

	var paths []string
	inRunEvent := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "func TestRunEvent(") {
			inRunEvent = true
			continue
		}
		if inRunEvent && strings.HasPrefix(line, "func ") {
			break
		}
		if !inRunEvent {
			continue
		}
		code, _, _ := strings.Cut(line, "//")
		if path, ok := quotedArg(code, "{workdir, \""); ok {
			paths = append(paths, path)
			continue
		}
		if path, ok := quotedArg(code, "{\"../model/testdata\", \""); ok {
			paths = append(paths, path)
		}
	}
	assert.NoError(t, scanner.Err())
	assert.NotEmpty(t, paths)
	return paths
}

func quotedArg(line string, prefix string) (string, bool) {
	i := strings.Index(line, prefix)
	if i < 0 {
		return "", false
	}
	rest := line[i+len(prefix):]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}
