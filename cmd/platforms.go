package cmd

import (
	"strings"
)

func (i *Input) newPlatforms() map[string]string {
	platforms := map[string]string{
		"ubuntu-latest": "node:16-buster-slim",
		"ubuntu-22.04":  "node:16-bullseye-slim",
		"ubuntu-20.04":  "node:16-buster-slim",
		"ubuntu-18.04":  "node:16-buster-slim",
	}

	for _, p := range i.platforms {
		separator := strings.LastIndex(p, "=")
		if separator > 0 && separator < len(p)-1 {
			platforms[strings.ToLower(p[:separator])] = p[separator+1:]
		}
	}
	return platforms
}
