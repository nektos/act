package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
)

func (rc *RunContext) newStepExecutor(step *model.Step) common.Executor {
	sc := &StepContext{
		RunContext: rc,
		Step:       step,
	}
	return func(ctx context.Context) error {
		rc.CurrentStep = sc.Step.ID
		rc.StepResults[rc.CurrentStep] = &model.StepResult{
			Outcome:    model.StepStatusSuccess,
			Conclusion: model.StepStatusSuccess,
			Outputs:    make(map[string]string),
		}

		runStep, err := sc.isEnabled(ctx)
		if err != nil {
			rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusFailure
			rc.StepResults[rc.CurrentStep].Outcome = model.StepStatusFailure
			return err
		}

		if !runStep {
			log.Debugf("Skipping step '%s' due to '%s'", sc.Step.String(), sc.Step.If.Value)
			rc.StepResults[rc.CurrentStep].Conclusion = model.StepStatusSkipped
			rc.StepResults[rc.CurrentStep].Outcome = model.StepStatusSkipped
			return nil
		}

		exprEval, err := sc.setupEnv(ctx)
		if err != nil {
			return err
		}
		rc.ExprEval = exprEval

		common.Logger(ctx).Infof("\u2B50  Run %s", sc.Step)
		err = sc.Executor(ctx)(ctx)
		if err == nil {
			common.Logger(ctx).Infof("  \u2705  Success - %s", sc.Step)
		} else {
			common.Logger(ctx).Errorf("  \u274C  Failure - %s", sc.Step)

			rc.StepResults[rc.CurrentStep].Outcome = model.StepStatusFailure
			if sc.Step.ContinueOnError {
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

func (sc *StepContext) setupEnv(ctx context.Context) (ExpressionEvaluator, error) {
	rc := sc.RunContext
	sc.Env = sc.mergeEnv()
	if sc.Env != nil {
		err := rc.JobContainer.UpdateFromImageEnv(&sc.Env)(ctx)
		if err != nil {
			return nil, err
		}
		err = rc.JobContainer.UpdateFromEnv(sc.Env["GITHUB_ENV"], &sc.Env)(ctx)
		if err != nil {
			return nil, err
		}
		err = rc.JobContainer.UpdateFromPath(&sc.Env)(ctx)
		if err != nil {
			return nil, err
		}
	}
	sc.Env = mergeMaps(sc.Env, sc.Step.GetEnv()) // step env should not be overwritten
	evaluator := sc.NewExpressionEvaluator()
	sc.interpolateEnv(evaluator)

	common.Logger(ctx).Debugf("setupEnv => %v", sc.Env)
	return evaluator, nil
}

func (sc *StepContext) mergeEnv() map[string]string {
	rc := sc.RunContext
	job := rc.Run.Job()

	var env map[string]string
	c := job.Container()
	if c != nil {
		env = mergeMaps(rc.GetEnv(), c.Env)
	} else {
		env = rc.GetEnv()
	}

	if env["PATH"] == "" {
		env["PATH"] = `/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`
	}
	if rc.ExtraPath != nil && len(rc.ExtraPath) > 0 {
		p := env["PATH"]
		env["PATH"] = strings.Join(rc.ExtraPath, `:`)
		env["PATH"] += `:` + p
	}

	sc.Env = rc.withGithubEnv(env)
	return env
}

func (sc *StepContext) isEnabled(ctx context.Context) (bool, error) {
	runStep, err := EvalBool(sc.NewExpressionEvaluator(), sc.Step.If.Value)
	if err != nil {
		return false, fmt.Errorf("  \u274C  Error in if-expression: \"if: %s\" (%s)", sc.Step.If.Value, err)
	}

	return runStep, nil
}
