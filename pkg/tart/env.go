package tart

import (
	"errors"
	"fmt"
)

var ErrGitLabEnv = errors.New("GitLab environment error")

type Env struct {
	JobID           string
	JobImage        string
	FailureExitCode int
	Registry        *Registry
}

type Registry struct {
	Address  string
	User     string
	Password string
}

func (e Env) VirtualMachineID() string {
	return fmt.Sprintf("gitlab-%s", e.JobID)
}
