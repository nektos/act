package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

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
	Rewrite(string) (string, error)
}

type expressionEvaluator struct {
	vm *otto.Otto
}

func (ee *expressionEvaluator) Evaluate(in string) (string, bool, error) {
	re, err := ee.Rewrite(in)
	if err != nil {
		return "", false, err
	}
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
func (ee *expressionEvaluator) Rewrite(in string) (string, error) {
	var buf strings.Builder
	r := strings.NewReader(in)
	if err := ee.rewrite(r, &buf, nil); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func isRewriteEnd(c rune, eof *rune, err error) bool {
	return eof == nil && err == io.EOF || eof != nil && *eof == c && err == nil
}

func (ee *expressionEvaluator) rewrite(r *strings.Reader, buf *strings.Builder, eof *rune) error {
	secrets := "secrets"
	i := 0
	for {
		c, _, err := r.ReadRune()
		if isRewriteEnd(c, eof, err) {
			break
		} else if err != nil {
			return err
		}
		switch c {
		case '"', '*', '+', '/', '?', ':':
			return errors.New("Syntax Error")
		case '\'':
			if _, err := buf.WriteRune(c); err != nil {
				return err
			}
			if err := ee.advString(buf, r, false); err != nil {
				return err
			}
		case '.', '[':
			if err := ee.rewriteProperties(buf, r, i == len(secrets), c == '['); err != nil {
				return err
			}
		default:
			if isNumberStart(c) {
				if err := ee.validateNumber(buf, r); err != nil {
					return err
				}
			} else if _, err := buf.WriteRune(c); err != nil {
				return err
			}
		}
		if i < len(secrets) && c == rune(secrets[i]) {
			i++
		} else {
			i = 0
		}
	}
	return nil
}

func isNumberStart(c rune) bool {
	return c == '-' || (c >= '0' && c <= '9')
}

func (ee *expressionEvaluator) validateNumber(buf *strings.Builder, r *strings.Reader) error {
	if err := r.UnreadRune(); err != nil {
		return err
	}
	cur, _ := r.Seek(0, io.SeekCurrent)
	expr := regexp.MustCompile(`^(0x[0-9a-fA-F]+|-?[0-9]+.?[0-9]*([eE][-\+]?[0-9]+)?)`)
	match := expr.FindReaderIndex(r)
	if match == nil {
		return errors.New("Syntax Error")
	}
	// Rewind to copy content into the Buffer
	if _, err := r.Seek(cur, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.CopyN(buf, r, int64(match[1])); err != nil {
		return err
	}
	return nil
}

func (ee *expressionEvaluator) rewriteBracketProperties(buf *strings.Builder, r *strings.Reader, toUpper bool) error {
	// var c rune
	// for {
	// 	var err error
	// 	c, _, err = r.ReadRune()
	// 	if err == io.EOF {
	// 		return nil
	// 	} else if err != nil {
	// 		return err
	// 	}
	// 	if !unicode.IsSpace(c) {
	// 		break
	// 	}
	// }
	if _, err := buf.WriteString("[("); err != nil {
		return err
	}
	run := ']'
	if err := ee.rewrite(r, buf, &run); err != nil {
		return err
	}

	// if c != '\'' {
	// 	if err := r.UnreadRune(); err != nil {
	// 		return err
	// 	}
	// 	return nil
	// }
	// if _, err := buf.WriteString("'"); err != nil {
	// 	return err
	// }
	// if err := ee.advString(buf, r, toUpper); err != nil {
	// 	return err
	// }
	// for {
	// 	var err error
	// 	c, _, err = r.ReadRune()
	// 	if err == io.EOF {
	// 		return nil
	// 	} else if err != nil {
	// 		return err
	// 	}
	// 	if !unicode.IsSpace(c) {
	// 		break
	// 	}
	// }
	// if c != ']' {
	// 	return errors.New("Syntax Error")
	// }
	suffix := ")]"
	if toUpper {
		suffix = ").toUpperCase()]"
	}
	if _, err := buf.WriteString(suffix); err != nil {
		return err
	}
	return nil
}

func (ee *expressionEvaluator) rewritePlainProperties(buf *strings.Builder, r *strings.Reader, toUpper bool) error {
	c, _, err := r.ReadRune()
	if err == io.EOF {
		return nil
	}
	if c == '*' {
		c, _, err := r.ReadRune()
		if err == io.EOF {
			return nil
		}
		if c != '.' && c != '[' {
			if c == '-' {
				return errors.New("Syntax Error")
			}
			if err := r.UnreadRune(); err != nil {
				return err
			}
			return nil
		}
		if _, err := buf.WriteString(".map(e => e"); err != nil {
			return err
		}
		if err := ee.rewriteProperties(buf, r, toUpper, c == '['); err != nil {
			return err
		}
		if _, err := buf.WriteString(")"); err != nil {
			return err
		}
	} else if err := r.UnreadRune(); err != nil {
		return err
	} else {
		if _, err := buf.WriteString("['"); err != nil {
			return err
		}
		if err := ee.advPropertyName(buf, r, toUpper); err != nil {
			return err
		}
		if _, err := buf.WriteString("']"); err != nil {
			return err
		}
	}
	return nil
}

func (ee *expressionEvaluator) rewriteProperties(buf *strings.Builder, r *strings.Reader, toUpper bool, brackets bool) error {
	if brackets {
		return ee.rewriteBracketProperties(buf, r, toUpper)
	}
	return ee.rewritePlainProperties(buf, r, toUpper)
}

func conditionalToUpper(c rune, toUpper bool) rune {
	if toUpper {
		return unicode.ToUpper(c)
	}
	return c
}

func (*expressionEvaluator) advString(w *strings.Builder, r *strings.Reader, toUpper bool) error {
	for {
		c, _, err := r.ReadRune()
		if err != nil {
			return err
		}
		switch c {
		case '\'':
			// Handles a escaped string: ex. 'It''s ok'
			c, _, err = r.ReadRune()
			if errors.Is(err, io.EOF) {
				_, err := w.WriteString("'")
				return err
			} else if err != nil {
				return err
			}
			if c != '\'' {
				w.WriteString("'") //nolint
				return r.UnreadRune()
			}
			w.WriteString(`\'`) //nolint
		case '\\':
			w.WriteString(`\\`) //nolint
		case '\000':
			w.WriteString(`\0`) //nolint
		case '\f':
			w.WriteString(`\f`) //nolint
		case '\r':
			w.WriteString(`\r`) //nolint
		case '\n':
			w.WriteString(`\n`) //nolint
		case '\t':
			w.WriteString(`\t`) //nolint
		case '\v':
			w.WriteString(`\v`) //nolint
		case '\b':
			w.WriteString(`\b`) //nolint
		default:
			w.WriteRune(conditionalToUpper(c, toUpper)) //nolint
		}
	}
}

func (*expressionEvaluator) advPropertyName(w *strings.Builder, r *strings.Reader, toUpper bool) error {
	for {
		c, _, err := r.ReadRune()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if !isLetter(c) {
			if err := r.UnreadRune(); err != nil {
				return err
			}
			break
		}
		if _, err := w.WriteRune(conditionalToUpper(c, toUpper)); err != nil {
			return err
		}
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
		} else if searchStringArray, ok := searchString.([]interface{}); ok {
			for _, i := range searchStringArray {
				s, ok := i.(string)
				if ok && strings.EqualFold(s, searchValue) {
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
	_ = vm.Set("format", func(s string, vals ...otto.Value) string {
		ex := regexp.MustCompile(`(\{[0-9]+\}|\{.?|\}.?)`)
		return ex.ReplaceAllStringFunc(s, func(seg string) string {
			switch seg {
			case "{{":
				return "{"
			case "}}":
				return "}"
			default:
				if len(seg) < 3 || !strings.HasPrefix(seg, "{") {
					panic(vm.MakeSyntaxError(fmt.Sprintf("The following format string is invalid: '%v'", s)))
				}
				_i := seg[1 : len(seg)-1]
				i, err := strconv.ParseInt(_i, 10, 32)
				if err != nil {
					panic(vm.MakeSyntaxError(fmt.Sprintf("The following format string is invalid: '%v'. Error: %v", s, err)))
				}
				if i >= int64(len(vals)) {
					panic(vm.MakeSyntaxError(fmt.Sprintf("The following format string references more arguments than were supplied: '%v'", s)))
				}
				if vals[i].IsNull() || vals[i].IsUndefined() {
					return ""
				}
				return vals[i].String()
			}
		})
	})
}

func vmJoin(vm *otto.Otto) {
	_ = vm.Set("join", func(element interface{}, optionalElem string) string {
		slist := make([]string, 0)
		if elementString, ok := element.(string); ok {
			slist = append(slist, elementString)
		} else if elementArray, ok := element.([]string); ok {
			slist = append(slist, elementArray...)
		} else if elementArray, ok := element.([]interface{}); ok {
			stringArray := make([]string, len(elementArray))
			for i, v := range elementArray {
				s, ok := v.(string)
				if !ok {
					panic(vm.MakeTypeError(fmt.Sprintf("Array Element %v is not a string value '%v'", i, v)))
				}
				stringArray[i] = s
			}
			slist = append(slist, stringArray...)
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
	fromJSON := func(str string) interface{} {
		var dat interface{}
		err := json.Unmarshal([]byte(str), &dat)
		if err != nil {
			panic(vm.MakeCustomError("fromJSON", fmt.Sprintf("Unable to unmarshal json '%v', %v", str, err.Error())))
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
