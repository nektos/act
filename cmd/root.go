package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/adrg/xdg"
	"github.com/andreaskoch/go-fswatch"
	docker_container "github.com/docker/docker/api/types/container"
	"github.com/joho/godotenv"
	gitignore "github.com/sabhiram/go-gitignore"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/nektos/act/pkg/artifactcache"
	"github.com/nektos/act/pkg/artifacts"
	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/container"
	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
)

// Execute is the entry point to running the CLI
func Execute(ctx context.Context, version string) {
	input := new(Input)
	var rootCmd = &cobra.Command{
		Use:               "act [event name to run] [flags]\n\nIf no event name passed, will default to \"on: push\"\nIf actions handles only one event it will be used as default instead of \"on: push\"",
		Short:             "Run GitHub actions locally by specifying the event name (e.g. `push`) or an action name directly.",
		Args:              cobra.MaximumNArgs(1),
		RunE:              newRunCommand(ctx, input),
		PersistentPreRun:  setup(input),
		PersistentPostRun: cleanup(input),
		Version:           version,
		SilenceUsage:      true,
	}
	rootCmd.Flags().BoolP("watch", "w", false, "watch the contents of the local repo and run when files change")
	rootCmd.Flags().BoolP("list", "l", false, "list workflows")
	rootCmd.Flags().BoolP("graph", "g", false, "draw workflows")
	rootCmd.Flags().StringP("job", "j", "", "run a specific job ID")
	rootCmd.Flags().BoolP("bug-report", "", false, "Display system information for bug report")

	rootCmd.Flags().StringVar(&input.remoteName, "remote-name", "origin", "git remote name that will be used to retrieve url of git repo")
	rootCmd.Flags().StringArrayVarP(&input.secrets, "secret", "s", []string{}, "secret to make available to actions with optional value (e.g. -s mysecret=foo or -s mysecret)")
	rootCmd.Flags().StringArrayVar(&input.vars, "var", []string{}, "variable to make available to actions with optional value (e.g. --var myvar=foo or --var myvar)")
	rootCmd.Flags().StringArrayVarP(&input.envs, "env", "", []string{}, "env to make available to actions with optional value (e.g. --env myenv=foo or --env myenv)")
	rootCmd.Flags().StringArrayVarP(&input.inputs, "input", "", []string{}, "action input to make available to actions (e.g. --input myinput=foo)")
	rootCmd.Flags().StringArrayVarP(&input.platforms, "platform", "P", []string{}, "custom image to use per platform (e.g. -P ubuntu-18.04=nektos/act-environments-ubuntu:18.04)")
	rootCmd.Flags().BoolVarP(&input.reuseContainers, "reuse", "r", false, "don't remove container(s) on successfully completed workflow(s) to maintain state between runs")
	rootCmd.Flags().BoolVarP(&input.bindWorkdir, "bind", "b", false, "bind working directory to container, rather than copy")
	rootCmd.Flags().BoolVarP(&input.forcePull, "pull", "p", true, "pull docker image(s) even if already present")
	rootCmd.Flags().BoolVarP(&input.forceRebuild, "rebuild", "", true, "rebuild local action docker image(s) even if already present")
	rootCmd.Flags().BoolVarP(&input.autodetectEvent, "detect-event", "", false, "Use first event type from workflow as event that triggered the workflow")
	rootCmd.Flags().StringVarP(&input.eventPath, "eventpath", "e", "", "path to event JSON file")
	rootCmd.Flags().StringVar(&input.defaultBranch, "defaultbranch", "", "the name of the main branch")
	rootCmd.Flags().BoolVar(&input.privileged, "privileged", false, "use privileged mode")
	rootCmd.Flags().StringVar(&input.usernsMode, "userns", "", "user namespace to use")
	rootCmd.Flags().BoolVar(&input.useGitIgnore, "use-gitignore", true, "Controls whether paths specified in .gitignore should be copied into container")
	rootCmd.Flags().StringArrayVarP(&input.containerCapAdd, "container-cap-add", "", []string{}, "kernel capabilities to add to the workflow containers (e.g. --container-cap-add SYS_PTRACE)")
	rootCmd.Flags().StringArrayVarP(&input.containerCapDrop, "container-cap-drop", "", []string{}, "kernel capabilities to remove from the workflow containers (e.g. --container-cap-drop SYS_PTRACE)")
	rootCmd.Flags().BoolVar(&input.autoRemove, "rm", false, "automatically remove container(s)/volume(s) after a workflow(s) failure")
	rootCmd.Flags().StringArrayVarP(&input.replaceGheActionWithGithubCom, "replace-ghe-action-with-github-com", "", []string{}, "If you are using GitHub Enterprise Server and allow specified actions from GitHub (github.com), you can set actions on this. (e.g. --replace-ghe-action-with-github-com =github/super-linter)")
	rootCmd.Flags().StringVar(&input.replaceGheActionTokenWithGithubCom, "replace-ghe-action-token-with-github-com", "", "If you are using replace-ghe-action-with-github-com  and you want to use private actions on GitHub, you have to set personal access token")
	rootCmd.Flags().StringArrayVarP(&input.matrix, "matrix", "", []string{}, "specify which matrix configuration to include (e.g. --matrix java:13")
	rootCmd.PersistentFlags().StringVarP(&input.actor, "actor", "a", "nektos/act", "user that triggered the event")
	rootCmd.PersistentFlags().StringVarP(&input.workflowsPath, "workflows", "W", "./.github/workflows/", "path to workflow file(s)")
	rootCmd.PersistentFlags().BoolVarP(&input.noWorkflowRecurse, "no-recurse", "", false, "Flag to disable running workflows from subdirectories of specified path in '--workflows'/'-W' flag")
	rootCmd.PersistentFlags().StringVarP(&input.workdir, "directory", "C", ".", "working directory")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&input.jsonLogger, "json", false, "Output logs in json format")
	rootCmd.PersistentFlags().BoolVar(&input.logPrefixJobID, "log-prefix-job-id", false, "Output the job id within non-json logs instead of the entire name")
	rootCmd.PersistentFlags().BoolVarP(&input.noOutput, "quiet", "q", false, "disable logging of output from steps")
	rootCmd.PersistentFlags().BoolVarP(&input.dryrun, "dryrun", "n", false, "dryrun mode")
	rootCmd.PersistentFlags().StringVarP(&input.secretfile, "secret-file", "", ".secrets", "file with list of secrets to read from (e.g. --secret-file .secrets)")
	rootCmd.PersistentFlags().StringVarP(&input.varfile, "var-file", "", ".vars", "file with list of vars to read from (e.g. --var-file .vars)")
	rootCmd.PersistentFlags().BoolVarP(&input.insecureSecrets, "insecure-secrets", "", false, "NOT RECOMMENDED! Doesn't hide secrets while printing logs.")
	rootCmd.PersistentFlags().StringVarP(&input.envfile, "env-file", "", ".env", "environment file to read and use as env in the containers")
	rootCmd.PersistentFlags().StringVarP(&input.inputfile, "input-file", "", ".input", "input file to read and use as action input")
	rootCmd.PersistentFlags().StringVarP(&input.containerArchitecture, "container-architecture", "", "", "Architecture which should be used to run containers, e.g.: linux/amd64. If not specified, will use host default architecture. Requires Docker server API Version 1.41+. Ignored on earlier Docker server platforms.")
	rootCmd.PersistentFlags().StringVarP(&input.containerDaemonSocket, "container-daemon-socket", "", "", "URI to Docker Engine socket (e.g.: unix://~/.docker/run/docker.sock or - to disable bind mounting the socket)")
	rootCmd.PersistentFlags().StringVarP(&input.containerOptions, "container-options", "", "", "Custom docker container options for the job container without an options property in the job definition")
	rootCmd.PersistentFlags().StringVarP(&input.githubInstance, "github-instance", "", "github.com", "GitHub instance to use. Don't use this if you are not using GitHub Enterprise Server.")
	rootCmd.PersistentFlags().StringVarP(&input.artifactServerPath, "artifact-server-path", "", "", "Defines the path where the artifact server stores uploads and retrieves downloads from. If not specified the artifact server will not start.")
	rootCmd.PersistentFlags().StringVarP(&input.artifactServerAddr, "artifact-server-addr", "", common.GetOutboundIP().String(), "Defines the address to which the artifact server binds.")
	rootCmd.PersistentFlags().StringVarP(&input.artifactServerPort, "artifact-server-port", "", "34567", "Defines the port where the artifact server listens.")
	rootCmd.PersistentFlags().BoolVarP(&input.noSkipCheckout, "no-skip-checkout", "", false, "Do not skip actions/checkout")
	rootCmd.PersistentFlags().BoolVarP(&input.noCacheServer, "no-cache-server", "", false, "Disable cache server")
	rootCmd.PersistentFlags().StringVarP(&input.cacheServerPath, "cache-server-path", "", filepath.Join(CacheHomeDir, "actcache"), "Defines the path where the cache server stores caches.")
	rootCmd.PersistentFlags().StringVarP(&input.cacheServerAddr, "cache-server-addr", "", common.GetOutboundIP().String(), "Defines the address to which the cache server binds.")
	rootCmd.PersistentFlags().Uint16VarP(&input.cacheServerPort, "cache-server-port", "", 0, "Defines the port where the artifact server listens. 0 means a randomly available port.")
	rootCmd.PersistentFlags().StringVarP(&input.actionCachePath, "action-cache-path", "", filepath.Join(CacheHomeDir, "act"), "Defines the path where the actions get cached and host workspaces created.")
	rootCmd.PersistentFlags().StringVarP(&input.networkName, "network", "", "host", "Sets a docker network name. Defaults to host.")
	rootCmd.PersistentFlags().BoolVarP(&input.useNewActionCache, "use-new-action-cache", "", false, "Enable using the new Action Cache for storing Actions locally")
	rootCmd.SetArgs(args())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func configLocations() []string {
	configFileName := ".actrc"

	// reference: https://specifications.freedesktop.org/basedir-spec/latest/ar01s03.html
	var actrcXdg string
	for _, fileName := range []string{"act/actrc", configFileName} {
		if foundConfig, err := xdg.SearchConfigFile(fileName); foundConfig != "" && err == nil {
			actrcXdg = foundConfig
			break
		}
	}

	return []string{
		filepath.Join(UserHomeDir, configFileName),
		actrcXdg,
		filepath.Join(".", configFileName),
	}
}

