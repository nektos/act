package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	fswatch "github.com/andreaskoch/go-fswatch"
	"github.com/nektos/act/actions"
	"github.com/nektos/act/common"
	gitignore "github.com/sabhiram/go-gitignore"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	git "gopkg.in/src-d/go-git.v4"
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

	rootCmd.Flags().BoolP("watch", "w", false, "watch the contents of the local repo and run when files change")
	rootCmd.Flags().BoolP("list", "l", false, "list actions")
	rootCmd.Flags().StringP("action", "a", "", "run action")
	rootCmd.Flags().BoolVarP(&runnerConfig.Init, "init", "i", false, "init the local action")
	rootCmd.Flags().StringVarP(&runnerConfig.InitRepo, "init-repo", "", "https://github.com/nektos/act", "init template repository")
	rootCmd.Flags().BoolVarP(&runnerConfig.ReuseContainers, "reuse", "r", false, "reuse action containers to maintain state")
	rootCmd.Flags().StringVarP(&runnerConfig.EventPath, "event", "e", "", "path to event JSON file")
	rootCmd.Flags().BoolVarP(&runnerConfig.ForcePull, "pull", "p", false, "pull docker image(s) if already present")
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
		if len(args) > 0 {
			runnerConfig.EventName = args[0]
		}

		watch, err := cmd.Flags().GetBool("watch")
		if err != nil {
			return err
		}
		if watch {
			return watchAndRun(runnerConfig.Ctx, func() error {
				return parseAndRun(cmd, runnerConfig)
			})
		}
		return parseAndRun(cmd, runnerConfig)
	}
}

func parseAndRun(cmd *cobra.Command, runnerConfig *actions.RunnerConfig) error {
	// check if we should scaffold a new action
	init, err := cmd.Flags().GetBool("init")
	if err != nil {
		return err
	}
	if init {
		return initAction(runnerConfig)
	}

	// create the runner
	runner, err := actions.NewRunner(runnerConfig)
	if err != nil {
		return err
	}
	defer runner.Close()

	// set default event type if we only have a single workflow in the file.
	// this way user dont have to specify the event.
	if runnerConfig.EventName == "" {
		if events := runner.ListEvents(); len(events) == 1 {
			log.Debugf("Using detected workflow event: %s", events[0])
			runnerConfig.EventName = events[0]
		}
	}

	// fall back to default event name if we could not detect one.
	if runnerConfig.EventName == "" {
		runnerConfig.EventName = "push"
	}

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

func watchAndRun(ctx context.Context, fn func() error) error {
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

	go func() {
		for folderWatcher.IsRunning() {
			if err = fn(); err != nil {
				break
			}
			log.Debugf("Watching %s for changes", dir)
			for changes := range folderWatcher.ChangeDetails() {
				log.Debugf("%s", changes.String())
				if err = fn(); err != nil {
					break
				}
				log.Debugf("Watching %s for changes", dir)
			}
		}
	}()
	<-ctx.Done()
	folderWatcher.Stop()
	return err
}

func initAction(config *actions.RunnerConfig) error {
	// use the default event name if we don't have one
	if config.EventName == "" {
		config.EventName = "push"
	}

	log.Printf("Setting up a new action for %s event\n", config.EventName)
	baseDir := filepath.Dir(config.WorkflowPath)

	// check if workflow directory exists, skip setup if it's already configured
	if _, err := os.Stat(baseDir); err == nil {
		log.Println("Workspace directory already exists, skipping")
		return nil
	}

	// prepare clone repository
	repoCloneDir, err := ioutil.TempDir("", "act")
	if err != nil {
		log.Println("Can't get temp directory for clone:", err)
		return err
	}
	defer os.RemoveAll(repoCloneDir)

	// clone repository
	_, err = git.PlainClone(repoCloneDir, false, &git.CloneOptions{
		URL: config.InitRepo,
	})
	if err != nil {
		log.Println("Can't clone repository:", err)
		return err
	}

	// copy template
	templateDir := filepath.Join(repoCloneDir, "templates/"+config.EventName)
	if err := common.CopyDir(templateDir, baseDir); err != nil {
		log.Println("Can't copy template:", err)
		return err
	}

	log.Printf("Done. You can now run `act %s` to test things out!", config.EventName)

	return nil
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
