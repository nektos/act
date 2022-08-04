package container

type ExecutionsEnvironment interface {
	Container
	ToContainerPath(string) string
	GetActPath() string
	GetPathVariableName() string
	DefaultPathVariable() string
	JoinPathVariable(...string) string
	GetRunnerContext() map[string]interface{}
}
