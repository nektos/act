package model

import (
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"

	log "github.com/sirupsen/logrus"
)

// WorkflowPlanner contains methods for creating plans
type WorkflowPlanner interface {
	PlanEvent(eventName string) *Plan
	PlanJob(jobName string) *Plan
	GetEvents() []string
}

// Plan contains a list of stages to run in series
type Plan struct {
	Stages []*Stage
}

// Stage contains a list of runs to execute in parallel
type Stage struct {
	Runs []*Run
}

// Run represents a job from a workflow that needs to be run
type Run struct {
	Workflow *Workflow
	JobID    string
}

// NewWorkflowPlanner will load all workflows from a directory
func NewWorkflowPlanner(dirname string) (WorkflowPlanner, error) {
	log.Debugf("Loading workflows from '%s'", dirname)
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		return nil, err
	}

	wp := new(workflowPlanner)
	for _, file := range files {
		ext := filepath.Ext(file.Name())
		if ext == ".yml" || ext == ".yaml" {
			f, err := os.Open(filepath.Join(dirname, file.Name()))
			if err != nil {
				return nil, err
			}

			workflow, err := ReadWorkflow(f)
			if err != nil {
				f.Close()
				return nil, err
			}
			wp.workflows = append(wp.workflows, workflow)
			f.Close()
		}
	}

	return wp, nil
}

type workflowPlanner struct {
	workflows []*Workflow
}

// PlanEvent builds a new list of runs to execute in parallel for an event name
func (wp *workflowPlanner) PlanEvent(eventName string) *Plan {
	plan := new(Plan)
	for _, w := range wp.workflows {
		if w.On == eventName {
			plan.mergeStages(createStages(w, w.GetJobIDs()...))
		}
	}
	return plan
}

// PlanJob builds a new run to execute in parallel for a job name
func (wp *workflowPlanner) PlanJob(jobName string) *Plan {
	plan := new(Plan)
	for _, w := range wp.workflows {
		plan.mergeStages(createStages(w, jobName))
	}
	return plan
}

// GetEvents gets all the events in the workflows file
func (wp *workflowPlanner) GetEvents() []string {
	events := make([]string, 0)
	for _, w := range wp.workflows {
		found := false
		for _, e := range events {
			if e == w.On {
				found = true
				break
			}
		}

		if !found {
			events = append(events, w.On)
		}
	}

	// sort the list based on depth of dependencies
	sort.Slice(events, func(i, j int) bool {
		return events[i] < events[j]
	})

	return events
}

// GetJobIDs will get all the job names in the stage
func (s *Stage) GetJobIDs() []string {
	names := make([]string, 0)
	for _, r := range s.Runs {
		names = append(names, r.JobID)
	}
	return names
}

// Merge stages with existing stages in plan
func (p *Plan) mergeStages(stages []*Stage) {
	newStages := make([]*Stage, int(math.Max(float64(len(p.Stages)), float64(len(stages)))))
	for i := 0; i < len(newStages); i++ {
		newStages[i] = new(Stage)
		if i >= len(p.Stages) {
			newStages[i].Runs = append(stages[i].Runs)
		} else if i >= len(stages) {
			newStages[i].Runs = append(p.Stages[i].Runs)
		} else {
			newStages[i].Runs = append(p.Stages[i].Runs, stages[i].Runs...)
		}
	}
	p.Stages = newStages
}

func createStages(w *Workflow, jobIDs ...string) []*Stage {
	// first, build a list of all the necessary jobs to run, and their dependencies
	jobDependencies := make(map[string][]string)
	for len(jobIDs) > 0 {
		newJobIDs := make([]string, 0)
		for _, jID := range jobIDs {
			// make sure we haven't visited this job yet
			if _, ok := jobDependencies[jID]; !ok {
				if job := w.GetJob(jID); job != nil {
					jobDependencies[jID] = job.Needs
					newJobIDs = append(newJobIDs, job.Needs...)
				}
			}
		}
		jobIDs = newJobIDs
	}

	// next, build an execution graph
	stages := make([]*Stage, 0)
	for len(jobDependencies) > 0 {
		stage := new(Stage)
		for jID, jDeps := range jobDependencies {
			// make sure all deps are in the graph already
			if listInStages(jDeps, stages...) {
				stage.Runs = append(stage.Runs, &Run{
					Workflow: w,
					JobID:    jID,
				})
				delete(jobDependencies, jID)
			}
		}
		if len(stage.Runs) == 0 {
			log.Fatalf("Unable to build dependency graph!")
		}
		stages = append(stages, stage)
	}

	return stages
}

// return true iff all strings in srcList exist in at least one of the stages
func listInStages(srcList []string, stages ...*Stage) bool {
	for _, src := range srcList {
		found := false
		for _, stage := range stages {
			for _, search := range stage.GetJobIDs() {
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
