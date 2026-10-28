package runner

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/exprparser"
	"github.com/nektos/act/pkg/model"
	"github.com/sirupsen/logrus"
)

type step interface {
	pre() common.Executor
	main() common.Executor
	post() common.Executor

	getRunContext() *RunContext
	getGithubContext(ctx context.Context) *model.GithubContext
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

// Controls how many symlinks are resolved for local and remote Actions
const maxSymlinkDepth = 10

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

func processRunnerSummaryCommand(ctx context.Context, fileName string, rc *RunContext) error {
	if common.Dryrun(ctx) {
		return nil
	}
	pathTar, err := rc.JobContainer.GetContainerArchive(ctx, path.Join(rc.JobContainer.GetActPath(), fileName))
	if err != nil {
		return err
	}
	defer pathTar.Close()

	reader := tar.NewReader(pathTar)
	_, err = reader.Next()
	if err != nil && err != io.EOF {
		return err
	}
	summary, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if len(summary) == 0 {
		return nil
	}
	common.Logger(ctx).WithFields(logrus.Fields{"command": "summary", "content": string(summary)}).Infof("  \U00002699  Summary - %s", string(summary))
	return nil
}

func processRunnerEnvFileCommand(ctx context.Context, fileName string, rc *RunContext, setter func(context.Context, map[string]string, string)) error {
	env := map[string]string{}
	err := rc.JobContainer.UpdateFromEnv(path.Join(rc.JobContainer.GetActPath(), fileName), &env)(ctx)
	if err != nil {
		return err
	}
	for k, v := range env {
		setter(ctx, map[string]string{"name": k}, v)
	}
	return nil
}

// stepFileCommandSeq makes the runner file command paths unique per step
// execution, so steps running concurrently because of `background: true` do
// not read each other's commands
var stepFileCommandSeq atomic.Int64

func runStepExecutor(step step, stage stepStage, executor common.Executor) common.Executor {
	return func(ctx context.Context) error {
		logger := common.Logger(ctx)
		rc := step.getRunContext()
		stepModel := step.getStepModel()

		// the shared job state is locked during the setup and the result
		// processing of the step, but not while the step itself runs, so
		// background steps run concurrently with other steps
		lock := rc.stepStateLock()
		lock.Lock()

		ifExpression := step.getIfExpression(ctx, stage)
		rc.setCurrentStep(stepModel.ID)

		dataLock := rc.stateDataLock()

		stepResult := &model.StepResult{
			Outcome:    model.StepStatusSuccess,
			Conclusion: model.StepStatusSuccess,
			Outputs:    make(map[string]string),
		}
		if stage == stepStageMain {
			dataLock.Lock()
			rc.StepResults[stepModel.ID] = stepResult
			dataLock.Unlock()
		}

		err := setupEnv(ctx, step)
		if err != nil {
			lock.Unlock()
			return err
		}

		cctx := common.JobCancelContext(ctx)
		dataLock.Lock()
		rc.Cancelled = cctx != nil && cctx.Err() != nil
		dataLock.Unlock()

		runStep, err := isStepEnabled(ctx, ifExpression, step, stage)
		if err != nil {
			dataLock.Lock()
			stepResult.Conclusion = model.StepStatusFailure
			stepResult.Outcome = model.StepStatusFailure
			dataLock.Unlock()
			lock.Unlock()
			return err
		}

		if !runStep {
			dataLock.Lock()
			stepResult.Conclusion = model.StepStatusSkipped
			stepResult.Outcome = model.StepStatusSkipped
			dataLock.Unlock()
			logger.WithField("stepResult", stepResult.Outcome).Debugf("Skipping step '%s' due to '%s'", stepModel, ifExpression)
			lock.Unlock()
			return nil
		}

		stepString := rc.ExprEval.Interpolate(ctx, stepModel.String())
		if strings.Contains(stepString, "::add-mask::") {
			stepString = "add-mask command"
		}
		logger.Infof("\u2B50 Run %s %s", stage, stepString)

		// Prepare and clean Runner File Commands
		actPath := rc.JobContainer.GetActPath()
		cmdDir := path.Join("workflow", fmt.Sprintf("cmds-%d-%s", stepFileCommandSeq.Add(1), stage.String()))

		outputFileCommand := path.Join(cmdDir, "outputcmd.txt")
		(*step.getEnv())["GITHUB_OUTPUT"] = path.Join(actPath, outputFileCommand)

		stateFileCommand := path.Join(cmdDir, "statecmd.txt")
		(*step.getEnv())["GITHUB_STATE"] = path.Join(actPath, stateFileCommand)

		pathFileCommand := path.Join(cmdDir, "pathcmd.txt")
		(*step.getEnv())["GITHUB_PATH"] = path.Join(actPath, pathFileCommand)

		envFileCommand := path.Join(cmdDir, "envs.txt")
		(*step.getEnv())["GITHUB_ENV"] = path.Join(actPath, envFileCommand)

		summaryFileCommand := path.Join(cmdDir, "SUMMARY.md")
		(*step.getEnv())["GITHUB_STEP_SUMMARY"] = path.Join(actPath, summaryFileCommand)

		_ = rc.JobContainer.Copy(actPath, &container.FileEntry{
			Name: outputFileCommand,
			Mode: 0o666,
		}, &container.FileEntry{
			Name: stateFileCommand,
			Mode: 0o666,
		}, &container.FileEntry{
			Name: pathFileCommand,
			Mode: 0o666,
		}, &container.FileEntry{
			Name: envFileCommand,
			Mode: 0666,
		}, &container.FileEntry{
			Name: summaryFileCommand,
			Mode: 0o666,
		})(ctx)

		timeout := rc.ExprEval.Interpolate(ctx, stepModel.TimeoutMinutes)
		lock.Unlock()

		stepCtx, cancelStepCtx := context.WithCancel(ctx)
		defer cancelStepCtx()
		var cancelTimeOut context.CancelFunc
		stepCtx, cancelTimeOut = evaluateStepTimeout(stepCtx, timeout)
		defer cancelTimeOut()
		monitorJobCancellation(ctx, stepCtx, cctx, rc, logger, ifExpression, step, stage, cancelStepCtx)
		startTime := time.Now()
		err = executor(stepCtx)
		executionTime := time.Since(startTime)

		lock.Lock()
		defer lock.Unlock()
		rc.setCurrentStep(stepModel.ID)

		if err == nil {
			logger.WithFields(logrus.Fields{"executionTime": executionTime, "stepResult": stepResult.Outcome}).Infof("  \u2705  Success - %s %s [%s]", stage, stepString, executionTime)
		} else {
			dataLock.Lock()
			stepResult.Outcome = model.StepStatusFailure
			dataLock.Unlock()

			continueOnError, parseErr := isContinueOnError(ctx, stepModel.RawContinueOnError, step, stage)
			if parseErr != nil {
				dataLock.Lock()
				stepResult.Conclusion = model.StepStatusFailure
				dataLock.Unlock()
				return parseErr
			}

			dataLock.Lock()
			if continueOnError {
				logger.Infof("Failed but continue next step")
				err = nil
				stepResult.Conclusion = model.StepStatusSuccess
			} else {
				stepResult.Conclusion = model.StepStatusFailure
			}
			dataLock.Unlock()

			logger.WithFields(logrus.Fields{"executionTime": executionTime, "stepResult": stepResult.Outcome}).Infof("  \u274C  Failure - %s %s [%s]", stage, stepString, executionTime)
		}
		// Process Runner File Commands
		stepID := stepModel.ID
		ferrors := []error{err}
		ferrors = append(ferrors, processRunnerEnvFileCommand(ctx, envFileCommand, rc, rc.setEnv))
		ferrors = append(ferrors, processRunnerEnvFileCommand(ctx, stateFileCommand, rc, func(ctx context.Context, kvPairs map[string]string, arg string) {
			rc.saveStateForStep(ctx, stepID, kvPairs, arg)
		}))
		ferrors = append(ferrors, processRunnerEnvFileCommand(ctx, outputFileCommand, rc, func(ctx context.Context, kvPairs map[string]string, arg string) {
			rc.setOutputForStep(ctx, stepID, kvPairs, arg)
		}))
		ferrors = append(ferrors, processRunnerSummaryCommand(ctx, summaryFileCommand, rc))
		ferrors = append(ferrors, rc.UpdateExtraPath(ctx, path.Join(actPath, pathFileCommand)))
		return errors.Join(ferrors...)
	}
}

func monitorJobCancellation(ctx context.Context, stepCtx context.Context, jobCancellationCtx context.Context, rc *RunContext, logger logrus.FieldLogger, ifExpression string, step step, stage stepStage, cancelStepCtx context.CancelFunc) {
	if !rc.Cancelled && jobCancellationCtx != nil {
		go func() {
			select {
			case <-jobCancellationCtx.Done():
				lock := rc.stepStateLock()
				lock.Lock()
				dataLock := rc.stateDataLock()
				dataLock.Lock()
				rc.Cancelled = true
				dataLock.Unlock()
				logger.Infof("Reevaluate condition %v due to cancellation", ifExpression)
				keepStepRunning, err := isStepEnabled(ctx, ifExpression, step, stage)
				lock.Unlock()
				logger.Infof("Result condition keepStepRunning=%v", keepStepRunning)
				if !keepStepRunning || err != nil {
					cancelStepCtx()
				}
			case <-stepCtx.Done():
			}
		}()
	}
}

func evaluateStepTimeout(ctx context.Context, timeout string) (context.Context, context.CancelFunc) {
	if timeout != "" {
		if timeOutMinutes, err := strconv.ParseInt(timeout, 10, 64); err == nil {
			return context.WithTimeout(ctx, time.Duration(timeOutMinutes)*time.Minute)
		}
	}
	return ctx, func() {}
}

func setupEnv(ctx context.Context, step step) error {
	rc := step.getRunContext()

	mergeEnv(ctx, step)
	// merge step env last, since it should not be overwritten
	mergeIntoMap(step, step.getEnv(), step.getStepModel().GetEnv())

	exprEval := rc.NewExpressionEvaluator(ctx)
	for k, v := range *step.getEnv() {
		if !strings.HasPrefix(k, "INPUT_") {
			(*step.getEnv())[k] = exprEval.Interpolate(ctx, v)
		}
	}
	// after we have an evaluated step context, update the expressions evaluator with a new env context
	// you can use step level env in the with property of a uses construct
	exprEval = rc.NewExpressionEvaluatorWithEnv(ctx, *step.getEnv())
	for k, v := range *step.getEnv() {
		if strings.HasPrefix(k, "INPUT_") {
			(*step.getEnv())[k] = exprEval.Interpolate(ctx, v)
		}
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
		mergeIntoMap(step, env, rc.GetEnv(), c.Env)
	} else {
		mergeIntoMap(step, env, rc.GetEnv())
	}

	rc.withGithubEnv(ctx, step.getGithubContext(ctx), *env)

	if step.getStepModel().Uses != "" {
		// prevent uses action input pollution of unset parameters, skip this for run steps
		// due to design flaw
		for key := range *env {
			if strings.Contains(key, "INPUT_") {
				delete(*env, key)
			}
		}
	}
}

func isStepEnabled(ctx context.Context, expr string, step step, stage stepStage) (bool, error) {
	rc := step.getRunContext()

	var defaultStatusCheck exprparser.DefaultStatusCheck
	if stage == stepStagePost {
		defaultStatusCheck = exprparser.DefaultStatusCheckAlways
	} else {
		defaultStatusCheck = exprparser.DefaultStatusCheckSuccess
	}

	runStep, err := EvalBool(ctx, rc.NewStepExpressionEvaluatorExt(ctx, step, stage == stepStageMain), expr, defaultStatusCheck)
	if err != nil {
		return false, fmt.Errorf("  \u274C  Error in if-expression: \"if: %s\" (%s)", expr, err)
	}

	return runStep, nil
}

func isContinueOnError(ctx context.Context, expr string, step step, _ stepStage) (bool, error) {
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

func mergeIntoMap(step step, target *map[string]string, maps ...map[string]string) {
	if rc := step.getRunContext(); rc != nil && rc.JobContainer != nil && rc.JobContainer.IsEnvironmentCaseInsensitive() {
		mergeIntoMapCaseInsensitive(*target, maps...)
	} else {
		mergeIntoMapCaseSensitive(*target, maps...)
	}
}

func mergeIntoMapCaseSensitive(target map[string]string, maps ...map[string]string) {
	for _, m := range maps {
		for k, v := range m {
			target[k] = v
		}
	}
}

func mergeIntoMapCaseInsensitive(target map[string]string, maps ...map[string]string) {
	foldKeys := make(map[string]string, len(target))
	for k := range target {
		foldKeys[strings.ToLower(k)] = k
	}
	toKey := func(s string) string {
		foldKey := strings.ToLower(s)
		if k, ok := foldKeys[foldKey]; ok {
			return k
		}
		foldKeys[strings.ToLower(foldKey)] = s
		return s
	}
	for _, m := range maps {
		for k, v := range m {
			target[toKey(k)] = v
		}
	}
}

func symlinkJoin(filename, sym, parent string) (string, error) {
	dir := path.Dir(filename)
	dest := path.Join(dir, sym)
	prefix := path.Clean(parent) + "/"
	if strings.HasPrefix(dest, prefix) || prefix == "./" {
		return dest, nil
	}
	return "", fmt.Errorf("symlink tries to access file '%s' outside of '%s'", strings.ReplaceAll(dest, "'", "''"), strings.ReplaceAll(parent, "'", "''"))
}
