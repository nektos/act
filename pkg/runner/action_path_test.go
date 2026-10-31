package runner

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRemoteActionRejectsEscapingPaths(t *testing.T) {
	for _, uses := range []string{
		"org/repo/../../..@v1",
		"org/repo/sub/../../x@v1",
		"org/repo//etc@v1",
		"../repo@v1",
		"org/..@v1",
		"./repo@v1",
		`org/repo\x@v1`,
	} {
		assert.Nil(t, newRemoteAction(uses), uses)
	}

	for uses, want := range map[string]string{
		"org/repo@v1":                 "",
		"org/repo/sub@v1":             "sub",
		"org/repo/sub/dir@v1":         "sub/dir",
		"org/repo/sub/../other@v1":    "sub/../other",
		"actions/checkout@abc1234567": "",
	} {
		ra := newRemoteAction(uses)
		if assert.NotNil(t, ra, uses) {
			assert.Equal(t, want, ra.Path, uses)
		}
	}
}

func TestValidateRemoteActionPaths(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		runs    model.ActionRuns
		wantErr bool
	}{
		{name: "main", runs: model.ActionRuns{Main: "dist/index.js"}},
		{name: "main parent inside repo", path: "sub", runs: model.ActionRuns{Main: "../dist/index.js"}},
		{name: "main escapes", runs: model.ActionRuns{Main: "../../../tmp/x.js"}, wantErr: true},
		{name: "main escapes from subpath", path: "sub", runs: model.ActionRuns{Main: "../../x.js"}, wantErr: true},
		{name: "main absolute", runs: model.ActionRuns{Main: "/tmp/x.js"}, wantErr: false},
		{name: "pre escapes", runs: model.ActionRuns{Main: "a.js", Pre: "../pre.js"}, wantErr: true},
		{name: "post escapes", runs: model.ActionRuns{Main: "a.js", Post: "../post.js"}, wantErr: true},
		{name: "image dockerfile", runs: model.ActionRuns{Image: "Dockerfile"}},
		{name: "image registry", runs: model.ActionRuns{Image: "docker://alpine:3"}},
		{name: "image escapes", runs: model.ActionRuns{Image: "../../../../home/user/Dockerfile"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRemoteActionPaths(&model.Action{Runs: tt.runs}, tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSymlinkJoinStaysInsideRepository(t *testing.T) {
	got, err := symlinkJoin("sub/action.yml", "other.yml", ".")
	assert.NoError(t, err)
	assert.Equal(t, "sub/other.yml", got)

	got, err = symlinkJoin("sub/action.yml", "../action.yml", ".")
	assert.NoError(t, err)
	assert.Equal(t, "action.yml", got)

	_, err = symlinkJoin("sub/action.yml", "../../action.yml", ".")
	assert.Error(t, err)

	_, err = symlinkJoin("/var/run/act/actions/x/action.yml", "../../../../etc/passwd", "/var/run/act/actions/x")
	assert.Error(t, err)
}

func TestLocalRepositoryCacheRejectsEscapingPrefix(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	assert.NoError(t, os.Mkdir(repo, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "secret"), []byte("x"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(repo, "action.yml"), []byte("runs: {using: node20, main: a.js}"), 0o600))

	c := &LocalRepositoryCache{CacheDirCache: map[string]string{"org/repo@sha": repo}}
	ctx := context.Background()
	_, err := c.GetTarArchive(ctx, "org/repo", "sha", "../secret")
	assert.Error(t, err)

	r, err := c.GetTarArchive(ctx, "org/repo", "sha", "action.yml")
	assert.NoError(t, err)
	r.Close()
}

func TestLocalTrampolineStaysInWorkspace(t *testing.T) {
	workdir := t.TempDir()
	outside := t.TempDir()
	rel, err := filepath.Rel(workdir, outside)
	require.NoError(t, err)
	_, err = resolveLocalActionDir(workdir, "./"+filepath.ToSlash(rel))
	assert.Error(t, err)

	require.NoError(t, os.Symlink(outside, filepath.Join(workdir, "link")))
	actionDir, err := resolveLocalActionDir(workdir, "./link")
	require.NoError(t, err)
	step := &model.Step{With: map[string]string{"args": "true"}}
	missing := func(string) (io.Reader, io.Closer, error) {
		return nil, nil, fs.ErrNotExist
	}
	_, err = readActionImpl(context.Background(), step, actionDir, "", missing, rootedWriter(workdir))
	assert.Error(t, err)
	_, statErr := os.Stat(filepath.Join(outside, "trampoline.js"))
	assert.ErrorIs(t, statErr, fs.ErrNotExist)

	action := filepath.Join(workdir, "action")
	require.NoError(t, os.Mkdir(action, 0o755))
	actionDir, err = resolveLocalActionDir(workdir, "./action")
	require.NoError(t, err)
	_, err = readActionImpl(context.Background(), step, actionDir, "", missing, rootedWriter(workdir))
	require.NoError(t, err)
	body, err := os.ReadFile(filepath.Join(action, "trampoline.js"))
	require.NoError(t, err)
	assert.Contains(t, string(body), "spawnSync")

	called := false
	_, err = readActionImpl(context.Background(), step, workdir, "../../tmp", missing, func(string, []byte, fs.FileMode) error {
		called = true
		return nil
	})
	assert.Error(t, err)
	assert.False(t, called)
}

func TestResolveDockerContextContained(t *testing.T) {
	_, _, err := resolveDockerContext("", "../../../../tmp/secret/Dockerfile", false)
	assert.Error(t, err)
	_, _, err = resolveDockerContext("/work/action", "../../../../../../tmp/secret/Dockerfile", true)
	assert.Error(t, err)

	dir, file, err := resolveDockerContext("", "/tmp/secret/Dockerfile", false)
	require.NoError(t, err)
	assert.Equal(t, "Dockerfile", file)
	assert.Equal(t, "tmp/secret/", dir)
	checkout := t.TempDir()
	host, err := joinUnder(checkout, strings.TrimSuffix(dir, "/"))
	require.NoError(t, err)
	rel, err := filepath.Rel(checkout, host)
	require.NoError(t, err)
	assert.True(t, filepath.IsLocal(rel), rel)

	dir, file, err = resolveDockerContext("/work/action", "Dockerfile", true)
	require.NoError(t, err)
	assert.Equal(t, "Dockerfile", file)
	assert.Equal(t, "/work/action", dir)
}

func TestDockerContextSymlinkStaysInCheckout(t *testing.T) {
	checkout := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(outside, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "sub", "SECRET.txt"), []byte("host-secret"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(checkout, "escape")))

	// Lexical validation still allows this path. The host context join follows
	// the symlink and can read the marker. The containment check must refuse.
	err := validateRemoteActionPaths(&model.Action{Runs: model.ActionRuns{Image: "escape/sub/Dockerfile"}}, "")
	assert.NoError(t, err)
	dir, file, err := resolveDockerContext("", "escape/sub/Dockerfile", false)
	require.NoError(t, err)
	assert.Equal(t, "Dockerfile", file)
	assert.Equal(t, "escape/sub/", dir)
	host, err := joinUnder(checkout, strings.TrimSuffix(dir, "/"))
	require.NoError(t, err)
	body, err := os.ReadFile(filepath.Join(host, "SECRET.txt"))
	require.NoError(t, err)
	assert.Equal(t, "host-secret", string(body))
	assert.Error(t, rejectHostContextEscape(checkout, host))

	realDir := filepath.Join(checkout, "real")
	require.NoError(t, os.Mkdir(realDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "Dockerfile"), []byte("FROM scratch\n"), 0o600))
	require.NoError(t, os.Symlink("real", filepath.Join(checkout, "link")))
	assert.NoError(t, rejectHostContextEscape(checkout, filepath.Join(checkout, "link")))
}

func TestLocalDockerContextRoot(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "SECRET.txt"), []byte("host-secret"), 0o600))

	t.Run("host executor", func(t *testing.T) {
		workdir := t.TempDir()
		copied := t.TempDir()
		action := filepath.Join(copied, "docker-action-host-env", "action")
		require.NoError(t, os.MkdirAll(action, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(action, "Dockerfile"), []byte("FROM scratch\n"), 0o600))
		require.NoError(t, os.Symlink(outside, filepath.Join(copied, "escape")))

		rc := &RunContext{
			Config: &Config{Workdir: workdir},
			JobContainer: &container.HostEnvironment{
				Path:    copied,
				Workdir: workdir,
			},
		}
		root := localActionWorkspace(rc)
		assert.Equal(t, copied, root)

		// The action lives in the copied workspace, not the original workdir.
		assert.NoError(t, rejectHostContextEscape(root, action))
		assert.Error(t, rejectHostContextEscape(workdir, action))

		// A plain read follows the symlink. Containment must still reject it,
		// and a ../ image path must still fail lexically.
		body, err := os.ReadFile(filepath.Join(copied, "escape", "SECRET.txt"))
		require.NoError(t, err)
		assert.Equal(t, "host-secret", string(body))
		assert.Error(t, rejectHostContextEscape(root, filepath.Join(copied, "escape")))
		_, _, err = resolveDockerContext(action, "../../../../../../tmp/secret/Dockerfile", true)
		assert.Error(t, err)
		_, err = resolveLocalActionDir(workdir, "./../../..")
		assert.Error(t, err)
	})

	t.Run("container executor", func(t *testing.T) {
		workdir := t.TempDir()
		action := filepath.Join(workdir, "action")
		require.NoError(t, os.Mkdir(action, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(action, "Dockerfile"), []byte("FROM scratch\n"), 0o600))
		require.NoError(t, os.Symlink(outside, filepath.Join(workdir, "escape")))

		rc := &RunContext{
			Config:       &Config{Workdir: workdir},
			JobContainer: &containerMock{},
		}
		root := localActionWorkspace(rc)
		assert.Equal(t, workdir, root)
		assert.NoError(t, rejectHostContextEscape(root, action))
		body, err := os.ReadFile(filepath.Join(workdir, "escape", "SECRET.txt"))
		require.NoError(t, err)
		assert.Equal(t, "host-secret", string(body))
		assert.Error(t, rejectHostContextEscape(root, filepath.Join(workdir, "escape")))
		_, _, err = resolveDockerContext(action, "../../../../../../tmp/secret/Dockerfile", true)
		assert.Error(t, err)
		_, err = resolveLocalActionDir(workdir, "./../../..")
		assert.Error(t, err)
	})
}

