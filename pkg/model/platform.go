package model

type PlatformEngine string

const (
	PlatformEngineDocker = "docker"
	PlatformEngineHost   = "host"
	PlatformEngineNone   = "none"
)

type Platform struct {
	Platform  string
	Engine    PlatformEngine
	Image     string
	Supported bool
}

