package cmd

import (
	"github.com/nektos/act/pkg/model"
	log "github.com/sirupsen/logrus"
	"strings"
)

func (i *Input) newPlatforms() map[string]*model.Platform {
	platforms := map[string]*model.Platform{
		"ubuntu-latest":  i.createSupportedPlatform("ubuntu-latest", model.PlatformEngineDocker, true, "node:12.6-buster-slim"),
		"ubuntu-18.04":   i.createSupportedPlatform("ubuntu-18.04", model.PlatformEngineDocker, true, "node:12.6-buster-slim"),
		"ubuntu-16.04":   i.createSupportedPlatform("ubuntu-16.04", model.PlatformEngineDocker, true, "node:12.6-stretch-slim"),
		"windows-latest": i.createUnsupportedPlatform("windows-latest"),
		"windows-2019":   i.createUnsupportedPlatform("windows-2019"),
		"macos-latest":   i.createSupportedPlatform("macos-latest", model.PlatformEngineHost, true, ""),
		"macos-10.15":    i.createSupportedPlatform("macos-10.15", model.PlatformEngineHost, true, ""),
	}

	for _, p := range i.platforms {
		pParts := strings.Split(p, "=")
		if len(pParts) == 2 {
			platform := platforms[pParts[0]]

			if platform.Supported == false {
				log.Warnf("\U0001F6A7  Unable to set custom image for non supported platform '%+v'", platform.Platform)
				continue
			}

			if platform.Engine == model.PlatformEngineHost {
				log.Warnf("\U0001F6A7  Unable to set custom image for non-docker based platforms '%+v'", platform.Platform)
				continue
			}

			platform.Image = pParts[1]
		}
	}
	return platforms
}

func (i* Input) createUnsupportedPlatform(platform string) *model.Platform {
	return &model.Platform{
		Platform:  platform,
		Engine:    model.PlatformEngineNone,
		Supported: false,
		Image:     "",
	}
}

func (i* Input) createSupportedPlatform(platform string, engine model.PlatformEngine, supported bool, image string) *model.Platform {
	return &model.Platform{
		Platform: platform,
		Engine: engine,
		Supported: supported,
		Image: image,
	}
}
