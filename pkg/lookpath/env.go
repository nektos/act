package lookpath

import "os"

type Env interface {
	Getenv(name string) string
}

type defaultEnv struct {
}

func (*defaultEnv) Getenv(name string) string {
	return os.Getenv(name)
}

func LookPath(file string) (string, error) {
	return LookPath2(file, &defaultEnv{})
}
