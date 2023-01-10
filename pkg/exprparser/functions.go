package exprparser

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/nektos/act/pkg/model"
	"github.com/rhysd/actionlint"
)

func (impl *interpreterImpl) contains(search, item reflect.Value) (bool, error) {
	switch search.Kind() {
	case reflect.String, reflect.Int, reflect.Float64, reflect.Bool, reflect.Invalid:
		return strings.Contains(
			strings.ToLower(impl.coerceToString(search).String()),
			strings.ToLower(impl.coerceToString(item).String()),
		), nil

	case reflect.Slice:
		for i := 0; i < search.Len(); i++ {
			arrayItem := search.Index(i).Elem()
			result, err := impl.compareValues(arrayItem, item, actionlint.CompareOpNodeKindEq)
			if err != nil {
				return false, err
			}

			if isEqual, ok := result.(bool); ok && isEqual {
				return true, nil
			}
		}
	}

	return false, nil
}

func (impl *interpreterImpl) startsWith(searchString, searchValue reflect.Value) (bool, error) {
	return strings.HasPrefix(
		strings.ToLower(impl.coerceToString(searchString).String()),
		strings.ToLower(impl.coerceToString(searchValue).String()),
	), nil
}

func (impl *interpreterImpl) endsWith(searchString, searchValue reflect.Value) (bool, error) {
	return strings.HasSuffix(
		strings.ToLower(impl.coerceToString(searchString).String()),
		strings.ToLower(impl.coerceToString(searchValue).String()),
	), nil
}

const (
	passThrough = iota
	bracketOpen
	bracketClose
)

func (impl *interpreterImpl) format(str reflect.Value, replaceValue ...reflect.Value) (string, error) {
	input := impl.coerceToString(str).String()
	output := ""
	replacementIndex := ""

	state := passThrough
	for _, character := range input {
		switch state {
		case passThrough: // normal buffer output
			switch character {
			case '{':
				state = bracketOpen

			case '}':
				state = bracketClose

			default:
				output += string(character)
			}

		case bracketOpen: // found {
			switch character {
			case '{':
				output += "{"
				replacementIndex = ""
				state = passThrough

			case '}':
				index, err := strconv.ParseInt(replacementIndex, 10, 32)
				if err != nil {
					return "", fmt.Errorf("The following format string is invalid: '%s'", input)
				}

				replacementIndex = ""

				if len(replaceValue) <= int(index) {
					return "", fmt.Errorf("The following format string references more arguments than were supplied: '%s'", input)
				}

				output += impl.coerceToString(replaceValue[index]).String()

				state = passThrough

			default:
				replacementIndex += string(character)
			}

		case bracketClose: // found }
			switch character {
			case '}':
				output += "}"
				replacementIndex = ""
				state = passThrough

			default:
				panic("Invalid format parser state")
			}
		}
	}

	if state != passThrough {
		switch state {
		case bracketOpen:
			return "", fmt.Errorf("Unclosed brackets. The following format string is invalid: '%s'", input)

		case bracketClose:
			return "", fmt.Errorf("Closing bracket without opening one. The following format string is invalid: '%s'", input)
		}
	}

	return output, nil
}

func (impl *interpreterImpl) join(array reflect.Value, sep reflect.Value) (string, error) {
	separator := impl.coerceToString(sep).String()
	switch array.Kind() {
	case reflect.Slice:
		var items []string
		for i := 0; i < array.Len(); i++ {
			items = append(items, impl.coerceToString(array.Index(i).Elem()).String())
		}

		return strings.Join(items, separator), nil
	default:
		return strings.Join([]string{impl.coerceToString(array).String()}, separator), nil
	}
}

func (impl *interpreterImpl) toJSON(value reflect.Value) (string, error) {
	if value.Kind() == reflect.Invalid {
		return "null", nil
	}

	json, err := json.MarshalIndent(value.Interface(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("Cannot convert value to JSON. Cause: %v", err)
	}

	return string(json), nil
}

func (impl *interpreterImpl) fromJSON(value reflect.Value) (interface{}, error) {
	if value.Kind() != reflect.String {
		return nil, fmt.Errorf("Cannot parse non-string type %v as JSON", value.Kind())
	}

	var data interface{}

	err := json.Unmarshal([]byte(value.String()), &data)
	if err != nil {
		return nil, fmt.Errorf("Invalid JSON: %v", err)
	}

	return data, nil
}

func (impl *interpreterImpl) hashFiles(paths ...reflect.Value) (string, error) {
	var ps []gitignore.Pattern

	const cwdPrefix = "." + string(filepath.Separator)
	const excludeCwdPrefix = "!" + cwdPrefix
	for _, path := range paths {
		if path.Kind() == reflect.String {
			cleanPath := path.String()
			if strings.HasPrefix(cleanPath, cwdPrefix) {
				cleanPath = cleanPath[len(cwdPrefix):]
			} else if strings.HasPrefix(cleanPath, excludeCwdPrefix) {
				cleanPath = "!" + cleanPath[len(excludeCwdPrefix):]
			}
			ps = append(ps, gitignore.ParsePattern(cleanPath, nil))
		} else {
			return "", fmt.Errorf("Non-string path passed to hashFiles")
		}
	}

	matcher := gitignore.NewMatcher(ps)

	var files []string
	if err := filepath.Walk(impl.config.WorkingDir, func(path string, fi fs.FileInfo, err error) error {
		sansPrefix := strings.TrimPrefix(path, impl.config.WorkingDir+string(filepath.Separator))
		parts := strings.Split(sansPrefix, string(filepath.Separator))
		if fi.IsDir() || !matcher.Match(parts, fi.IsDir()) {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return "", fmt.Errorf("Unable to filepath.Walk: %v", err)
	}

	if len(files) == 0 {
		return "", nil
	}

	hasher := sha256.New()

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return "", fmt.Errorf("Unable to os.Open: %v", err)
		}

		if _, err := io.Copy(hasher, f); err != nil {
			return "", fmt.Errorf("Unable to io.Copy: %v", err)
		}

		if err := f.Close(); err != nil {
			return "", fmt.Errorf("Unable to Close file: %v", err)
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (impl *interpreterImpl) getNeedsTransitive(job *model.Job) []string {
	needs := job.Needs()

	for _, need := range needs {
		parentNeeds := impl.getNeedsTransitive(impl.config.Run.Workflow.GetJob(need))
		needs = append(needs, parentNeeds...)
	}

	return needs
}

func (impl *interpreterImpl) always() (bool, error) {
	return true, nil
}

func (impl *interpreterImpl) jobSuccess() (bool, error) {
	jobs := impl.config.Run.Workflow.Jobs
	jobNeeds := impl.getNeedsTransitive(impl.config.Run.Job())

	for _, needs := range jobNeeds {
		if jobs[needs].Result != "success" {
			return false, nil
		}
	}

	return true, nil
}

func (impl *interpreterImpl) stepSuccess() (bool, error) {
	return impl.env.Job.Status == "success", nil
}

func (impl *interpreterImpl) jobFailure() (bool, error) {
	jobs := impl.config.Run.Workflow.Jobs
	jobNeeds := impl.getNeedsTransitive(impl.config.Run.Job())

	for _, needs := range jobNeeds {
		if jobs[needs].Result == "failure" {
			return true, nil
		}
	}

	return false, nil
}

func (impl *interpreterImpl) stepFailure() (bool, error) {
	return impl.env.Job.Status == "failure", nil
}

func (impl *interpreterImpl) cancelled() (bool, error) {
	return impl.env.Job.Status == "cancelled", nil
}