func TestRestrictWorkflowContainerOptions(t *testing.T) {
	for _, tt := range []struct {
		policy, socket string
		want, wantErr  bool
	}{
		{policy: "", socket: "", want: false},
		{policy: "auto", socket: "/var/run/docker.sock", want: false},
		{policy: "auto", socket: "-", want: true},
		{policy: "", socket: "-", want: true},
		{policy: "restrict", socket: "/var/run/docker.sock", want: true},
		{policy: "allow", socket: "-", want: false},
		{policy: "bogus", wantErr: true},
	} {
		rc := &RunContext{Config: &Config{ContainerOptionsPolicy: tt.policy, ContainerDaemonSocket: tt.socket}}
		got, err := rc.restrictWorkflowContainerOptions()
		if tt.wantErr {
			assert.Error(t, err)
			continue
		}
		assert.NoError(t, err)
		assert.Equal(t, tt.want, got, "%s/%s", tt.policy, tt.socket)
	}
}

func TestCheckWorkflowVolumes(t *testing.T) {
	assert.NoError(t, checkWorkflowVolumes("container.volumes", []string{"data:/data", "/anonymous", "my_docker_volume:/volume_mount"}))
	assert.Error(t, checkWorkflowVolumes("container.volumes", []string{"/:/host"}))
	assert.Error(t, checkWorkflowVolumes("services.db.volumes", []string{"/var/run/docker.sock:/var/run/docker.sock"}))
}

