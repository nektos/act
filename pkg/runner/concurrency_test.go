package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nektos/act/pkg/model"
)

func TestConcurrencyManagerSerializes(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()
	spec := concurrencySpec{group: "group"}

	releaseFirst, err := manager.acquire(ctx, nil, spec, func() {})
	require.NoError(t, err)

	acquired := make(chan struct{})
	go func() {
		defer close(acquired)
		releaseSecond, err := manager.acquire(ctx, nil, spec, func() {})
		assert.NoError(t, err)
		releaseSecond()
	}()

	select {
	case <-acquired:
		t.Fatal("the second request must wait until the group is released")
	case <-time.After(100 * time.Millisecond):
	}

	releaseFirst()

	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("the pending request must be promoted when the group is released")
	}
}

func TestConcurrencyManagerSupersedesPending(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()
	spec := concurrencySpec{group: "group"}

	releaseFirst, err := manager.acquire(ctx, nil, spec, func() {})
	require.NoError(t, err)

	superseded := make(chan error, 1)
	go func() {
		_, err := manager.acquire(ctx, nil, spec, func() {})
		superseded <- err
	}()

	// make sure the second request is pending before the third arrives
	require.Eventually(t, func() bool {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		return manager.groups["group"].pending != nil
	}, 5*time.Second, 10*time.Millisecond)

	third := make(chan error, 1)
	releases := make(chan func(), 1)
	go func() {
		release, err := manager.acquire(ctx, nil, spec, func() {})
		if release != nil {
			releases <- release
		}
		third <- err
	}()

	// the second (previously pending) request is cancelled by the third
	select {
	case err := <-superseded:
		assert.ErrorIs(t, err, errCancelledByConcurrencyGroup)
	case <-time.After(5 * time.Second):
		t.Fatal("the pending request must be superseded by a newer request")
	}

	releaseFirst()

	select {
	case err := <-third:
		assert.NoError(t, err, "the newest request must acquire the group")
		(<-releases)()
	case <-time.After(5 * time.Second):
		t.Fatal("the newest request must acquire the group after the release")
	}
}

func TestConcurrencyManagerCancelInProgress(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()

	cancelled := make(chan struct{})
	releaseFirst, err := manager.acquire(ctx, nil, concurrencySpec{group: "group"}, func() { close(cancelled) })
	require.NoError(t, err)

	// the cancelled holder releases the group once it stopped running
	go func() {
		<-cancelled
		releaseFirst()
	}()

	release, err := manager.acquire(ctx, nil, concurrencySpec{group: "group", cancelInProgress: true}, func() {})
	assert.NoError(t, err)
	release()

	select {
	case <-cancelled:
	default:
		t.Fatal("the holder must have been cancelled by cancel-in-progress")
	}
}

func TestConcurrencyManagerCancelCtxAbortsWaiting(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()
	spec := concurrencySpec{group: "group"}

	release, err := manager.acquire(ctx, nil, spec, func() {})
	require.NoError(t, err)
	defer release()

	// the waiting request is cancelled itself (e.g. by cancel-in-progress
	// of the job while it waits) and must stop waiting
	cancelCtx, cancel := context.WithCancel(context.Background())
	waited := make(chan error, 1)
	go func() {
		_, err := manager.acquire(ctx, cancelCtx, spec, func() {})
		waited <- err
	}()

	require.Eventually(t, func() bool {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		return manager.groups["group"].pending != nil
	}, 5*time.Second, 10*time.Millisecond)

	cancel()

	select {
	case err := <-waited:
		assert.ErrorIs(t, err, errCancelledByConcurrencyGroup)
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled waiter must stop waiting for the group")
	}

	// the aborted waiter is no longer pending
	require.Eventually(t, func() bool {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		return manager.groups["group"].pending == nil
	}, 5*time.Second, 10*time.Millisecond)
}

