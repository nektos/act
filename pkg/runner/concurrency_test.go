package runner

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/nektos/act/pkg/exprparser"
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
		if assert.NoError(t, err) && releaseSecond != nil {
			releaseSecond()
		}
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
		g := manager.groups["group"]
		return g != nil && len(g.pending) > 0
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
		assert.ErrorIs(t, err, errSupersededByConcurrencyGroup)
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

func TestConcurrencyManagerCaseInsensitiveGroups(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()

	// GitHub treats concurrency group names case insensitively, so 'prod'
	// and 'Prod' are the same group
	lower, _, err := evaluateConcurrency(ctx, testEvaluator{}, &model.Concurrency{Group: "prod"})
	require.NoError(t, err)
	upper, _, err := evaluateConcurrency(ctx, testEvaluator{}, &model.Concurrency{Group: "Prod"})
	require.NoError(t, err)
	assert.Equal(t, concurrencyGroupKey(lower.group), concurrencyGroupKey(upper.group))
	assert.Equal(t, "Prod", upper.group, "the original spelling is kept for log messages")

	releaseFirst, err := manager.acquire(ctx, nil, lower, func() {})
	require.NoError(t, err)

	acquired := make(chan struct{})
	go func() {
		defer close(acquired)
		release, err := manager.acquire(ctx, nil, upper, func() {})
		if assert.NoError(t, err) && release != nil {
			release()
		}
	}()

	select {
	case <-acquired:
		t.Fatal("'Prod' must wait for 'prod', they are the same concurrency group")
	case <-time.After(100 * time.Millisecond):
	}

	releaseFirst()

	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("'Prod' must acquire the group after 'prod' released it")
	}
}

func TestConcurrencyManagerQueueMaxIsFIFO(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()
	spec := concurrencySpec{group: "group", queueMax: true}

	releaseFirst, err := manager.acquire(ctx, nil, spec, func() {})
	require.NoError(t, err)

	// queue three requests in a known order and record the order in which
	// they acquire the group
	var mu sync.Mutex
	order := []int{}
	done := make(chan struct{}, 3)
	releases := make(chan func(), 3)
	for i := 1; i <= 3; i++ {
		queuePending(ctx, t, manager, spec, i, &mu, &order, done, releases)
	}

	releaseFirst()
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("all queued requests must eventually acquire the group")
		}
		(<-releases)()
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{1, 2, 3}, order, "queued runs must be promoted first-in-first-out")
}

// queuePending starts a request that waits for the group and records its id
// once it acquires it, returning after the request is registered as pending
func queuePending(ctx context.Context, t *testing.T, manager *concurrencyManager, spec concurrencySpec, id int, mu *sync.Mutex, order *[]int, done chan struct{}, releases chan func()) {
	t.Helper()
	expected := 0
	manager.mu.Lock()
	if group := manager.groups[concurrencyGroupKey(spec.group)]; group != nil {
		expected = len(group.pending) + 1
	}
	manager.mu.Unlock()

	go func() {
		release, err := manager.acquire(ctx, nil, spec, func() {})
		if !assert.NoError(t, err) || release == nil {
			done <- struct{}{}
			return
		}
		mu.Lock()
		*order = append(*order, id)
		mu.Unlock()
		releases <- release
		done <- struct{}{}
	}()

	require.Eventually(t, func() bool {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		group := manager.groups[concurrencyGroupKey(spec.group)]
		return group != nil && len(group.pending) == expected
	}, 5*time.Second, 5*time.Millisecond, "request %d should be pending", id)
}

