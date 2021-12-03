package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
)

func escapeID(id string) string {
	return strings.ReplaceAll(id, "|", "||")
}

func printPlan(plan *model.Plan) {
	log.Debugln("################################################################################")
	log.Debugf("# %s", plan.Stages[0].Runs[0].Workflow.Name)
	for _, stage := range plan.Stages {
		for _, run := range stage.Runs {
			log.Debugf("## %s | %s", escapeID(run.JobID), run.Job().Name)
			for n, step := range run.Job().Steps {
				id := step.ID
				if id == "" {
					id = fmt.Sprint(n)
				}

				log.Debugf("### %s | %s", escapeID(id), step)
			}
		}
	}
	log.Debugln("################################################################################")
}

func (rc *RunContext) logJobBoundaries(executor common.Executor) common.Executor {
	id := escapeID(rc.Run.JobID)
	jobName := escapeID(rc.JobName)

	var jobStatus string

	return common.NewDebugExecutor("@@ job | start | %s | %s @@", id, jobName).
		Then(executor).
		Finally(func(ctx context.Context) error {
			jobStatus = rc.getJobContext().Status
			return nil
		}).
		Finally(common.NewDebugExecutor("@@ job | end | %s | %s | %s @@", id, jobName, jobStatus))
}

func (rc *RunContext) logStepBoundaries(step *model.Step, executor common.Executor) common.Executor {
	id := escapeID(step.ID)
	stepIdentifier := escapeID(step.String())

	var stepStatus stepStatus

	return common.NewDebugExecutor("@@ step | start | %s | %s @@", id, stepIdentifier).
		Then(executor).
		Finally(func(ctx context.Context) error {
			stepStatus = rc.StepResults[step.ID].Conclusion
			return nil
		}).
		Finally(common.NewDebugExecutor("@@ step | stop | %s | %s | %s @@", id, stepIdentifier, stepStatus))
}