func TestCheckWorkflowContainerVolumes(t *testing.T) {
	load := func(t *testing.T, yaml string, policy string) *RunContext {
		t.Helper()
		wf, err := model.ReadWorkflow(strings.NewReader(yaml), false)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		rc := &RunContext{
			Run:    &model.Run{JobID: "test", Workflow: wf},
			Config: &Config{ContainerOptionsPolicy: policy, ContainerDaemonSocket: "-"},
		}
		rc.ExprEval = rc.NewExpressionEvaluator(context.Background())
		return rc
	}
	const services = `
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      db:
        image: postgres
        volumes: ["/etc:/host-etc"]
    steps: [{run: "true"}]
`
	const jobContainer = `
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    container:
      image: alpine
      volumes: ["data:/data", "/:/host"]
    steps: [{run: "true"}]
`
	const safe = `
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    container:
      image: alpine
      volumes: ["data:/data"]
    services:
      db:
        image: postgres
        volumes: ["pgdata:/var/lib/postgresql/data"]
    steps: [{run: "true"}]
`
	_, _, err := load(t, services, "auto").checkWorkflowContainerVolumes(context.Background())
	assert.ErrorContains(t, err, "services.db.volumes")
	_, _, err = load(t, jobContainer, "auto").checkWorkflowContainerVolumes(context.Background())
	assert.ErrorContains(t, err, "container.volumes")

	restrict, restrictJob, err := load(t, safe, "auto").checkWorkflowContainerVolumes(context.Background())
	assert.NoError(t, err)
	assert.True(t, restrict)
	assert.True(t, restrictJob)

	restrict, _, err = load(t, jobContainer, "allow").checkWorkflowContainerVolumes(context.Background())
	assert.NoError(t, err)
	assert.False(t, restrict)
}
