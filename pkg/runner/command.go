package runner

import (
	"context"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/common"
)

var commandPatternGA *regexp.Regexp
var commandPatternADO *regexp.Regexp

func init() {
	commandPatternGA = regexp.MustCompile("^::([^ ]+)( (.+))?::([^\r\n]*)[\r\n]+$")
	commandPatternADO = regexp.MustCompile("^##\\[([^ ]+)( (.+))?]([^\r\n]*)[\r\n]+$")
}

func tryParseRawActionCommand(line string) (command string, kvPairs map[string]string, arg string, ok bool) {
	if m := commandPatternGA.FindStringSubmatch(line); m != nil {
		command = m[1]
		kvPairs = parseKeyValuePairs(m[3], ",")
		arg = m[4]
		ok = true
	} else if m := commandPatternADO.FindStringSubmatch(line); m != nil {
		command = m[1]
		kvPairs = parseKeyValuePairs(m[3], ";")
		arg = m[4]
		ok = true
	}
	return
}

func (rc *RunContext) commandHandler(ctx context.Context) common.LineHandler {
	logger := common.OutputLogger(ctx)
	resumeCommand := ""
	return func(line string) bool {
		command, kvPairs, arg, ok := tryParseRawActionCommand(line)
		if !ok {
			return true
		}

		if resumeCommand != "" && command != resumeCommand {
			logger.Infof("  \U00002699  %s", line)
			return false
		}
		arg = unescapeCommandData(arg)
		kvPairs = unescapeKvPairs(kvPairs)
		switch command {
		case "set-env":
			rc.setEnv(ctx, kvPairs, arg)
		case "set-output":
			rc.setOutput(ctx, kvPairs, arg)
		case "add-path":
			rc.addPath(ctx, arg)
		case "debug":
			logger.Infof("  \U0001F4AC  %s", line)
		case "warning":
			logger.Infof("  \U0001F6A7  %s", line)
		case "error":
			logger.Infof("  \U00002757  %s", line)
		case "add-mask":
			rc.AddMask(arg)
			logger.Infof("  \U00002699  %s", "***")
		case "stop-commands":
			resumeCommand = arg
			logger.Infof("  \U00002699  %s", line)
		case resumeCommand:
			resumeCommand = ""
			logger.Infof("  \U00002699  %s", line)
		case "save-state":
			logger.Infof("  \U0001f4be  %s", line)
			rc.saveState(ctx, kvPairs, arg)
		case "add-matcher":
			logger.Infof("  \U00002753 add-matcher %s", arg)
		default:
			logger.Infof("  \U00002753  %s", line)
		}

		return false
	}
}

func (rc *RunContext) setEnv(ctx context.Context, kvPairs map[string]string, arg string) {
	name := kvPairs["name"]
	common.Logger(ctx).Infof("  \U00002699  ::set-env:: %s=%s", name, arg)
	if rc.Env == nil {
		rc.Env = make(map[string]string)
	}
	if rc.GlobalEnv == nil {
		rc.GlobalEnv = map[string]string{}
	}
	newenv := map[string]string{
		name: arg,
	}
	mergeIntoMap := mergeIntoMapCaseSensitive
	if rc.JobContainer != nil && rc.JobContainer.IsEnvironmentCaseInsensitive() {
		mergeIntoMap = mergeIntoMapCaseInsensitive
	}
	mergeIntoMap(rc.Env, newenv)
	mergeIntoMap(rc.GlobalEnv, newenv)
}
func (rc *RunContext) setOutput(ctx context.Context, kvPairs map[string]string, arg string) {
	logger := common.Logger(ctx)
	stepID := rc.CurrentStep
	outputName := kvPairs["name"]
	if outputMapping, ok := rc.OutputMappings[MappableOutput{StepID: stepID, OutputName: outputName}]; ok {
		stepID = outputMapping.StepID
		outputName = outputMapping.OutputName
	}

	result, ok := rc.StepResults[stepID]
	if !ok {
		logger.Infof("  \U00002757  no outputs used step '%s'", stepID)
		return
	}

	logger.Infof("  \U00002699  ::set-output:: %s=%s", outputName, arg)
	result.Outputs[outputName] = arg
}
func (rc *RunContext) addPath(ctx context.Context, arg string) {
	common.Logger(ctx).Infof("  \U00002699  ::add-path:: %s", arg)
	extraPath := []string{arg}
	for _, v := range rc.ExtraPath {
		if v != arg {
			extraPath = append(extraPath, v)
		}
	}
	rc.ExtraPath = extraPath
}

func parseKeyValuePairs(kvPairs string, separator string) map[string]string {
	rtn := make(map[string]string)
	kvPairList := strings.Split(kvPairs, separator)
	for _, kvPair := range kvPairList {
		kv := strings.Split(kvPair, "=")
		if len(kv) == 2 {
			rtn[kv[0]] = kv[1]
		}
	}
	return rtn
}
func unescapeCommandData(arg string) string {
	escapeMap := map[string]string{
		"%25": "%",
		"%0D": "\r",
		"%0A": "\n",
	}
	for k, v := range escapeMap {
		arg = strings.ReplaceAll(arg, k, v)
	}
	return arg
}
func unescapeCommandProperty(arg string) string {
	escapeMap := map[string]string{
		"%25": "%",
		"%0D": "\r",
		"%0A": "\n",
		"%3A": ":",
		"%2C": ",",
	}
	for k, v := range escapeMap {
		arg = strings.ReplaceAll(arg, k, v)
	}
	return arg
}
func unescapeKvPairs(kvPairs map[string]string) map[string]string {
	for k, v := range kvPairs {
		kvPairs[k] = unescapeCommandProperty(v)
	}
	return kvPairs
}

func (rc *RunContext) saveState(_ context.Context, kvPairs map[string]string, arg string) {
	stepID := rc.CurrentStep
	if stepID != "" {
		if rc.IntraActionState == nil {
			rc.IntraActionState = map[string]map[string]string{}
		}
		state, ok := rc.IntraActionState[stepID]
		if !ok {
			state = map[string]string{}
			rc.IntraActionState[stepID] = state
		}
		state[kvPairs["name"]] = arg
	}
}
