package protocol

type JobEvent struct {
	Name               string
	JobId              string
	RequestId          int64
	Result             string
	Outputs            *map[string]VariableValue    `json:",omitempty"`
	ActionsEnvironment *ActionsEnvironmentReference `json:",omitempty"`
}
