package actions

import (
	"fmt"
	"log"
	"os"

	"github.com/actions/workflow-parser/model"
	"github.com/howeyc/gopass"
)

var secretCache map[string]string

type actionEnvironmentApplier struct {
	*model.Action
}

func newActionEnvironmentApplier(action *model.Action) environmentApplier {
	return &actionEnvironmentApplier{action}
}

func (action *actionEnvironmentApplier) applyEnvironment(env map[string]string) {
	for envKey, envValue := range action.Env {
		env[envKey] = envValue
	}

	for _, secret := range action.Secrets {
		if secretVal, ok := os.LookupEnv(secret); ok {
			env[secret] = secretVal
		} else {
			if secretCache == nil {
				secretCache = make(map[string]string)
			}

			if _, ok := secretCache[secret]; !ok {
				fmt.Printf("Provide value for '%s': ", secret)
				val, err := gopass.GetPasswdMasked()
				if err != nil {
					log.Fatal("abort")
				}

				secretCache[secret] = string(val)
			}
			env[secret] = secretCache[secret]
		}
	}
}
