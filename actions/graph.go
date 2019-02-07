package actions

import (
	"log"

	"github.com/actions/workflow-parser/model"
)

// return a pipeline that is run in series.  pipeline is a list of steps to run in parallel
func newExecutionGraph(workflowConfig *model.Configuration, actionNames ...string) [][]string {
	// first, build a list of all the necessary actions to run, and their dependencies
	actionDependencies := make(map[string][]string)
	for len(actionNames) > 0 {
		newActionNames := make([]string, 0)
		for _, aName := range actionNames {
			// make sure we haven't visited this action yet
			if _, ok := actionDependencies[aName]; !ok {
				action := workflowConfig.GetAction(aName)
				if action != nil {
					actionDependencies[aName] = action.Needs
					newActionNames = append(newActionNames, action.Needs...)
				}
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
