package tart

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
	return e.JobID
}
