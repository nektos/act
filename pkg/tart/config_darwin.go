package tart

type Config struct {
	SSHUsername string
	SSHPassword string
	Softnet     bool
	Headless    bool
	AlwaysPull  bool
}
