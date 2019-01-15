package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/nektos/act/actions"
	"github.com/nektos/act/common"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var verbose bool
var workflowPath string
var workingDir string
var list bool
var actionName string
var dryrun bool

// Execute is the entry point to running the CLI
func Execute(ctx context.Context, version string) {
	var rootCmd = &cobra.Command{
		Use:          "act [event name to run]",
		Short:        "Run Github actions locally by specifying the event name (e.g. `push`) or an action name directly.",
		Args:         cobra.MaximumNArgs(1),
		RunE:         newRunAction(ctx),
		Version:      version,
		SilenceUsage: true,
	}
	rootCmd.Flags().BoolVarP(&list, "list", "l", false, "list actions")
	rootCmd.Flags().StringVarP(&actionName, "action", "a", "", "run action")
	rootCmd.PersistentFlags().BoolVarP(&dryrun, "dryrun", "n", false, "dryrun mode")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&workflowPath, "file", "f", "./.github/main.workflow", "path to workflow file")
	rootCmd.PersistentFlags().StringVarP(&workingDir, "directory", "C", ".", "working directory")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

}

func newRunAction(ctx context.Context) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if verbose {
			log.SetLevel(log.DebugLevel)
		}

		workflows, err := actions.ParseWorkflows(workingDir, workflowPath)
		if err != nil {
			return err
		}

		defer workflows.Close()

		if list {
			return listEvents(workflows)
		}

		if actionName != "" {
			return workflows.RunAction(ctx, dryrun, actionName)
		}

		if len(args) == 0 {
			return workflows.RunEvent(ctx, dryrun, "push")
		}
		return workflows.RunEvent(ctx, dryrun, args[0])
	}
}

func listEvents(workflows actions.Workflows) error {
	eventNames := workflows.ListEvents()
	for _, eventName := range eventNames {
		graph, err := workflows.GraphEvent(eventName)
		if err != nil {
			return err
		}

		drawings := make([]*common.Drawing, 0)
		eventPen := common.NewPen(common.StyleDoubleLine, 91 /*34*/)

		drawings = append(drawings, eventPen.DrawBoxes(fmt.Sprintf("EVENT: %s", eventName)))

		actionPen := common.NewPen(common.StyleSingleLine, 96)
		arrowPen := common.NewPen(common.StyleNoLine, 97)
		drawings = append(drawings, arrowPen.DrawArrow())
		for i, stage := range graph {
			if i > 0 {
				drawings = append(drawings, arrowPen.DrawArrow())
			}
			drawings = append(drawings, actionPen.DrawBoxes(stage...))
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
	}
	return nil
}
