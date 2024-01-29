package model

import (
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	log "github.com/sirupsen/logrus"
)

// WorkflowPlanner contains methods for creating plans
type WorkflowPlanner interface {
	PlanEvent(eventName string) (*Plan, error)
	PlanJob(jobName string) (*Plan, error)
	PlanAll() (*Plan, error)
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

func (r *Run) String() string {
	jobName := r.Job().Name
	if jobName == "" {
		jobName = r.JobID
	}
	return jobName
}

// Job returns the job for this Run
func (r *Run) Job() *Job {
	return r.Workflow.GetJob(r.JobID)
}

type WorkflowFiles struct {
	workflowDirEntry os.DirEntry
	dirPath          string
}

// NewWorkflowPlanner will load a specific workflow, all workflows from a directory or all workflows from a directory and its subdirectories
//
//nolint:gocyclo
func NewWorkflowPlanner(path string, noWorkflowRecurse bool) (WorkflowPlanner, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var workflows []WorkflowFiles

	if fi.IsDir() {
		log.Debugf("Loading workflows from '%s'", path)
		if noWorkflowRecurse {
			files, err := os.ReadDir(path)
			if err != nil {
				return nil, err
			}

			for _, v := range files {
				workflows = append(workflows, WorkflowFiles{
					dirPath:          path,
					workflowDirEntry: v,
				})
			}
		} else {
			log.Debug("Loading workflows recursively")
			if err := filepath.Walk(path,
				func(p string, f os.FileInfo, err error) error {
					if err != nil {
						return err
					}

					if !f.IsDir() {
						log.Debugf("Found workflow '%s' in '%s'", f.Name(), p)
						workflows = append(workflows, WorkflowFiles{
							dirPath:          filepath.Dir(p),
							workflowDirEntry: fs.FileInfoToDirEntry(f),
						})
					}

					return nil
				}); err != nil {
				return nil, err
			}
		}
	} else {
		log.Debugf("Loading workflow '%s'", path)
		dirname := filepath.Dir(path)

		workflows = append(workflows, WorkflowFiles{
			dirPath:          dirname,
			workflowDirEntry: fs.FileInfoToDirEntry(fi),
		})
	}
	if err != nil {
		return nil, err
	}

	wp := new(workflowPlanner)
	for _, wf := range workflows {
		ext := filepath.Ext(wf.workflowDirEntry.Name())
		if ext == ".yml" || ext == ".yaml" {
			f, err := os.Open(filepath.Join(wf.dirPath, wf.workflowDirEntry.Name()))
			if err != nil {
				return nil, err
			}

			log.Debugf("Reading workflow '%s'", f.Name())
			workflow, err := ReadWorkflow(f)
			if err != nil {
				_ = f.Close()
				if err == io.EOF {
					return nil, fmt.Errorf("unable to read workflow '%s': file is empty: %w", wf.workflowDirEntry.Name(), err)
				}
				return nil, fmt.Errorf("workflow is not valid. '%s': %w", wf.workflowDirEntry.Name(), err)
			}
			_, err = f.Seek(0, 0)
			if err != nil {
				_ = f.Close()
				return nil, fmt.Errorf("error occurring when resetting io pointer in '%s': %w", wf.workflowDirEntry.Name(), err)
			}

			workflow.File = wf.workflowDirEntry.Name()
			if workflow.Name == "" {
				workflow.Name = wf.workflowDirEntry.Name()
			}

			err = validateJobName(workflow)
			if err != nil {
				_ = f.Close()
				return nil, err
			}

			wp.workflows = append(wp.workflows, workflow)
			_ = f.Close()
		}
	}

	return wp, nil
}

func NewSingleWorkflowPlanner(name string, f io.Reader) (WorkflowPlanner, error) {
	wp := new(workflowPlanner)

	log.Debugf("Reading workflow %s", name)
	workflow, err := ReadWorkflow(f)
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("unable to read workflow '%s': file is empty: %w", name, err)
		}
		return nil, fmt.Errorf("workflow is not valid. '%s': %w", name, err)
	}
	workflow.File = name
	if workflow.Name == "" {
		workflow.Name = name
	}

	err = validateJobName(workflow)
	if err != nil {
		return nil, err
	}

	wp.workflows = append(wp.workflows, workflow)

	return wp, nil
}

