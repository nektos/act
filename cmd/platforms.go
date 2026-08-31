package cmd

import (
	"strings"

	"github.com/nektos/act/pkg/runner"
)

func (i *Input) newPlatforms() map[string]string {
	platforms := map[string]string{
		"ubuntu-latest": "node:16-buster-slim",
		"ubuntu-22.04":  "node:16-bullseye-slim",
		"ubuntu-20.04":  "node:16-buster-slim",
		"ubuntu-18.04":  "node:16-buster-slim",
	}

	for _, p := range i.platforms {
		pParts := strings.Split(p, "=")
		if len(pParts) == 2 {
			platforms[strings.ToLower(pParts[0])] = pParts[1]
		}
	}
	return platforms
}

func (i *Input) newPlatformOptions() map[string]runner.PlatformOptions {
	platformOptions := make(map[string]runner.PlatformOptions, len(i.platformOptions))
	for _, option := range i.platformOptions {
		optionParts := strings.SplitN(option, "=", 2)
		if len(optionParts) == 2 {
			platformName := optionParts[0]
			appendOptions := strings.HasSuffix(platformName, "+")
			platformName = strings.TrimSuffix(platformName, "+")
			platformOptions[strings.ToLower(platformName)] = runner.PlatformOptions{
				Options: optionParts[1],
				Append:  appendOptions,
			}
		}
	}
	return platformOptions
}
