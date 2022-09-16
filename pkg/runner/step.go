package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/exprparser"
	"github.com/nektos/act/pkg/model"
)

type step interface {
	pre() common.Executor
	main() common.Executor
	post() common.Executor

	getRunContext() *RunContext
	getStepModel() *model.Step
	getEnv() *map[string]string
	getIfExpression(context context.Context, stage stepStage) string
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
		logger := common.Logger(ctx)
		rc := step.getRunContext()
		stepModel := step.getStepModel()

		ifExpression := step.getIfExpression(ctx, stage)
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

		runStep, err := isStepEnabled(ctx, ifExpression, step, stage)
		if err != nil {
			rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusFailure
			rc.StepResults[rc.CurrentStep].Outcome = model.StepStatusFailure
			return err
		}

		if !runStep {
			rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusSkipped
			rc.StepResults[rc.CurrentStep].Outcome = model.StepStatusSkipped
			logger.WithField("stepResult", rc.StepResults[rc.CurrentStep].Outcome).Debugf("Skipping step '%s' due to '%s'", stepModel, ifExpression)
			return nil
		}

		stepString := stepModel.String()
		if strings.Contains(stepString, "::add-mask::") {
			stepString = "add-mask command"
		}
		logger.Infof("\u2B50 Run %s %s", stage, stepString)

		err = executor(ctx)

		if err == nil {
			logger.WithField("stepResult", rc.StepResults[rc.CurrentStep].Outcome).Infof("  \u2705  Success - %s %s", stage, stepString)
		} else {
			rc.StepResults[rc.CurrentStep].Outcome = model.StepStatusFailure

			continueOnError, parseErr := isContinueOnError(ctx, stepModel.RawContinueOnError, step, stage)
			if parseErr != nil {
				rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusFailure
				return parseErr
			}

			if continueOnError {
				logger.Infof("Failed but continue next step")
				err = nil
				rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusSuccess
			} else {
				rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusFailure
			}

			logger.WithField("stepResult", rc.StepResults[rc.CurrentStep].Outcome).Errorf("  \u274C  Failure - %s %s", stage, stepString)
		}
		return err
	}
}

func setupEnv(ctx context.Context, step step) error {
	rc := step.getRunContext()

	mergeEnv(ctx, step)
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

	exprEval := rc.NewStepExpressionEvaluator(ctx, step)
	for k, v := range *step.getEnv() {
		(*step.getEnv())[k] = exprEval.Interpolate(ctx, v)
	}

	common.Logger(ctx).Debugf("setupEnv => %v", *step.getEnv())

	return nil
}

func mergeEnv(ctx context.Context, step step) {
	env := step.getEnv()
	rc := step.getRunContext()
	job := rc.Run.Job()

	c := job.Container()
	if c != nil {
		mergeIntoMap(env, rc.GetEnv(), c.Env)
	} else {
		mergeIntoMap(env, rc.GetEnv())
	}

	path := rc.JobContainer.GetPathVariableName()
	if (*env)[path] == "" {
		(*env)[path] = rc.JobContainer.DefaultPathVariable()
	}
	if rc.ExtraPath != nil && len(rc.ExtraPath) > 0 {
		(*env)[path] = rc.JobContainer.JoinPathVariable(append(rc.ExtraPath, (*env)[path])...)
	}

	mergeIntoMap(env, rc.withGithubEnv(ctx, *env))
}

func isStepEnabled(ctx context.Context, expr string, step step, stage stepStage) (bool, error) {
	rc := step.getRunContext()

	var defaultStatusCheck exprparser.DefaultStatusCheck
	if stage == stepStagePost {
		defaultStatusCheck = exprparser.DefaultStatusCheckAlways
	} else {
		defaultStatusCheck = exprparser.DefaultStatusCheckSuccess
	}

	runStep, err := EvalBool(ctx, rc.NewStepExpressionEvaluator(ctx, step), expr, defaultStatusCheck)
	if err != nil {
		return false, fmt.Errorf("  \u274C  Error in if-expression: \"if: %s\" (%s)", expr, err)
	}

	return runStep, nil
}

func isContinueOnError(ctx context.Context, expr string, step step, stage stepStage) (bool, error) {
	// https://github.com/github/docs/blob/3ae84420bd10997bb5f35f629ebb7160fe776eae/content/actions/reference/workflow-syntax-for-github-actions.md?plain=true#L962
	if len(strings.TrimSpace(expr)) == 0 {
		return false, nil
	}

	rc := step.getRunContext()

	continueOnError, err := EvalBool(ctx, rc.NewStepExpressionEvaluator(ctx, step), expr, exprparser.DefaultStatusCheckNone)
	if err != nil {
		return false, fmt.Errorf("  \u274C  Error in continue-on-error-expression: \"continue-on-error: %s\" (%s)", expr, err)
	}

	return continueOnError, nil
}

func mergeIntoMap(target *map[string]string, maps ...map[string]string) {
	for _, m := range maps {
		for k, v := range m {
			(*target)[k] = v
		}
	}
}
