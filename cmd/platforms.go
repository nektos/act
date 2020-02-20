package cmd

import (
	"strings"
)

func (i *Input) newPlatforms() map[string]string {
	platforms := map[string]string{
		"ubuntu-latest":  "ubuntu:18.04",
		"ubuntu-18.04":   "ubuntu:18.04",
		"ubuntu-16.04":   "ubuntu:16.04",
		"windows-latest": "",
		"windows-2019":   "",
		"macos-latest":   "",
		"macos-10.15":    "",
	}

	for _, p := range i.platforms {
		pParts := strings.Split(p, "=")
		if len(pParts) == 2 {
			platforms[pParts[0]] = pParts[1]
		}
	}
	return platforms
}
