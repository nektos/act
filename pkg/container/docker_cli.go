//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

// This file is adapted from https://github.com/docker/cli/blob/v29.3.0/cli/command/container/opts.go
// with import paths adjusted for the act codebase.
//
// docker/cli is licensed under the Apache License, Version 2.0.
// See DOCKER_LICENSE for the full license text.
//

//nolint:unparam,errcheck
package container

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/docker/cli/cli/compose/loader"
	"github.com/docker/cli/opts"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/spf13/pflag"
	cdi "tags.cncf.io/container-device-interface/pkg/parser"
)

const (
	// TODO(thaJeztah): define these in the API-types, or query available defaults
	//  from the daemon, or require "local" profiles to be an absolute path or
	//  relative paths starting with "./". The daemon-config has consts for this
	//  but we don't want to import that package:
	//  https://github.com/moby/moby/blob/v23.0.0/daemon/config/config.go#L63-L67

	// seccompProfileDefault is the built-in default seccomp profile.
	seccompProfileDefault = "builtin"
	// seccompProfileUnconfined is a special profile name for seccomp to use an
	// "unconfined" seccomp profile.
	seccompProfileUnconfined = "unconfined"
)

var deviceCgroupRuleRegexp = regexp.MustCompile(`^[acb] ([0-9]+|\*):([0-9]+|\*) [rwm]{1,3}$`)

// containerOptions is a data object with all the options for creating a container
type containerOptions struct {
	attach              opts.ListOpts
	volumes             opts.ListOpts
	tmpfs               opts.ListOpts
	mounts              opts.MountOpt
	blkioWeightDevice   opts.WeightdeviceOpt
	deviceReadBps       opts.ThrottledeviceOpt
	deviceWriteBps      opts.ThrottledeviceOpt
	links               opts.ListOpts
	aliases             opts.ListOpts
	linkLocalIPs        opts.ListOpts // TODO(thaJeztah): we need a flag-type to handle []netip.Addr directly
	deviceReadIOps      opts.ThrottledeviceOpt
	deviceWriteIOps     opts.ThrottledeviceOpt
	env                 opts.ListOpts
	labels              opts.ListOpts
	deviceCgroupRules   opts.ListOpts
	devices             opts.ListOpts
	gpus                opts.GpuOpts
	ulimits             *opts.UlimitOpt
	sysctls             *opts.MapOpts
	publish             opts.ListOpts
	expose              opts.ListOpts
	dns                 opts.ListOpts // TODO(thaJeztah): we need a flag-type to handle []netip.Addr directly
	dnsSearch           opts.ListOpts
	dnsOptions          opts.ListOpts
	extraHosts          opts.ListOpts
	volumesFrom         opts.ListOpts
	envFile             opts.ListOpts
	capAdd              opts.ListOpts
	capDrop             opts.ListOpts
	groupAdd            opts.ListOpts
	securityOpt         opts.ListOpts
	storageOpt          opts.ListOpts
	labelsFile          opts.ListOpts
	loggingOpts         opts.ListOpts
	privileged          bool
	pidMode             string
	utsMode             string
	usernsMode          string
	cgroupnsMode        string
	publishAll          bool
	stdin               bool
	tty                 bool
	oomKillDisable      bool
	oomScoreAdj         int
	containerIDFile     string
	entrypoint          string
	hostname            string
	domainname          string
	memory              opts.MemBytes
	memoryReservation   opts.MemBytes
	memorySwap          opts.MemSwapBytes
	user                string
	workingDir          string
	cpuCount            int64
	cpuShares           int64
	cpuPercent          int64
	cpuPeriod           int64
	cpuRealtimePeriod   int64
	cpuRealtimeRuntime  int64
	cpuQuota            int64
	cpus                opts.NanoCPUs
	cpusetCpus          string
	cpusetMems          string
	blkioWeight         uint16
	ioMaxBandwidth      opts.MemBytes
	ioMaxIOps           uint64
	swappiness          int64
	netMode             opts.NetworkOpt
	macAddress          string
	ipv4Address         net.IP // TODO(thaJeztah): we need a flag-type to handle netip.Addr directly
	ipv6Address         net.IP // TODO(thaJeztah): we need a flag-type to handle netip.Addr directly
	ipcMode             string
	pidsLimit           int64
	restartPolicy       string
	readonlyRootfs      bool
	loggingDriver       string
	cgroupParent        string
	volumeDriver        string
	stopSignal          string
	stopTimeout         int
	isolation           string
	shmSize             opts.MemBytes
	noHealthcheck       bool
	healthCmd           string
	healthInterval      time.Duration
	healthTimeout       time.Duration
	healthStartPeriod   time.Duration
	healthStartInterval time.Duration
	healthRetries       int
	runtime             string
	autoRemove          bool
	init                bool
	annotations         *opts.MapOpts

	Image string
	Args  []string
}

