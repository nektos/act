package actions

import (
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestParseWorkflowsFile(t *testing.T) {
	log.SetLevel(log.DebugLevel)

	conf := `
	  workflow "build-and-deploy" {
			on = "push"
			resolves = ["deploy"]
	  }
	  
	  action "build" {
			uses = "./action1"
			args = "echo 'build'"
	  }
	  
	  action "test" {
			uses = "docker://ubuntu:18.04"
			runs = "echo 'test'"
			needs = ["build"]
	  }

	  action "deploy" {
			uses = "./action2"
			args = ["echo","deploy"]
			needs = ["test"]
	  }

	  action "docker-login" {
			uses = "docker://docker"
			runs = ["sh", "-c", "echo $DOCKER_AUTH | docker login --username $REGISTRY_USER --password-stdin"]
			secrets = ["DOCKER_AUTH"]
			env = {
				REGISTRY_USER = "username"
			}
	  }

	  action "unit-tests" {
			uses = "./scripts/github_actions"
			runs = "yarn test:ci-unittest || echo \"Unit tests failed, but running danger to present the results!\" 2>&1"
		}

		action "regex-in-args" {
			uses = "actions/bin/filter@master"
			args = "tag v?[0-9]+\\.[0-9]+\\.[0-9]+"
		}

		action "regex-in-args-array" {
			uses = "actions/bin/filter@master"
			args = ["tag","v?[0-9]+\\.[0-9]+\\.[0-9]+"]
		}
	`

	workflows, err := parseWorkflowsFile(strings.NewReader(conf))

	assert.Nil(t, err)
	assert.Equal(t, 1, len(workflows.Workflow))

	w, wName, _ := workflows.getWorkflow("push")
	assert.Equal(t, "build-and-deploy", wName)
	assert.ElementsMatch(t, []string{"deploy"}, w.Resolves)

	actions := []struct {
		name    string
		uses    string
		needs   []string
		runs    []string
		args    []string
		secrets []string
	}{
		{"build",
			"./action1",
			nil,
			nil,
			[]string{"echo", "build"},
			nil,
		},
		{"test",
			"docker://ubuntu:18.04",
			[]string{"build"},
			[]string{"echo", "test"},
			nil,
			nil,
		},
		{"deploy",
			"./action2",
			[]string{"test"},
			nil,
			[]string{"echo", "deploy"},
			nil,
		},
		{"docker-login",
			"docker://docker",
			nil,
			[]string{"sh", "-c", "echo $DOCKER_AUTH | docker login --username $REGISTRY_USER --password-stdin"},
			nil,
			[]string{"DOCKER_AUTH"},
		},
		{"unit-tests",
			"./scripts/github_actions",
			nil,
			[]string{"yarn", "test:ci-unittest", "||", "echo", "Unit tests failed, but running danger to present the results!", "2>&1"},
			nil,
			nil,
		},
		{"regex-in-args",
			"actions/bin/filter@master",
			nil,
			nil,
			[]string{"tag", `v?[0-9]+\.[0-9]+\.[0-9]+`},
			nil,
		},
		{"regex-in-args-array",
			"actions/bin/filter@master",
			nil,
			nil,
			[]string{"tag", `v?[0-9]+\.[0-9]+\.[0-9]+`},
			nil,
		},
	}

	for _, exp := range actions {
		act, _ := workflows.getAction(exp.name)
		assert.Equal(t, exp.uses, act.Uses, "[%s] Uses", exp.name)
		if exp.needs == nil {
			assert.Nil(t, act.Needs, "[%s] Needs", exp.name)
		} else {
			assert.ElementsMatch(t, exp.needs, act.Needs, "[%s] Needs", exp.name)
		}
		if exp.runs == nil {
			assert.Nil(t, act.Runs, "[%s] Runs", exp.name)
		} else {
			assert.ElementsMatch(t, exp.runs, act.Runs, "[%s] Runs", exp.name)
		}
		if exp.args == nil {
			assert.Nil(t, act.Args, "[%s] Args", exp.name)
		} else {
			assert.ElementsMatch(t, exp.args, act.Args, "[%s] Args", exp.name)
		}
		/*
			if exp.env == nil {
				assert.Nil(t, act.Env, "[%s] Env", exp.name)
			} else {
				assert.ElementsMatch(t, exp.env, act.Env, "[%s] Env", exp.name)
			}
		*/
		if exp.secrets == nil {
			assert.Nil(t, act.Secrets, "[%s] Secrets", exp.name)
		} else {
			assert.ElementsMatch(t, exp.secrets, act.Secrets, "[%s] Secrets", exp.name)
		}
	}
}
