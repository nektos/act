package cmd

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/nektos/act/pkg/common"

	"github.com/AlecAivazis/survey/v2"
	"github.com/andreaskoch/go-fswatch"
	"github.com/joho/godotenv"
	"github.com/mitchellh/go-homedir"
	gitignore "github.com/sabhiram/go-gitignore"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
)

// Execute is the entry point to running the CLI
func Execute(ctx context.Context, version string) {
	input := new(Input)
	var rootCmd = &cobra.Command{
		Use:              "act [event name to run]\nIf no event name passed, will default to \"on: push\"",
		Short:            "Run GitHub actions locally by specifying the event name (e.g. `push`) or an action name directly.",
		Args:             cobra.MaximumNArgs(1),
		RunE:             newRunCommand(ctx, input),
		PersistentPreRun: setupLogging,
		Version:          version,
		SilenceUsage:     true,
	}
	rootCmd.Flags().BoolP("watch", "w", false, "watch the contents of the local repo and run when files change")
	rootCmd.Flags().BoolP("list", "l", false, "list workflows")
	rootCmd.Flags().BoolP("graph", "g", false, "draw workflows")
	rootCmd.Flags().StringP("job", "j", "", "run job")
	rootCmd.Flags().StringArrayVarP(&input.secrets, "secret", "s", []string{}, "secret to make available to actions with optional value (e.g. -s mysecret=foo or -s mysecret)")
	rootCmd.Flags().StringArrayVarP(&input.envs, "env", "", []string{}, "env to make available to actions with optional value (e.g. --e myenv=foo or -s myenv)")
	rootCmd.Flags().StringArrayVarP(&input.platforms, "platform", "P", []string{}, "custom image to use per platform (e.g. -P ubuntu-18.04=nektos/act-environments-ubuntu:18.04)")
	rootCmd.Flags().BoolVarP(&input.reuseContainers, "reuse", "r", false, "reuse action containers to maintain state")
	rootCmd.Flags().BoolVarP(&input.bindWorkdir, "bind", "b", false, "bind working directory to container, rather than copy")
	rootCmd.Flags().BoolVarP(&input.forcePull, "pull", "p", false, "pull docker image(s) even if already present")
	rootCmd.Flags().BoolVarP(&input.autodetectEvent, "detect-event", "", false, "Use first event type from workflow as event that triggered the workflow")
	rootCmd.Flags().StringVarP(&input.eventPath, "eventpath", "e", "", "path to event JSON file")
	rootCmd.Flags().StringVar(&input.defaultBranch, "defaultbranch", "", "the name of the main branch")
	rootCmd.Flags().BoolVar(&input.privileged, "privileged", false, "use privileged mode")
	rootCmd.Flags().StringVar(&input.usernsMode, "userns", "", "user namespace to use")
	rootCmd.PersistentFlags().StringVarP(&input.actor, "actor", "a", "nektos/act", "user that triggered the event")
	rootCmd.PersistentFlags().StringVarP(&input.workflowsPath, "workflows", "W", "./.github/workflows/", "path to workflow file(s)")
	rootCmd.PersistentFlags().StringVarP(&input.workdir, "directory", "C", ".", "working directory")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&input.noOutput, "quiet", "q", false, "disable logging of output from steps")
	rootCmd.PersistentFlags().BoolVarP(&input.dryrun, "dryrun", "n", false, "dryrun mode")
	rootCmd.PersistentFlags().StringVarP(&input.secretfile, "secret-file", "", ".secrets", "file with list of secrets to read from (e.g. --secret-file .secrets)")
	rootCmd.PersistentFlags().BoolVarP(&input.insecureSecrets, "insecure-secrets", "", false, "NOT RECOMMENDED! Doesn't hide secrets while printing logs.")
	rootCmd.PersistentFlags().StringVarP(&input.envfile, "env-file", "", ".env", "environment file to read and use as env in the containers")
	rootCmd.PersistentFlags().StringVarP(&input.containerArchitecture, "container-architecture", "", "", "Architecture which should be used to run containers, e.g.: linux/amd64. Defaults to linux/<your machine architecture> [linux/"+runtime.GOARCH+"]. Requires Docker server API Version 1.41+. Ignored on earlier Docker server platforms.")
	rootCmd.SetArgs(args())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func configLocations() []string {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatal(err)
	}

	// reference: https://specifications.freedesktop.org/basedir-spec/latest/ar01s03.html
	var actrcXdg string
	if xdg, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok && xdg != "" {
		actrcXdg = filepath.Join(xdg, ".actrc")
	} else {
		actrcXdg = filepath.Join(home, ".config", ".actrc")
	}

	return []string{
		filepath.Join(home, ".actrc"),
		actrcXdg,
		filepath.Join(".", ".actrc"),
	}
}

