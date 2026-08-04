package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nektos/act/pkg/model"
)

func TestConcurrencyManagerSerializesDifferentOwners(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()

	var mu sync.Mutex
	active := 0
	maxActive := 0

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := manager.acquire(ctx, func() {}, concurrencySpec{group: "group", owner: manager.newOwner("job")})
			assert.NoError(t, err)
			defer release()

			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			mu.Unlock()

			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			active--
			mu.Unlock()
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, maxActive, "only one job of a concurrency group may run at a time")
}

func TestConcurrencyManagerSharedOwnerDoesNotBlock(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()
	spec := concurrencySpec{group: "group", owner: "workflow-1"}

	releaseFirst, err := manager.acquire(ctx, func() {}, spec)
	require.NoError(t, err)
	defer releaseFirst()

	// jobs of the same workflow run share the workflow level group and must
	// not wait for each other
	done := make(chan struct{})
	go func() {
		defer close(done)
		releaseSecond, err := manager.acquire(ctx, func() {}, spec)
		assert.NoError(t, err)
		releaseSecond()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("job sharing the owner of the group holder should not wait")
	}
}

func TestConcurrencyManagerDifferentGroupsDoNotBlock(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()

	releaseFirst, err := manager.acquire(ctx, func() {}, concurrencySpec{group: "group-1", owner: "job-1"})
	require.NoError(t, err)
	defer releaseFirst()

	done := make(chan struct{})
	go func() {
		defer close(done)
		releaseSecond, err := manager.acquire(ctx, func() {}, concurrencySpec{group: "group-2", owner: "job-2"})
		assert.NoError(t, err)
		releaseSecond()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("jobs in different concurrency groups should not wait for each other")
	}
}

func TestConcurrencyManagerCancelInProgress(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()

	firstOwner := manager.newOwner("workflow")
	cancelled := make(chan struct{})
	releaseFirst, err := manager.acquire(ctx, func() { close(cancelled) }, concurrencySpec{group: "group", owner: firstOwner})
	require.NoError(t, err)

	// the cancelled job releases the group once it stopped running
	go func() {
		<-cancelled
		releaseFirst()
	}()

	release, err := manager.acquire(ctx, func() {}, concurrencySpec{group: "group", owner: manager.newOwner("workflow"), cancelInProgress: true})
	assert.NoError(t, err)
	release()

	select {
	case <-cancelled:
	default:
		t.Fatal("the job holding the group should have been cancelled")
	}

	// remaining jobs of the cancelled owner are cancelled instead of run
	_, err = manager.acquire(ctx, func() {}, concurrencySpec{group: "another-group", owner: firstOwner})
	assert.ErrorIs(t, err, errCancelledByConcurrencyGroup)
}

func TestConcurrencyManagerContextCancelledWhileWaiting(t *testing.T) {
	manager := newConcurrencyManager()

	release, err := manager.acquire(context.Background(), func() {}, concurrencySpec{group: "group", owner: "job-1"})
	require.NoError(t, err)
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = manager.acquire(ctx, func() {}, concurrencySpec{group: "group", owner: "job-2"})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestConcurrencyManagerReleasesAcquiredGroupsOnError(t *testing.T) {
	manager := newConcurrencyManager()

	release, err := manager.acquire(context.Background(), func() {}, concurrencySpec{group: "group-2", owner: "job-1"})
	require.NoError(t, err)
	defer release()

	// requesting group-1 and group-2 acquires group-1 first, then fails on
	// the held group-2 and must release group-1 again
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = manager.acquire(ctx, func() {},
		concurrencySpec{group: "group-1", owner: "job-2"},
		concurrencySpec{group: "group-2", owner: "job-2"})
	require.ErrorIs(t, err, context.DeadlineExceeded)

	done := make(chan struct{})
	go func() {
		defer close(done)
		releaseAgain, err := manager.acquire(context.Background(), func() {}, concurrencySpec{group: "group-1", owner: "job-3"})
		assert.NoError(t, err)
		releaseAgain()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("group-1 should have been released after the failed acquisition")
	}
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

	// jobs of the same workflow run share the workflow level group and may
	// run in parallel, but jobs of different workflow runs must not overlap
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
