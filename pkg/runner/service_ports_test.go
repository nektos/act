package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/exprparser"
	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGithubServicePorts(t *testing.T) {
	tests := []struct {
		name      string
		expected  nat.PortSet
		inspected nat.PortMap
		want      map[string]string
		wantErr   string
	}{
		{
			name:     "dynamic tcp with dual stack bindings",
			expected: nat.PortSet{nat.Port("5432/tcp"): {}},
			inspected: nat.PortMap{
				nat.Port("5432/tcp"): {
					{HostIP: "0.0.0.0", HostPort: "49153"},
					{HostIP: "::", HostPort: "49153"},
				},
			},
			want: map[string]string{"5432": "49153"},
		},
		{
			name: "multiple declared ports",
			expected: nat.PortSet{
				nat.Port("5432/tcp"): {},
				nat.Port("9187/tcp"): {},
			},
			inspected: nat.PortMap{
				nat.Port("5432/tcp"): {{HostPort: "49153"}},
				nat.Port("9187/tcp"): {{HostPort: "49154"}},
			},
			want: map[string]string{"5432": "49153", "9187": "49154"},
		},
		{
			name:     "explicit mapping",
			expected: nat.PortSet{nat.Port("5432/tcp"): {}},
			inspected: nat.PortMap{
				nat.Port("5432/tcp"): {{HostIP: "127.0.0.1", HostPort: "15432"}},
			},
			want: map[string]string{"5432": "15432"},
		},
		{
			name:     "option-published port is included",
			expected: nat.PortSet{nat.Port("5432/tcp"): {}},
			inspected: nat.PortMap{
				nat.Port("5432/tcp"): {{HostPort: "49153"}},
				nat.Port("9187/tcp"): {{HostPort: "49154"}},
			},
			want: map[string]string{"5432": "49153", "9187": "49154"},
		},
		{
			name:     "partial dual stack binding",
			expected: nat.PortSet{nat.Port("5432/tcp"): {}},
			inspected: nat.PortMap{
				nat.Port("5432/tcp"): {{HostIP: "::"}, {HostIP: "0.0.0.0", HostPort: "49153"}},
			},
			want: map[string]string{"5432": "49153"},
		},
		{
			name:      "no declared ports",
			expected:  nat.PortSet{},
			inspected: nat.PortMap{},
			want:      map[string]string{},
		},
		{
			name:     "missing binding",
			expected: nat.PortSet{nat.Port("5432/tcp"): {}},
			inspected: nat.PortMap{
				nat.Port("5432/tcp"): nil,
			},
			wantErr: "has no published binding",
		},
		{
			name:     "invalid binding",
			expected: nat.PortSet{nat.Port("5432/tcp"): {}},
			inspected: nat.PortMap{
				nat.Port("5432/tcp"): {{HostPort: "not-a-port"}},
			},
			wantErr: "invalid host port",
		},
		{
			name:     "ambiguous host bindings",
			expected: nat.PortSet{nat.Port("5432/tcp"): {}},
			inspected: nat.PortMap{
				nat.Port("5432/tcp"): {{HostPort: "49153"}, {HostPort: "49154"}},
			},
			wantErr: "ambiguous host ports",
		},
		{
			name:     "protocol ambiguity",
			expected: nat.PortSet{nat.Port("53/tcp"): {}, nat.Port("53/udp"): {}},
			inspected: nat.PortMap{
				nat.Port("53/tcp"): {{HostPort: "49153"}},
				nat.Port("53/udp"): {{HostPort: "49154"}},
			},
			wantErr: "ambiguous protocol bindings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := githubServicePorts(tt.expected, tt.inspected)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

type serviceContainerWithoutInspection struct {
	container.ExecutionsEnvironment
}

func TestServiceContextRequiresPrivateInspectionCapability(t *testing.T) {
	_, err := serviceContextAfterStart(
		context.Background(),
		&serviceContainerWithoutInspection{},
		serviceContainerMetadata{contextName: "postgres"},
	)
	require.ErrorContains(t, err, "does not support port inspection")
}

type serviceContainerFake struct {
	container.ExecutionsEnvironment
	id            string
	bindings      nat.PortMap
	inspectErr    error
	bindingsAfter int
	inspectCalls  int
	startCalled   bool
	removeCalled  bool
	removeErr     error
	closeCalled   bool
}

func (c *serviceContainerFake) Pull(bool) common.Executor {
	return func(context.Context) error { return nil }
}

func (c *serviceContainerFake) Create([]string, []string) common.Executor {
	return func(context.Context) error { return nil }
}

func (c *serviceContainerFake) Start(bool) common.Executor {
	return func(context.Context) error {
		c.startCalled = true
		return nil
	}
}

func (c *serviceContainerFake) GetContainerID() string {
	return c.id
}

func (c *serviceContainerFake) GetPortBindings(context.Context) (nat.PortMap, error) {
	c.inspectCalls++
	if !c.startCalled {
		return nil, errors.New("inspected before start")
	}
	if c.inspectErr != nil {
		return nil, c.inspectErr
	}
	if c.inspectCalls <= c.bindingsAfter {
		return nat.PortMap{}, nil
	}
	return c.bindings, nil
}

func (c *serviceContainerFake) Remove() common.Executor {
	return func(context.Context) error {
		c.removeCalled = true
		return c.removeErr
	}
}

func (c *serviceContainerFake) Close() common.Executor {
	return func(context.Context) error {
		c.closeCalled = true
		return nil
	}
}

func (c *serviceContainerFake) GetHealth(context.Context) container.Health {
	return container.HealthStarting
}

func TestWaitForServiceContainerCancellationInterruptsBackoff(t *testing.T) {
	rc := &RunContext{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	err := rc.waitForServiceContainer(&serviceContainerFake{})(ctx)

	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(started), 500*time.Millisecond)
}

func TestStartServiceContainersPublishesContextAfterStart(t *testing.T) {
	postgres := &serviceContainerFake{
		id: "postgres-container-id",
		bindings: nat.PortMap{
			nat.Port("5432/tcp"): {{HostPort: "49153"}},
		},
	}
	redis := &serviceContainerFake{
		id: "redis-container-id",
		bindings: nat.PortMap{
			nat.Port("6379/tcp"): {{HostPort: "49154"}},
		},
	}
	rc := &RunContext{
		Config:            &Config{},
		ServiceContainers: []container.ExecutionsEnvironment{postgres, redis},
		serviceContainerMetadata: []serviceContainerMetadata{
			{contextName: "postgres", networkName: "act-test-network", exposedPorts: nat.PortSet{nat.Port("5432/tcp"): {}}},
			{contextName: "redis", networkName: "act-test-network", exposedPorts: nat.PortSet{nat.Port("6379/tcp"): {}}},
		},
		StepResults: map[string]*model.StepResult{},
	}

	require.NoError(t, rc.startServiceContainers("")(context.Background()))
	job := rc.getJobContext()
	require.Equal(t, "postgres-container-id", job.Services["postgres"].ID)
	require.Equal(t, "act-test-network", job.Services["postgres"].Network)
	require.Equal(t, "49153", job.Services["postgres"].Ports["5432"])
	require.Equal(t, "redis-container-id", job.Services["redis"].ID)
	require.Equal(t, "49154", job.Services["redis"].Ports["6379"])
}

func TestStartServiceContainersFailsClosedOnInvalidBinding(t *testing.T) {
	postgres := &serviceContainerFake{
		id: "postgres-container-id",
		bindings: nat.PortMap{
			nat.Port("5432/tcp"): {{HostPort: "not-a-port"}},
		},
	}
	rc := &RunContext{
		Config:            &Config{},
		ServiceContainers: []container.ExecutionsEnvironment{postgres},
		serviceContainerMetadata: []serviceContainerMetadata{
			{contextName: "postgres", exposedPorts: nat.PortSet{nat.Port("5432/tcp"): {}}},
		},
	}

	err := rc.startServiceContainers("")(context.Background())
	require.ErrorContains(t, err, "resolve service postgres port bindings")
	require.ErrorContains(t, err, "invalid host port")
	require.Empty(t, rc.serviceContexts)
}

func TestServiceContextAfterStartRetriesInspectErrors(t *testing.T) {
	postgres := &serviceContainerFake{
		id:          "postgres-container-id",
		inspectErr:  errors.New("inspect unavailable"),
		startCalled: true,
	}
	_, err := serviceContextAfterStartWithRetry(
		context.Background(),
		postgres,
		serviceContainerMetadata{contextName: "postgres", exposedPorts: nat.PortSet{nat.Port("5432/tcp"): {}}},
		20*time.Millisecond,
		time.Millisecond,
	)
	require.ErrorContains(t, err, "inspect service postgres port bindings")
	require.ErrorContains(t, err, "inspect unavailable")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Greater(t, postgres.inspectCalls, 1)
}

func TestServiceContextAfterStartPreservesParentCancellation(t *testing.T) {
	postgres := &serviceContainerFake{
		id:          "postgres-container-id",
		inspectErr:  errors.New("inspect unavailable"),
		startCalled: true,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := serviceContextAfterStartWithRetry(
		ctx,
		postgres,
		serviceContainerMetadata{contextName: "postgres", exposedPorts: nat.PortSet{nat.Port("5432/tcp"): {}}},
		time.Second,
		time.Millisecond,
	)
	require.ErrorContains(t, err, "inspect unavailable")
	require.ErrorIs(t, err, context.Canceled)
}

func TestServiceContextAfterStartRetriesPendingBindings(t *testing.T) {
	postgres := &serviceContainerFake{
		id:            "postgres-container-id",
		bindingsAfter: 2,
		startCalled:   true,
		bindings: nat.PortMap{
			nat.Port("5432/tcp"): {{HostPort: "49153"}},
		},
	}
	serviceContext, err := serviceContextAfterStart(context.Background(), postgres, serviceContainerMetadata{
		contextName:  "postgres",
		networkName:  "act-test-network",
		exposedPorts: nat.PortSet{nat.Port("5432/tcp"): {}},
	})
	require.NoError(t, err)
	require.Equal(t, 3, postgres.inspectCalls)
	require.Equal(t, "49153", serviceContext.Ports["5432"])
}

func TestStartServiceContainersDryrunSkipsInspection(t *testing.T) {
	postgres := &serviceContainerFake{id: "postgres-container-id"}
	rc := &RunContext{
		Config:            &Config{},
		ServiceContainers: []container.ExecutionsEnvironment{postgres},
		serviceContainerMetadata: []serviceContainerMetadata{
			{contextName: "postgres", exposedPorts: nat.PortSet{nat.Port("5432/tcp"): {}}},
		},
	}

	ctx := common.WithDryrun(context.Background(), true)
	require.NoError(t, rc.startServiceContainers("")(ctx))
	require.Zero(t, postgres.inspectCalls)
	require.Empty(t, rc.serviceContexts)
}

func TestContainerSetupFailureCleansPartiallyStartedServices(t *testing.T) {
	postgres := &serviceContainerFake{
		id: "postgres-container-id",
		bindings: nat.PortMap{
			nat.Port("5432/tcp"): {{HostPort: "49153"}},
		},
	}
	redis := &serviceContainerFake{
		id: "redis-container-id",
		bindings: nat.PortMap{
			nat.Port("6379/tcp"): {{HostPort: "not-a-port"}},
		},
	}
	jobRemoveErr := errors.New("job remove failed")
	job := &serviceContainerFake{id: "job-container-id", removeErr: jobRemoveErr}
	rc := &RunContext{
		Config:            &Config{},
		JobContainer:      job,
		ServiceContainers: []container.ExecutionsEnvironment{postgres, redis},
		serviceContainerMetadata: []serviceContainerMetadata{
			{contextName: "postgres", exposedPorts: nat.PortSet{nat.Port("5432/tcp"): {}}},
			{contextName: "redis", exposedPorts: nat.PortSet{nat.Port("6379/tcp"): {}}},
		},
		serviceContexts: map[string]model.ServiceContext{
			"stale": {ID: "stale-container-id"},
		},
	}
	networkRemoved := false
	rc.cleanUpJobContainer = func(ctx context.Context) error {
		return cleanupContainerResources(
			ctx,
			false,
			job.Remove(),
			nil,
			rc.stopServiceContainers(),
			func(context.Context) error {
				networkRemoved = true
				return nil
			},
		)
	}

	err := rc.cleanupContainerSetupFailure(rc.startServiceContainers(""))(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid host port")
	require.ErrorIs(t, err, jobRemoveErr)
	require.True(t, postgres.startCalled)
	require.True(t, redis.startCalled)
	require.True(t, postgres.removeCalled)
	require.True(t, redis.removeCalled)
	require.True(t, postgres.closeCalled)
	require.True(t, redis.closeCalled)
	require.True(t, job.removeCalled)
	require.True(t, job.closeCalled)
	require.True(t, networkRemoved)
	require.Empty(t, rc.serviceContexts)
}

func TestContainerSetupFailurePreservesSetupAndCleanupErrors(t *testing.T) {
	setupErr := errors.New("setup failed")
	cleanupErr := errors.New("cleanup failed")

	for _, tt := range []struct {
		name       string
		cleanupErr error
	}{
		{name: "cleanup succeeds"},
		{name: "cleanup fails", cleanupErr: cleanupErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cleanupCalled := false
			rc := &RunContext{
				cleanUpJobContainer: func(ctx context.Context) error {
					cleanupCalled = true
					require.NoError(t, ctx.Err())
					return tt.cleanupErr
				},
			}

			err := rc.cleanupContainerSetupFailure(func(context.Context) error {
				return setupErr
			})(context.Background())

			require.True(t, cleanupCalled)
			require.ErrorIs(t, err, setupErr)
			if tt.cleanupErr != nil {
				require.ErrorIs(t, err, tt.cleanupErr)
			}
		})
	}

	t.Run("warning keeps upstream success semantics", func(t *testing.T) {
		cleanupCalled := false
		rc := &RunContext{
			cleanUpJobContainer: func(context.Context) error {
				cleanupCalled = true
				return nil
			},
		}

		err := rc.cleanupContainerSetupFailure(func(context.Context) error {
			return common.Warning{Message: "setup warning"}
		})(context.Background())

		require.NoError(t, err)
		require.False(t, cleanupCalled)
	})
}

func TestContainerSetupFailureOnCancellationAfterSuccessfulSetupStillCleans(t *testing.T) {
	type cleanupContextKey struct{}
	const contextValue = "preserved"

	parent, cancel := context.WithCancel(context.WithValue(
		context.Background(),
		cleanupContextKey{},
		contextValue,
	))
	cleanupCalled := false
	rc := &RunContext{
		cleanUpJobContainer: func(ctx context.Context) error {
			cleanupCalled = true
			require.NoError(t, ctx.Err())
			require.Equal(t, contextValue, ctx.Value(cleanupContextKey{}))
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			remaining := time.Until(deadline)
			require.Positive(t, remaining)
			require.LessOrEqual(t, remaining, containerSetupCleanupTimeout)
			return nil
		},
	}

	err := rc.cleanupContainerSetupFailure(func(context.Context) error {
		cancel()
		return nil
	})(parent)

	require.True(t, cleanupCalled)
	require.ErrorIs(t, err, context.Canceled)
}

func TestCleanupContainerResourcesContinuesAfterErrors(t *testing.T) {
	jobErr := errors.New("job removal failed")
	networkErr := errors.New("network removal failed")
	calls := make([]string, 0, 5)
	step := func(name string, err error) common.Executor {
		return func(context.Context) error {
			calls = append(calls, name)
			return err
		}
	}

	err := cleanupContainerResources(
		context.Background(),
		false,
		step("job", jobErr),
		[]common.Executor{step("volume-1", nil), step("volume-2", nil)},
		step("services", nil),
		step("network", networkErr),
	)

	require.ErrorIs(t, err, jobErr)
	require.ErrorIs(t, err, networkErr)
	require.Equal(t, []string{"job", "volume-1", "volume-2", "services", "network"}, calls)
}

func TestCleanupContainerResourcesReuseStillCleansServicesAndNetwork(t *testing.T) {
	calls := make([]string, 0, 2)
	step := func(name string) common.Executor {
		return func(context.Context) error {
			calls = append(calls, name)
			return nil
		}
	}

	require.NoError(t, cleanupContainerResources(
		context.Background(),
		true,
		step("job"),
		[]common.Executor{step("volume")},
		step("services"),
		step("network"),
	))
	require.Equal(t, []string{"services", "network"}, calls)
}

func TestGetJobContextIncludesIndependentServiceSnapshot(t *testing.T) {
	rc := &RunContext{
		serviceContexts: map[string]model.ServiceContext{
			"postgres": {
				ID:      "postgres-container-id",
				Network: "act-test-network",
				Ports:   map[string]string{"5432": "49153"},
			},
		},
		StepResults: map[string]*model.StepResult{},
	}

	job := rc.getJobContext()
	require.Equal(t, "success", job.Status)
	require.Equal(t, "postgres-container-id", job.Services["postgres"].ID)
	require.Equal(t, "act-test-network", job.Services["postgres"].Network)
	require.Equal(t, "49153", job.Services["postgres"].Ports["5432"])

	job.Services["postgres"].Ports["5432"] = "1"
	require.Equal(t, "49153", rc.serviceContexts["postgres"].Ports["5432"])
}

func TestCompositeRunContextInheritsServiceSnapshot(t *testing.T) {
	parent := &RunContext{
		serviceContexts: map[string]model.ServiceContext{
			"postgres": {
				ID:      "postgres-container-id",
				Network: "act-test-network",
				Ports:   map[string]string{"5432": "49153"},
			},
		},
	}
	child := &RunContext{Parent: parent, StepResults: map[string]*model.StepResult{}}
	job := child.getJobContext()
	require.Equal(t, "49153", job.Services["postgres"].Ports["5432"])
	job.Services["postgres"].Ports["5432"] = "1"
	require.Equal(t, "49153", parent.serviceContexts["postgres"].Ports["5432"])
}

func TestServicePortExpressionUsesNumericIndex(t *testing.T) {
	rc := createRunContext(t)
	rc.serviceContexts = map[string]model.ServiceContext{
		"postgres": {
			ID:      "postgres-container-id",
			Network: "act-test-network",
			Ports:   map[string]string{"5432": "49153"},
		},
	}

	ctx := context.Background()
	evaluator := rc.NewExpressionEvaluator(ctx)
	got, err := evaluator.evaluate(ctx, "job.services.postgres.ports[5432]", exprparser.DefaultStatusCheckNone)
	require.NoError(t, err)
	require.Equal(t, "49153", got)
	require.Equal(t, "127.0.0.1:49153", evaluator.Interpolate(ctx, "127.0.0.1:${{ job.services.postgres.ports[5432] }}"))
}