func TestConcurrencyManagerQueueMaxCancelsOverflow(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()
	spec := concurrencySpec{group: "group", queueMax: true}

	release, err := manager.acquire(ctx, nil, spec, func() {})
	require.NoError(t, err)
	defer release()

	key := concurrencyGroupKey(spec.group)
	pendingCount := func() int {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		if group := manager.groups[key]; group != nil {
			return len(group.pending)
		}
		return 0
	}

	// queue every request through acquire, so the cap is exercised on the
	// real path rather than against hand-built internal state
	results := make(chan error, maxPendingRuns)
	for i := 0; i < maxPendingRuns; i++ {
		go func() {
			release, err := manager.acquire(ctx, nil, spec, func() {})
			if release != nil {
				release()
			}
			results <- err
		}()
		// queue them one at a time so the boundary is approached exactly and
		// the FIFO order stays deterministic
		want := i + 1
		require.Eventually(t, func() bool { return pendingCount() == want }, 5*time.Second, 5*time.Millisecond,
			"request %d must be accepted into the queue, not cancelled", want)
	}

	// the queue now holds exactly the documented limit, which pins the
	// boundary to maxPendingRuns rather than to any smaller cap
	require.Equal(t, maxPendingRuns, pendingCount())

	// the next request arrives at a full queue and is cancelled, the queued
	// ones keep their place
	_, err = manager.acquire(ctx, nil, spec, func() {})
	assert.ErrorIs(t, err, errSupersededByConcurrencyGroup)
	assert.Equal(t, maxPendingRuns, pendingCount(), "a full queue must not grow beyond the limit")

	// drain: releasing the holder lets the whole queue through
	release()
	for i := 0; i < maxPendingRuns; i++ {
		select {
		case err := <-results:
			assert.NoError(t, err, "every queued request must eventually acquire the group")
		case <-time.After(30 * time.Second):
			t.Fatal("the queue must drain once the holder releases the group")
		}
	}
}

func TestConcurrencyManagerAbandonThroughAcquireReleasesGroup(t *testing.T) {
	// a waiter can be promoted at the very moment its own cancellation
	// fires; acquire's select may then take the cancelled branch while the
	// waiter already holds the group. Drive that race through acquire
	// repeatedly: if the group is left held, the follow-up acquire blocks.
	for i := 0; i < 100; i++ {
		manager := newConcurrencyManager()
		spec := concurrencySpec{group: "group"}
		key := concurrencyGroupKey(spec.group)

		releaseFirst, err := manager.acquire(context.Background(), nil, spec, func() {})
		require.NoError(t, err)

		cancelCtx, cancel := context.WithCancel(context.Background())
		waiterDone := make(chan func(), 1)
		go func() {
			release, _ := manager.acquire(context.Background(), cancelCtx, spec, func() {})
			waiterDone <- release
		}()
		require.Eventually(t, func() bool {
			manager.mu.Lock()
			defer manager.mu.Unlock()
			group := manager.groups[key]
			return group != nil && len(group.pending) == 1
		}, 5*time.Second, time.Millisecond)

		// promote and cancel concurrently so both select branches are live
		go cancel()
		releaseFirst()

		if release := <-waiterDone; release != nil {
			release()
		}

		// whichever branch won, the group must not stay held
		acquired := make(chan struct{})
		go func() {
			defer close(acquired)
			release, err := manager.acquire(context.Background(), nil, spec, func() {})
			if assert.NoError(t, err) && release != nil {
				release()
			}
		}()
		select {
		case <-acquired:
		case <-time.After(5 * time.Second):
			t.Fatalf("iteration %d: the group stayed held by a waiter that gave up", i)
		}
		cancel()
	}
}

func TestConcurrencyManagerRemovesPendingFromMiddleOfQueue(t *testing.T) {
	manager := newConcurrencyManager()
	spec := concurrencySpec{group: "group", queueMax: true}
	key := concurrencyGroupKey(spec.group)

	releaseFirst, err := manager.acquire(context.Background(), nil, spec, func() {})
	require.NoError(t, err)

	var mu sync.Mutex
	order := []int{}
	done := make(chan struct{}, 3)
	cancels := make([]context.CancelFunc, 3)
	for i := 0; i < 3; i++ {
		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cancels[i] = cancel
		id := i + 1
		go func() {
			release, err := manager.acquire(context.Background(), cancelCtx, spec, func() {})
			if err == nil {
				mu.Lock()
				order = append(order, id)
				mu.Unlock()
				release()
			}
			done <- struct{}{}
		}()
		want := i + 1
		require.Eventually(t, func() bool {
			manager.mu.Lock()
			defer manager.mu.Unlock()
			return len(manager.groups[key].pending) == want
		}, 5*time.Second, time.Millisecond)
	}

	// cancel the middle waiter, which is spliced out from between its
	// neighbours
	cancels[1]()
	require.Eventually(t, func() bool {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		return len(manager.groups[key].pending) == 2
	}, 5*time.Second, time.Millisecond)

	releaseFirst()
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("removing a waiter from the middle must not strand its neighbours")
		}
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []int{1, 3}, order, "the neighbours of a cancelled waiter keep their order")
}

