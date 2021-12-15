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
			logger.WithField("emoji", "  \U00002699").Infof("  %s", line)
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
			logger.WithField("emoji", "  \U0001F4AC").Infof("  %s", line)
		case "warning":
			logger.WithField("emoji", "  \U0001F6A7").Infof("  %s", line)
		case "error":
			logger.WithField("emoji", "  \U00002757").Infof("  %s", line)
		case "add-mask":
			logger.WithField("emoji", "  \U00002699").Infof("  %s", "***")
		case "stop-commands":
			resumeCommand = arg
			logger.WithField("emoji", "  \U00002699").Infof("  %s", line)
		case resumeCommand:
			resumeCommand = ""
			logger.WithField("emoji", "  \U00002699").Infof("  %s", line)
		default:
			logger.WithField("emoji", "  \U00002753").Infof("  %s", line)
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

	result, ok := rc.StepResults[stepID]
	if !ok {
		common.Logger(ctx).WithField("emoji", "  \U00002757").Infof("  no outputs used step '%s'", stepID)
		return
	}

	common.Logger(ctx).WithField("emoji", "  \U00002699").Infof("  ::set-output:: %s=%s", outputName, arg)
	result.Outputs[outputName] = arg
}

func (rc *RunContext) addPath(ctx context.Context, arg string) {
	common.Logger(ctx).WithField("emoji", "  \U00002699").Infof("  ::add-path:: %s", arg)
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
