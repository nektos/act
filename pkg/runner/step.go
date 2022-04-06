package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
)

type step interface {
	pre() common.Executor
	main() common.Executor
	post() common.Executor

	getRunContext() *RunContext
	getStepModel() *model.Step
	getEnv() *map[string]string
	getIfExpression(stage stepStage) string
}

type stepStage int

const (
	stepStagePre stepStage = iota
	stepStageMain
	stepStagePost
)

func (s stepStage) String() string {
	switch s {
	case stepStagePre:
		return "Pre"
	case stepStageMain:
		return "Main"
	case stepStagePost:
		return "Post"
	}
	return "Unknown"
}

func (s stepStage) getStepName(stepModel *model.Step) string {
	switch s {
	case stepStagePre:
		return fmt.Sprintf("pre-%s", stepModel.ID)
	case stepStageMain:
		return stepModel.ID
	case stepStagePost:
		return fmt.Sprintf("post-%s", stepModel.ID)
	}
	return "unknown"
}

func runStepExecutor(step step, stage stepStage, executor common.Executor) common.Executor {
	return func(ctx context.Context) error {
		rc := step.getRunContext()
		stepModel := step.getStepModel()

		ifExpression := step.getIfExpression(stage)
		rc.CurrentStep = stage.getStepName(stepModel)

		rc.StepResults[rc.CurrentStep] = &model.StepResult{
			Outcome:    model.StepStatusSuccess,
			Conclusion: model.StepStatusSuccess,
			Outputs:    make(map[string]string),
		}

		err := setupEnv(ctx, step)
		if err != nil {
			return err
		}

		runStep, err := isStepEnabled(ctx, ifExpression, step)
		if err != nil {
			rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusFailure
			rc.StepResults[rc.CurrentStep].Outcome = model.StepStatusFailure
			return err
		}

		if !runStep {
			log.Debugf("Skipping step '%s' due to '%s'", stepModel, stepModel.If.Value)
			rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusSkipped
			rc.StepResults[rc.CurrentStep].Outcome = model.StepStatusSkipped
			return nil
		}

		stepString := stepModel.String()
		if strings.Contains(stepString, "::add-mask::") {
			stepString = "add-mask command"
		}
		common.Logger(ctx).Infof("\u2B50 Run %s %s", stage, stepString)

		err = executor(ctx)

		if err == nil {
			common.Logger(ctx).Infof("  \u2705  Success - %s %s", stage, stepString)
		} else {
			common.Logger(ctx).Errorf("  \u274C  Failure - %s %s", stage, stepString)

			rc.StepResults[rc.CurrentStep].Outcome = model.StepStatusFailure
			if stepModel.ContinueOnError {
				common.Logger(ctx).Infof("Failed but continue next step")
				err = nil
				rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusSuccess
			} else {
				rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusFailure
			}
		}
		return err
	}
}

func setupEnv(ctx context.Context, step step) error {
	rc := step.getRunContext()

	mergeEnv(step)
	err := rc.JobContainer.UpdateFromImageEnv(step.getEnv())(ctx)
	if err != nil {
		return err
	}
	err = rc.JobContainer.UpdateFromEnv((*step.getEnv())["GITHUB_ENV"], step.getEnv())(ctx)
	if err != nil {
		return err
	}
	err = rc.JobContainer.UpdateFromPath(step.getEnv())(ctx)
	if err != nil {
		return err
	}
	mergeIntoMap(step.getEnv(), step.getStepModel().GetEnv()) // step env should not be overwritten

	exprEval := rc.NewStepExpressionEvaluator(step)
	for k, v := range *step.getEnv() {
		(*step.getEnv())[k] = exprEval.Interpolate(v)
	}

	common.Logger(ctx).Debugf("setupEnv => %v", *step.getEnv())

	return nil
}

func mergeEnv(step step) {
	env := step.getEnv()
	rc := step.getRunContext()
	job := rc.Run.Job()

	c := job.Container()
	if c != nil {
		mergeIntoMap(env, rc.GetEnv(), c.Env)
	} else {
		mergeIntoMap(env, rc.GetEnv())
	}

	if (*env)["PATH"] == "" {
		(*env)["PATH"] = `/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`
	}
	if rc.ExtraPath != nil && len(rc.ExtraPath) > 0 {
		p := (*env)["PATH"]
		(*env)["PATH"] = strings.Join(rc.ExtraPath, `:`)
		(*env)["PATH"] += `:` + p
	}

	mergeIntoMap(env, rc.withGithubEnv(*env))
}

func isStepEnabled(ctx context.Context, expr string, step step) (bool, error) {
	rc := step.getRunContext()

	runStep, err := EvalBool(rc.NewStepExpressionEvaluator(step), expr)
	if err != nil {
		return false, fmt.Errorf("  \u274C  Error in if-expression: \"if: %s\" (%s)", step.getStepModel().If.Value, err)
	}

	return runStep, nil
}

func mergeIntoMap(target *map[string]string, maps ...map[string]string) {
	for _, m := range maps {
		for k, v := range m {
			(*target)[k] = v
		}
	}
}