func TestConcurrencyManagerContextCancelledWhileWaiting(t *testing.T) {
	manager := newConcurrencyManager()
	spec := concurrencySpec{group: "group"}

	release, err := manager.acquire(context.Background(), nil, spec, func() {})
	require.NoError(t, err)
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = manager.acquire(ctx, nil, spec, func() {})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func runConcurrencyWorkflow(t *testing.T, workflowPath string, markerDir string) *model.Plan {
	t.Helper()

	log.SetLevel(logLevel)

	workdir, err := filepath.Abs(workdir)
	require.NoError(t, err)

	config := &Config{
		Workdir:        workdir,
		EventName:      "push",
		Platforms:      map[string]string{"self-hosted": "-self-hosted"},
		GitHubInstance: "github.com",
		Env:            map[string]string{"MARKER_DIR": markerDir},
		ConcurrentJobs: 4,
	}
	runner, err := New(config)
	require.NoError(t, err)

	planner, err := model.NewWorkflowPlanner(filepath.Join(workdir, workflowPath), true, false)
	require.NoError(t, err)
	plan, err := planner.PlanEvent("push")
	require.NoError(t, err)

	require.NoError(t, runner.NewPlanExecutor(plan)(context.Background()))
	return plan
}

type jobInterval struct {
	name  string
	start time.Time
	end   time.Time
}

func markerInterval(t *testing.T, dir string, name string) jobInterval {
	t.Helper()
	start, err := os.Stat(filepath.Join(dir, name+"-start"))
	require.NoError(t, err)
	end, err := os.Stat(filepath.Join(dir, name+"-end"))
	require.NoError(t, err)
	return jobInterval{name: name, start: start.ModTime(), end: end.ModTime()}
}

func assertIntervalsDisjoint(t *testing.T, a jobInterval, b jobInterval) {
	t.Helper()
	if !a.end.After(b.start) || !b.end.After(a.start) {
		return
	}
	t.Errorf("jobs '%s' (%v - %v) and '%s' (%v - %v) overlapped although they share a concurrency group",
		a.name, a.start, a.end, b.name, b.start, b.end)
}

func concurrencyTestWorkflow(name string) string {
	if runtime.GOOS == "windows" {
		return name + "-windows"
	}
	return name
}

func TestRunConcurrencyGroupSerializesJobs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	markerDir := filepath.ToSlash(t.TempDir())
	plan := runConcurrencyWorkflow(t, concurrencyTestWorkflow("concurrency-serialize"), markerDir)

	first := markerInterval(t, markerDir, "first")
	second := markerInterval(t, markerDir, "second")
	assertIntervalsDisjoint(t, first, second)

	for _, stage := range plan.Stages {
		for _, run := range stage.Runs {
			assert.Equal(t, "success", run.Job().Result, "job %s should have succeeded", run.String())
		}
	}
}

func TestRunConcurrencyGroupSerializesWorkflows(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	markerDir := filepath.ToSlash(t.TempDir())
	plan := runConcurrencyWorkflow(t, concurrencyTestWorkflow("concurrency-workflow-group"), markerDir)

	// jobs of the same workflow run may run in parallel (the run holds the
	// group, not the individual jobs), but jobs of different workflow runs
	// sharing the group must not overlap
	for _, w1 := range []string{"w1-a", "w1-b"} {
		for _, w2 := range []string{"w2-a", "w2-b"} {
			assertIntervalsDisjoint(t, markerInterval(t, markerDir, w1), markerInterval(t, markerDir, w2))
		}
	}

	for _, stage := range plan.Stages {
		for _, run := range stage.Runs {
			assert.Equal(t, "success", run.Job().Result, "job %s should have succeeded", run.String())
		}
	}
}

func TestRunConcurrencyCancelInProgress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	plan := runConcurrencyWorkflow(t, concurrencyTestWorkflow("concurrency-cancel"), "")

	results := map[string]string{}
	for _, stage := range plan.Stages {
		for _, run := range stage.Runs {
			results[run.JobID] = run.Job().Result
		}
	}
	require.Len(t, results, 2)

	successes := 0
	for jobID, result := range results {
		assert.Contains(t, []string{"success", "cancelled"}, result, "job %s must either succeed or be cancelled", jobID)
		if result == "success" {
			successes++
		}
	}
	assert.GreaterOrEqual(t, successes, 1, "at least one job must have succeeded: %v", results)
}

func TestRunConcurrencySupersedesPending(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	markerDir := filepath.ToSlash(t.TempDir())
	plan := runConcurrencyWorkflow(t, concurrencyTestWorkflow("concurrency-supersede"), markerDir)

	ran := make([]jobInterval, 0, 3)
	cancelled := 0
	for _, stage := range plan.Stages {
		for _, run := range stage.Runs {
			result := run.Job().Result
			assert.Contains(t, []string{"success", "cancelled"}, result, "job %s must either succeed or be cancelled", run.String())
			switch result {
			case "success":
				ran = append(ran, markerInterval(t, markerDir, run.Workflow.Name))
			case "cancelled":
				cancelled++
				assert.NoFileExists(t, filepath.Join(markerDir, run.Workflow.Name+"-start"), "a superseded run must not have executed")
			}
		}
	}

	// GitHub keeps at most one pending run per group: with three runs
	// entering the group at once, at most one is superseded (a run may
	// arrive only after another one already finished, so all three running
	// is possible, but at least two always run)
	assert.GreaterOrEqual(t, len(ran), 2, "at least two runs must have executed")
	assert.LessOrEqual(t, cancelled, 1, "at most one run can be superseded")
	for i := 0; i < len(ran); i++ {
		for j := i + 1; j < len(ran); j++ {
			assertIntervalsDisjoint(t, ran[i], ran[j])
		}
	}
}

func TestRunConcurrencyReusableWorkflowSharedGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// the caller holds the group while the called workflow declares the same
	// group: the called workflow must run instead of deadlocking on its own
	// caller
	type result struct{ plan *model.Plan }
	done := make(chan result, 1)
	go func() {
		done <- result{runConcurrencyWorkflow(t, concurrencyTestWorkflow("concurrency-reusable"), "")}
	}()

	select {
	case res := <-done:
		for _, stage := range res.plan.Stages {
			for _, run := range stage.Runs {
				assert.Equal(t, "success", run.Job().Result, "job %s should have succeeded", run.String())
			}
		}
	case <-time.After(120 * time.Second):
		t.Fatal("the run deadlocked: the called workflow waits for the concurrency group its caller holds")
	}
}
