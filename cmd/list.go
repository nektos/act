package cmd

import (
	"fmt"
	"strconv"

	"github.com/nektos/act/pkg/model"
)

func printList(plan *model.Plan) error {
	type lineInfoDef struct {
		id    string
		stage string
		name  string
	}
	lineInfos := []lineInfoDef{}

	header := lineInfoDef{
		id:    "ID",
		stage: "Stage",
		name:  "Name",
	}

	jobs := map[string]bool{}
	duplicateJobIDs := false

	idMaxWidth := len(header.id)
	stageMaxWidth := len(header.stage)
	nameMaxWidth := len(header.name)

	for i, stage := range plan.Stages {
		for _, r := range stage.Runs {
			jobID := r.JobID
			line := lineInfoDef{
				id:    jobID,
				stage: strconv.Itoa(i),
				name:  r.String(),
			}
			if _, ok := jobs[jobID]; ok {
				duplicateJobIDs = true
			} else {
				jobs[jobID] = true
			}
			lineInfos = append(lineInfos, line)
			if idMaxWidth < len(line.id) {
				idMaxWidth = len(line.id)
			}
			if stageMaxWidth < len(line.stage) {
				stageMaxWidth = len(line.stage)
			}
			if nameMaxWidth < len(line.name) {
				nameMaxWidth = len(line.name)
			}
		}
	}

	idMaxWidth += 2
	stageMaxWidth += 2
	nameMaxWidth += 2

	fmt.Printf("%*s%*s%*s\n", -idMaxWidth, header.id, -stageMaxWidth, header.stage, -nameMaxWidth, header.name)
	for _, line := range lineInfos {
		fmt.Printf("%*s%*s%*s\n", -idMaxWidth, line.id, -stageMaxWidth, line.stage, -nameMaxWidth, line.name)
	}
	if duplicateJobIDs {
		fmt.Print("\nDetected multiple jobs with the same job name, use `-W` to specify the path to the specific workflow.\n")
	}
	return nil
}
