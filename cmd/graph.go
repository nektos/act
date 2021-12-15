package cmd

import (
	"os"

	"github.com/nektos/act/pkg/common/utils"
	"github.com/nektos/act/pkg/model"
)

func drawGraph(plan *model.Plan) error {
	drawings := make([]*utils.Drawing, 0)

	jobPen := utils.NewPen(utils.StyleSingleLine, 96)
	arrowPen := utils.NewPen(utils.StyleNoLine, 97)
	for i, stage := range plan.Stages {
		if i > 0 {
			drawings = append(drawings, arrowPen.DrawArrow())
		}

		ids := make([]string, 0)
		for _, r := range stage.Runs {
			ids = append(ids, r.String())
		}
		drawings = append(drawings, jobPen.DrawBoxes(ids...))
	}

	maxWidth := 0
	for _, d := range drawings {
		if d.GetWidth() > maxWidth {
			maxWidth = d.GetWidth()
		}
	}

	for _, d := range drawings {
		d.Draw(os.Stdout, maxWidth)
	}
	return nil
}
