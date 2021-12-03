package runner

import (
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
	for _, stage := range plan.Stages {
		for _, run := range stage.Runs {
			log.Debugf("# %s | %s", escapeID(run.JobID), run.Job().Name)
			for n, step := range run.Job().Steps {
				id := step.ID
				if id == "" {
					id = fmt.Sprint(n)
				}

				log.Debugf("## %s | %s", escapeID(id), step)
			}
		}
	}
	log.Debugln("################################################################################")
}

func (rc *RunContext) logJobBoundaries(executor common.Executor) common.Executor {
	id := escapeID(rc.Run.JobID)

	return common.NewDebugExecutor("@@ job | start | %s | %s @@", id, rc.JobName).
		Then(executor).
		Finally(common.NewDebugExecutor("@@ job | end | %s | %s @@", id, rc.JobName))
}

func logStepBoundaries(step *model.Step, executor common.Executor) common.Executor {
	id := escapeID(step.ID)

	return common.NewDebugExecutor("@@ step | start | %s | %s @@", id, step).
		Then(executor).
		Finally(common.NewDebugExecutor("@@ step | stop | %s | %s @@", id, step))
}
