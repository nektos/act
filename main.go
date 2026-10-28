package main

import (
	_ "embed"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/nektos/act/cmd"
	"github.com/nektos/act/pkg/common"
)

// version is set at build time with -ldflags "-X main.version=...".
var version string

// VERSION is a fallback for builds without ldflags or module version info.
// Releases are tagged without committing a bump, so it can lag the tag.
//
//go:embed VERSION
var embeddedVersion string

var (
	releaseVersionRegex = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)
	pseudoVersionRegex  = regexp.MustCompile(`[0-9]{14}-[0-9a-f]{12}$`)
)

func resolveVersion(ldflagsVersion string, info *debug.BuildInfo, ok bool, fileVersion string) string {
	if v := strings.TrimSpace(ldflagsVersion); v != "" {
		return v
	}
	if ok && info != nil {
		v := info.Main.Version
		if releaseVersionRegex.MatchString(v) && !pseudoVersionRegex.MatchString(v) {
			return strings.TrimPrefix(v, "v")
		}
	}
	return strings.TrimSpace(fileVersion)
}

func main() {
	ctx, cancel := common.CreateGracefulJobCancellationContext()
	defer cancel()

	info, ok := debug.ReadBuildInfo()
	// run the command
	cmd.Execute(ctx, resolveVersion(version, info, ok, embeddedVersion))
}
