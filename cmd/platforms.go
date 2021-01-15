package cmd

import (
	"strings"
)

func (i *Input) newPlatforms() map[string]string {
	platforms := map[string]string{
		"ubuntu-latest":  "node:12.6-buster-slim",
		"ubuntu-20.04":   "node:12.6-buster-slim",
		"ubuntu-18.04":   "node:12.6-buster-slim",
		"ubuntu-16.04":   "node:12.6-stretch-slim",
		"windows-latest": "",
		"windows-2019":   "",
		"macos-latest":   "",
		"macos-10.15":    "",
	}

	if Config.Sub(`images`) != nil {
		configPlatforms := Config.GetStringMapString(`images`)
		for k, v := range configPlatforms {
			platforms[k] = v
		}
	}

	for _, p := range i.platforms {
		pParts := strings.Split(p, "=")
		if len(pParts) == 2 {
			platforms[pParts[0]] = pParts[1]
		}

	}
	return platforms
}
