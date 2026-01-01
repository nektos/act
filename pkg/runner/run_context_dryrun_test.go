package runner

import (
	"context"
	"testing"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/stretchr/testify/assert"
)

func TestServiceContainerWorkflowDryRun(t *testing.T) {
	// Test the actual functionality: service containers should work in dry-run mode
	// This reproduces the exact scenario that was causing segmentation faults

	ctx := common.WithDryrun(context.Background(), true)

	// Create service containers like the workflow that was failing
	serviceContainers := []container.ExecutionsEnvironment{
		container.NewContainer(&container.NewContainerInput{
			Image: "postgres:15-alpine",
			Name:  "postgres-service",
		}),
	}

	// Create RunContext with service containers (the actual failing scenario)
	rc := &RunContext{
		ServiceContainers: serviceContainers,
	}

	// Test the actual workflow that was crashing:
	// 1. waitForServiceContainers calls waitForServiceContainer for each service
	// 2. waitForServiceContainer calls GetHealth() in a timeout loop
	// 3. GetHealth() should handle the case where Docker client isn't available (dry-run)

	// This should complete without segmentation fault
	err := rc.waitForServiceContainers()(ctx)
	assert.NoError(t, err, "Service container workflow should work in dry-run mode")

	// Verify that individual health checks also work (the core of the bug)
	for _, serviceContainer := range serviceContainers {
		health := serviceContainer.GetHealth(ctx)
		// In dry-run mode, we expect containers to report as healthy
		// (since they're not actually running, we mock them as ready)
		assert.Equal(t, container.HealthHealthy, health,
			"Service containers should report healthy in dry-run mode")
	}
}
