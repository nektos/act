package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/common"

	fswatch "github.com/andreaskoch/go-fswatch"
	"github.com/joho/godotenv"
	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
	gitignore "github.com/sabhiram/go-gitignore"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Execute is the entry point to running the CLI
func Execute(ctx context.Context, version string) {
	input := new(Input)
	var rootCmd = &cobra.Command{
		Use:              "act [event name to run]",
		Short:            "Run Github actions locally by specifying the event name (e.g. `push`) or an action name directly.",
		Args:             cobra.MaximumNArgs(1),
		RunE:             newRunCommand(ctx, input),
		PersistentPreRun: setupLogging,
		Version:          version,
		SilenceUsage:     true,
	}
	rootCmd.Flags().BoolP("watch", "w", false, "watch the contents of the local repo and run when files change")
	rootCmd.Flags().BoolP("list", "l", false, "list workflows")
	rootCmd.Flags().StringP("job", "j", "", "run job")
	rootCmd.Flags().StringArrayVarP(&input.secrets, "secret", "s", []string{}, "secret to make available to actions with optional value (e.g. -s mysecret=foo or -s mysecret)")
	rootCmd.Flags().StringArrayVarP(&input.platforms, "platform", "P", []string{}, "custom image to use per platform (e.g. -P ubuntu-18.04=nektos/act-environments-ubuntu:18.04)")
	rootCmd.Flags().BoolVarP(&input.reuseContainers, "reuse", "r", false, "reuse action containers to maintain state")
	rootCmd.Flags().BoolVarP(&input.bindWorkdir, "bind", "b", false, "bind working directory to container, rather than copy")
	rootCmd.Flags().BoolVarP(&input.forcePull, "pull", "p", false, "pull docker image(s) if already present")
	rootCmd.Flags().StringVarP(&input.eventPath, "eventpath", "e", "", "path to event JSON file")
	rootCmd.PersistentFlags().StringVarP(&input.actor, "actor", "a", "nektos/act", "user that triggered the event")
	rootCmd.PersistentFlags().StringVarP(&input.workflowsPath, "workflows", "W", "./.github/workflows/", "path to workflow files")
	rootCmd.PersistentFlags().StringVarP(&input.workdir, "directory", "C", ".", "working directory")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&input.noOutput, "quiet", "q", false, "disable logging of output from steps")
	rootCmd.PersistentFlags().BoolVarP(&input.dryrun, "dryrun", "n", false, "dryrun mode")
	rootCmd.PersistentFlags().StringVarP(&input.secretfile, "secret-file", "", "", "file with list of secrets to read from")
	rootCmd.PersistentFlags().StringVarP(&input.envfile, "env-file", "", ".env", "environment file to read and use as env in the containers")
	rootCmd.SetArgs(args())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

}

func args() []string {
	args := make([]string, 0)
	for _, dir := range []string{
		os.Getenv("HOME"),
		".",
	} {
		args = append(args, readArgsFile(fmt.Sprintf("%s/.actrc", dir))...)
	}
	args = append(args, os.Args[1:]...)
	return args
}

func readArgsFile(file string) []string {
	args := make([]string, 0)
	f, err := os.Open(file)
	if err != nil {
		return args
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		arg := scanner.Text()
		if strings.HasPrefix(arg, "-") {
			args = append(args, regexp.MustCompile(`\s`).Split(arg, 2)...)
		}
	}
	return args

}

func setupLogging(cmd *cobra.Command, args []string) {
	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		log.SetLevel(log.DebugLevel)
	}
}

func readEnvs(path string, envs map[string]string) bool {
	if _, err := os.Stat(path); err == nil {
		env, err := godotenv.Read(path)
		if err != nil {
			log.Fatalf("Error loading from %s: %v", path, err)
		}
		for k, v := range env {
			envs[k] = v
		}
		return true
	}
	return false
}

func newRunCommand(ctx context.Context, input *Input) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.Debugf("Loading environment from %s", input.Envfile())
		envs := make(map[string]string)
		_ = readEnvs(input.Envfile(), envs)

		log.Debugf("Loading secrets from %s", input.Secretfile())
		secrets := newSecrets(input.secrets)
		_ = readEnvs(input.Secretfile(), secrets)

		planner, err := model.NewWorkflowPlanner(input.WorkflowsPath())
		if err != nil {
			return err
		}

		// Determine the event name
		var eventName string
		if len(args) > 0 {
			eventName = args[0]
		} else if plan := planner.PlanEvent("push"); plan != nil {
			eventName = "push"
		} else if events := planner.GetEvents(); len(events) > 0 {
			// set default event type to first event
			// this way user dont have to specify the event.
			log.Debugf("Using detected workflow event: %s", events[0])
			eventName = events[0]
		}

		// build the plan for this run
		var plan *model.Plan
		if jobID, err := cmd.Flags().GetString("job"); err != nil {
			return err
		} else if jobID != "" {
			log.Debugf("Planning job: %s", jobID)
			plan = planner.PlanJob(jobID)
		} else {
			log.Debugf("Planning event: %s", eventName)
			plan = planner.PlanEvent(eventName)
		}

		// check if we should just print the graph
		if list, err := cmd.Flags().GetBool("list"); err != nil {
			return err
		} else if list {
			return drawGraph(plan)
		}

		// run the plan
		config := &runner.Config{
			Actor:           input.actor,
			EventName:       eventName,
			EventPath:       input.EventPath(),
			ForcePull:       input.forcePull,
			ReuseContainers: input.reuseContainers,
			Workdir:         input.Workdir(),
			BindWorkdir:     input.bindWorkdir,
			LogOutput:       !input.noOutput,
			Env:             envs,
			Secrets:         secrets,
			Platforms:       input.newPlatforms(),
		}
		runner, err := runner.New(config)
		if err != nil {
			return err
		}

		ctx = common.WithDryrun(ctx, input.dryrun)
		if watch, err := cmd.Flags().GetBool("watch"); err != nil {
			return err
		} else if watch {
			return watchAndRun(ctx, runner.NewPlanExecutor(plan))
		}

		return runner.NewPlanExecutor(plan)(ctx)
	}
}

func watchAndRun(ctx context.Context, fn common.Executor) error {
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
			if err = fn(ctx); err != nil {
				break
			}
			log.Debugf("Watching %s for changes", dir)
			for changes := range folderWatcher.ChangeDetails() {
				log.Debugf("%s", changes.String())
				if err = fn(ctx); err != nil {
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
