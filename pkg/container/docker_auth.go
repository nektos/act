package container

import (
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/credentials"
	"github.com/docker/docker/api/types"

	log "github.com/sirupsen/logrus"
)

func LoadDockerAuthConfig(image string) (types.AuthConfig, error) {
	config, err := config.Load(config.Dir())
	if err != nil {
		log.Warnf("Could not load docker config: %v", err)
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
		log.Warnf("Could not get auth config from docker config: %v", err)
		return types.AuthConfig{}, err
	}

	return types.AuthConfig(authConfig), nil
}