var commonSocketPaths = []string{
	"/var/run/docker.sock",
	"/run/podman/podman.sock",
	"$HOME/.colima/docker.sock",
	"$XDG_RUNTIME_DIR/docker.sock",
	"$XDG_RUNTIME_DIR/podman/podman.sock",
	`\\.\pipe\docker_engine`,
	"$HOME/.docker/run/docker.sock",
}

// returns socket path or false if not found any
func socketLocation() (string, bool) {
	if dockerHost, exists := os.LookupEnv("DOCKER_HOST"); exists {
		return dockerHost, true
	}

	for _, p := range commonSocketPaths {
		if _, err := os.Lstat(os.ExpandEnv(p)); err == nil {
			if strings.HasPrefix(p, `\\.\`) {
				return "npipe://" + filepath.ToSlash(os.ExpandEnv(p)), true
			}
			return "unix://" + filepath.ToSlash(os.ExpandEnv(p)), true
		}
	}

	return "", false
}

func args() []string {
	actrc := configLocations()

	args := make([]string, 0)
	for _, f := range actrc {
		args = append(args, readArgsFile(f, true)...)
	}

	args = append(args, os.Args[1:]...)
	return args
}

func bugReport(ctx context.Context, version string) error {
	sprintf := func(key, val string) string {
		return fmt.Sprintf("%-24s%s\n", key, val)
	}

	report := sprintf("act version:", version)
	report += sprintf("GOOS:", runtime.GOOS)
	report += sprintf("GOARCH:", runtime.GOARCH)
	report += sprintf("NumCPU:", fmt.Sprint(runtime.NumCPU()))

	var dockerHost string
	var exists bool
	if dockerHost, exists = os.LookupEnv("DOCKER_HOST"); !exists {
		dockerHost = "DOCKER_HOST environment variable is not set"
	} else if dockerHost == "" {
		dockerHost = "DOCKER_HOST environment variable is empty."
	}

	report += sprintf("Docker host:", dockerHost)
	report += fmt.Sprintln("Sockets found:")
	for _, p := range commonSocketPaths {
		if _, err := os.Lstat(os.ExpandEnv(p)); err != nil {
			continue
		} else if _, err := os.Stat(os.ExpandEnv(p)); err != nil {
			report += fmt.Sprintf("\t%s(broken)\n", p)
		} else {
			report += fmt.Sprintf("\t%s\n", p)
		}
	}

	report += sprintf("Config files:", "")
	for _, c := range configLocations() {
		args := readArgsFile(c, false)
		if len(args) > 0 {
			report += fmt.Sprintf("\t%s:\n", c)
			for _, l := range args {
				report += fmt.Sprintf("\t\t%s\n", l)
			}
		}
	}

	vcs, ok := debug.ReadBuildInfo()
	if ok && vcs != nil {
		report += fmt.Sprintln("Build info:")
		vcs := *vcs
		report += sprintf("\tGo version:", vcs.GoVersion)
		report += sprintf("\tModule path:", vcs.Path)
		report += sprintf("\tMain version:", vcs.Main.Version)
		report += sprintf("\tMain path:", vcs.Main.Path)
		report += sprintf("\tMain checksum:", vcs.Main.Sum)

		report += fmt.Sprintln("\tBuild settings:")
		for _, set := range vcs.Settings {
			report += sprintf(fmt.Sprintf("\t\t%s:", set.Key), set.Value)
		}
	}

	info, err := container.GetHostInfo(ctx)
	if err != nil {
		fmt.Println(report)
		return err
	}

	report += fmt.Sprintln("Docker Engine:")

	report += sprintf("\tEngine version:", info.ServerVersion)
	report += sprintf("\tEngine runtime:", info.DefaultRuntime)
	report += sprintf("\tCgroup version:", info.CgroupVersion)
	report += sprintf("\tCgroup driver:", info.CgroupDriver)
	report += sprintf("\tStorage driver:", info.Driver)
	report += sprintf("\tRegistry URI:", info.IndexServerAddress)

	report += sprintf("\tOS:", info.OperatingSystem)
	report += sprintf("\tOS type:", info.OSType)
	report += sprintf("\tOS version:", info.OSVersion)
	report += sprintf("\tOS arch:", info.Architecture)
	report += sprintf("\tOS kernel:", info.KernelVersion)
	report += sprintf("\tOS CPU:", fmt.Sprint(info.NCPU))
	report += sprintf("\tOS memory:", fmt.Sprintf("%d MB", info.MemTotal/1024/1024))

	report += fmt.Sprintln("\tSecurity options:")
	for _, secopt := range info.SecurityOptions {
		report += fmt.Sprintf("\t\t%s\n", secopt)
	}

	fmt.Println(report)
	return nil
}

func readArgsFile(file string, split bool) []string {
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
		arg := os.ExpandEnv(strings.TrimSpace(scanner.Text()))

		if strings.HasPrefix(arg, "-") && split {
			args = append(args, regexp.MustCompile(`\s`).Split(arg, 2)...)
		} else if !split {
			args = append(args, arg)
		}
	}
	return args
}

func setup(_ *Input) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, _ []string) {
		verbose, _ := cmd.Flags().GetBool("verbose")
		if verbose {
			log.SetLevel(log.DebugLevel)
		}
		loadVersionNotices(cmd.Version)
	}
}

func cleanup(inputs *Input) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, _ []string) {
		displayNotices(inputs)
	}
}

func parseEnvs(env []string, envs map[string]string) bool {
	if env != nil {
		for _, envVar := range env {
			e := strings.SplitN(envVar, `=`, 2)
			if len(e) == 2 {
				envs[e[0]] = e[1]
			} else {
				envs[e[0]] = ""
			}
		}
		return true
	}
	return false
}

func readYamlFile(file string) (map[string]string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	ret := map[string]string{}
	if err = yaml.Unmarshal(content, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func readEnvs(path string, envs map[string]string) bool {
	if _, err := os.Stat(path); err == nil {
		var env map[string]string
		if ext := filepath.Ext(path); ext == ".yml" || ext == ".yaml" {
			env, err = readYamlFile(path)
		} else {
			env, err = godotenv.Read(path)
		}
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

func parseMatrix(matrix []string) map[string]map[string]bool {
	// each matrix entry should be of the form - string:string
	r := regexp.MustCompile(":")
	matrixes := make(map[string]map[string]bool)
	for _, m := range matrix {
		matrix := r.Split(m, 2)
		if len(matrix) < 2 {
			log.Fatalf("Invalid matrix format. Failed to parse %s", m)
		}
		if _, ok := matrixes[matrix[0]]; !ok {
			matrixes[matrix[0]] = make(map[string]bool)
		}
		matrixes[matrix[0]][matrix[1]] = true
	}
	return matrixes
}

func isDockerHostURI(daemonPath string) bool {
	if protoIndex := strings.Index(daemonPath, "://"); protoIndex != -1 {
		scheme := daemonPath[:protoIndex]
		if strings.IndexFunc(scheme, func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
		}) == -1 {
			return true
		}
	}
	return false
}

//nolint:gocyclo
func newRunCommand(ctx context.Context, input *Input) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if input.jsonLogger {
			log.SetFormatter(&log.JSONFormatter{})
		}

		if ok, _ := cmd.Flags().GetBool("bug-report"); ok {
			return bugReport(ctx, cmd.Version)
		}

		// Prefer DOCKER_HOST, don't override it
		socketPath, hasDockerHost := os.LookupEnv("DOCKER_HOST")
		if !hasDockerHost {
			// a - in containerDaemonSocket means don't mount, preserve this value
			// otherwise if input.containerDaemonSocket is a filepath don't use it as socketPath
			skipMount := input.containerDaemonSocket == "-" || !isDockerHostURI(input.containerDaemonSocket)
			if input.containerDaemonSocket != "" && !skipMount {
				socketPath = input.containerDaemonSocket
			} else {
				socket, found := socketLocation()
				if !found {
					log.Errorln("daemon Docker Engine socket not found and containerDaemonSocket option was not set")
				} else {
					socketPath = socket
				}
				if !skipMount {
					input.containerDaemonSocket = socketPath
				}
			}
			os.Setenv("DOCKER_HOST", socketPath)
		}

		if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" && input.containerArchitecture == "" {
			l := log.New()
			l.SetFormatter(&log.TextFormatter{
				DisableQuote:     true,
				DisableTimestamp: true,
			})
			l.Warnf(" \U000026A0 You are using Apple M-series chip and you have not specified container architecture, you might encounter issues while running act. If so, try running it with '--container-architecture linux/amd64'. \U000026A0 \n")
		}

		log.Debugf("Loading environment from %s", input.Envfile())
		envs := make(map[string]string)
		_ = parseEnvs(input.envs, envs)
		_ = readEnvs(input.Envfile(), envs)

		log.Debugf("Loading action inputs from %s", input.Inputfile())
		inputs := make(map[string]string)
		_ = parseEnvs(input.inputs, inputs)
		_ = readEnvs(input.Inputfile(), inputs)

		log.Debugf("Loading secrets from %s", input.Secretfile())
		secrets := newSecrets(input.secrets)
		_ = readEnvs(input.Secretfile(), secrets)

		log.Debugf("Loading vars from %s", input.Varfile())
		vars := newSecrets(input.vars)
		_ = readEnvs(input.Varfile(), vars)

		matrixes := parseMatrix(input.matrix)
		log.Debugf("Evaluated matrix inclusions: %v", matrixes)

		planner, err := model.NewWorkflowPlanner(input.WorkflowsPath(), input.noWorkflowRecurse)
		if err != nil {
			return err
		}

		jobID, err := cmd.Flags().GetString("job")
		if err != nil {
			return err
		}

		// check if we should just list the workflows
		list, err := cmd.Flags().GetBool("list")
		if err != nil {
			return err
		}

		// check if we should just draw the graph
		graph, err := cmd.Flags().GetBool("graph")
		if err != nil {
			return err
		}

		// collect all events from loaded workflows
		events := planner.GetEvents()

		// plan with filtered jobs - to be used for filtering only
		var filterPlan *model.Plan

		// Determine the event name to be filtered
		var filterEventName string

		if len(args) > 0 {
			log.Debugf("Using first passed in arguments event for filtering: %s", args[0])
			filterEventName = args[0]
		} else if input.autodetectEvent && len(events) > 0 && len(events[0]) > 0 {
			// set default event type to first event from many available
			// this way user dont have to specify the event.
			log.Debugf("Using first detected workflow event for filtering: %s", events[0])
			filterEventName = events[0]
		}

		var plannerErr error
		if jobID != "" {
			log.Debugf("Preparing plan with a job: %s", jobID)
			filterPlan, plannerErr = planner.PlanJob(jobID)
		} else if filterEventName != "" {
			log.Debugf("Preparing plan for a event: %s", filterEventName)
			filterPlan, plannerErr = planner.PlanEvent(filterEventName)
		} else {
			log.Debugf("Preparing plan with all jobs")
			filterPlan, plannerErr = planner.PlanAll()
		}
		if filterPlan == nil && plannerErr != nil {
			return plannerErr
		}

		if list {
			err = printList(filterPlan)
			if err != nil {
				return err
			}
			return plannerErr
		}

		if graph {
			err = drawGraph(filterPlan)
			if err != nil {
				return err
			}
			return plannerErr
		}

		// plan with triggered jobs
		var plan *model.Plan

		// Determine the event name to be triggered
		var eventName string

		if len(args) > 0 {
			log.Debugf("Using first passed in arguments event: %s", args[0])
			eventName = args[0]
		} else if len(events) == 1 && len(events[0]) > 0 {
			log.Debugf("Using the only detected workflow event: %s", events[0])
			eventName = events[0]
		} else if input.autodetectEvent && len(events) > 0 && len(events[0]) > 0 {
			// set default event type to first event from many available
			// this way user dont have to specify the event.
			log.Debugf("Using first detected workflow event: %s", events[0])
			eventName = events[0]
		} else {
			log.Debugf("Using default workflow event: push")
			eventName = "push"
		}

		// build the plan for this run
		if jobID != "" {
			log.Debugf("Planning job: %s", jobID)
			plan, plannerErr = planner.PlanJob(jobID)
		} else {
			log.Debugf("Planning jobs for event: %s", eventName)
			plan, plannerErr = planner.PlanEvent(eventName)
		}
		if plan == nil && plannerErr != nil {
			return plannerErr
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
				input.platforms = readArgsFile(cfgLocations[0], true)
			}
		}
		deprecationWarning := "--%s is deprecated and will be removed soon, please switch to cli: `--container-options \"%[2]s\"` or `.actrc`: `--container-options %[2]s`."
		if input.privileged {
			log.Warnf(deprecationWarning, "privileged", "--privileged")
		}
		if len(input.usernsMode) > 0 {
			log.Warnf(deprecationWarning, "userns", fmt.Sprintf("--userns=%s", input.usernsMode))
		}
		if len(input.containerCapAdd) > 0 {
			log.Warnf(deprecationWarning, "container-cap-add", fmt.Sprintf("--cap-add=%s", input.containerCapAdd))
		}
		if len(input.containerCapDrop) > 0 {
			log.Warnf(deprecationWarning, "container-cap-drop", fmt.Sprintf("--cap-drop=%s", input.containerCapDrop))
		}

		// run the plan
		config := &runner.Config{
			Actor:                              input.actor,
			EventName:                          eventName,
			EventPath:                          input.EventPath(),
			DefaultBranch:                      defaultbranch,
			ForcePull:                          input.forcePull,
			ForceRebuild:                       input.forceRebuild,
			ReuseContainers:                    input.reuseContainers,
			Workdir:                            input.Workdir(),
			ActionCacheDir:                     input.actionCachePath,
			BindWorkdir:                        input.bindWorkdir,
			LogOutput:                          !input.noOutput,
			JSONLogger:                         input.jsonLogger,
			LogPrefixJobID:                     input.logPrefixJobID,
			Env:                                envs,
			Secrets:                            secrets,
			Vars:                               vars,
			Inputs:                             inputs,
			Token:                              secrets["GITHUB_TOKEN"],
			InsecureSecrets:                    input.insecureSecrets,
			Platforms:                          input.newPlatforms(),
			Privileged:                         input.privileged,
			UsernsMode:                         input.usernsMode,
			ContainerArchitecture:              input.containerArchitecture,
			ContainerDaemonSocket:              input.containerDaemonSocket,
			ContainerOptions:                   input.containerOptions,
			UseGitIgnore:                       input.useGitIgnore,
			GitHubInstance:                     input.githubInstance,
			ContainerCapAdd:                    input.containerCapAdd,
			ContainerCapDrop:                   input.containerCapDrop,
			AutoRemove:                         input.autoRemove,
			ArtifactServerPath:                 input.artifactServerPath,
			ArtifactServerAddr:                 input.artifactServerAddr,
			ArtifactServerPort:                 input.artifactServerPort,
			NoSkipCheckout:                     input.noSkipCheckout,
			RemoteName:                         input.remoteName,
			ReplaceGheActionWithGithubCom:      input.replaceGheActionWithGithubCom,
			ReplaceGheActionTokenWithGithubCom: input.replaceGheActionTokenWithGithubCom,
			Matrix:                             matrixes,
			ContainerNetworkMode:               docker_container.NetworkMode(input.networkName),
		}
		if input.useNewActionCache {
			config.ActionCache = &runner.GoGitActionCache{
				Path: config.ActionCacheDir,
			}
		}
		r, err := runner.New(config)
		if err != nil {
			return err
		}

		cancel := artifacts.Serve(ctx, input.artifactServerPath, input.artifactServerAddr, input.artifactServerPort)

		const cacheURLKey = "ACTIONS_CACHE_URL"
		var cacheHandler *artifactcache.Handler
		if !input.noCacheServer && envs[cacheURLKey] == "" {
			var err error
			cacheHandler, err = artifactcache.StartHandler(input.cacheServerPath, input.cacheServerAddr, input.cacheServerPort, common.Logger(ctx))
			if err != nil {
				return err
			}
			envs[cacheURLKey] = cacheHandler.ExternalURL() + "/"
		}

		ctx = common.WithDryrun(ctx, input.dryrun)
		if watch, err := cmd.Flags().GetBool("watch"); err != nil {
			return err
		} else if watch {
			err = watchAndRun(ctx, r.NewPlanExecutor(plan))
			if err != nil {
				return err
			}
			return plannerErr
		}

		executor := r.NewPlanExecutor(plan).Finally(func(ctx context.Context) error {
			cancel()
			_ = cacheHandler.Close()
			return nil
		})
		err = executor(ctx)
		if err != nil {
			return err
		}
		return plannerErr
	}
}

func defaultImageSurvey(actrc string) error {
	var answer string
	confirmation := &survey.Select{
		Message: "Please choose the default image you want to use with act:\n  - Large size image: ca. 17GB download + 53.1GB storage, you will need 75GB of free disk space, snapshots of GitHub Hosted Runners without snap and pulled docker images\n  - Medium size image: ~500MB, includes only necessary tools to bootstrap actions and aims to be compatible with most actions\n  - Micro size image: <200MB, contains only NodeJS required to bootstrap actions, doesn't work with all actions\n\nDefault image and other options can be changed manually in ~/.actrc (please refer to https://github.com/nektos/act#configuration for additional information about file structure)",
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
		option = "-P ubuntu-latest=catthehacker/ubuntu:full-latest\n-P ubuntu-22.04=catthehacker/ubuntu:full-22.04\n-P ubuntu-20.04=catthehacker/ubuntu:full-20.04\n-P ubuntu-18.04=catthehacker/ubuntu:full-18.04\n"
	case "Medium":
		option = "-P ubuntu-latest=catthehacker/ubuntu:act-latest\n-P ubuntu-22.04=catthehacker/ubuntu:act-22.04\n-P ubuntu-20.04=catthehacker/ubuntu:act-20.04\n-P ubuntu-18.04=catthehacker/ubuntu:act-18.04\n"
	case "Micro":
		option = "-P ubuntu-latest=node:16-buster-slim\n-P ubuntu-22.04=node:16-bullseye-slim\n-P ubuntu-20.04=node:16-buster-slim\n-P ubuntu-18.04=node:16-buster-slim\n"
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
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	ignoreFile := filepath.Join(dir, ".gitignore")
	ignore := &gitignore.GitIgnore{}
	if info, err := os.Stat(ignoreFile); err == nil && !info.IsDir() {
		ignore, err = gitignore.CompileIgnoreFile(ignoreFile)
		if err != nil {
			return fmt.Errorf("compile %q: %w", ignoreFile, err)
		}
	}

	folderWatcher := fswatch.NewFolderWatcher(
		dir,
		true,
		ignore.MatchesPath,
		2, // 2 seconds
	)

	folderWatcher.Start()
	defer folderWatcher.Stop()

	// run once before watching
	if err := fn(ctx); err != nil {
		return err
	}

	for folderWatcher.IsRunning() {
		log.Debugf("Watching %s for changes", dir)
		select {
		case <-ctx.Done():
			return nil
		case changes := <-folderWatcher.ChangeDetails():
			log.Debugf("%s", changes.String())
			if err := fn(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}