func args() []string {
	actrc := configLocations()

	args := make([]string, 0)
	for _, f := range actrc {
		args = append(args, readArgsFile(f)...)
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
	defer func() {
		err := f.Close()
		if err != nil {
			log.Errorf("Failed to close args file: %v", err)
		}
	}()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		arg := scanner.Text()
		if strings.HasPrefix(arg, "-") {
			args = append(args, regexp.MustCompile(`\s`).Split(arg, 2)...)
		}
	}
	return args
}

func setupLogging(cmd *cobra.Command, _ []string) {
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
		if input.envs != nil {
			for _, envVar := range input.envs {
				e := strings.SplitN(envVar, `=`, 2)
				if len(e) == 2 {
					envs[e[0]] = e[1]
				} else {
					envs[e[0]] = ""
				}
			}
		}
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
		events := planner.GetEvents()
		if input.autodetectEvent && len(events) > 0 {
			// set default event type to first event
			// this way user dont have to specify the event.
			log.Debugf("Using detected workflow event: %s", events[0])
			eventName = events[0]
		} else {
			if len(args) > 0 {
				eventName = args[0]
			} else if plan := planner.PlanEvent("push"); plan != nil {
				eventName = "push"
			}
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

		// check if we should just list the workflows
		if list, err := cmd.Flags().GetBool("list"); err != nil {
			return err
		} else if list {
			return printList(plan)
		}

		// check if we should just print the graph
		if list, err := cmd.Flags().GetBool("graph"); err != nil {
			return err
		} else if list {
			return drawGraph(plan)
		}

		// check to see if the main branch was defined
		defaultbranch, err := cmd.Flags().GetString("defaultbranch")
		if err != nil {
			return err
		}

		// Check if platforms flag is set, if not, run default image survey
		if len(input.platforms) == 0 {
			cfgFound := false
			cfgLocations := configLocations()
			for _, v := range cfgLocations {
				_, err := os.Stat(v)
				if os.IsExist(err) {
					cfgFound = true
				}
			}
			if !cfgFound && len(cfgLocations) > 0 {
				if err := defaultImageSurvey(cfgLocations[0]); err != nil {
					log.Fatal(err)
				}
				input.platforms = readArgsFile(cfgLocations[0])
			}
		}

		// run the plan
		config := &runner.Config{
			Actor:                 input.actor,
			EventName:             eventName,
			EventPath:             input.EventPath(),
			DefaultBranch:         defaultbranch,
			ForcePull:             input.forcePull,
			ReuseContainers:       input.reuseContainers,
			Workdir:               input.Workdir(),
			BindWorkdir:           input.bindWorkdir,
			LogOutput:             !input.noOutput,
			Env:                   envs,
			Secrets:               secrets,
			InsecureSecrets:       input.insecureSecrets,
			Platforms:             input.newPlatforms(),
			Privileged:            input.privileged,
			UsernsMode:            input.usernsMode,
			ContainerArchitecture: input.containerArchitecture,
		}
		r, err := runner.New(config)
		if err != nil {
			return err
		}

		ctx = common.WithDryrun(ctx, input.dryrun)
		if watch, err := cmd.Flags().GetBool("watch"); err != nil {
			return err
		} else if watch {
			return watchAndRun(ctx, r.NewPlanExecutor(plan))
		}

		return r.NewPlanExecutor(plan)(ctx)
	}
}

func defaultImageSurvey(actrc string) error {
	var answer string
	confirmation := &survey.Select{
		Message: "Please choose the default image you want to use with act:\n\n  - Large size image: +20GB Docker image, includes almost all tools used on GitHub Actions (IMPORTANT: currently only ubuntu-18.04 platform is available)\n  - Medium size image: ~500MB, includes only necessary tools to bootstrap actions and aims to be compatible with all actions\n  - Micro size image: <200MB, contains only NodeJS required to bootstrap actions, doesn't work with all actions\n\nDefault image and other options can be changed manually in ~/.actrc (please refer to https://github.com/nektos/act#configuration for additional information about file structure)",
		Help:    "If you want to know why act asks you that, please go to https://github.com/nektos/act/issues/107",
		Default: "Medium",
		Options: []string{"Large", "Medium", "Micro"},
	}

	err := survey.AskOne(confirmation, &answer)
	if err != nil {
		return err
	}

	var option string
	switch answer {
	case "Large":
		option = "-P ubuntu-18.04=nektos/act-environments-ubuntu:18.04"
	case "Medium":
		option = "-P ubuntu-latest=catthehacker/ubuntu:act-latest\n-P ubuntu-20.04=catthehacker/ubuntu:act-20.04\n-P ubuntu-18.04=catthehacker/ubuntu:act-18.04\nubuntu-16.04=catthehacker/ubuntu:act-16.04"
	case "Micro":
		option = "-P ubuntu-latest=node:12.20.1-buster-slim\n-P ubuntu-20.04=node:12.20.1-buster-slim\n-P ubuntu-18.04=node:12.20.1-buster-slim\n-P ubuntu-16.04=node:12.20.1-stretch-slim"
	}

	f, err := os.Create(actrc)
	if err != nil {
		return err
	}

	_, err = f.WriteString(option)
	if err != nil {
		_ = f.Close()
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	return nil
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
