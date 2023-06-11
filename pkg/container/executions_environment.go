package container

import "context"

type ExecutionsEnvironment interface {
	Container
	ToContainerPath(string) string
	GetActPath() string
	GetPathVariableName() string
	DefaultPathVariable() string
	JoinPathVariable(...string) string
	GetRunnerContext(ctx context.Context) map[string]interface{}
	// On windows PATH and Path are the same key
	IsEnvironmentCaseInsensitive() bool
}
