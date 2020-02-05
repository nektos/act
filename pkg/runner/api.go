package runner

type environmentApplier interface {
	applyEnvironment(map[string]string)
}
