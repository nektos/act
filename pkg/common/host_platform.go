package common

import "strings"

// HostPlatformImage is the platform image that runs job steps on the host
// instead of in a container. The leading hyphen is part of the token.
const HostPlatformImage = "-self-hosted"

// IsHostPlatform reports whether image selects host execution.
// Surrounding whitespace is ignored and the comparison is case-insensitive
// under Unicode case folding. The project .actrc allowlist and the runner
// both use this so a spelling the runner treats as host mode cannot pass
// the allowlist.
func IsHostPlatform(image string) bool {
	return strings.EqualFold(strings.TrimSpace(image), HostPlatformImage)
}
