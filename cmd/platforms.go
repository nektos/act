package cmd

import (
	"strings"

	log "github.com/sirupsen/logrus"
)

func (i *Input) newPlatforms() map[string]string {
	platforms := map[string]string{
		"ubuntu-latest": "node:16-buster-slim",
		"ubuntu-22.04":  "node:16-bullseye-slim",
		"ubuntu-20.04":  "node:16-buster-slim",
		"ubuntu-18.04":  "node:16-buster-slim",
	}

	for _, p := range i.platforms {
		// = is a valid character for a runs-on definition, not for a docker image
		// Take the last = to support runs-on definitions with = in them
		lastEq := strings.LastIndex(p, "=")
		if lastEq > 0 {
			from := p[:lastEq]
			to := p[lastEq+1:]
			platforms[from] = to
			log.Debugf("Parsed runs-on platform mapping: '%s' -> '%s'", from, to)
		} else {
			log.Warnf("Ignoring invalid platform argument (missing =) '%s'", p)
		}
	}
	return platforms
}