func TestConcurrencyManagerSingleGroupStaysSingle(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()
	singleSpec := concurrencySpec{group: "group"}
	maxSpec := concurrencySpec{group: "group", queueMax: true}
	key := concurrencyGroupKey(singleSpec.group)

	// the group is created by a request using the default queue: single
	release, err := manager.acquire(ctx, nil, singleSpec, func() {})
	require.NoError(t, err)
	defer release()

	superseded := make(chan error, 2)
	go func() {
		_, err := manager.acquire(ctx, nil, singleSpec, func() {})
		superseded <- err
	}()
	require.Eventually(t, func() bool {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		return len(manager.groups[key].pending) == 1
	}, 5*time.Second, time.Millisecond)

	// a later request asking for `queue: max` must not bypass the supersede
	// the single-queue group guarantees, otherwise a stale run would still
	// execute alongside the newest one
	go func() {
		r, err := manager.acquire(ctx, nil, maxSpec, func() {})
		if r != nil {
			r()
		}
		superseded <- err
	}()

	select {
	case err := <-superseded:
		assert.ErrorIs(t, err, errSupersededByConcurrencyGroup, "the stale pending run must be superseded")
	case <-time.After(5 * time.Second):
		t.Fatal("a 'queue: max' arrival must still supersede the pending run of a single-queue group")
	}

	manager.mu.Lock()
	assert.Equal(t, 1, len(manager.groups[key].pending), "a single-queue group keeps at most one pending request")
	manager.mu.Unlock()
}

func TestEvaluateConcurrencyQueueValidation(t *testing.T) {
	ctx := context.Background()

	spec, ok, err := evaluateConcurrency(ctx, testEvaluator{}, &model.Concurrency{Group: "g", Queue: "max"})
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, spec.queueMax)

	spec, ok, err = evaluateConcurrency(ctx, testEvaluator{}, &model.Concurrency{Group: "g", Queue: "single"})
	require.NoError(t, err)
	require.True(t, ok)
	assert.False(t, spec.queueMax)

	// GitHub rejects this combination with a workflow validation error
	_, _, err = evaluateConcurrency(ctx, testEvaluator{}, &model.Concurrency{Group: "g", Queue: "max", CancelInProgress: "true"})
	assert.ErrorContains(t, err, "'queue: max' and 'cancel-in-progress: true' is not allowed")

	_, _, err = evaluateConcurrency(ctx, testEvaluator{}, &model.Concurrency{Group: "g", Queue: "bogus"})
	assert.ErrorContains(t, err, "invalid value 'bogus' for 'concurrency.queue'")

	// queue: max with cancel-in-progress explicitly false is allowed
	_, _, err = evaluateConcurrency(ctx, testEvaluator{}, &model.Concurrency{Group: "g", Queue: "max", CancelInProgress: "false"})
	assert.NoError(t, err)
}

// testEvaluator interpolates the expressions in `values`, mimicking the real
// evaluator which yields an empty string for expressions it cannot resolve
type testEvaluator struct {
	values map[string]string
}

func (testEvaluator) evaluate(context.Context, string, exprparser.DefaultStatusCheck) (interface{}, error) {
	return nil, nil
}
func (testEvaluator) EvaluateYamlNode(context.Context, *yaml.Node) error { return nil }
func (e testEvaluator) Interpolate(_ context.Context, s string) string {
	if !strings.Contains(s, "${{") {
		return s
	}
	return e.values[s]
}

func TestEvaluateConcurrencyExpressionValues(t *testing.T) {
	ctx := context.Background()
	const queueExpr = "${{ vars.QUEUE_MODE }}"

	ee := testEvaluator{values: map[string]string{queueExpr: "max"}}
	spec, ok, err := evaluateConcurrency(ctx, ee, &model.Concurrency{Group: "g", Queue: queueExpr})
	require.NoError(t, err)
	require.True(t, ok)
	assert.True(t, spec.queueMax, "an expression evaluating to 'max' must enable the bounded queue")

	// an expression the evaluator cannot resolve yields "", which is the
	// documented default rather than an error
	ee = testEvaluator{values: map[string]string{}}
	spec, ok, err = evaluateConcurrency(ctx, ee, &model.Concurrency{Group: "g", Queue: queueExpr})
	require.NoError(t, err)
	require.True(t, ok)
	assert.False(t, spec.queueMax, "an unresolved queue expression falls back to the default 'single'")

	// an expression resolving to something else is still rejected
	ee = testEvaluator{values: map[string]string{queueExpr: "bogus"}}
	_, _, err = evaluateConcurrency(ctx, ee, &model.Concurrency{Group: "g", Queue: queueExpr})
	assert.ErrorContains(t, err, "invalid value 'bogus' for 'concurrency.queue'")

	// the invalid combination is rejected even when it only becomes visible
	// after evaluation
	ee = testEvaluator{values: map[string]string{queueExpr: "max", "${{ true }}": "true"}}
	_, _, err = evaluateConcurrency(ctx, ee, &model.Concurrency{Group: "g", Queue: queueExpr, CancelInProgress: "${{ true }}"})
	assert.ErrorContains(t, err, "'queue: max' and 'cancel-in-progress: true' is not allowed")
}

