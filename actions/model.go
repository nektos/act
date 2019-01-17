package actions

import (
	"fmt"
	"log"
	"os"

	"github.com/howeyc/gopass"
)

type workflowModel struct {
	On       string
	Resolves []string
}

type actionModel struct {
	Needs   []string
	Uses    string
	Runs    []string
	Args    []string
	Env     map[string]string
	Secrets []string
}

type workflowsFile struct {
	Workflow map[string]workflowModel
	Action   map[string]actionModel
}

func (wFile *workflowsFile) getWorkflow(eventName string) (*workflowModel, string, error) {
	var rtn workflowModel
	for wName, w := range wFile.Workflow {
		if w.On == eventName {
			rtn = w
			return &rtn, wName, nil
		}
	}
	return nil, "", fmt.Errorf("unsupported event: %v", eventName)
}

func (wFile *workflowsFile) getAction(actionName string) (*actionModel, error) {
	if a, ok := wFile.Action[actionName]; ok {
		return &a, nil
	}
	return nil, fmt.Errorf("unsupported action: %v", actionName)
}

// return a pipeline that is run in series.  pipeline is a list of steps to run in parallel
func (wFile *workflowsFile) newExecutionGraph(actionNames ...string) [][]string {
	// first, build a list of all the necessary actions to run, and their dependencies
	actionDependencies := make(map[string][]string)
	for len(actionNames) > 0 {
		newActionNames := make([]string, 0)
		for _, aName := range actionNames {
			// make sure we haven't visited this action yet
			if _, ok := actionDependencies[aName]; !ok {
				actionDependencies[aName] = wFile.Action[aName].Needs
				newActionNames = append(newActionNames, wFile.Action[aName].Needs...)
			}
		}
		actionNames = newActionNames
	}

	// next, build an execution graph
	graph := make([][]string, 0)
	for len(actionDependencies) > 0 {
		stage := make([]string, 0)
		for aName, aDeps := range actionDependencies {
			// make sure all deps are in the graph already
			if listInLists(aDeps, graph...) {
				stage = append(stage, aName)
				delete(actionDependencies, aName)
			}
		}
		if len(stage) == 0 {
			log.Fatalf("Unable to build dependency graph!")
		}
		graph = append(graph, stage)
	}

	return graph
}

// return true iff all strings in srcList exist in at least one of the searchLists
func listInLists(srcList []string, searchLists ...[]string) bool {
	for _, src := range srcList {
		found := false
		for _, searchList := range searchLists {
			for _, search := range searchList {
				if src == search {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

var secretCache map[string]string

func (action *actionModel) applyEnvironment(env map[string]string) {
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
