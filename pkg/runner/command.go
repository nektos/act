package runner

import (
	"context"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/common"
)

var commandPatternGA *regexp.Regexp
var commandPatternADO *regexp.Regexp
var commandPatternEscapeChar1 *regexp.Regexp
var commandPatternEscapeChar2 *regexp.Regexp
var commandPatternEscapeChar3 *regexp.Regexp
var commandPatternEscapeChar4 *regexp.Regexp
var commandPatternEscapeChar5 *regexp.Regexp

func init() {
	commandPatternGA = regexp.MustCompile("^::([^ ]+)( (.+))?::([^\r\n]*)[\r\n]+$")
	commandPatternADO = regexp.MustCompile("^##\\[([^ ]+)( (.+))?\\]([^\r\n]*)[\r\n]+$")
	commandPatternEscapeChar1 = regexp.MustCompile("%25") // %
	commandPatternEscapeChar2 = regexp.MustCompile("%0D") // \r
	commandPatternEscapeChar3 = regexp.MustCompile("%0A") // \n
	commandPatternEscapeChar4 = regexp.MustCompile("%3A") // :
	commandPatternEscapeChar5 = regexp.MustCompile("%2C") // ,

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
		for k, v := range kvPairs {
			kvPairs[k] = unescapeCommandProperty(v)
		}
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
	common.Logger(ctx).Infof("  \U00002699  ::set-output:: %s=%s", kvPairs["name"], arg)
	rc.StepResults[rc.CurrentStep].Outputs[kvPairs["name"]] = arg
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
	// unescape command data string
	arg = commandPatternEscapeChar1.ReplaceAllString(arg, "%")
	arg = commandPatternEscapeChar2.ReplaceAllString(arg, "\r")
	arg = commandPatternEscapeChar3.ReplaceAllString(arg, "\n")
	return arg
}
func unescapeCommandProperty(arg string) string {
	arg = commandPatternEscapeChar1.ReplaceAllString(arg, "%")
	arg = commandPatternEscapeChar2.ReplaceAllString(arg, "\r")
	arg = commandPatternEscapeChar3.ReplaceAllString(arg, "\n")
	arg = commandPatternEscapeChar4.ReplaceAllString(arg, ":")
	arg = commandPatternEscapeChar5.ReplaceAllString(arg, ",")
	return arg
}