func TestEvaluateConcurrencyValidatesEmptyGroup(t *testing.T) {
	ctx := context.Background()
	ee := testEvaluator{values: map[string]string{"${{ inputs.env }}": ""}}

	// an invalid concurrency block is rejected even when its group
	// interpolates to an empty string, which GitHub refuses to run
	_, _, err := evaluateConcurrency(ctx, ee, &model.Concurrency{
		Group:            "${{ inputs.env }}",
		Queue:            "max",
		CancelInProgress: "true",
	})
	assert.ErrorContains(t, err, "'queue: max' and 'cancel-in-progress: true' is not allowed")

	_, _, err = evaluateConcurrency(ctx, ee, &model.Concurrency{Group: "${{ inputs.env }}", Queue: "bogus"})
	assert.ErrorContains(t, err, "invalid value 'bogus' for 'concurrency.queue'")

	// a valid block with an empty group is still just ignored
	_, ok, err := evaluateConcurrency(ctx, ee, &model.Concurrency{Group: "${{ inputs.env }}", Queue: "max"})
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestConcurrencyManagerGroupPolicyWins(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()
	maxSpec := concurrencySpec{group: "group", queueMax: true}
	singleSpec := concurrencySpec{group: "group"}
	key := concurrencyGroupKey(maxSpec.group)

	release, err := manager.acquire(ctx, nil, maxSpec, func() {})
	require.NoError(t, err)
	defer release()

	// three runs queue under the group's `queue: max` policy
	results := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func() {
			r, err := manager.acquire(ctx, nil, maxSpec, func() {})
			if r != nil {
				r()
			}
			results <- err
		}()
		want := i + 1
		require.Eventually(t, func() bool {
			manager.mu.Lock()
			defer manager.mu.Unlock()
			return len(manager.groups[key].pending) == want
		}, 5*time.Second, time.Millisecond)
	}

	// a request that omits `queue: max` must not discard the queue the group
	// admitted under it
	go func() {
		r, err := manager.acquire(ctx, nil, singleSpec, func() {})
		if r != nil {
			r()
		}
		results <- err
	}()
	require.Eventually(t, func() bool {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		return len(manager.groups[key].pending) == 4
	}, 5*time.Second, time.Millisecond)

	select {
	case err := <-results:
		t.Fatalf("no queued run may be cancelled by an arrival that omits 'queue: max', got %v", err)
	case <-time.After(100 * time.Millisecond):
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
		g := manager.groups["group"]
		return g != nil && len(g.pending) > 0
	}, 5*time.Second, 10*time.Millisecond)

	cancel()

	select {
	case err := <-waited:
		assert.ErrorIs(t, err, errConcurrencyWaitCancelled)
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled waiter must stop waiting for the group")
	}

	// the aborted waiter is no longer pending
	require.Eventually(t, func() bool {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		g := manager.groups["group"]
		return g == nil || len(g.pending) == 0
	}, 5*time.Second, 10*time.Millisecond)
}

