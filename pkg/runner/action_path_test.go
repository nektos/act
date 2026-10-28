package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
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
