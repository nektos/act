package runner

import (
	"context"
	"testing"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/stretchr/testify/assert"
)

func TestWaitForServiceContainerDryRun(t *testing.T) {
	// Test that waitForServiceContainer returns immediately in dry-run mode
	// without calling GetHealth() which would cause a segmentation fault

	ctx := common.WithDryrun(context.Background(), true)

	// Create a mock RunContext
	rc := &RunContext{}

	// Create a mock container (normally this would have nil Docker client in dry-run)
	mockContainer := container.NewContainer(&container.NewContainerInput{
		Image: "test:latest",
		Name:  "test-container",
	})

	// This should complete without error and without calling GetHealth()
	executor := rc.waitForServiceContainer(mockContainer)
	err := executor(ctx)

	assert.NoError(t, err, "waitForServiceContainer should not error in dry-run mode")
}

func TestGetHealthDryRun(t *testing.T) {
	// Test that GetHealth returns HealthHealthy in dry-run mode
	// without trying to inspect containers

	ctx := common.WithDryrun(context.Background(), true)

	// Create a container using the public interface
	mockContainer := container.NewContainer(&container.NewContainerInput{
		Image: "test:latest",
		Name:  "test-container",
	})

	health := mockContainer.GetHealth(ctx)

	assert.Equal(t, container.HealthHealthy, health, "GetHealth should return HealthHealthy in dry-run mode")
}

func TestServiceContainersDryRun(t *testing.T) {
	// Test that simulates the exact segmentation fault scenario:
	// Multiple service containers calling GetHealth() in dry-run mode

	ctx := common.WithDryrun(context.Background(), true)

	// Create multiple service containers (simulating the postgres service from the bug report)
	serviceContainers := []container.ExecutionsEnvironment{
		container.NewContainer(&container.NewContainerInput{
			Image: "postgres:17-alpine",
			Name:  "postgres-service",
		}),
		container.NewContainer(&container.NewContainerInput{
			Image: "redis:latest",
			Name:  "redis-service",
		}),
	}

	// Create a RunContext to test the service container workflow
	rc := &RunContext{
		ServiceContainers: serviceContainers,
	}

	// Test that waitForServiceContainers (which calls waitForServiceContainer for each service)
	// completes without segmentation fault in dry-run mode
	executor := rc.waitForServiceContainers()
	err := executor(ctx)

	assert.NoError(t, err, "waitForServiceContainers should complete without segfault in dry-run mode")

	// Also test each individual service container health check
	for i, serviceContainer := range serviceContainers {
		health := serviceContainer.GetHealth(ctx)
		assert.Equal(t, container.HealthHealthy, health,
			"Service container %d should report healthy in dry-run mode", i)
	}
}
