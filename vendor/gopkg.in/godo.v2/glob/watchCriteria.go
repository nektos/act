package glob

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgutz/str"
)

// WatchCriterion is the criteria needed to test if a file
// matches a pattern.
type WatchCriterion struct {
	// Root is the root directory to start watching.
	Root string
	// Includes are the regexp for including files
	IncludesRegexp []*regexp.Regexp
	// Excludes are the regexp for excluding files
	ExcludesRegexp []*regexp.Regexp
	Includes       []string
	Excludes       []string
}

func newWatchCriterion(r string) *WatchCriterion {
	return &WatchCriterion{
		Root:           r,
		IncludesRegexp: []*regexp.Regexp{},
		ExcludesRegexp: []*regexp.Regexp{},
		Includes:       []string{},
		Excludes:       []string{},
	}
}

// WatchCriteria is the set of criterion to watch one or more glob patterns.
type WatchCriteria struct {
	Items []*WatchCriterion
}

func newWatchCriteria() *WatchCriteria {
	return &WatchCriteria{
		Items: []*WatchCriterion{},
	}
}

func (cr *WatchCriteria) findParent(root string) *WatchCriterion {
	for _, item := range cr.Items {
		if item.Root == root || strings.Contains(item.Root, root) {
			return item
		}
	}
	return nil
}

func (cr *WatchCriteria) add(glob string) error {
	var err error

	if glob == "" || glob == "!" {
		return nil
	}

	isExclude := strings.HasPrefix(glob, "!")
	if isExclude {
		glob = glob[1:]
	}

	// determine if the root of pattern already exists
	root := PatternRoot(glob)
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	root = filepath.ToSlash(root)
	cri := cr.findParent(root)
	if cri == nil {
		cri = newWatchCriterion(root)
		cr.Items = append(cr.Items, cri)
	}

	glob, err = filepath.Abs(glob)
	if err != nil {
		return err
	}

	// add glob to {in,ex}cludes
	if isExclude {
		if str.SliceIndexOf(cri.Excludes, glob) < 0 {
			re := Globexp(glob)
			cri.ExcludesRegexp = append(cri.ExcludesRegexp, re)
			cri.Excludes = append(cri.Excludes, glob)
		}
	} else {
		if str.SliceIndexOf(cri.Includes, glob) < 0 {
			re := Globexp(glob)
			cri.IncludesRegexp = append(cri.IncludesRegexp, re)
			cri.Includes = append(cri.Includes, glob)
		}
	}

	return nil
}

// Roots returns the root paths of all criteria.
func (cr *WatchCriteria) Roots() []string {
	if cr.Items == nil || len(cr.Items) == 0 {
		return nil
	}

	roots := make([]string, len(cr.Items))
	for i, it := range cr.Items {
		roots[i] = it.Root
	}
	return roots
}

// Matches determines if pth is matched by internal criteria.
func (cr *WatchCriteria) Matches(pth string) bool {
	match := false
	pth = filepath.ToSlash(pth)
	for _, it := range cr.Items {
		// if sub path
		if strings.HasPrefix(pth, it.Root) {
			// check if matches an include pattern
			for _, re := range it.IncludesRegexp {
				if re.MatchString(pth) {
					match = true
					break
				}
			}
			// when found, check if it is excluded
			if match {
				for _, re := range it.ExcludesRegexp {
					if re.MatchString(pth) {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}

	return false
}

// EffectiveCriteria is the minimum set of criteria to watch the
// items in patterns
func EffectiveCriteria(globs ...string) (*WatchCriteria, error) {
	if len(globs) == 0 {
		return nil, nil
	}
	result := newWatchCriteria()
	for _, glob := range globs {
		err := result.add(glob)
		if err != nil {
			fmt.Println(err.Error())
			return nil, err
		}
	}

	return result, nil
}
