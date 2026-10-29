//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/assert"
)

func mergeOptions(t *testing.T, options string, restrict bool, capDrop []string) (*container.Config, *container.HostConfig, error) {
	t.Helper()
	cr := &containerReference{input: &NewContainerInput{
		Options:         options,
		RestrictOptions: restrict,
		NetworkMode:     "act-network",
	}}
	return cr.mergeContainerConfigs(context.Background(), &container.Config{}, &container.HostConfig{
		NetworkMode: "act-network",
		CapDrop:     capDrop,
		Binds:       []string{"/var/run/docker.sock:/var/run/docker.sock"},
	})
}

func TestRestrictedContainerOptionsRejected(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "host.env")
	assert.NoError(t, os.WriteFile(envFile, []byte("SECRET=1\n"), 0o600))

	for _, options := range []string{
		"--privileged",
		"--cap-add SYS_ADMIN",
		"--pid host",
		"--pid container:other",
		"--ipc host",
		"--uts host",
		"--userns host",
		"--cgroupns host",
		"--cgroup-parent /",
		"--device /dev/sda",
		"--device-cgroup-rule 'b 8:* rwm'",
		"--gpus all",
		"-v /:/host",
		"--volume /etc:/etc:ro",
		"-v ./relative:/x",
		"--mount type=bind,source=/,target=/host",
		"--mount type=volume,source=v,target=/v,volume-driver=local,volume-opt=type=none,volume-opt=o=bind,volume-opt=device=/",
		"--volumes-from other",
		"--network host",
		"--env-file " + envFile,
		"--label-file " + envFile,
		"--security-opt seccomp=unconfined",
		"--security-opt seccomp=" + envFile,
		"--security-opt apparmor=unconfined",
		"--security-opt systempaths=unconfined",
		"--security-opt label=disable",
	} {
		_, _, err := mergeOptions(t, options, true, nil)
		if assert.Error(t, err, options) {
			assert.Contains(t, err.Error(), "--container-options-policy=allow", options)
		}
	}
}

func TestRestrictedContainerOptionsAllowed(t *testing.T) {
	for _, options := range []string{
		"--cpus 2",
		"--memory 1g",
		"--user 1000",
		"--hostname svc",
		"--health-cmd 'pg_isready' --health-interval 10s",
		"-v data:/data",
		"-v /anonymous",
		"--mount type=volume,source=v,target=/v",
		"--tmpfs /run",
		"--security-opt no-new-privileges",
		"--cap-drop ALL",
		"--ipc shareable",
		"--network act-network",
		"-e FOO=bar",
	} {
		_, _, err := mergeOptions(t, options, true, nil)
		assert.NoError(t, err, options)
	}
}

func TestUnrestrictedContainerOptionsKeepWorking(t *testing.T) {
	_, hc, err := mergeOptions(t, "--privileged -v /tmp:/tmp", false, nil)
	assert.NoError(t, err)
	assert.True(t, hc.Privileged)
	assert.Contains(t, hc.Binds, "/tmp:/tmp")
	assert.Contains(t, hc.Binds, "/var/run/docker.sock:/var/run/docker.sock")
}

func TestOperatorCapDropSurvivesOptions(t *testing.T) {
	_, hc, err := mergeOptions(t, "--cap-drop CHOWN --cap-add NET_RAW --cap-add SYS_TIME", false, []string{"NET_RAW", "MKNOD"})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"CHOWN", "NET_RAW", "MKNOD"}, hc.CapDrop)
	assert.ElementsMatch(t, []string{"SYS_TIME"}, hc.CapAdd)

	_, hc, err = mergeOptions(t, "--cap-add CAP_NET_RAW", false, []string{"ALL"})
	assert.NoError(t, err)
	assert.Empty(t, hc.CapAdd)
}
