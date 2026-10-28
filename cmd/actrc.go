package cmd

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

const trustProjectActrcFlag = "trust-project-actrc"

// projectActrcFlags lists the flags an untrusted ./.actrc may set. They
// select what to run and how to present it. Flags that reach host
// resources, change container isolation, or point act at other services are
// left out; set those in the user config or on the command line.
var projectActrcFlags = map[string]projectFlagCheck{
	"platform":               checkPlatform,
	"env":                    checkAssignment,
	"var":                    checkAssignment,
	"input":                  checkAssignment,
	"matrix":                 nil,
	"actor":                  nil,
	"defaultbranch":          nil,
	"detect-event":           nil,
	"eventpath":              checkLocalPath,
	"workflows":              checkLocalPath,
	"no-recurse":             nil,
	"job":                    nil,
	"remote-name":            nil,
	"pull":                   nil,
	"rebuild":                nil,
	"reuse":                  nil,
	"rm":                     nil,
	"bind":                   nil,
	"use-gitignore":          nil,
	"container-architecture": nil,
	"json":                   nil,
	"log-prefix-job-id":      nil,
	"quiet":                  nil,
	"verbose":                nil,
	"strict":                 nil,
	"validate":               nil,
	"concurrent-jobs":        nil,
	"no-skip-checkout":       nil,
	"use-new-action-cache":   nil,
	"action-offline-mode":    nil,
	"secret-file":            checkLocalPath,
	"var-file":               checkLocalPath,
	"env-file":               checkLocalPath,
	"input-file":             checkLocalPath,
}

type projectFlagCheck func(value string) string

// checkPlatform rejects self-hosted platforms, which run steps on the host.
func checkPlatform(value string) string {
	_, image, _ := strings.Cut(value, "=")
	if strings.TrimSpace(image) == "-self-hosted" {
		return "self-hosted platforms run steps on the host"
	}
	return ""
}

// checkAssignment requires NAME=value. A bare NAME copies the value from the
// host environment.
func checkAssignment(value string) string {
	if !strings.Contains(value, "=") {
		return "a value without = is read from the host environment"
	}
	return ""
}

func checkLocalPath(value string) string {
	if !filepath.IsLocal(filepath.FromSlash(value)) {
		return "the path must stay inside the project directory"
	}
	return ""
}

// actrcArgs returns the arguments from the user config files and ./.actrc,
// followed by the command line arguments. ./.actrc is trusted like a user
// config only with --trust-project-actrc. Otherwise it is read without
// environment expansion and only flags in projectActrcFlags are kept.
func actrcArgs(flags []*pflag.FlagSet, userFiles []string, projectFile string, cliArgs []string) []string {
	args := make([]string, 0)
	for _, f := range userFiles {
		args = append(args, readArgsFile(f, true)...)
	}
	if !isUserConfigFile(projectFile, userFiles) {
		if trustProjectActrc(append(append([]string{}, args...), cliArgs...)) {
			args = append(args, readArgsFile(projectFile, true)...)
		} else {
			args = append(args, readProjectArgsFile(flags, projectFile)...)
		}
	}
	return append(args, cliArgs...)
}

// isUserConfigFile reports whether file is one of the user config files, as
// when act runs from the home directory.
func isUserConfigFile(file string, userFiles []string) bool {
	fi, err := os.Stat(file)
	if err != nil {
		return false
	}
	for _, u := range userFiles {
		if ui, err := os.Stat(u); err == nil && os.SameFile(fi, ui) {
			return true
		}
	}
	return false
}

func trustProjectActrc(args []string) bool {
	trust := false
	for _, a := range args {
		if a == "--"+trustProjectActrcFlag {
			trust = true
		} else if v, ok := strings.CutPrefix(a, "--"+trustProjectActrcFlag+"="); ok {
			if b, err := strconv.ParseBool(v); err == nil {
				trust = b
			}
		}
	}
	return trust
}

var whitespace = regexp.MustCompile(`\s`)

func readProjectArgsFile(flags []*pflag.FlagSet, file string) []string {
	args := make([]string, 0)
	f, err := os.Open(file)
	if err != nil {
		return args
	}
	defer f.Close()
	absFile, _ := filepath.Abs(file)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, 1024*1024)
	ignored := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "-") {
			continue
		}
		parts := whitespace.Split(line, 2)
		name, value := projectFlag(flags, parts)
		if reason := allowProjectFlag(name, value); reason != "" {
			log.Warnf("Ignoring %q from %s: %s. Set it in your user config or on the command line, or pass --%s.", line, absFile, reason, trustProjectActrcFlag)
			ignored = true
			continue
		}
		args = append(args, parts...)
	}
	if ignored {
		log.Warnf("%s is a project file. It may only set flags that do not reach host resources.", absFile)
	}
	return args
}

// projectFlag returns the long flag name and value of one .actrc line.
func projectFlag(flags []*pflag.FlagSet, parts []string) (string, string) {
	tok := parts[0]
	value := ""
	if len(parts) > 1 {
		value = strings.TrimSpace(parts[1])
	}
	if long, ok := strings.CutPrefix(tok, "--"); ok {
		if n, v, found := strings.Cut(long, "="); found {
			return n, v
		}
		return long, value
	}
	short := strings.TrimPrefix(tok, "-")
	if short == "" {
		return "", value
	}
	var f *pflag.Flag
	for _, fs := range flags {
		if f = fs.ShorthandLookup(short[:1]); f != nil {
			break
		}
	}
	if f == nil {
		return "", value
	}
	if len(short) > 1 {
		value = strings.TrimPrefix(short[1:], "=")
	}
	return f.Name, value
}

func allowProjectFlag(name, value string) string {
	check, ok := projectActrcFlags[name]
	if !ok {
		return "this flag is not allowed in a project .actrc"
	}
	if check != nil {
		return check(value)
	}
	return ""
}
