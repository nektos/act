package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/robertkrimen/otto"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"gopkg.in/godo.v2/glob"
)

var contextPattern, expressionPattern *regexp.Regexp

func init() {
	contextPattern = regexp.MustCompile(`^(\w+(?:\[.+\])*)(?:\.([\w-]+))?(.*)$`)
	expressionPattern = regexp.MustCompile(`\${{\s*(.+?)\s*}}`)
}

// NewExpressionEvaluator creates a new evaluator
func (rc *RunContext) NewExpressionEvaluator() ExpressionEvaluator {
	vm := rc.newVM()
	return &expressionEvaluator{
		vm,
	}
}

// NewExpressionEvaluator creates a new evaluator
func (sc *StepContext) NewExpressionEvaluator() ExpressionEvaluator {
	vm := sc.RunContext.newVM()
	configers := []func(*otto.Otto){
		sc.vmEnv(),
		sc.vmInputs(),
	}
	for _, configer := range configers {
		configer(vm)
	}

	return &expressionEvaluator{
		vm,
	}
}

// ExpressionEvaluator is the interface for evaluating expressions
type ExpressionEvaluator interface {
	Evaluate(string) (string, error)
	Interpolate(string) string
	Rewrite(string) string
}

type expressionEvaluator struct {
	vm *otto.Otto
}

func (ee *expressionEvaluator) Evaluate(in string) (string, error) {
	re := ee.Rewrite(in)
	if re != in {
		logrus.Debugf("Evaluating '%s' instead of '%s'", re, in)
	}

	val, err := ee.vm.Run(re)
	if err != nil {
		return "", err
	}
	if val.IsNull() || val.IsUndefined() {
		return "", nil
	}
	return val.ToString()
}

func (ee *expressionEvaluator) Interpolate(in string) string {
	errList := make([]error, 0)

	out := in
	for {
		out = expressionPattern.ReplaceAllStringFunc(in, func(match string) string {
			// Extract and trim the actual expression inside ${{...}} delimiters
			expression := expressionPattern.ReplaceAllString(match, "$1")
			// Evaluate the expression and retrieve errors if any
			evaluated, err := ee.Evaluate(expression)
			if err != nil {
				errList = append(errList, err)
			}
			return evaluated
		})
		if len(errList) > 0 {
			logrus.Errorf("Unable to interpolate string '%s' - %v", in, errList)
			break
		}
		if out == in {
			// No replacement occurred, we're done!
			break
		}
		in = out
	}
	return out
}

// Rewrite tries to transform any javascript property accessor into its bracket notation.
// For instance, "object.property" would become "object['property']".
func (ee *expressionEvaluator) Rewrite(in string) string {
	re := in
	for {
		matches := contextPattern.FindStringSubmatch(re)
		if matches == nil {
			// No global match, we're done!
			break
		}
		if matches[2] == "" {
			// No property match, we're done!
			break
		}

		re = fmt.Sprintf("%s['%s']%s", matches[1], matches[2], matches[3])
	}

	return re
}

func (rc *RunContext) newVM() *otto.Otto {
	configers := []func(*otto.Otto){
		vmContains,
		vmStartsWith,
		vmEndsWith,
		vmFormat,
		vmJoin,
		vmToJSON,
		vmFromJSON,
		vmAlways,
		rc.vmCancelled(),
		rc.vmSuccess(),
		rc.vmFailure(),
		rc.vmHashFiles(),

		rc.vmGithub(),
		rc.vmJob(),
		rc.vmSteps(),
		rc.vmRunner(),

		rc.vmSecrets(),
		rc.vmStrategy(),
		rc.vmMatrix(),
		rc.vmEnv(),
	}
	vm := otto.New()
	for _, configer := range configers {
		configer(vm)
	}
	return vm
}

func vmContains(vm *otto.Otto) {
	_ = vm.Set("contains", func(searchString interface{}, searchValue string) bool {
		if searchStringString, ok := searchString.(string); ok {
			return strings.Contains(strings.ToLower(searchStringString), strings.ToLower(searchValue))
		} else if searchStringArray, ok := searchString.([]string); ok {
			for _, s := range searchStringArray {
				if strings.EqualFold(s, searchValue) {
					return true
				}
			}
		}
		return false
	})
}

func vmStartsWith(vm *otto.Otto) {
	_ = vm.Set("startsWith", func(searchString string, searchValue string) bool {
		return strings.HasPrefix(strings.ToLower(searchString), strings.ToLower(searchValue))
	})
}

func vmEndsWith(vm *otto.Otto) {
	_ = vm.Set("endsWith", func(searchString string, searchValue string) bool {
		return strings.HasSuffix(strings.ToLower(searchString), strings.ToLower(searchValue))
	})
}

func vmFormat(vm *otto.Otto) {
	_ = vm.Set("format", func(s string, vals ...string) string {
		for i, v := range vals {
			s = strings.ReplaceAll(s, fmt.Sprintf("{%d}", i), v)
		}
		return s
	})
}

func vmJoin(vm *otto.Otto) {
	_ = vm.Set("join", func(element interface{}, optionalElem string) string {
		slist := make([]string, 0)
		if elementString, ok := element.(string); ok {
			slist = append(slist, elementString)
		} else if elementArray, ok := element.([]string); ok {
			slist = append(slist, elementArray...)
		}
		if optionalElem != "" {
			slist = append(slist, optionalElem)
		}
		return strings.Join(slist, " ")
	})
}

