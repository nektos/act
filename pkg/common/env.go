package common

import (
	"os"
)

var DefaultEnvironment = map[string][]string{
	"RUNNER_TOOL_CACHE": {"$XDG_CACHE_HOME/hostedtoolcache", "$HOME/.cache/hostedtoolcache", "/opt/hostedtoolcache"},
	"RUNNER_TEMP":       {"$XDG_CACHE_HOME", "/tmp"},
}

func lookupDefaultEnv(envKey string) string {
	envValue := os.Getenv(envKey)
	if envValue != "" {
		return envValue
	}

	// Expand environment variables, and return it if it's a valid (existing) path;
	// defaulting to the last element in the list
	env := ""
	for _, v := range DefaultEnvironment[envKey] {
		env = os.ExpandEnv(v)
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	return env
}
