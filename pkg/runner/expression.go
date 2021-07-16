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
	log "github.com/sirupsen/logrus"
)

var expressionPattern, operatorPattern *regexp.Regexp

func init() {
	expressionPattern = regexp.MustCompile(`\${{\s*(.+?)\s*}}`)
	operatorPattern = regexp.MustCompile("^[!=><|&]+$")
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
	Evaluate(string) (string, bool, error)
	Interpolate(string) string
	InterpolateWithStringCheck(string) (string, bool)
	Rewrite(string) string
}

type expressionEvaluator struct {
	vm *otto.Otto
}

func (ee *expressionEvaluator) Evaluate(in string) (string, bool, error) {
	if strings.HasPrefix(in, `secrets.`) {
		in = `secrets.` + strings.ToUpper(strings.SplitN(in, `.`, 2)[1])
	}
	re := ee.Rewrite(in)
	if re != in {
		log.Debugf("Evaluating '%s' instead of '%s'", re, in)
	}

	val, err := ee.vm.Run(re)
	if err != nil {
		return "", false, err
	}
	if val.IsNull() || val.IsUndefined() {
		return "", false, nil
	}
	valAsString, err := val.ToString()
	if err != nil {
		return "", false, err
	}

	return valAsString, val.IsString(), err
}

func (ee *expressionEvaluator) Interpolate(in string) string {
	interpolated, _ := ee.InterpolateWithStringCheck(in)
	return interpolated
}

func (ee *expressionEvaluator) InterpolateWithStringCheck(in string) (string, bool) {
	errList := make([]error, 0)

	out := in
	isString := false
	for {
		out = expressionPattern.ReplaceAllStringFunc(in, func(match string) string {
			// Extract and trim the actual expression inside ${{...}} delimiters
			expression := expressionPattern.ReplaceAllString(match, "$1")

			// Evaluate the expression and retrieve errors if any
			evaluated, evaluatedIsString, err := ee.Evaluate(expression)
			if err != nil {
				errList = append(errList, err)
			}
			isString = evaluatedIsString
			return evaluated
		})
		if len(errList) > 0 {
			log.Errorf("Unable to interpolate string '%s' - %v", in, errList)
			break
		}
		if out == in {
			// No replacement occurred, we're done!
			break
		}
		in = out
	}
	return out, isString
}

// Rewrite tries to transform any javascript property accessor into its bracket notation.
// For instance, "object.property" would become "object['property']".
func (ee *expressionEvaluator) Rewrite(in string) string {
	var buf strings.Builder
	r := strings.NewReader(in)
	for {
		c, _, err := r.ReadRune()
		if err == io.EOF {
			break
		}
		//nolint
		switch {
		default:
			buf.WriteRune(c)
		case c == '\'':
			buf.WriteRune(c)
			ee.advString(&buf, r)
		case c == '.':
			buf.WriteString("['")
			ee.advPropertyName(&buf, r)
			buf.WriteString("']")
		}
	}
	return buf.String()
}

func (*expressionEvaluator) advString(w *strings.Builder, r *strings.Reader) error {
	for {
		c, _, err := r.ReadRune()
		if err != nil {
			return err
		}
		if c != '\'' {
			w.WriteRune(c) //nolint
			continue
		}

		// Handles a escaped string: ex. 'It''s ok'
		c, _, err = r.ReadRune()
		if err != nil {
			w.WriteString("'") //nolint
			return err
		}
		if c != '\'' {
			w.WriteString("'") //nolint
			if err := r.UnreadRune(); err != nil {
				return err
			}
			break
		}
		w.WriteString(`\'`) //nolint
	}
	return nil
}

func (*expressionEvaluator) advPropertyName(w *strings.Builder, r *strings.Reader) error {
	for {
		c, _, err := r.ReadRune()
		if err != nil {
			return err
		}
		if !isLetter(c) {
			if err := r.UnreadRune(); err != nil {
				return err
			}
			break
		}
		w.WriteRune(c) //nolint
	}
	return nil
}

func isLetter(c rune) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '_' || c == '-':
		return true
	default:
		return false
	}
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
		rc.vmNeeds(),
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
			log.Errorf("Unable to marshal: %v", err)
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
			log.Errorf("Unable to unmarshal: %v", err)
			return dat
		}
		return dat
	}
	_ = vm.Set("fromJSON", fromJSON)
	_ = vm.Set("fromJson", fromJSON)
}

func (rc *RunContext) vmHashFiles() func(*otto.Otto) {
	return func(vm *otto.Otto) {
		_ = vm.Set("hashFiles", func(paths ...string) string {
			var files []string
			for i := range paths {
				newFiles, err := filepath.Glob(filepath.Join(rc.Config.Workdir, paths[i]))
				if err != nil {
					log.Errorf("Unable to glob.Glob: %v", err)
					return ""
				}
				files = append(files, newFiles...)
			}
			hasher := sha256.New()
			for _, file := range files {
				f, err := os.Open(file)
				if err != nil {
					log.Errorf("Unable to os.Open: %v", err)
				}
				if _, err := io.Copy(hasher, f); err != nil {
					log.Errorf("Unable to io.Copy: %v", err)
				}
				if err := f.Close(); err != nil {
					log.Errorf("Unable to Close file: %v", err)
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
			inputs[k] = sc.RunContext.NewExpressionEvaluator().Interpolate(input.Default)
		}
	}

	for k, v := range sc.Step.With {
		inputs[k] = sc.RunContext.NewExpressionEvaluator().Interpolate(v)
	}

	return func(vm *otto.Otto) {
		_ = vm.Set("inputs", inputs)
	}
}

func (rc *RunContext) vmNeeds() func(*otto.Otto) {
	jobs := rc.Run.Workflow.Jobs
	jobNeeds := rc.Run.Job().Needs()

	using := make(map[string]map[string]map[string]string)
	for _, needs := range jobNeeds {
		using[needs] = map[string]map[string]string{
			"outputs": jobs[needs].Outputs,
		}
	}

	return func(vm *otto.Otto) {
		log.Debugf("context needs => %v", using)
		_ = vm.Set("needs", using)
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
