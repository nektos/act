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

// VERSION is the last resort, for builds with neither ldflags nor module
// version info (for example a source archive built without VCS data).
// Releases are tagged without committing a bump, so it can lag the tag.
//
//go:embed VERSION
var embeddedVersion string

// moduleVersionRegex accepts release, pseudo, and +dirty module versions and
// rejects "(devel)".
var moduleVersionRegex = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.+-]*)?$`)

func resolveVersion(ldflagsVersion string, info *debug.BuildInfo, ok bool, fileVersion string) string {
	if v := strings.TrimSpace(ldflagsVersion); v != "" {
		return v
	}
	if ok && info != nil {
		v := info.Main.Version
		if moduleVersionRegex.MatchString(v) {
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
