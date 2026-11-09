package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

const trustProjectActrcFlag = "trust-project-actrc"

const projectFlagDisallowed = "this flag is not allowed in a project .actrc"

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
		parsed, reason := parseProjectLine(flags, line)
		if reason != "" {
			log.Warnf("Ignoring %q from %s: %s. Set it in your user config or on the command line, or pass --%s.", line, absFile, reason, trustProjectActrcFlag)
			ignored = true
			continue
		}
		args = append(args, parsed...)
	}
	if ignored {
		log.Warnf("%s is a project file. It may only set flags that do not reach host resources.", absFile)
	}
	return args
}

// parseProjectLine converts one project .actrc line into argv.
//
// Each line is a single flag. Its value is an "=" suffix, an attached
// shorthand, or the remainder of the line. A second flag token rejects the
// line, so a boolean flag or an already-valued flag cannot smuggle another
// argument. A value-taking flag with no value on that line is rejected, so
// it cannot consume the next line or a later argument. Clustered shorthands
// are accepted only when every letter is allowlisted and none of them take a
// value; they are expanded to one flag per letter before pflag sees them.
func parseProjectLine(flags []*pflag.FlagSet, line string) ([]string, string) {
	tok, rest, hasRest := cutActrcField(line)
	if strings.HasPrefix(tok, "--") {
		return parseLongProjectFlag(flags, tok, rest, hasRest)
	}
	if strings.HasPrefix(tok, "-") {
		return parseShortProjectFlag(flags, tok, rest, hasRest)
	}
	return nil, projectFlagDisallowed
}

func cutActrcField(line string) (string, string, bool) {
	parts := whitespace.Split(strings.TrimSpace(line), 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", "", false
	}
	if len(parts) == 1 {
		return parts[0], "", false
	}
	rest := strings.TrimSpace(parts[1])
	if rest == "" {
		return parts[0], "", false
	}
	return parts[0], rest, true
}

func parseLongProjectFlag(flags []*pflag.FlagSet, tok, rest string, hasRest bool) ([]string, string) {
	body := strings.TrimPrefix(tok, "--")
	if body == "" || body[0] == '-' || body[0] == '=' {
		return nil, projectFlagDisallowed
	}
	name, inline, hasInline := strings.Cut(body, "=")
	f := lookupLongFlag(flags, name)
	if f == nil {
		return nil, projectFlagDisallowed
	}
	if !flagTakesValue(f) {
		return parseBoolProjectFlag(name, inline, hasInline, tok, rest, hasRest)
	}
	if hasInline {
		if reason := inlineProjectValue(inline, rest, hasRest); reason != "" {
			return nil, reason
		}
		if reason := allowProjectFlag(name, inline); reason != "" {
			return nil, reason
		}
		return []string{tok}, ""
	}
	value, reason := separateProjectValue(rest, hasRest)
	if reason != "" {
		return nil, reason
	}
	if reason := allowProjectFlag(name, value); reason != "" {
		return nil, reason
	}
	return []string{"--" + name, value}, ""
}

func parseShortProjectFlag(flags []*pflag.FlagSet, tok, rest string, hasRest bool) ([]string, string) {
	body := strings.TrimPrefix(tok, "-")
	if body == "" || body[0] == '-' {
		return nil, projectFlagDisallowed
	}
	f := lookupShortFlag(flags, body[:1])
	if f == nil {
		return nil, projectFlagDisallowed
	}
	tail := body[1:]
	if tail == "" {
		return parseShortBare(f, tok, rest, hasRest)
	}
	if strings.HasPrefix(tail, "=") {
		return parseShortEquals(f, tok, strings.TrimPrefix(tail, "="), rest, hasRest)
	}
	if flagTakesValue(f) {
		return parseShortAttached(f, tok, tail, rest, hasRest)
	}
	if hasRest {
		return nil, unexpectedProjectToken(rest)
	}
	return parseBoolCluster(flags, body)
}

