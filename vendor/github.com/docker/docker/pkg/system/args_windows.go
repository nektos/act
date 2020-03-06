package system // import "github.com/docker/docker/pkg/system"

import (
	"strings"

	"golang.org/x/sys/windows"
)

// EscapeArgs makes a Windows-style escaped command line from a set of arguments
func EscapeArgs(args []string) string {
	escapedArgs := make([]string, len(args))
	for i, a := range args {
		escapedArgs[i] = windows.EscapeArg(a)
	}
	return strings.Join(escapedArgs, " ")
}
