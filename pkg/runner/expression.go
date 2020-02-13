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
	"gopkg.in/godo.v2/glob"
)

const prefix = "${{"
const suffix = "}}"

var pattern *regexp.Regexp

func init() {
	pattern = regexp.MustCompile(fmt.Sprintf("\\%s.+?%s", prefix, suffix))
}

// NewExpressionEvaluator creates a new evaluator
func (rc *RunContext) NewExpressionEvaluator() ExpressionEvaluator {
	vm := rc.newVM()
	return &expressionEvaluator{
		vm,
	}
}

// ExpressionEvaluator is the interface for evaluating expressions
type ExpressionEvaluator interface {
	Evaluate(string) (string, error)
	Interpolate(string) (string, error)
}

type expressionEvaluator struct {
	vm *otto.Otto
}

func (ee *expressionEvaluator) Evaluate(in string) (string, error) {
	val, err := ee.vm.Run(in)
	if err != nil {
		return "", err
	}
	return val.ToString()
}

func (ee *expressionEvaluator) Interpolate(in string) (string, error) {
	errList := make([]error, 0)
	out := pattern.ReplaceAllStringFunc(in, func(match string) string {
		expression := strings.TrimPrefix(strings.TrimSuffix(match, suffix), prefix)
		evaluated, err := ee.Evaluate(expression)
		if err != nil {
			errList = append(errList, err)
		}
		return evaluated
	})
	if len(errList) > 0 {
		return "", fmt.Errorf("Unable to interpolate string '%s' - %v", in, errList)
	}
	return out, nil
}

func (rc *RunContext) newVM() *otto.Otto {
	configers := []func(*otto.Otto){
		vmContains,
		vmStartsWith,
		vmEndsWith,
		vmFormat,
		vmJoin,
		vmToJSON,
		vmAlways,
		vmCancelled,
		rc.vmHashFiles(),
		rc.vmSuccess(),
		rc.vmFailure(),

		rc.vmGithub(),
		rc.vmEnv(),
		rc.vmJob(),
		rc.vmSteps(),
		rc.vmRunner(),
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
	_ = vm.Set("toJSON", func(o interface{}) string {
		rtn, err := json.MarshalIndent(o, "", "  ")
		if err != nil {
			logrus.Errorf("Unable to marsal: %v", err)
			return ""
		}
		return string(rtn)
	})
}

func (rc *RunContext) vmHashFiles() func(*otto.Otto) {
	return func(vm *otto.Otto) {
		_ = vm.Set("hashFiles", func(path string) string {
			files, _, err := glob.Glob([]string{filepath.Join(rc.Config.Workdir, path)})
			if err != nil {
				logrus.Error(err)
				return ""
			}
			hasher := sha256.New()
			for _, file := range files {
				f, err := os.Open(file.Path)
				if err != nil {
					logrus.Error(err)
				}
				defer f.Close()
				if _, err := io.Copy(hasher, f); err != nil {
					logrus.Error(err)
				}
			}
			return hex.EncodeToString(hasher.Sum(nil))
		})
	}
}

func (rc *RunContext) vmSuccess() func(*otto.Otto) {
	return func(vm *otto.Otto) {
		_ = vm.Set("success", func() bool {
			return !rc.PriorStepFailed
		})
	}
}
func (rc *RunContext) vmFailure() func(*otto.Otto) {
	return func(vm *otto.Otto) {
		_ = vm.Set("failure", func() bool {
			return rc.PriorStepFailed
		})
	}
}

func vmAlways(vm *otto.Otto) {
	_ = vm.Set("always", func() bool {
		return true
	})
}
func vmCancelled(vm *otto.Otto) {
	_ = vm.Set("cancelled", func() bool {
		return false
	})
}

func (rc *RunContext) vmGithub() func(*otto.Otto) {
	github := map[string]interface{}{
		"event":      make(map[string]interface{}),
		"event_path": "/github/workflow/event.json",
		"workflow":   rc.Run.Workflow.Name,
		"run_id":     "1",
		"run_number": "1",
		"actor":      "nektos/act",

		// TODO
		"repository": "",
		"event_name": "",
		"sha":        "",
		"ref":        "",
		"head_ref":   "",
		"base_ref":   "",
		"token":      "",
		"workspace":  rc.Config.Workdir,
		"action":     "",
	}

	err := json.Unmarshal([]byte(rc.EventJSON), github["event"])
	if err != nil {
		logrus.Error(err)
	}

	return func(vm *otto.Otto) {
		_ = vm.Set("github", github)
	}
}

func (rc *RunContext) vmEnv() func(*otto.Otto) {
	env := map[string]interface{}{}
	// TODO

	return func(vm *otto.Otto) {
		_ = vm.Set("env", env)
	}
}

func (rc *RunContext) vmJob() func(*otto.Otto) {
	job := map[string]interface{}{}
	// TODO

	return func(vm *otto.Otto) {
		_ = vm.Set("job", job)
	}
}

func (rc *RunContext) vmSteps() func(*otto.Otto) {
	steps := map[string]interface{}{}
	// TODO

	return func(vm *otto.Otto) {
		_ = vm.Set("steps", steps)
	}
}

func (rc *RunContext) vmRunner() func(*otto.Otto) {
	runner := map[string]interface{}{
		"os":         "Linux",
		"temp":       "/tmp",
		"tool_cache": "/tmp",
	}

	return func(vm *otto.Otto) {
		_ = vm.Set("runner", runner)
	}
}
