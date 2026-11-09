//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/docker/cli/cli/compose/loader"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
)

// checkRestrictedFileOptions rejects options that make act read files on the
// host while the options are parsed.
func checkRestrictedFileOptions(copts *containerOptions) error {
	var errs []error
	if copts.envFile.Len() > 0 {
		errs = append(errs, errors.New("--env-file reads a file on the host"))
	}
	if copts.labelsFile.Len() > 0 {
		errs = append(errs, errors.New("--label-file reads a file on the host"))
	}
	for _, opt := range copts.securityOpt.GetSlice() {
		if !isNoNewPrivileges(opt) {
			errs = append(errs, fmt.Errorf("--security-opt %q weakens container isolation", opt))
		}
	}
	return errors.Join(errs...)
}

func isNoNewPrivileges(opt string) bool {
	switch strings.ToLower(opt) {
	case "no-new-privileges", "no-new-privileges=true", "no-new-privileges:true":
		return true
	}
	return false
}

// checkRestrictedHostConfig rejects parsed options that give a container
// access to host resources or weaken its isolation. networkMode is the network
// act chose for the container.
//
//nolint:gocyclo
func checkRestrictedHostConfig(hc *container.HostConfig, networkMode string) error {
	var errs []error
	reject := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}
	if hc.Privileged {
		reject("--privileged")
	}
	if len(hc.CapAdd) > 0 {
		reject("--cap-add %s", strings.Join(hc.CapAdd, ","))
	}
	if hc.PidMode != "" {
		reject("--pid %s", hc.PidMode)
	}
	if hc.IpcMode != "" && !hc.IpcMode.IsPrivate() && !hc.IpcMode.IsShareable() && !hc.IpcMode.IsNone() {
		reject("--ipc %s", hc.IpcMode)
	}
	if hc.UTSMode != "" {
		reject("--uts %s", hc.UTSMode)
	}
	if hc.UsernsMode != "" {
		reject("--userns %s", hc.UsernsMode)
	}
	if hc.CgroupnsMode.IsHost() {
		reject("--cgroupns %s", hc.CgroupnsMode)
	}
	if hc.CgroupParent != "" {
		reject("--cgroup-parent %s", hc.CgroupParent)
	}
	if len(hc.Devices) > 0 {
		reject("--device")
	}
	if len(hc.DeviceCgroupRules) > 0 {
		reject("--device-cgroup-rule")
	}
	if len(hc.DeviceRequests) > 0 {
		reject("--gpus")
	}
	if len(hc.VolumesFrom) > 0 {
		reject("--volumes-from %s", strings.Join(hc.VolumesFrom, ","))
	}
	if n := string(hc.NetworkMode); n != "" && n != networkMode && (hc.NetworkMode.IsHost() || hc.NetworkMode.IsContainer()) {
		reject("--network %s", n)
	}
	for _, b := range hc.Binds {
		if isHostBind(b) {
			reject("host bind mount %q", b)
		}
	}
	for _, m := range hc.Mounts {
		switch {
		case m.Type == mount.TypeBind:
			reject("--mount type=bind,source=%s", m.Source)
		case m.Type == mount.TypeVolume && m.VolumeOptions != nil && m.VolumeOptions.DriverConfig != nil:
			reject("--mount volume driver options for %s", m.Target)
		}
	}
	if len(hc.MaskedPaths) > 0 || len(hc.ReadonlyPaths) > 0 {
		reject("--security-opt systempaths")
	}
	return errors.Join(errs...)
}

func isHostBind(spec string) bool {
	parsed, err := loader.ParseVolume(spec)
	if err != nil {
		return true
	}
	return parsed.Type == string(mount.TypeBind)
}

// keepOperatorCapDrop makes the operator's --container-cap-drop list win over
// capabilities that container options add back.
func keepOperatorCapDrop(hc *container.HostConfig, operatorCapDrop []string) {
	if len(operatorCapDrop) == 0 {
		return
	}
	for _, c := range operatorCapDrop {
		if !slices.ContainsFunc(hc.CapDrop, func(s string) bool { return strings.EqualFold(s, c) }) {
			hc.CapDrop = append(hc.CapDrop, c)
		}
	}
	hc.CapAdd = slices.DeleteFunc(hc.CapAdd, func(a string) bool {
		return slices.ContainsFunc(operatorCapDrop, func(d string) bool {
			return strings.EqualFold(normalizeCap(a), normalizeCap(d)) || strings.EqualFold(d, "ALL")
		})
	})
}

func normalizeCap(c string) string {
	c = strings.ToUpper(c)
	if c == "ALL" {
		return c
	}
	return strings.TrimPrefix(c, "CAP_")
}
