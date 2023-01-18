//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows))

package container

import (
	"context"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/credentials"
	"github.com/docker/docker/api/types"
	"github.com/nektos/act/pkg/common"
)

func LoadDockerAuthConfig(ctx context.Context, image string) (types.AuthConfig, error) {
	logger := common.Logger(ctx)
	config, err := config.Load(config.Dir())
	if err != nil {
		logger.Warnf("Could not load docker config: %v", err)
		return types.AuthConfig{}, err
	}

	if !config.ContainsAuth() {
		config.CredentialsStore = credentials.DetectDefaultStore(config.CredentialsStore)
	}

	hostName := "index.docker.io"
	index := strings.IndexRune(image, '/')
	if index > -1 && (strings.ContainsAny(image[:index], ".:") || image[:index] == "localhost") {
		hostName = image[:index]
	}

	authConfig, err := config.GetAuthConfig(hostName)
	if err != nil {
		logger.Warnf("Could not get auth config from docker config: %v", err)
		return types.AuthConfig{}, err
	}

	return types.AuthConfig(authConfig), nil
}

func LoadDockerAuthConfigs(ctx context.Context) map[string]types.AuthConfig {
	logger := common.Logger(ctx)
	config, err := config.Load(config.Dir())
	if err != nil {
		logger.Warnf("Could not load docker config: %v", err)
		return nil
	}

	if !config.ContainsAuth() {
		config.CredentialsStore = credentials.DetectDefaultStore(config.CredentialsStore)
	}

	creds, _ := config.GetAllCredentials()
	authConfigs := make(map[string]types.AuthConfig, len(creds))
	for k, v := range creds {
		authConfigs[k] = types.AuthConfig(v)
	}

	return authConfigs
}
