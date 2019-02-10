package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/nektos/act/actions"
	"github.com/nektos/act/common"

	fswatch "github.com/andreaskoch/go-fswatch"
	gitignore "github.com/sabhiram/go-gitignore"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var verbose bool
var workflowPath string
var workingDir string
var list bool
var watch bool
var actionName string
var dryrun bool
var eventPath string

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
	rootCmd.Flags().BoolVarP(&watch, "watch", "w", false, "watch the contents of the local repo and run when files change")
	rootCmd.Flags().BoolVarP(&list, "list", "l", false, "list actions")
	rootCmd.Flags().StringVarP(&actionName, "action", "a", "", "run action")
	rootCmd.Flags().StringVarP(&eventPath, "event", "e", "", "path to event JSON file")
	rootCmd.PersistentFlags().BoolVarP(&dryrun, "dryrun", "n", false, "dryrun mode")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&workflowPath, "file", "f", "./.github/main.workflow", "path to workflow file")
	rootCmd.PersistentFlags().StringVarP(&workingDir, "directory", "C", ".", "working directory")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

}

func watchAndRun(ctx context.Context, fn func()) error {
	recurse := true
	checkIntervalInSeconds := 2
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	var ignore *gitignore.GitIgnore
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		ignore, _ = gitignore.CompileIgnoreFile(filepath.Join(dir, ".gitignore"))
	} else {
		ignore = &gitignore.GitIgnore{}
	}

	folderWatcher := fswatch.NewFolderWatcher(
		dir,
		recurse,
		ignore.MatchesPath,
		checkIntervalInSeconds,
	)

	folderWatcher.Start()
	log.Debugf("Watching %s for changes", dir)

	go func() {
		for folderWatcher.IsRunning() {
			for changes := range folderWatcher.ChangeDetails() {
				log.Debugf("%s", changes.String())
				fn()
				log.Debugf("Watching %s for changes", dir)
			}
		}
	}()
	<-ctx.Done()
	folderWatcher.Stop()
	return nil
}

func parseAndRun(ctx context.Context, args []string) error {
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

	eventJSON := "{}"
	if eventPath != "" {
		if !filepath.IsAbs(eventPath) {
			eventPath = filepath.Join(workingDir, eventPath)
		}
		log.Debugf("Reading event.json from %s", eventPath)
		eventJSONBytes, err := ioutil.ReadFile(eventPath)
		if err != nil {
			return err
		}
		eventJSON = string(eventJSONBytes)
	}

	if actionName != "" {
		return workflows.RunAction(ctx, dryrun, actionName, eventJSON)
	}

	if len(args) == 0 {
		return workflows.RunEvent(ctx, dryrun, "push", eventJSON)
	}
	return workflows.RunEvent(ctx, dryrun, args[0], eventJSON)

}

func newRunAction(ctx context.Context) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := parseAndRun(ctx, args)

		if err == nil && watch {
			return watchAndRun(ctx, func() {
				err = parseAndRun(ctx, args)
			})
		}

		return err
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