func validateJobName(workflow *Workflow) error {
	jobNameRegex := regexp.MustCompile(`^([[:alpha:]_][[:alnum:]_\-]*)$`)
	for k := range workflow.Jobs {
		if ok := jobNameRegex.MatchString(k); !ok {
			return fmt.Errorf("workflow is not valid. '%s': Job name '%s' is invalid. Names must start with a letter or '_' and contain only alphanumeric characters, '-', or '_'", workflow.Name, k)
		}
	}
	return nil
}

type workflowPlanner struct {
	workflows []*Workflow
}

// PlanEvent builds a new list of runs to execute in parallel for an event name
func (wp *workflowPlanner) PlanEvent(eventName string) (*Plan, error) {
	plan := new(Plan)
	if len(wp.workflows) == 0 {
		log.Debug("no workflows found by planner")
		return plan, nil
	}
	var lastErr error

	for _, w := range wp.workflows {
		events := w.On()
		if len(events) == 0 {
			log.Debugf("no events found for workflow: %s", w.File)
			continue
		}

		for _, e := range events {
			if e == eventName {
				stages, err := createStages(w, w.GetJobIDs()...)
				if err != nil {
					log.Warn(err)
					lastErr = err
				} else {
					plan.mergeStages(stages)
				}
			}
		}
	}
	return plan, lastErr
}

// PlanJob builds a new run to execute in parallel for a job name
func (wp *workflowPlanner) PlanJob(jobName string) (*Plan, error) {
	plan := new(Plan)
	if len(wp.workflows) == 0 {
		log.Debugf("no jobs found for workflow: %s", jobName)
	}
	var lastErr error

	for _, w := range wp.workflows {
		stages, err := createStages(w, jobName)
		if err != nil {
			log.Warn(err)
			lastErr = err
		} else {
			plan.mergeStages(stages)
		}
	}
	return plan, lastErr
}

// PlanAll builds a new run to execute in parallel all
func (wp *workflowPlanner) PlanAll() (*Plan, error) {
	plan := new(Plan)
	if len(wp.workflows) == 0 {
		log.Debug("no workflows found by planner")
		return plan, nil
	}
	var lastErr error

	for _, w := range wp.workflows {
		stages, err := createStages(w, w.GetJobIDs()...)
		if err != nil {
			log.Warn(err)
			lastErr = err
		} else {
			plan.mergeStages(stages)
		}
	}

	return plan, lastErr
}

// GetEvents gets all the events in the workflows file
func (wp *workflowPlanner) GetEvents() []string {
	events := make([]string, 0)
	for _, w := range wp.workflows {
		found := false
		for _, e := range events {
			for _, we := range w.On() {
				if e == we {
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			events = append(events, w.On()...)
		}
	}

	// sort the list based on depth of dependencies
	sort.Slice(events, func(i, j int) bool {
		return events[i] < events[j]
	})

	return events
}

// MaxRunNameLen determines the max name length of all jobs
func (p *Plan) MaxRunNameLen() int {
	maxRunNameLen := 0
	for _, stage := range p.Stages {
		for _, run := range stage.Runs {
			runNameLen := len(run.String())
			if runNameLen > maxRunNameLen {
				maxRunNameLen = runNameLen
			}
		}
	}
	return maxRunNameLen
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
			newStages[i].Runs = append(newStages[i].Runs, stages[i].Runs...)
		} else if i >= len(stages) {
			newStages[i].Runs = append(newStages[i].Runs, p.Stages[i].Runs...)
		} else {
			newStages[i].Runs = append(newStages[i].Runs, p.Stages[i].Runs...)
			newStages[i].Runs = append(newStages[i].Runs, stages[i].Runs...)
		}
	}
	p.Stages = newStages
}

func createStages(w *Workflow, jobIDs ...string) ([]*Stage, error) {
	// first, build a list of all the necessary jobs to run, and their dependencies
	jobDependencies := make(map[string][]string)
	for len(jobIDs) > 0 {
		newJobIDs := make([]string, 0)
		for _, jID := range jobIDs {
			// make sure we haven't visited this job yet
			if _, ok := jobDependencies[jID]; !ok {
				if job := w.GetJob(jID); job != nil {
					jobDependencies[jID] = job.Needs()
					newJobIDs = append(newJobIDs, job.Needs()...)
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
			return nil, fmt.Errorf("unable to build dependency graph for %s (%s)", w.Name, w.File)
		}
		stages = append(stages, stage)
	}

	if len(stages) == 0 {
		return nil, fmt.Errorf("Could not find any stages to run. View the valid jobs with `act --list`. Use `act --help` to find how to filter by Job ID/Workflow/Event Name")
	}

	return stages, nil
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
