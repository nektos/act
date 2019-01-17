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

// Execute is the entry point to running the CLI
func Execute(ctx context.Context, version string) {
	runnerConfig := &actions.RunnerConfig{Ctx: ctx}
	var rootCmd = &cobra.Command{
		Use:              "act [event name to run]",
		Short:            "Run Github actions locally by specifying the event name (e.g. `push`) or an action name directly.",
		Args:             cobra.MaximumNArgs(1),
		RunE:             newRunCommand(runnerConfig),
		PersistentPreRun: setupLogging,
		Version:          version,
		SilenceUsage:     true,
	}
	rootCmd.Flags().BoolP("list", "l", false, "list actions")
	rootCmd.Flags().StringP("action", "a", "", "run action")
	rootCmd.Flags().StringVarP(&runnerConfig.EventPath, "event", "e", "", "path to event JSON file")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&runnerConfig.Dryrun, "dryrun", "n", false, "dryrun mode")
	rootCmd.PersistentFlags().StringVarP(&runnerConfig.WorkflowPath, "file", "f", "./.github/main.workflow", "path to workflow file")
	rootCmd.PersistentFlags().StringVarP(&runnerConfig.WorkingDir, "directory", "C", ".", "working directory")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

}

func setupLogging(cmd *cobra.Command, args []string) {
	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		log.SetLevel(log.DebugLevel)
	}
}

func newRunCommand(runnerConfig *actions.RunnerConfig) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			runnerConfig.EventName = "push"
		} else {
			runnerConfig.EventName = args[0]
		}

		// create the runner
		runner, err := actions.NewRunner(runnerConfig)
		if err != nil {
			return err
		}
		defer runner.Close()

		// check if we should just print the graph
		list, err := cmd.Flags().GetBool("list")
		if err != nil {
			return err
		}
		if list {
			return drawGraph(runner)
		}

		// check if we are running just a single action
		actionName, err := cmd.Flags().GetString("action")
		if err != nil {
			return err
		}
		if actionName != "" {
			return runner.RunActions(actionName)
		}

		// run the event in the RunnerRonfig
		return runner.RunEvent()
	}
}

func drawGraph(runner actions.Runner) error {
	eventNames := runner.ListEvents()
	for _, eventName := range eventNames {
		graph, err := runner.GraphEvent(eventName)
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