// addFlags adds all command line flags that will be used by parse to the FlagSet
func addFlags(flags *pflag.FlagSet) *containerOptions {
	copts := &containerOptions{
		aliases:           opts.NewListOpts(nil),
		attach:            opts.NewListOpts(validateAttach),
		blkioWeightDevice: opts.NewWeightdeviceOpt(opts.ValidateWeightDevice),
		capAdd:            opts.NewListOpts(nil),
		capDrop:           opts.NewListOpts(nil),
		dns:               opts.NewListOpts(opts.ValidateIPAddress),
		dnsOptions:        opts.NewListOpts(nil),
		dnsSearch:         opts.NewListOpts(opts.ValidateDNSSearch),
		deviceCgroupRules: opts.NewListOpts(validateDeviceCgroupRule),
		deviceReadBps:     opts.NewThrottledeviceOpt(opts.ValidateThrottleBpsDevice),
		deviceReadIOps:    opts.NewThrottledeviceOpt(opts.ValidateThrottleIOpsDevice),
		deviceWriteBps:    opts.NewThrottledeviceOpt(opts.ValidateThrottleBpsDevice),
		deviceWriteIOps:   opts.NewThrottledeviceOpt(opts.ValidateThrottleIOpsDevice),
		devices:           opts.NewListOpts(nil), // devices can only be validated after we know the server OS
		env:               opts.NewListOpts(opts.ValidateEnv),
		envFile:           opts.NewListOpts(nil),
		expose:            opts.NewListOpts(nil),
		extraHosts:        opts.NewListOpts(opts.ValidateExtraHost),
		groupAdd:          opts.NewListOpts(nil),
		labels:            opts.NewListOpts(opts.ValidateLabel),
		labelsFile:        opts.NewListOpts(nil),
		linkLocalIPs:      opts.NewListOpts(nil),
		links:             opts.NewListOpts(opts.ValidateLink),
		loggingOpts:       opts.NewListOpts(nil),
		publish:           opts.NewListOpts(nil),
		securityOpt:       opts.NewListOpts(nil),
		storageOpt:        opts.NewListOpts(nil),
		sysctls:           opts.NewMapOpts(nil, opts.ValidateSysctl),
		tmpfs:             opts.NewListOpts(nil),
		ulimits:           opts.NewUlimitOpt(nil),
		volumes:           opts.NewListOpts(nil),
		volumesFrom:       opts.NewListOpts(nil),
		annotations:       opts.NewMapOpts(nil, nil),
	}

	// General purpose flags
	flags.VarP(&copts.attach, "attach", "a", "Attach to STDIN, STDOUT or STDERR")
	flags.Var(&copts.deviceCgroupRules, "device-cgroup-rule", "Add a rule to the cgroup allowed devices list")
	flags.Var(&copts.devices, "device", "Add a host device to the container")
	flags.Var(&copts.gpus, "gpus", "GPU devices to add to the container ('all' to pass all GPUs)")
	flags.SetAnnotation("gpus", "version", []string{"1.40"})
	flags.VarP(&copts.env, "env", "e", "Set environment variables")
	flags.Var(&copts.envFile, "env-file", "Read in a file of environment variables")
	flags.StringVar(&copts.entrypoint, "entrypoint", "", "Overwrite the default ENTRYPOINT of the image")
	flags.Var(&copts.groupAdd, "group-add", "Add additional groups to join")
	flags.StringVarP(&copts.hostname, "hostname", "h", "", "Container host name")
	flags.StringVar(&copts.domainname, "domainname", "", "Container NIS domain name")
	flags.BoolVarP(&copts.stdin, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.VarP(&copts.labels, "label", "l", "Set meta data on a container")
	flags.Var(&copts.labelsFile, "label-file", "Read in a line delimited file of labels")
	flags.BoolVar(&copts.readonlyRootfs, "read-only", false, "Mount the container's root filesystem as read only")
	flags.StringVar(&copts.restartPolicy, "restart", string(container.RestartPolicyDisabled), "Restart policy to apply when a container exits")
	flags.StringVar(&copts.stopSignal, "stop-signal", "", "Signal to stop the container")
	flags.IntVar(&copts.stopTimeout, "stop-timeout", 0, "Timeout (in seconds) to stop a container")
	flags.SetAnnotation("stop-timeout", "version", []string{"1.25"})
	flags.Var(copts.sysctls, "sysctl", "Sysctl options")
	flags.BoolVarP(&copts.tty, "tty", "t", false, "Allocate a pseudo-TTY")
	flags.Var(copts.ulimits, "ulimit", "Ulimit options")
	flags.StringVarP(&copts.user, "user", "u", "", "Username or UID (format: <name|uid>[:<group|gid>])")
	flags.StringVarP(&copts.workingDir, "workdir", "w", "", "Working directory inside the container")
	flags.BoolVar(&copts.autoRemove, "rm", false, "Automatically remove the container and its associated anonymous volumes when it exits")

	// Security
	flags.Var(&copts.capAdd, "cap-add", "Add Linux capabilities")
	flags.Var(&copts.capDrop, "cap-drop", "Drop Linux capabilities")
	flags.BoolVar(&copts.privileged, "privileged", false, "Give extended privileges to this container")
	flags.Var(&copts.securityOpt, "security-opt", "Security Options")
	flags.StringVar(&copts.usernsMode, "userns", "", "User namespace to use")
	flags.StringVar(&copts.cgroupnsMode, "cgroupns", "", `Cgroup namespace to use (host|private)
'host':    Run the container in the Docker host's cgroup namespace
'private': Run the container in its own private cgroup namespace
'':        Use the cgroup namespace as configured by the
           default-cgroupns-mode option on the daemon (default)`)
	flags.SetAnnotation("cgroupns", "version", []string{"1.41"})

	// Network and port publishing flag
	flags.Var(&copts.extraHosts, "add-host", "Add a custom host-to-IP mapping (host:ip)")
	flags.Var(&copts.dns, "dns", "Set custom DNS servers")
	// We allow for both "--dns-opt" and "--dns-option", although the latter is the recommended way.
	// This is to be consistent with service create/update
	flags.Var(&copts.dnsOptions, "dns-opt", "Set DNS options")
	flags.Var(&copts.dnsOptions, "dns-option", "Set DNS options")
	flags.MarkHidden("dns-opt")
	flags.Var(&copts.dnsSearch, "dns-search", "Set custom DNS search domains")
	flags.Var(&copts.expose, "expose", "Expose a port or a range of ports")
	flags.IPVar(&copts.ipv4Address, "ip", nil, "IPv4 address (e.g., 172.30.100.104)")
	flags.IPVar(&copts.ipv6Address, "ip6", nil, "IPv6 address (e.g., 2001:db8::33)")
	flags.Var(&copts.links, "link", "Add link to another container")
	flags.Var(&copts.linkLocalIPs, "link-local-ip", "Container IPv4/IPv6 link-local addresses")
	flags.StringVar(&copts.macAddress, "mac-address", "", "Container MAC address (e.g., 92:d0:c6:0a:29:33)")
	flags.VarP(&copts.publish, "publish", "p", "Publish a container's port(s) to the host")
	flags.BoolVarP(&copts.publishAll, "publish-all", "P", false, "Publish all exposed ports to random ports")
	// We allow for both "--net" and "--network", although the latter is the recommended way.
	flags.Var(&copts.netMode, "net", "Connect a container to a network")
	flags.Var(&copts.netMode, "network", "Connect a container to a network")
	flags.MarkHidden("net")
	// We allow for both "--net-alias" and "--network-alias", although the latter is the recommended way.
	flags.Var(&copts.aliases, "net-alias", "Add network-scoped alias for the container")
	flags.Var(&copts.aliases, "network-alias", "Add network-scoped alias for the container")
	flags.MarkHidden("net-alias")

	// Logging and storage
	flags.StringVar(&copts.loggingDriver, "log-driver", "", "Logging driver for the container")
	flags.StringVar(&copts.volumeDriver, "volume-driver", "", "Optional volume driver for the container")
	flags.Var(&copts.loggingOpts, "log-opt", "Log driver options")
	flags.Var(&copts.storageOpt, "storage-opt", "Storage driver options for the container")
	flags.Var(&copts.tmpfs, "tmpfs", "Mount a tmpfs directory")
	flags.Var(&copts.volumesFrom, "volumes-from", "Mount volumes from the specified container(s)")
	flags.VarP(&copts.volumes, "volume", "v", "Bind mount a volume")
	flags.Var(&copts.mounts, "mount", "Attach a filesystem mount to the container")

	// Health-checking
	flags.StringVar(&copts.healthCmd, "health-cmd", "", "Command to run to check health")
	flags.DurationVar(&copts.healthInterval, "health-interval", 0, "Time between running the check (ms|s|m|h) (default 0s)")
	flags.IntVar(&copts.healthRetries, "health-retries", 0, "Consecutive failures needed to report unhealthy")
	flags.DurationVar(&copts.healthTimeout, "health-timeout", 0, "Maximum time to allow one check to run (ms|s|m|h) (default 0s)")
	flags.DurationVar(&copts.healthStartPeriod, "health-start-period", 0, "Start period for the container to initialize before starting health-retries countdown (ms|s|m|h) (default 0s)")
	flags.SetAnnotation("health-start-period", "version", []string{"1.29"})
	flags.DurationVar(&copts.healthStartInterval, "health-start-interval", 0, "Time between running the check during the start period (ms|s|m|h) (default 0s)")
	flags.SetAnnotation("health-start-interval", "version", []string{"1.44"})
	flags.BoolVar(&copts.noHealthcheck, "no-healthcheck", false, "Disable any container-specified HEALTHCHECK")

	// Resource management
	flags.Uint16Var(&copts.blkioWeight, "blkio-weight", 0, "Block IO (relative weight), between 10 and 1000, or 0 to disable (default 0)")
	flags.Var(&copts.blkioWeightDevice, "blkio-weight-device", "Block IO weight (relative device weight)")
	flags.StringVar(&copts.containerIDFile, "cidfile", "", "Write the container ID to the file")
	flags.StringVar(&copts.cpusetCpus, "cpuset-cpus", "", "CPUs in which to allow execution (0-3, 0,1)")
	flags.StringVar(&copts.cpusetMems, "cpuset-mems", "", "MEMs in which to allow execution (0-3, 0,1)")
	flags.Int64Var(&copts.cpuCount, "cpu-count", 0, "CPU count (Windows only)")
	flags.SetAnnotation("cpu-count", "ostype", []string{"windows"})
	flags.Int64Var(&copts.cpuPercent, "cpu-percent", 0, "CPU percent (Windows only)")
	flags.SetAnnotation("cpu-percent", "ostype", []string{"windows"})
	flags.Int64Var(&copts.cpuPeriod, "cpu-period", 0, "Limit CPU CFS (Completely Fair Scheduler) period")
	flags.Int64Var(&copts.cpuQuota, "cpu-quota", 0, "Limit CPU CFS (Completely Fair Scheduler) quota")
	flags.Int64Var(&copts.cpuRealtimePeriod, "cpu-rt-period", 0, "Limit CPU real-time period in microseconds")
	flags.SetAnnotation("cpu-rt-period", "version", []string{"1.25"})
	flags.Int64Var(&copts.cpuRealtimeRuntime, "cpu-rt-runtime", 0, "Limit CPU real-time runtime in microseconds")
	flags.SetAnnotation("cpu-rt-runtime", "version", []string{"1.25"})
	flags.Int64VarP(&copts.cpuShares, "cpu-shares", "c", 0, "CPU shares (relative weight)")
	flags.Var(&copts.cpus, "cpus", "Number of CPUs")
	flags.SetAnnotation("cpus", "version", []string{"1.25"})
	flags.Var(&copts.deviceReadBps, "device-read-bps", "Limit read rate (bytes per second) from a device")
	flags.Var(&copts.deviceReadIOps, "device-read-iops", "Limit read rate (IO per second) from a device")
	flags.Var(&copts.deviceWriteBps, "device-write-bps", "Limit write rate (bytes per second) to a device")
	flags.Var(&copts.deviceWriteIOps, "device-write-iops", "Limit write rate (IO per second) to a device")
	flags.Var(&copts.ioMaxBandwidth, "io-maxbandwidth", "Maximum IO bandwidth limit for the system drive (Windows only)")
	flags.SetAnnotation("io-maxbandwidth", "ostype", []string{"windows"})
	flags.Uint64Var(&copts.ioMaxIOps, "io-maxiops", 0, "Maximum IOps limit for the system drive (Windows only)")
	flags.SetAnnotation("io-maxiops", "ostype", []string{"windows"})
	flags.VarP(&copts.memory, "memory", "m", "Memory limit")
	flags.Var(&copts.memoryReservation, "memory-reservation", "Memory soft limit")
	flags.Var(&copts.memorySwap, "memory-swap", "Swap limit equal to memory plus swap: '-1' to enable unlimited swap")
	flags.Int64Var(&copts.swappiness, "memory-swappiness", -1, "Tune container memory swappiness (0 to 100)")
	flags.BoolVar(&copts.oomKillDisable, "oom-kill-disable", false, "Disable OOM Killer")
	flags.IntVar(&copts.oomScoreAdj, "oom-score-adj", 0, "Tune host's OOM preferences (-1000 to 1000)")
	flags.Int64Var(&copts.pidsLimit, "pids-limit", 0, "Tune container pids limit (set -1 for unlimited)")

	// Low-level execution (cgroups, namespaces, ...)
	flags.StringVar(&copts.cgroupParent, "cgroup-parent", "", "Optional parent cgroup for the container")
	flags.StringVar(&copts.ipcMode, "ipc", "", "IPC mode to use")
	flags.StringVar(&copts.isolation, "isolation", "", "Container isolation technology")
	flags.StringVar(&copts.pidMode, "pid", "", "PID namespace to use")
	flags.Var(&copts.shmSize, "shm-size", "Size of /dev/shm")
	flags.StringVar(&copts.utsMode, "uts", "", "UTS namespace to use")
	flags.StringVar(&copts.runtime, "runtime", "", "Runtime to use for this container")

	flags.BoolVar(&copts.init, "init", false, "Run an init inside the container that forwards signals and reaps processes")
	flags.SetAnnotation("init", "version", []string{"1.25"})

	flags.Var(copts.annotations, "annotation", "Add an annotation to the container (passed through to the OCI runtime)")
	flags.SetAnnotation("annotation", "version", []string{"1.43"})

	// TODO(thaJeztah): remove in next release (v30.0, or v29.x)
	var stub opts.MemBytes
	flags.Var(&stub, "kernel-memory", "Kernel memory limit (deprecated)")
	_ = flags.MarkDeprecated("kernel-memory", "and no longer supported by the kernel")

	return copts
}

type containerConfig struct {
	Config           *container.Config
	HostConfig       *container.HostConfig
	NetworkingConfig *network.NetworkingConfig
}

// parse parses the args for the specified command and generates a Config,
// a HostConfig and returns them with the specified command.
// If the specified args are not valid, it will return an error.
//
//nolint:gocyclo
func parse(flags *pflag.FlagSet, copts *containerOptions, serverOS string) (*containerConfig, error) {
	var (
		attachStdin  = copts.attach.Get("stdin")
		attachStdout = copts.attach.Get("stdout")
		attachStderr = copts.attach.Get("stderr")
	)

	// Validate the input mac address
	if copts.macAddress != "" {
		if _, err := net.ParseMAC(strings.TrimSpace(copts.macAddress)); err != nil {
			return nil, fmt.Errorf("%s is not a valid mac address", copts.macAddress)
		}
	}
	if copts.stdin {
		attachStdin = true
	}
	// If -a is not set, attach to stdout and stderr
	if copts.attach.Len() == 0 {
		attachStdout = true
		attachStderr = true
	}

	var err error

	swappiness := copts.swappiness
	if swappiness != -1 && (swappiness < 0 || swappiness > 100) {
		return nil, fmt.Errorf("invalid value: %d. Valid memory swappiness range is 0-100", swappiness)
	}

	var binds []string
	volumes := copts.volumes.GetMap()
	// add any bind targets to the list of container volumes
	for bind := range copts.volumes.GetMap() {
		parsed, err := loader.ParseVolume(bind)
		if err != nil {
			return nil, err
		}

		if parsed.Source != "" {
			toBind := bind

			if parsed.Type == string(mount.TypeBind) {
				if hostPart, targetPath, ok := strings.Cut(bind, ":"); ok {
					if !filepath.IsAbs(hostPart) && strings.HasPrefix(hostPart, ".") {
						if absHostPart, err := filepath.Abs(hostPart); err == nil {
							hostPart = absHostPart
						}
					}
					toBind = hostPart + ":" + targetPath
				}
			}

			// after creating the bind mount we want to delete it from the copts.volumes values because
			// we do not want bind mounts being committed to image configs
			binds = append(binds, toBind)
			// We should delete from the map (`volumes`) here, as deleting from copts.volumes will not work if
			// there are duplicates entries.
			delete(volumes, bind)
		}
	}

	// Can't evaluate options passed into --tmpfs until we actually mount
	tmpfs := make(map[string]string)
	for _, t := range copts.tmpfs.GetSlice() {
		k, v, _ := strings.Cut(t, ":")
		tmpfs[k] = v
	}

	var runCmd, entrypoint []string

	if len(copts.Args) > 0 {
		runCmd = copts.Args
	}

	if copts.entrypoint != "" {
		entrypoint = []string{copts.entrypoint}
	} else if flags.Changed("entrypoint") {
		// if `--entrypoint=` is parsed then Entrypoint is reset
		entrypoint = []string{""}
	}

	// TODO(thaJeztah): remove uses of go-connections/nat here.
	convertedOpts, err := convertToStandardNotation(copts.publish.GetSlice())
	if err != nil {
		return nil, err
	}

	// short syntax ([ip:]public:private[/proto])
	//
	// TODO(thaJeztah): we need an equivalent that handles the "ip-address" part without depending on the nat package.
	ports, natPortBindings, err := nat.ParsePortSpecs(convertedOpts)
	if err != nil {
		return nil, err
	}
	portBindings := network.PortMap{}
	for port, bindings := range natPortBindings {
		p, err := network.ParsePort(string(port))
		if err != nil {
			return nil, err
		}
		portBindings[p] = []network.PortBinding{}
		for _, b := range bindings {
			var hostIP netip.Addr
			if b.HostIP != "" {
				hostIP, err = netip.ParseAddr(b.HostIP)
				if err != nil {
					return nil, err
				}
			}
			portBindings[p] = append(portBindings[p], network.PortBinding{
				HostIP:   hostIP,
				HostPort: b.HostPort,
			})
		}
	}

	// Add published ports as exposed ports.
	exposedPorts := network.PortSet{}
	for port := range ports {
		p, err := network.ParsePort(string(port))
		if err != nil {
			return nil, err
		}
		exposedPorts[p] = struct{}{}
	}

	// Merge in exposed ports to the map of published ports
	for _, e := range copts.expose.GetSlice() {
		// support two formats for expose, original format <portnum>/[<proto>]
		// or <startport-endport>/[<proto>]
		pr, err := network.ParsePortRange(e)
		if err != nil {
			return nil, fmt.Errorf("invalid range format for --expose: %w", err)
		}
		// parse the start and end port and create a sequence of ports to expose
		// if expose a port, the start and end port are the same
		for p := range pr.All() {
			exposedPorts[p] = struct{}{}
		}
	}

	// validate and parse device mappings. Note we do late validation of the
	// device path (as opposed to during flag parsing), as at the time we are
	// parsing flags, we haven't yet sent a _ping to the daemon to determine
	// what operating system it is.
	devices := copts.devices.GetSlice()
	deviceMappings := make([]container.DeviceMapping, 0, len(devices))
	cdiDeviceNames := make([]string, 0, len(devices))
	for _, device := range devices {
		if cdi.IsQualifiedName(device) {
			cdiDeviceNames = append(cdiDeviceNames, device)
			continue
		}
		validated, err := validateDevice(device, serverOS)
		if err != nil {
			return nil, err
		}
		deviceMapping, err := parseDevice(validated, serverOS)
		if err != nil {
			return nil, err
		}
		deviceMappings = append(deviceMappings, deviceMapping)
	}

	// collect all the environment variables for the container
	envVariables, err := opts.ReadKVEnvStrings(copts.envFile.GetSlice(), copts.env.GetSlice())
	if err != nil {
		return nil, err
	}

	// collect all the labels for the container
	labels, err := opts.ReadKVStrings(copts.labelsFile.GetSlice(), copts.labels.GetSlice())
	if err != nil {
		return nil, err
	}

	pidMode := container.PidMode(copts.pidMode)
	if !pidMode.Valid() {
		return nil, errors.New("--pid: invalid PID mode")
	}

	utsMode := container.UTSMode(copts.utsMode)
	if !utsMode.Valid() {
		return nil, errors.New("--uts: invalid UTS mode")
	}

	usernsMode := container.UsernsMode(copts.usernsMode)
	if !usernsMode.Valid() {
		return nil, errors.New("--userns: invalid USER mode")
	}

	cgroupnsMode := container.CgroupnsMode(copts.cgroupnsMode)
	if !cgroupnsMode.Valid() {
		return nil, errors.New("--cgroupns: invalid CGROUP mode")
	}

	restartPolicy, err := opts.ParseRestartPolicy(copts.restartPolicy)
	if err != nil {
		return nil, err
	}

	loggingOpts, err := parseLoggingOpts(copts.loggingDriver, copts.loggingOpts.GetSlice())
	if err != nil {
		return nil, err
	}

	securityOpts, err := parseSecurityOpts(copts.securityOpt.GetSlice())
	if err != nil {
		return nil, err
	}

	securityOpts, maskedPaths, readonlyPaths := parseSystemPaths(securityOpts)

	storageOpts, err := parseStorageOpts(copts.storageOpt.GetSlice())
	if err != nil {
		return nil, err
	}

	// Healthcheck
	var healthConfig *container.HealthConfig
	haveHealthSettings := copts.healthCmd != "" ||
		copts.healthInterval != 0 ||
		copts.healthTimeout != 0 ||
		copts.healthStartPeriod != 0 ||
		copts.healthRetries != 0 ||
		copts.healthStartInterval != 0
	if copts.noHealthcheck {
		if haveHealthSettings {
			return nil, errors.New("--no-healthcheck conflicts with --health-* options")
		}
		healthConfig = &container.HealthConfig{Test: []string{"NONE"}}
	} else if haveHealthSettings {
		var probe []string
		if copts.healthCmd != "" {
			probe = []string{"CMD-SHELL", copts.healthCmd}
		}
		if copts.healthInterval < 0 {
			return nil, errors.New("--health-interval cannot be negative")
		}
		if copts.healthTimeout < 0 {
			return nil, errors.New("--health-timeout cannot be negative")
		}
		if copts.healthRetries < 0 {
			return nil, errors.New("--health-retries cannot be negative")
		}
		if copts.healthStartPeriod < 0 {
			return nil, errors.New("--health-start-period cannot be negative")
		}
		if copts.healthStartInterval < 0 {
			return nil, errors.New("--health-start-interval cannot be negative")
		}

		healthConfig = &container.HealthConfig{
			Test:          probe,
			Interval:      copts.healthInterval,
			Timeout:       copts.healthTimeout,
			StartPeriod:   copts.healthStartPeriod,
			StartInterval: copts.healthStartInterval,
			Retries:       copts.healthRetries,
		}
	}

	deviceRequests := copts.gpus.Value()
	if len(cdiDeviceNames) > 0 {
		cdiDeviceRequest := container.DeviceRequest{
			Driver:    "cdi",
			DeviceIDs: cdiDeviceNames,
		}
		deviceRequests = append(deviceRequests, cdiDeviceRequest)
	}

	resources := container.Resources{
		CgroupParent:         copts.cgroupParent,
		Memory:               copts.memory.Value(),
		MemoryReservation:    copts.memoryReservation.Value(),
		MemorySwap:           copts.memorySwap.Value(),
		MemorySwappiness:     &copts.swappiness,
		OomKillDisable:       &copts.oomKillDisable,
		NanoCPUs:             copts.cpus.Value(),
		CPUCount:             copts.cpuCount,
		CPUPercent:           copts.cpuPercent,
		CPUShares:            copts.cpuShares,
		CPUPeriod:            copts.cpuPeriod,
		CpusetCpus:           copts.cpusetCpus,
		CpusetMems:           copts.cpusetMems,
		CPUQuota:             copts.cpuQuota,
		CPURealtimePeriod:    copts.cpuRealtimePeriod,
		CPURealtimeRuntime:   copts.cpuRealtimeRuntime,
		PidsLimit:            &copts.pidsLimit,
		BlkioWeight:          copts.blkioWeight,
		BlkioWeightDevice:    copts.blkioWeightDevice.GetList(),
		BlkioDeviceReadBps:   copts.deviceReadBps.GetList(),
		BlkioDeviceWriteBps:  copts.deviceWriteBps.GetList(),
		BlkioDeviceReadIOps:  copts.deviceReadIOps.GetList(),
		BlkioDeviceWriteIOps: copts.deviceWriteIOps.GetList(),
		IOMaximumIOps:        copts.ioMaxIOps,
		IOMaximumBandwidth:   uint64(copts.ioMaxBandwidth),
		Ulimits:              copts.ulimits.GetList(),
		DeviceCgroupRules:    copts.deviceCgroupRules.GetSlice(),
		Devices:              deviceMappings,
		DeviceRequests:       deviceRequests,
	}

	config := &container.Config{
		Hostname:     copts.hostname,
		Domainname:   copts.domainname,
		ExposedPorts: exposedPorts,
		User:         copts.user,
		Tty:          copts.tty,
		OpenStdin:    copts.stdin,
		AttachStdin:  attachStdin,
		AttachStdout: attachStdout,
		AttachStderr: attachStderr,
		Env:          envVariables,
		Cmd:          runCmd,
		Image:        copts.Image,
		Volumes:      volumes,
		Entrypoint:   entrypoint,
		WorkingDir:   copts.workingDir,
		Labels:       opts.ConvertKVStringsToMap(labels),
		StopSignal:   copts.stopSignal,
		Healthcheck:  healthConfig,
	}
	if flags.Changed("stop-timeout") {
		config.StopTimeout = &copts.stopTimeout
	}

	hostConfig := &container.HostConfig{
		Binds:           binds,
		ContainerIDFile: copts.containerIDFile,
		OomScoreAdj:     copts.oomScoreAdj,
		AutoRemove:      copts.autoRemove,
		Privileged:      copts.privileged,
		PortBindings:    portBindings,
		Links:           copts.links.GetSlice(),
		PublishAllPorts: copts.publishAll,
		// Make sure the dns fields are never nil.
		// New containers don't ever have those fields nil,
		// but pre created containers can still have those nil values.
		// See https://github.com/docker/docker/pull/17779
		// for a more detailed explanation on why we don't want that.
		DNS:            toNetipAddrSlice(copts.dns.GetAllOrEmpty()),
		DNSSearch:      copts.dnsSearch.GetAllOrEmpty(),
		DNSOptions:     copts.dnsOptions.GetAllOrEmpty(),
		ExtraHosts:     copts.extraHosts.GetSlice(),
		VolumesFrom:    copts.volumesFrom.GetSlice(),
		IpcMode:        container.IpcMode(copts.ipcMode),
		NetworkMode:    container.NetworkMode(copts.netMode.NetworkMode()),
		PidMode:        pidMode,
		UTSMode:        utsMode,
		UsernsMode:     usernsMode,
		CgroupnsMode:   cgroupnsMode,
		CapAdd:         copts.capAdd.GetSlice(),
		CapDrop:        copts.capDrop.GetSlice(),
		GroupAdd:       copts.groupAdd.GetSlice(),
		RestartPolicy:  restartPolicy,
		SecurityOpt:    securityOpts,
		StorageOpt:     storageOpts,
		ReadonlyRootfs: copts.readonlyRootfs,
		LogConfig:      container.LogConfig{Type: copts.loggingDriver, Config: loggingOpts},
		VolumeDriver:   copts.volumeDriver,
		Isolation:      container.Isolation(copts.isolation),
		ShmSize:        copts.shmSize.Value(),
		Resources:      resources,
		Tmpfs:          tmpfs,
		Sysctls:        copts.sysctls.GetAll(),
		Runtime:        copts.runtime,
		Mounts:         copts.mounts.Value(),
		MaskedPaths:    maskedPaths,
		ReadonlyPaths:  readonlyPaths,
		Annotations:    copts.annotations.GetAll(),
	}

	if copts.autoRemove && !hostConfig.RestartPolicy.IsNone() {
		return nil, errors.New("conflicting options: cannot specify both --restart and --rm")
	}

	// only set this value if the user provided the flag, else it should default to nil
	if flags.Changed("init") {
		hostConfig.Init = &copts.init
	}

	// When allocating stdin in attached mode, close stdin at client disconnect
	if config.OpenStdin && config.AttachStdin {
		config.StdinOnce = true
	}

	epCfg, err := parseNetworkOpts(copts)
	if err != nil {
		return nil, err
	}

	return &containerConfig{
		Config:     config,
		HostConfig: hostConfig,
		NetworkingConfig: &network.NetworkingConfig{
			EndpointsConfig: epCfg,
		},
	}, nil
}

// parseNetworkOpts converts --network advanced options to endpoint-specs, and combines
// them with the old --network-alias and --links. If returns an error if conflicting options
// are found.
//
// this function may return _multiple_ endpoints, which is not currently supported
// by the daemon, but may be in future; it's up to the daemon to produce an error
// in case that is not supported.
func parseNetworkOpts(copts *containerOptions) (map[string]*network.EndpointSettings, error) {
	var (
		endpoints                         = make(map[string]*network.EndpointSettings, len(copts.netMode.Value()))
		hasUserDefined, hasNonUserDefined bool
	)

	if len(copts.netMode.Value()) == 0 {
		n := opts.NetworkAttachmentOpts{
			Target: "default",
		}
		if err := applyContainerOptions(&n, copts); err != nil {
			return nil, err
		}
		ep, err := parseNetworkAttachmentOpt(n)
		if err != nil {
			return nil, err
		}
		endpoints["default"] = ep
	}

	for i, n := range copts.netMode.Value() {
		if container.NetworkMode(n.Target).IsUserDefined() {
			hasUserDefined = true
		} else {
			hasNonUserDefined = true
		}
		if i == 0 {
			// The first network corresponds with what was previously the "only"
			// network, and what would be used when using the non-advanced syntax
			// `--network-alias`, `--link`, `--ip`, `--ip6`, and `--link-local-ip`
			// are set on this network, to preserve backward compatibility with
			// the non-advanced notation
			if err := applyContainerOptions(&n, copts); err != nil {
				return nil, err
			}
		}
		ep, err := parseNetworkAttachmentOpt(n)
		if err != nil {
			return nil, err
		}
		if _, ok := endpoints[n.Target]; ok {
			return nil, errdefs.InvalidParameter(fmt.Errorf("network %q is specified multiple times", n.Target))
		}

		// For backward compatibility: if no custom options are provided for the network,
		// and only a single network is specified, omit the endpoint-configuration
		// on the client (the daemon will still create it when creating the container)
		if i == 0 && len(copts.netMode.Value()) == 1 {
			if ep == nil || reflect.ValueOf(*ep).IsZero() {
				continue
			}
		}
		endpoints[n.Target] = ep
	}
	if hasUserDefined && hasNonUserDefined {
		return nil, errdefs.InvalidParameter(errors.New("conflicting options: cannot attach both user-defined and non-user-defined network-modes"))
	}
	return endpoints, nil
}

func applyContainerOptions(n *opts.NetworkAttachmentOpts, copts *containerOptions) error { //nolint:gocyclo
	// TODO should we error if _any_ advanced option is used? (i.e. forbid to combine advanced notation with the "old" flags (`--network-alias`, `--link`, `--ip`, `--ip6`)?
	if len(n.Aliases) > 0 && copts.aliases.Len() > 0 {
		return errdefs.InvalidParameter(errors.New("conflicting options: cannot specify both --network-alias and per-network alias"))
	}
	if len(n.Links) > 0 && copts.links.Len() > 0 {
		return errdefs.InvalidParameter(errors.New("conflicting options: cannot specify both --link and per-network links"))
	}
	if n.IPv4Address.IsValid() && copts.ipv4Address != nil {
		return errdefs.InvalidParameter(errors.New("conflicting options: cannot specify both --ip and per-network IPv4 address"))
	}
	if n.IPv6Address.IsValid() && copts.ipv6Address != nil {
		return errdefs.InvalidParameter(errors.New("conflicting options: cannot specify both --ip6 and per-network IPv6 address"))
	}
	if n.MacAddress != "" && copts.macAddress != "" {
		return errdefs.InvalidParameter(errors.New("conflicting options: cannot specify both --mac-address and per-network MAC address"))
	}
	if len(n.LinkLocalIPs) > 0 && copts.linkLocalIPs.Len() > 0 {
		return errdefs.InvalidParameter(errors.New("conflicting options: cannot specify both --link-local-ip and per-network link-local IP addresses"))
	}
	if copts.aliases.Len() > 0 {
		n.Aliases = make([]string, copts.aliases.Len())
		copy(n.Aliases, copts.aliases.GetSlice())
	}
	// For a user-defined network, "--link" is an endpoint option, it creates an alias. But,
	// for the default bridge it defines a legacy-link.
	if container.NetworkMode(n.Target).IsUserDefined() && copts.links.Len() > 0 {
		n.Links = make([]string, copts.links.Len())
		copy(n.Links, copts.links.GetSlice())
	}
	if copts.ipv4Address != nil {
		if ipv4, ok := netip.AddrFromSlice(copts.ipv4Address.To4()); ok {
			n.IPv4Address = ipv4
		}
	}
	if copts.ipv6Address != nil {
		if ipv6, ok := netip.AddrFromSlice(copts.ipv6Address.To16()); ok {
			n.IPv6Address = ipv6
		}
	}
	if copts.macAddress != "" {
		n.MacAddress = copts.macAddress
	}
	if copts.linkLocalIPs.Len() > 0 {
		n.LinkLocalIPs = toNetipAddrSlice(copts.linkLocalIPs.GetSlice())
	}
	return nil
}

func parseNetworkAttachmentOpt(ep opts.NetworkAttachmentOpts) (*network.EndpointSettings, error) {
	if strings.TrimSpace(ep.Target) == "" {
		return nil, errors.New("no name set for network")
	}
	if !container.NetworkMode(ep.Target).IsUserDefined() {
		if len(ep.Aliases) > 0 {
			return nil, errors.New("network-scoped aliases are only supported for user-defined networks")
		}
		if len(ep.Links) > 0 {
			return nil, errors.New("links are only supported for user-defined networks")
		}
	}

	epConfig := &network.EndpointSettings{
		GwPriority: ep.GwPriority,
	}
	epConfig.Aliases = append(epConfig.Aliases, ep.Aliases...)
	if len(ep.DriverOpts) > 0 {
		epConfig.DriverOpts = make(map[string]string)
		epConfig.DriverOpts = ep.DriverOpts
	}
	if len(ep.Links) > 0 {
		epConfig.Links = ep.Links
	}
	if ep.IPv4Address.IsValid() || ep.IPv6Address.IsValid() || len(ep.LinkLocalIPs) > 0 {
		epConfig.IPAMConfig = &network.EndpointIPAMConfig{
			IPv4Address:  ep.IPv4Address,
			IPv6Address:  ep.IPv6Address,
			LinkLocalIPs: ep.LinkLocalIPs,
		}
	}
	if ep.MacAddress != "" {
		ma, err := net.ParseMAC(strings.TrimSpace(ep.MacAddress))
		if err != nil {
			return nil, fmt.Errorf("%s is not a valid mac address", ep.MacAddress)
		}
		epConfig.MacAddress = network.HardwareAddr(ma)
	}
	return epConfig, nil
}

func convertToStandardNotation(ports []string) ([]string, error) {
	optsList := []string{}
	for _, publish := range ports {
		if strings.Contains(publish, "=") {
			params := map[string]string{"protocol": "tcp"}
			for param := range strings.SplitSeq(publish, ",") {
				k, v, ok := strings.Cut(param, "=")
				if !ok || k == "" {
					return optsList, fmt.Errorf("invalid publish opts format (should be name=value but got '%s')", param)
				}
				params[k] = v
			}
			optsList = append(optsList, fmt.Sprintf("%s:%s/%s", params["published"], params["target"], params["protocol"]))
		} else {
			optsList = append(optsList, publish)
		}
	}
	return optsList, nil
}

func parseLoggingOpts(loggingDriver string, loggingOpts []string) (map[string]string, error) {
	loggingOptsMap := opts.ConvertKVStringsToMap(loggingOpts)
	if loggingDriver == "none" && len(loggingOpts) > 0 {
		return map[string]string{}, fmt.Errorf("invalid logging opts for driver %s", loggingDriver)
	}
	return loggingOptsMap, nil
}

// takes a local seccomp daemon, reads the file contents for sending to the daemon
func parseSecurityOpts(securityOpts []string) ([]string, error) {
	for key, opt := range securityOpts {
		k, v, ok := strings.Cut(opt, "=")
		if !ok && k != "no-new-privileges" {
			k, v, ok = strings.Cut(opt, ":")
		}
		if (!ok || v == "") && k != "no-new-privileges" {
			// "no-new-privileges" is the only option that does not require a value.
			return securityOpts, fmt.Errorf("invalid --security-opt: %q", opt)
		}
		if k == "seccomp" {
			switch v {
			case seccompProfileDefault, seccompProfileUnconfined:
				// known special names for built-in profiles, nothing to do.
			default:
				// value may be a filename, in which case we send the profile's
				// content if it's valid JSON.
				f, err := os.ReadFile(v)
				if err != nil {
					return securityOpts, fmt.Errorf("opening seccomp profile (%s) failed: %w", v, err)
				}
				var b bytes.Buffer
				if err := json.Compact(&b, f); err != nil {
					return securityOpts, fmt.Errorf("compacting json for seccomp profile (%s) failed: %w", v, err)
				}
				securityOpts[key] = "seccomp=" + b.String()
			}
		}
	}

	return securityOpts, nil
}

// parseSystemPaths checks if `systempaths=unconfined` security option is set,
// and returns the `MaskedPaths` and `ReadonlyPaths` accordingly. An updated
// list of security options is returned with this option removed, because the
// `unconfined` option is handled client-side, and should not be sent to the
// daemon.
func parseSystemPaths(securityOpts []string) (filtered, maskedPaths, readonlyPaths []string) {
	filtered = securityOpts[:0]
	for _, opt := range securityOpts {
		if opt == "systempaths=unconfined" {
			maskedPaths = []string{}
			readonlyPaths = []string{}
		} else {
			filtered = append(filtered, opt)
		}
	}

	return filtered, maskedPaths, readonlyPaths
}

// parses storage options per container into a map
func parseStorageOpts(storageOpts []string) (map[string]string, error) {
	m := make(map[string]string)
	for _, option := range storageOpts {
		k, v, ok := strings.Cut(option, "=")
		if !ok {
			return nil, errors.New("invalid storage option")
		}
		m[k] = v
	}
	return m, nil
}

// parseDevice parses a device mapping string to a container.DeviceMapping struct
func parseDevice(device, serverOS string) (container.DeviceMapping, error) {
	switch serverOS {
	case "linux":
		return parseLinuxDevice(device)
	case "windows":
		// Windows doesn't support mapping, so passing the given value as-is.
		return container.DeviceMapping{PathOnHost: device}, nil
	default:
		return container.DeviceMapping{}, fmt.Errorf("unknown server OS: %s", serverOS)
	}
}

// parseLinuxDevice parses a device mapping string to a container.DeviceMapping struct
// knowing that the target is a Linux daemon
func parseLinuxDevice(device string) (container.DeviceMapping, error) {
	var src, dst string
	permissions := "rwm"
	// We expect 3 parts at maximum; limit to 4 parts to detect invalid options.
	arr := strings.SplitN(device, ":", 4)
	switch len(arr) {
	case 3:
		permissions = arr[2]
		fallthrough
	case 2:
		if validDeviceMode(arr[1]) {
			permissions = arr[1]
		} else {
			dst = arr[1]
		}
		fallthrough
	case 1:
		src = arr[0]
	default:
		return container.DeviceMapping{}, fmt.Errorf("invalid device specification: %s", device)
	}

	if dst == "" {
		dst = src
	}

	return container.DeviceMapping{
		PathOnHost:        src,
		PathInContainer:   dst,
		CgroupPermissions: permissions,
	}, nil
}

// validateDeviceCgroupRule validates a device cgroup rule string format
// It will make sure 'val' is in the form:
//
//	'type major:minor mode'
func validateDeviceCgroupRule(val string) (string, error) {
	if deviceCgroupRuleRegexp.MatchString(val) {
		return val, nil
	}

	return val, fmt.Errorf("invalid device cgroup format '%s'", val)
}

// validDeviceMode checks if the mode for device is valid or not.
// Valid mode is a composition of r (read), w (write), and m (mknod).
func validDeviceMode(mode string) bool {
	legalDeviceMode := map[rune]bool{
		'r': true,
		'w': true,
		'm': true,
	}
	if mode == "" {
		return false
	}
	for _, c := range mode {
		if !legalDeviceMode[c] {
			return false
		}
		legalDeviceMode[c] = false
	}
	return true
}

// validateDevice validates a path for devices
func validateDevice(val string, serverOS string) (string, error) {
	switch serverOS {
	case "linux":
		return validateLinuxPath(val, validDeviceMode)
	case "windows":
		// Windows does validation entirely server-side
		return val, nil
	}
	return "", fmt.Errorf("unknown server OS: %s", serverOS)
}

// validateLinuxPath is the implementation of validateDevice knowing that the
// target server operating system is a Linux daemon.
// It will make sure 'val' is in the form:
//
//	[host-dir:]container-path[:mode]
//
// It also validates the device mode.
func validateLinuxPath(val string, validator func(string) bool) (string, error) {
	var containerPath string
	var mode string

	if strings.Count(val, ":") > 2 {
		return val, fmt.Errorf("bad format for path: %s", val)
	}

	split := strings.SplitN(val, ":", 3)
	if split[0] == "" {
		return val, fmt.Errorf("bad format for path: %s", val)
	}
	switch len(split) {
	case 1:
		containerPath = split[0]
		val = path.Clean(containerPath)
	case 2:
		if isValid := validator(split[1]); isValid {
			containerPath = split[0]
			mode = split[1]
			val = fmt.Sprintf("%s:%s", path.Clean(containerPath), mode)
		} else {
			containerPath = split[1]
			val = fmt.Sprintf("%s:%s", split[0], path.Clean(containerPath))
		}
	case 3:
		containerPath = split[1]
		mode = split[2]
		if isValid := validator(split[2]); !isValid {
			return val, fmt.Errorf("bad mode specified: %s", mode)
		}
		val = fmt.Sprintf("%s:%s:%s", split[0], containerPath, mode)
	}

	if !path.IsAbs(containerPath) {
		return val, fmt.Errorf("%s is not an absolute path", containerPath)
	}
	return val, nil
}

// validateAttach validates that the specified string is a valid attach option.
func validateAttach(val string) (string, error) {
	s := strings.ToLower(val)
	if slices.Contains([]string{"stdin", "stdout", "stderr"}, s) {
		return s, nil
	}
	return val, errors.New("valid streams are STDIN, STDOUT and STDERR")
}

func toNetipAddrSlice(ips []string) []netip.Addr {
	if len(ips) == 0 {
		return nil
	}
	netIPs := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			continue
		}
		netIPs = append(netIPs, addr)
	}
	return netIPs
}
