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
		pParts := strings.Split(p, "=")
		// pop operation
		pValue := pParts[len(pParts)-1]
		pParts = pParts[:len(pParts)-1]

		pName := strings.Join(pParts, "=")
		platforms[strings.ToLower(pName)] = pValue
	}
	return platforms
}