func vmToJSON(vm *otto.Otto) {
	toJSON := func(o interface{}) string {
		rtn, err := json.MarshalIndent(o, "", "  ")
		if err != nil {
			logrus.Errorf("Unable to marshal: %v", err)
			return ""
		}
		return string(rtn)
	}
	_ = vm.Set("toJSON", toJSON)
	_ = vm.Set("toJson", toJSON)
}

func vmFromJSON(vm *otto.Otto) {
	fromJSON := func(str string) map[string]interface{} {
		var dat map[string]interface{}
		err := json.Unmarshal([]byte(str), &dat)
		if err != nil {
			logrus.Errorf("Unable to unmarshal: %v", err)
			return dat
		}
		return dat
	}
	_ = vm.Set("fromJSON", fromJSON)
	_ = vm.Set("fromJson", fromJSON)
}

func (rc *RunContext) vmHashFiles() func(*otto.Otto) {
	return func(vm *otto.Otto) {
		_ = vm.Set("hashFiles", func(path string) string {
			files, _, err := glob.Glob([]string{filepath.Join(rc.Config.Workdir, path)})
			if err != nil {
				logrus.Errorf("Unable to glob.Glob: %v", err)
				return ""
			}
			hasher := sha256.New()
			for _, file := range files {
				f, err := os.Open(file.Path)
				if err != nil {
					logrus.Errorf("Unable to os.Open: %v", err)
				}
				defer f.Close()
				if _, err := io.Copy(hasher, f); err != nil {
					logrus.Errorf("Unable to io.Copy: %v", err)
				}
			}
			return hex.EncodeToString(hasher.Sum(nil))
		})
	}
}

func (rc *RunContext) vmSuccess() func(*otto.Otto) {
	return func(vm *otto.Otto) {
		_ = vm.Set("success", func() bool {
			return rc.getJobContext().Status == "success"
		})
	}
}
func (rc *RunContext) vmFailure() func(*otto.Otto) {
	return func(vm *otto.Otto) {
		_ = vm.Set("failure", func() bool {
			return rc.getJobContext().Status == "failure"
		})
	}
}

func vmAlways(vm *otto.Otto) {
	_ = vm.Set("always", func() bool {
		return true
	})
}
func (rc *RunContext) vmCancelled() func(vm *otto.Otto) {
	return func(vm *otto.Otto) {
		_ = vm.Set("cancelled", func() bool {
			return rc.getJobContext().Status == "cancelled"
		})
	}
}

func (rc *RunContext) vmGithub() func(*otto.Otto) {
	github := rc.getGithubContext()

	return func(vm *otto.Otto) {
		_ = vm.Set("github", github)
	}
}

func (rc *RunContext) vmEnv() func(*otto.Otto) {
	return func(vm *otto.Otto) {
		env := rc.GetEnv()
		log.Debugf("context env => %v", env)
		_ = vm.Set("env", env)
	}
}

func (sc *StepContext) vmEnv() func(*otto.Otto) {
	return func(vm *otto.Otto) {
		log.Debugf("context env => %v", sc.Env)
		_ = vm.Set("env", sc.Env)
	}
}

func (sc *StepContext) vmInputs() func(*otto.Otto) {
	inputs := make(map[string]string)

	// Set Defaults
	if sc.Action != nil {
		for k, input := range sc.Action.Inputs {
			inputs[k] = input.Default
		}
	}

	for k, v := range sc.Step.With {
		inputs[k] = v
	}
	return func(vm *otto.Otto) {
		_ = vm.Set("inputs", inputs)
	}
}

func (rc *RunContext) vmJob() func(*otto.Otto) {
	job := rc.getJobContext()

	return func(vm *otto.Otto) {
		_ = vm.Set("job", job)
	}
}

func (rc *RunContext) vmSteps() func(*otto.Otto) {
	steps := rc.getStepsContext()

	return func(vm *otto.Otto) {
		_ = vm.Set("steps", steps)
	}
}

func (rc *RunContext) vmRunner() func(*otto.Otto) {
	runner := map[string]interface{}{
		"os":         "Linux",
		"temp":       "/tmp",
		"tool_cache": "/opt/hostedtoolcache",
	}

	return func(vm *otto.Otto) {
		_ = vm.Set("runner", runner)
	}
}

func (rc *RunContext) vmSecrets() func(*otto.Otto) {
	return func(vm *otto.Otto) {
		_ = vm.Set("secrets", rc.Config.Secrets)
	}
}

func (rc *RunContext) vmStrategy() func(*otto.Otto) {
	job := rc.Run.Job()
	strategy := make(map[string]interface{})
	if job.Strategy != nil {
		strategy["fail-fast"] = job.Strategy.FailFast
		strategy["max-parallel"] = job.Strategy.MaxParallel
	}
	return func(vm *otto.Otto) {
		_ = vm.Set("strategy", strategy)
	}
}

func (rc *RunContext) vmMatrix() func(*otto.Otto) {
	return func(vm *otto.Otto) {
		_ = vm.Set("matrix", rc.Matrix)
	}
}
