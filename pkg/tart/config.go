package tart

import (
	"errors"
)

var ErrConfigFromEnvironmentFailed = errors.New("failed to load config from environment")

const (
	// GitLab CI/CD environment adds "CUSTOM_ENV_" prefix[1] to prevent
	// conflicts with system environment variables.
	//
	// [1]: https://docs.gitlab.com/runner/executors/custom.html#stages
	envPrefixGitLabRunner = "CUSTOM_ENV_"

	// The prefix that we use to avoid confusion with Cirrus CI Cloud variables
	// and remove repetition from the Config's struct declaration.
	envPrefixTartExecutor = "TART_EXECUTOR_"

	// EnvTartExecutorInternalBuildsDir is an internal environment variable
	// that does not use the "CUSTOM_ENV_" prefix, thus preventing the override
	// by the user.
	EnvTartExecutorInternalBuildsDir = "TART_EXECUTOR_INTERNAL_BUILDS_DIR"

	// EnvTartExecutorInternalCacheDir is an internal environment variable
	// that does not use the "CUSTOM_ENV_" prefix, thus preventing the override
	// by the user.
	EnvTartExecutorInternalCacheDir = "TART_EXECUTOR_INTERNAL_CACHE_DIR"
)

type Config struct {
	SSHUsername         string `env:"SSH_USERNAME" envDefault:"admin"`
	SSHPassword         string `env:"SSH_PASSWORD" envDefault:"admin"`
	Softnet             bool   `env:"SOFTNET"`
	Headless            bool   `env:"HEADLESS"  envDefault:"true"`
	AlwaysPull          bool   `env:"ALWAYS_PULL"  envDefault:"true"`
	HostDir             bool   `env:"HOST_DIR"`
	Shell               string `env:"SHELL"`
	InstallGitlabRunner bool   `env:"INSTALL_GITLAB_RUNNER"`
	Timezone            string `env:"TIMEZONE"`
}

func NewConfigFromEnvironment() (Config, error) {
	var config Config

	// if err := env.ParseWithOptions(&config, env.Options{
	// 	Prefix: envPrefixGitLabRunner + envPrefixTartExecutor,
	// }); err != nil {
	// 	return config, fmt.Errorf("%w: %v", ErrConfigFromEnvironmentFailed, err)
	// }

	return config, nil
}
