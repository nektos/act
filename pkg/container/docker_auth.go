package container

import (
	"net/url"
	"regexp"

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

	if matches, _ := regexp.MatchString("^[^.:]+\\/", image); matches {
		image = "index.docker.io/v1/" + image
	}

	parsed, err := url.Parse("http://" + image)
	if err != nil {
		log.Warnf("Could not parse image url: %v", err)
		return types.AuthConfig{}, err
	}

	authConfig, err := config.GetAuthConfig(parsed.Hostname())
	if err != nil {
		log.Warnf("Could not get auth config from docker config: %v", err)
		return types.AuthConfig{}, err
	}

	return types.AuthConfig(authConfig), nil
}
