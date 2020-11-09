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

	idMaxWidth := len(header.id)
	stageMaxWidth := len(header.stage)
	nameMaxWidth := len(header.name)

	for i, stage := range plan.Stages {
		for _, r := range stage.Runs {
			line := lineInfoDef{
				id:    r.JobID,
				stage: strconv.Itoa(i),
				name:  r.String(),
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
	return nil
}
