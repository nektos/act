package system // import "github.com/docker/docker/pkg/system"

import (
	"runtime"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// IsOSSupported determines if an operating system is supported by the host
func IsOSSupported(os string) bool {
	if strings.EqualFold(runtime.GOOS, os) {
		return true
	}
	if LCOWSupported() && strings.EqualFold(os, "linux") {
		return true
	}
	return false
}

// ValidatePlatform determines if a platform structure is valid.
// TODO This is a temporary windows-only function, should be replaced by
// comparison of worker capabilities
func ValidatePlatform(platform specs.Platform) error {
	if runtime.GOOS == "windows" {
		if !(platform.OS == runtime.GOOS || (LCOWSupported() && platform.OS == "linux")) {
			return errors.Errorf("unsupported os %s", platform.OS)
		}
	}
	return nil
}
