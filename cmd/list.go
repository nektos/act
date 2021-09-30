package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nektos/act/pkg/model"
)

func printList(plan *model.Plan) error {
	type lineInfoDef struct {
		jobID   string
		jobName string
		stage   string
		wfName  string
		wfFile  string
		events  string
	}
	lineInfos := []lineInfoDef{}

	header := lineInfoDef{
		jobID:   "Job ID",
		jobName: "Job name",
		stage:   "Stage",
		wfName:  "Workflow name",
		wfFile:  "Workflow file",
		events:  "Events",
	}

	jobs := map[string]bool{}
	duplicateJobIDs := false

	jobIDMaxWidth := len(header.jobID)
	jobNameMaxWidth := len(header.jobName)
	stageMaxWidth := len(header.stage)
	wfNameMaxWidth := len(header.wfName)
	wfFileMaxWidth := len(header.wfFile)
	eventsMaxWidth := len(header.events)

	for i, stage := range plan.Stages {
		for _, r := range stage.Runs {
			jobID := r.JobID
			line := lineInfoDef{
				jobID:   jobID,
				jobName: r.String(),
				stage:   strconv.Itoa(i),
				wfName:  r.Workflow.Name,
				wfFile:  r.Workflow.File,
				events:  strings.Join(r.Workflow.On(), `,`),
			}
			if _, ok := jobs[jobID]; ok {
				duplicateJobIDs = true
			} else {
				jobs[jobID] = true
			}
			lineInfos = append(lineInfos, line)
			if jobIDMaxWidth < len(line.jobID) {
				jobIDMaxWidth = len(line.jobID)
			}
			if jobNameMaxWidth < len(line.jobName) {
				jobNameMaxWidth = len(line.jobName)
			}
			if stageMaxWidth < len(line.stage) {
				stageMaxWidth = len(line.stage)
			}
			if wfNameMaxWidth < len(line.wfName) {
				wfNameMaxWidth = len(line.wfName)
			}
			if wfFileMaxWidth < len(line.wfFile) {
				wfFileMaxWidth = len(line.wfFile)
			}
			if eventsMaxWidth < len(line.events) {
				eventsMaxWidth = len(line.events)
			}
		}
	}

	jobIDMaxWidth += 2
	jobNameMaxWidth += 2
	stageMaxWidth += 2
	wfNameMaxWidth += 2
	wfFileMaxWidth += 2

	fmt.Printf("%*s%*s%*s%*s%*s%*s\n",
		-stageMaxWidth, header.stage,
		-jobIDMaxWidth, header.jobID,
		-jobNameMaxWidth, header.jobName,
		-wfNameMaxWidth, header.wfName,
		-wfFileMaxWidth, header.wfFile,
		-eventsMaxWidth, header.events,
	)
	for _, line := range lineInfos {
		fmt.Printf("%*s%*s%*s%*s%*s%*s\n",
			-stageMaxWidth, line.stage,
			-jobIDMaxWidth, line.jobID,
			-jobNameMaxWidth, line.jobName,
			-wfNameMaxWidth, line.wfName,
			-wfFileMaxWidth, line.wfFile,
			-eventsMaxWidth, line.events,
		)
	}
	if duplicateJobIDs {
		fmt.Print("\nDetected multiple jobs with the same job name, use `-W` to specify the path to the specific workflow.\n")
	}
	return nil
}
