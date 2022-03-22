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
}

func runStepExecutor(step step, executor common.Executor) common.Executor {
	return func(ctx context.Context) error {
		rc := step.getRunContext()
		stepModel := step.getStepModel()

		rc.CurrentStep = stepModel.ID
		rc.StepResults[rc.CurrentStep] = &model.StepResult{
			Outcome:    model.StepStatusSuccess,
			Conclusion: model.StepStatusSuccess,
			Outputs:    make(map[string]string),
		}

		err := setupEnv(ctx, step)
		if err != nil {
			return err
		}

		runStep, err := isStepEnabled(ctx, step)
		if err != nil {
			rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusFailure
			rc.StepResults[rc.CurrentStep].Outcome = model.StepStatusFailure
			return err
		}

		if !runStep {
			log.Debugf("Skipping step '%s' due to '%s'", stepModel.String(), stepModel.If.Value)
			rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusSkipped
			rc.StepResults[rc.CurrentStep].Outcome = model.StepStatusSkipped
			return nil
		}

		common.Logger(ctx).Infof("\u2B50  Run %s", stepModel)

		err = executor(ctx)

		if err == nil {
			common.Logger(ctx).Infof("  \u2705  Success - %s", stepModel)
		} else {
			common.Logger(ctx).Errorf("  \u274C  Failure - %s", stepModel)

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

func isStepEnabled(ctx context.Context, step step) (bool, error) {
	rc := step.getRunContext()

	runStep, err := EvalBool(rc.NewStepExpressionEvaluator(step), step.getStepModel().If.Value)
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
