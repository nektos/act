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

func (rc *RunContext) commandHandler(ctx context.Context) common.LineHandler {
	logger := common.Logger(ctx)
	resumeCommand := ""
	return func(line string) bool {
		var command string
		var kvPairs map[string]string
		var arg string
		if m := commandPatternGA.FindStringSubmatch(line); m != nil {
			command = m[1]
			kvPairs = parseKeyValuePairs(m[3], ",")
			arg = m[4]
		} else if m := commandPatternADO.FindStringSubmatch(line); m != nil {
			command = m[1]
			kvPairs = parseKeyValuePairs(m[3], ";")
			arg = m[4]
		} else {
			return true
		}

		if resumeCommand != "" && command != resumeCommand {
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
			logger.Infof("  \U00002699  %s", line)
		case "stop-commands":
			resumeCommand = arg
			logger.Infof("  \U00002699  %s", line)
		case resumeCommand:
			resumeCommand = ""
			logger.Infof("  \U00002699  %s", line)
		default:
			logger.Infof("  \U00002753  %s", line)
		}

		return false
	}
}

func (rc *RunContext) setEnv(ctx context.Context, kvPairs map[string]string, arg string) {
	common.Logger(ctx).Infof("  \U00002699  ::set-env:: %s=%s", kvPairs["name"], arg)
	if rc.Env == nil {
		rc.Env = make(map[string]string)
	}
	rc.Env[kvPairs["name"]] = arg
}
func (rc *RunContext) setOutput(ctx context.Context, kvPairs map[string]string, arg string) {
	stepID := rc.CurrentStep
	outputName := kvPairs["name"]
	if outputMapping, ok := rc.OutputMappings[MappableOutput{StepID: stepID, OutputName: outputName}]; ok {
		stepID = outputMapping.StepID
		outputName = outputMapping.OutputName
	}

	common.Logger(ctx).Infof("  \U00002699  %s ::set-output:: %s=%s", stepID, outputName, arg)
    if _, ok := rc.StepResults[stepID]; ! ok {
	    common.Logger(ctx).Errorf("  \U00002699 %s NOT in rc.StepResults[]", stepID)
	    return
	}

	common.Logger(ctx).Infof("  \U00002699  ::set-output:: %s=%s", outputName, arg)
	rc.StepResults[stepID].Outputs[outputName] = arg
}
func (rc *RunContext) addPath(ctx context.Context, arg string) {
	common.Logger(ctx).Infof("  \U00002699  ::add-path:: %s", arg)
	rc.ExtraPath = append(rc.ExtraPath, arg)
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