func parseShortBare(f *pflag.Flag, tok, rest string, hasRest bool) ([]string, string) {
	if !flagTakesValue(f) {
		return parseBoolProjectFlag(f.Name, "", false, tok, rest, hasRest)
	}
	value, reason := separateProjectValue(rest, hasRest)
	if reason != "" {
		return nil, reason
	}
	if reason := allowProjectFlag(f.Name, value); reason != "" {
		return nil, reason
	}
	return []string{tok, value}, ""
}

func parseShortEquals(f *pflag.Flag, tok, inline, rest string, hasRest bool) ([]string, string) {
	if !flagTakesValue(f) {
		return parseBoolProjectFlag(f.Name, inline, true, tok, rest, hasRest)
	}
	if reason := inlineProjectValue(inline, rest, hasRest); reason != "" {
		return nil, reason
	}
	if reason := allowProjectFlag(f.Name, inline); reason != "" {
		return nil, reason
	}
	return []string{tok}, ""
}

func parseShortAttached(f *pflag.Flag, tok, value, rest string, hasRest bool) ([]string, string) {
	if hasRest {
		return nil, unexpectedProjectToken(rest)
	}
	if reason := allowProjectFlag(f.Name, value); reason != "" {
		return nil, reason
	}
	return []string{tok}, ""
}

func parseBoolProjectFlag(name, inline string, hasInline bool, tok, rest string, hasRest bool) ([]string, string) {
	if hasRest {
		return nil, unexpectedProjectToken(rest)
	}
	if hasInline {
		if _, err := strconv.ParseBool(inline); err != nil {
			return nil, "a boolean flag value must be true or false"
		}
	}
	if reason := allowProjectFlag(name, ""); reason != "" {
		return nil, reason
	}
	return []string{tok}, ""
}

func parseBoolCluster(flags []*pflag.FlagSet, body string) ([]string, string) {
	args := make([]string, 0, len(body))
	for _, r := range body {
		sh := string(r)
		f := lookupShortFlag(flags, sh)
		if f == nil {
			return nil, projectFlagDisallowed
		}
		if flagTakesValue(f) {
			return nil, "clustered shorthand includes a flag that takes a value"
		}
		if reason := allowProjectFlag(f.Name, ""); reason != "" {
			return nil, reason
		}
		args = append(args, "-"+sh)
	}
	return args, ""
}

func inlineProjectValue(value, rest string, hasRest bool) string {
	if hasRest {
		return unexpectedProjectToken(rest)
	}
	if value == "" {
		return "flag needs a value on the same line"
	}
	return ""
}

func separateProjectValue(rest string, hasRest bool) (string, string) {
	if !hasRest {
		return "", "flag needs a value on the same line"
	}
	if extra := flagLikeField(rest); extra != "" {
		return "", unexpectedProjectToken(extra)
	}
	return rest, ""
}

func unexpectedProjectToken(rest string) string {
	return fmt.Sprintf("unexpected extra token %q", firstActrcField(rest))
}

func firstActrcField(rest string) string {
	for _, field := range whitespace.Split(rest, -1) {
		if field != "" {
			return field
		}
	}
	return rest
}

func flagLikeField(rest string) string {
	for _, field := range whitespace.Split(rest, -1) {
		if field != "" && strings.HasPrefix(field, "-") {
			return field
		}
	}
	return ""
}

func lookupLongFlag(flags []*pflag.FlagSet, name string) *pflag.Flag {
	for _, fs := range flags {
		if f := fs.Lookup(name); f != nil {
			return f
		}
	}
	return nil
}

func lookupShortFlag(flags []*pflag.FlagSet, shorthand string) *pflag.Flag {
	for _, fs := range flags {
		if f := fs.ShorthandLookup(shorthand); f != nil {
			return f
		}
	}
	return nil
}

func flagTakesValue(f *pflag.Flag) bool {
	return f.NoOptDefVal == ""
}

func allowProjectFlag(name, value string) string {
	check, ok := projectActrcFlags[name]
	if !ok {
		return projectFlagDisallowed
	}
	if check != nil {
		return check(value)
	}
	return ""
}
