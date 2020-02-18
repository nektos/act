package glob

import (
	"bytes"
	"fmt"
	//"log"
	"os"
	gpath "path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/MichaelTJones/walk"
)

const (
	// NotSlash is any rune but path separator.
	notSlash = "[^/]"
	// AnyRune is zero or more non-path separators.
	anyRune = notSlash + "*"
	// ZeroOrMoreDirectories is used by ** patterns.
	zeroOrMoreDirectories = `(?:[.{}\w\-\ ]+\/)*`
	// TrailingStarStar matches everything inside directory.
	trailingStarStar = "/**"
	// SlashStarStarSlash maches zero or more directories.
	slashStarStarSlash = "/**/"
)

// RegexpInfo contains additional info about the Regexp created by a glob pattern.
type RegexpInfo struct {
	Regexp *regexp.Regexp
	Negate bool
	Path   string
	Glob   string
}

// MatchString matches a string with either a regexp or direct string match
func (ri *RegexpInfo) MatchString(s string) bool {
	if ri.Regexp != nil {
		return ri.Regexp.MatchString(s)
	} else if ri.Path != "" {
		return strings.HasSuffix(s, ri.Path)
	}
	return false
}

// Globexp builds a regular express from from extended glob pattern and then
// returns a Regexp object.
func Globexp(glob string) *regexp.Regexp {
	var re bytes.Buffer

	re.WriteString("^")

	i, inGroup, L := 0, false, len(glob)

	for i < L {
		r, w := utf8.DecodeRuneInString(glob[i:])

		switch r {
		default:
			re.WriteRune(r)

		case '\\', '$', '^', '+', '.', '(', ')', '=', '!', '|':
			re.WriteRune('\\')
			re.WriteRune(r)

		case '/':
			// TODO optimize later, string could be long
			rest := glob[i:]
			re.WriteRune('/')
			if strings.HasPrefix(rest, "/**/") {
				re.WriteString(zeroOrMoreDirectories)
				w *= 4
			} else if rest == "/**" {
				re.WriteString(".*")
				w *= 3
			}

		case '?':
			re.WriteRune('.')

		case '[', ']':
			re.WriteRune(r)

		case '{':
			if i < L-1 {
				if glob[i+1:i+2] == "{" {
					re.WriteString("\\{")
					w *= 2
					break
				}
			}
			inGroup = true
			re.WriteRune('(')

		case '}':
			if inGroup {
				inGroup = false
				re.WriteRune(')')
			} else {
				re.WriteRune('}')
			}

		case ',':
			if inGroup {
				re.WriteRune('|')
			} else {
				re.WriteRune('\\')
				re.WriteRune(r)
			}

		case '*':
			rest := glob[i:]
			if strings.HasPrefix(rest, "**/") {
				re.WriteString(zeroOrMoreDirectories)
				w *= 3
			} else {
				re.WriteString(anyRune)
			}
		}

		i += w
	}

	re.WriteString("$")
	//log.Printf("regex string %s", re.String())
	return regexp.MustCompile(re.String())
}

// Glob returns files and dirctories that match patterns. Patterns must use
// slashes, even Windows.
//
// Special chars.
//
//   /**/   - match zero or more directories
//   {a,b}  - match a or b, no spaces
//   *      - match any non-separator char
//   ?      - match a single non-separator char
//   **/    - match any directory, start of pattern only
//   /**    - match any this directory, end of pattern only
//   !      - removes files from resultset, start of pattern only
//
func Glob(patterns []string) ([]*FileAsset, []*RegexpInfo, error) {
	// TODO very inefficient and unintelligent, optimize later

	m := map[string]*FileAsset{}
	regexps := []*RegexpInfo{}

	for _, pattern := range patterns {
		remove := strings.HasPrefix(pattern, "!")
		if remove {
			pattern = pattern[1:]
			if hasMeta(pattern) {
				re := Globexp(pattern)
				regexps = append(regexps, &RegexpInfo{Regexp: re, Glob: pattern, Negate: true})
				for path := range m {
					if re.MatchString(path) {
						m[path] = nil
					}
				}
			} else {
				path := gpath.Clean(pattern)
				m[path] = nil
				regexps = append(regexps, &RegexpInfo{Path: path, Glob: pattern, Negate: true})
			}
		} else {
			if hasMeta(pattern) {
				re := Globexp(pattern)
				regexps = append(regexps, &RegexpInfo{Regexp: re, Glob: pattern})
				root := PatternRoot(pattern)
				if root == "" {
					return nil, nil, fmt.Errorf("Cannot get root from pattern: %s", pattern)
				}
				fileAssets, err := walkFiles(root)
				if err != nil {
					return nil, nil, err
				}

				for _, file := range fileAssets {
					if re.MatchString(file.Path) {
						// TODO closure problem assigning &file
						tmp := file
						m[file.Path] = tmp
					}
				}
			} else {
				path := gpath.Clean(pattern)
				info, err := os.Stat(path)
				if err != nil {
					return nil, nil, err
				}
				regexps = append(regexps, &RegexpInfo{Path: path, Glob: pattern, Negate: false})
				fa := &FileAsset{Path: path, FileInfo: info}
				m[path] = fa
			}
		}
	}

	//log.Printf("m %v", m)
	keys := []*FileAsset{}
	for _, it := range m {
		if it != nil {
			keys = append(keys, it)
		}
	}
	return keys, regexps, nil
}

// hasMeta determines if a path has special chars used to build a Regexp.
func hasMeta(path string) bool {
	return strings.IndexAny(path, "*?[{") >= 0
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return st.IsDir()
}

// PatternRoot gets a real directory root from a pattern. The directory
// returned is used as the start location for globbing.
func PatternRoot(s string) string {
	if isDir(s) {
		return s
	}

	// No directory in pattern
	parts := strings.Split(s, "/")
	if len(parts) == 1 {
		return "."
	}
	// parts returns an empty string at positio 0 if the s starts with "/"
	root := ""

	// Build path until a dirname has a char used to build regex
	for i, part := range parts {
		if hasMeta(part) {
			break
		}
		if i > 0 {
			root += "/"
		}
		root += part
	}
	// Default to cwd
	if root == "" {
		root = "."
	}
	return root
}

// walkFiles walks a directory starting at root returning all directories and files
// include those found in subdirectories.
func walkFiles(root string) ([]*FileAsset, error) {
	fileAssets := []*FileAsset{}
	var lock sync.Mutex
	visitor := func(path string, info os.FileInfo, err error) error {
		// if err != nil {
		// 	fmt.Println("visitor err", err.Error(), "root", root)
		// }
		if err == nil {
			lock.Lock()
			fileAssets = append(fileAssets, &FileAsset{FileInfo: info, Path: filepath.ToSlash(path)})
			lock.Unlock()
		}
		return nil
	}
	err := walk.Walk(root, visitor)
	if err != nil {
		return nil, err
	}
	return fileAssets, nil
}