func TestConcurrencyManagerAbandonedHolderReleasesGroup(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()
	spec := concurrencySpec{group: "group"}

	releaseFirst, err := manager.acquire(ctx, nil, spec, func() {})
	require.NoError(t, err)

	// a waiter can be promoted at the very moment it gives up waiting; its
	// select may then take the cancelled branch even though it now holds the
	// group. Reproduce that ordering deterministically: queue a waiter,
	// promote it by releasing, then abandon it.
	key := concurrencyGroupKey(spec.group)
	waiter := &concurrencyWaiter{promoted: make(chan struct{}), superseded: make(chan struct{})}
	manager.mu.Lock()
	manager.groups[key].pending = append(manager.groups[key].pending, waiter)
	manager.mu.Unlock()

	releaseFirst()

	manager.mu.Lock()
	require.Equal(t, waiter, manager.groups[key].holder, "the queued waiter must have been promoted")
	manager.mu.Unlock()

	manager.abandon(key, waiter)

	// the group must not stay held by the request that gave up
	acquired := make(chan struct{})
	go func() {
		defer close(acquired)
		release, err := manager.acquire(ctx, nil, spec, func() {})
		assert.NoError(t, err)
		release()
	}()

	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("a promoted waiter that gave up must release the group instead of holding it forever")
	}
}

func TestConcurrencyManagerAbandonedHolderPromotesNextWaiter(t *testing.T) {
	manager := newConcurrencyManager()
	ctx := context.Background()
	spec := concurrencySpec{group: "group", queueMax: true}

	releaseFirst, err := manager.acquire(ctx, nil, spec, func() {})
	require.NoError(t, err)

	key := concurrencyGroupKey(spec.group)
	abandoned := &concurrencyWaiter{promoted: make(chan struct{}), superseded: make(chan struct{})}
	next := &concurrencyWaiter{promoted: make(chan struct{}), superseded: make(chan struct{})}
	manager.mu.Lock()
	manager.groups[key].pending = append(manager.groups[key].pending, abandoned, next)
	manager.mu.Unlock()

	releaseFirst()
	manager.abandon(key, abandoned)

	// the queue keeps moving: the request behind the abandoned one is promoted
	select {
	case <-next.promoted:
	case <-time.After(5 * time.Second):
		t.Fatal("the next queued request must be promoted when a promoted waiter gives up")
	}
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

func TestRunConcurrencyQueueMaxRunsEveryQueuedRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	logs := captureRunnerLogs(t)
	markerDir := filepath.ToSlash(t.TempDir())
	plan := runConcurrencyWorkflow(t, concurrencyTestWorkflow("concurrency-queue-max"), markerDir)

	// with `queue: max` no run is superseded, all of them queue up and run
	// one after another. The third workflow spells the group in upper case,
	// which GitHub treats as the same group.
	ran := make([]jobInterval, 0, 3)
	for _, stage := range plan.Stages {
		for _, run := range stage.Runs {
			assert.Equal(t, "success", run.Job().Result, "job %s should have succeeded", run.String())
			ran = append(ran, markerInterval(t, markerDir, run.Workflow.Name))
		}
	}
	require.Len(t, ran, 3, "every queued run must execute with queue: max")

	for i := 0; i < len(ran); i++ {
		for j := i + 1; j < len(ran); j++ {
			assertIntervalsDisjoint(t, ran[i], ran[j])
		}
	}

	// the group must actually have been contended, otherwise three runs that
	// happened to arrive one after another would satisfy the assertions
	// above even if `queue: max` were ignored entirely. The runner logs when
	// a run queues, which proves contention without depending on timing.
	assert.Contains(t, logs.String(), "Waiting for concurrency group",
		"the runs must have queued on the group rather than arriving one after another")
}

// threadSafeBuffer collects log output written from the goroutines of
// concurrently executing workflow runs
type threadSafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *threadSafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *threadSafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureRunnerLogs redirects the standard logger, which the plan executor
// uses for concurrency group messages, into a buffer for the test
func captureRunnerLogs(t *testing.T) *threadSafeBuffer {
	t.Helper()
	buf := &threadSafeBuffer{}
	original := log.StandardLogger().Out
	log.SetOutput(io.MultiWriter(original, buf))
	t.Cleanup(func() { log.SetOutput(original) })
	return buf
}

func TestRunConcurrencyQueueMaxWithCancelInProgressIsRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	log.SetLevel(logLevel)
	absWorkdir, err := filepath.Abs(workdir)
	require.NoError(t, err)

	// GitHub rejects this workflow before anything runs, so act must fail
	// while reading it rather than after earlier jobs produced side effects
	_, err = model.NewWorkflowPlanner(filepath.Join(absWorkdir, "concurrency-queue-invalid"), true, false)
	assert.ErrorContains(t, err, "'queue: max' and 'cancel-in-progress: true' is not allowed")
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
