// This file is exact copy of https://github.com/docker/cli/blob/9ac8584acfd501c3f4da0e845e3a40ed15c85041/cli/command/container/opts_test.go with:
// * appended with license information
// * commented out case 'invalid-mixed-network-types' in test TestParseNetworkConfig
//
// docker/cli is licensed under the Apache License, Version 2.0.
// See DOCKER_LICENSE for the full license text.
//

//nolint:unparam,whitespace,depguard,dupl,gocritic
package container

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestValidateAttach(t *testing.T) {
	valid := []string{
		"stdin",
		"stdout",
		"stderr",
		"STDIN",
		"STDOUT",
		"STDERR",
	}
	if _, err := validateAttach("invalid"); err == nil {
		t.Fatal("Expected error with [valid streams are STDIN, STDOUT and STDERR], got nothing")
	}

	for _, attach := range valid {
		value, err := validateAttach(attach)
		if err != nil {
			t.Fatal(err)
		}
		if value != strings.ToLower(attach) {
			t.Fatalf("Expected [%v], got [%v]", attach, value)
		}
	}
}

func parseRun(args []string) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig, error) {
	flags, copts := setupRunFlags()
	if err := flags.Parse(args); err != nil {
		return nil, nil, nil, err
	}
	// TODO: fix tests to accept ContainerConfig
	containerConfig, err := parse(flags, copts, runtime.GOOS)
	if err != nil {
		return nil, nil, nil, err
	}
	return containerConfig.Config, containerConfig.HostConfig, containerConfig.NetworkingConfig, err
}

func setupRunFlags() (*pflag.FlagSet, *containerOptions) {
	flags := pflag.NewFlagSet("run", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = nil
	copts := addFlags(flags)
	return flags, copts
}

func mustParse(t *testing.T, args string) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig) {
	t.Helper()
	config, hostConfig, networkingConfig, err := parseRun(append(strings.Split(args, " "), "ubuntu", "bash"))
	assert.NilError(t, err)
	return config, hostConfig, networkingConfig
}

func TestParseRunLinks(t *testing.T) {
	if _, hostConfig, _ := mustParse(t, "--link a:b"); len(hostConfig.Links) == 0 || hostConfig.Links[0] != "a:b" {
		t.Fatalf("Error parsing links. Expected []string{\"a:b\"}, received: %v", hostConfig.Links)
	}
	if _, hostConfig, _ := mustParse(t, "--link a:b --link c:d"); len(hostConfig.Links) < 2 || hostConfig.Links[0] != "a:b" || hostConfig.Links[1] != "c:d" {
		t.Fatalf("Error parsing links. Expected []string{\"a:b\", \"c:d\"}, received: %v", hostConfig.Links)
	}
	if _, hostConfig, _ := mustParse(t, ""); len(hostConfig.Links) != 0 {
		t.Fatalf("Error parsing links. No link expected, received: %v", hostConfig.Links)
	}
}

func TestParseRunAttach(t *testing.T) {
	tests := []struct {
		input    string
		expected container.Config
	}{
		{
			input: "",
			expected: container.Config{
				AttachStdout: true,
				AttachStderr: true,
			},
		},
		{
			input: "-i",
			expected: container.Config{
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
			},
		},
		{
			input: "-a stdin",
			expected: container.Config{
				AttachStdin: true,
			},
		},
		{
			input: "-a stdin -a stdout",
			expected: container.Config{
				AttachStdin:  true,
				AttachStdout: true,
			},
		},
		{
			input: "-a stdin -a stdout -a stderr",
			expected: container.Config{
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			config, _, _ := mustParse(t, tc.input)
			assert.Equal(t, config.AttachStdin, tc.expected.AttachStdin)
			assert.Equal(t, config.AttachStdout, tc.expected.AttachStdout)
			assert.Equal(t, config.AttachStderr, tc.expected.AttachStderr)
		})
	}
}

func TestParseRunWithInvalidArgs(t *testing.T) {
	tests := []struct {
		args  []string
		error string
	}{
		{
			args:  []string{"-a", "ubuntu", "bash"},
			error: `invalid argument "ubuntu" for "-a, --attach" flag: valid streams are STDIN, STDOUT and STDERR`,
		},
		{
			args:  []string{"-a", "invalid", "ubuntu", "bash"},
			error: `invalid argument "invalid" for "-a, --attach" flag: valid streams are STDIN, STDOUT and STDERR`,
		},
		{
			args:  []string{"-a", "invalid", "-a", "stdout", "ubuntu", "bash"},
			error: `invalid argument "invalid" for "-a, --attach" flag: valid streams are STDIN, STDOUT and STDERR`,
		},
		{
			args:  []string{"-a", "stdout", "-a", "stderr", "-z", "ubuntu", "bash"},
			error: `unknown shorthand flag: 'z' in -z`,
		},
		{
			args:  []string{"-a", "stdin", "-z", "ubuntu", "bash"},
			error: `unknown shorthand flag: 'z' in -z`,
		},
		{
			args:  []string{"-a", "stdout", "-z", "ubuntu", "bash"},
			error: `unknown shorthand flag: 'z' in -z`,
		},
		{
			args:  []string{"-a", "stderr", "-z", "ubuntu", "bash"},
			error: `unknown shorthand flag: 'z' in -z`,
		},
		{
			args:  []string{"-z", "--rm", "ubuntu", "bash"},
			error: `unknown shorthand flag: 'z' in -z`,
		},
	}
	flags, _ := setupRunFlags()
	for _, tc := range tests {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			assert.Error(t, flags.Parse(tc.args), tc.error)
		})
	}
}

//nolint:gocyclo
func TestParseWithVolumes(t *testing.T) {

	// A single volume
	arr, tryit := setupPlatformVolume([]string{`/tmp`}, []string{`c:\tmp`})
	if config, hostConfig, _ := mustParse(t, tryit); hostConfig.Binds != nil {
		t.Fatalf("Error parsing volume flags, %q should not mount-bind anything. Received %v", tryit, hostConfig.Binds)
	} else if _, exists := config.Volumes[arr[0]]; !exists {
		t.Fatalf("Error parsing volume flags, %q is missing from volumes. Received %v", tryit, config.Volumes)
	}

	// Two volumes
	arr, tryit = setupPlatformVolume([]string{`/tmp`, `/var`}, []string{`c:\tmp`, `c:\var`})
	if config, hostConfig, _ := mustParse(t, tryit); hostConfig.Binds != nil {
		t.Fatalf("Error parsing volume flags, %q should not mount-bind anything. Received %v", tryit, hostConfig.Binds)
	} else if _, exists := config.Volumes[arr[0]]; !exists {
		t.Fatalf("Error parsing volume flags, %s is missing from volumes. Received %v", arr[0], config.Volumes)
	} else if _, exists := config.Volumes[arr[1]]; !exists {
		t.Fatalf("Error parsing volume flags, %s is missing from volumes. Received %v", arr[1], config.Volumes)
	}

	// A single bind mount
	arr, tryit = setupPlatformVolume([]string{`/hostTmp:/containerTmp`}, []string{os.Getenv("TEMP") + `:c:\containerTmp`})
	if config, hostConfig, _ := mustParse(t, tryit); hostConfig.Binds == nil || hostConfig.Binds[0] != arr[0] {
		t.Fatalf("Error parsing volume flags, %q should mount-bind the path before the colon into the path after the colon. Received %v %v", arr[0], hostConfig.Binds, config.Volumes)
	}

	// Two bind mounts.
	arr, tryit = setupPlatformVolume([]string{`/hostTmp:/containerTmp`, `/hostVar:/containerVar`}, []string{os.Getenv("ProgramData") + `:c:\ContainerPD`, os.Getenv("TEMP") + `:c:\containerTmp`})
	if _, hostConfig, _ := mustParse(t, tryit); hostConfig.Binds == nil || compareRandomizedStrings(hostConfig.Binds[0], hostConfig.Binds[1], arr[0], arr[1]) != nil {
		t.Fatalf("Error parsing volume flags, `%s and %s` did not mount-bind correctly. Received %v", arr[0], arr[1], hostConfig.Binds)
	}

	// Two bind mounts, first read-only, second read-write.
	// TODO Windows: The Windows version uses read-write as that's the only mode it supports. Can change this post TP4
	arr, tryit = setupPlatformVolume(
		[]string{`/hostTmp:/containerTmp:ro`, `/hostVar:/containerVar:rw`},
		[]string{os.Getenv("TEMP") + `:c:\containerTmp:rw`, os.Getenv("ProgramData") + `:c:\ContainerPD:rw`})
	if _, hostConfig, _ := mustParse(t, tryit); hostConfig.Binds == nil || compareRandomizedStrings(hostConfig.Binds[0], hostConfig.Binds[1], arr[0], arr[1]) != nil {
		t.Fatalf("Error parsing volume flags, `%s and %s` did not mount-bind correctly. Received %v", arr[0], arr[1], hostConfig.Binds)
	}

	// Similar to previous test but with alternate modes which are only supported by Linux
	if runtime.GOOS != "windows" {
		arr, tryit = setupPlatformVolume([]string{`/hostTmp:/containerTmp:ro,Z`, `/hostVar:/containerVar:rw,Z`}, []string{})
		if _, hostConfig, _ := mustParse(t, tryit); hostConfig.Binds == nil || compareRandomizedStrings(hostConfig.Binds[0], hostConfig.Binds[1], arr[0], arr[1]) != nil {
			t.Fatalf("Error parsing volume flags, `%s and %s` did not mount-bind correctly. Received %v", arr[0], arr[1], hostConfig.Binds)
		}

		arr, tryit = setupPlatformVolume([]string{`/hostTmp:/containerTmp:Z`, `/hostVar:/containerVar:z`}, []string{})
		if _, hostConfig, _ := mustParse(t, tryit); hostConfig.Binds == nil || compareRandomizedStrings(hostConfig.Binds[0], hostConfig.Binds[1], arr[0], arr[1]) != nil {
			t.Fatalf("Error parsing volume flags, `%s and %s` did not mount-bind correctly. Received %v", arr[0], arr[1], hostConfig.Binds)
		}
	}

	// One bind mount and one volume
	arr, tryit = setupPlatformVolume([]string{`/hostTmp:/containerTmp`, `/containerVar`}, []string{os.Getenv("TEMP") + `:c:\containerTmp`, `c:\containerTmp`})
	if config, hostConfig, _ := mustParse(t, tryit); hostConfig.Binds == nil || len(hostConfig.Binds) > 1 || hostConfig.Binds[0] != arr[0] {
		t.Fatalf("Error parsing volume flags, %s and %s should only one and only one bind mount %s. Received %s", arr[0], arr[1], arr[0], hostConfig.Binds)
	} else if _, exists := config.Volumes[arr[1]]; !exists {
		t.Fatalf("Error parsing volume flags %s and %s. %s is missing from volumes. Received %v", arr[0], arr[1], arr[1], config.Volumes)
	}

	// Root to non-c: drive letter (Windows specific)
	if runtime.GOOS == "windows" {
		arr, tryit = setupPlatformVolume([]string{}, []string{os.Getenv("SystemDrive") + `\:d:`})
		if config, hostConfig, _ := mustParse(t, tryit); hostConfig.Binds == nil || len(hostConfig.Binds) > 1 || hostConfig.Binds[0] != arr[0] || len(config.Volumes) != 0 {
			t.Fatalf("Error parsing %s. Should have a single bind mount and no volumes", arr[0])
		}
	}

}

// setupPlatformVolume takes two arrays of volume specs - a Unix style
// spec and a Windows style spec. Depending on the platform being unit tested,
// it returns one of them, along with a volume string that would be passed
// on the docker CLI (e.g. -v /bar -v /foo).
func setupPlatformVolume(u []string, w []string) ([]string, string) {
	var a []string
	if runtime.GOOS == "windows" {
		a = w
	} else {
		a = u
	}
	s := ""
	for _, v := range a {
		s = s + "-v " + v + " "
	}
	return a, s
}

// check if (a == c && b == d) || (a == d && b == c)
// because maps are randomized
func compareRandomizedStrings(a, b, c, d string) error {
	if a == c && b == d {
		return nil
	}
	if a == d && b == c {
		return nil
	}
	return errors.Errorf("strings don't match")
}

// Simple parse with MacAddress validation
func TestParseWithMacAddress(t *testing.T) {
	invalidMacAddress := "--mac-address=invalidMacAddress"
	validMacAddress := "--mac-address=92:d0:c6:0a:29:33"
	if _, _, _, err := parseRun([]string{invalidMacAddress, "img", "cmd"}); err != nil && err.Error() != "invalidMacAddress is not a valid mac address" {
		t.Fatalf("Expected an error with %v mac-address, got %v", invalidMacAddress, err)
	}
	config, hostConfig, _ := mustParse(t, validMacAddress)
	fmt.Printf("MacAddress: %+v\n", hostConfig)
	assert.Equal(t, "92:d0:c6:0a:29:33", config.MacAddress) //nolint:staticcheck
}

func TestRunFlagsParseWithMemory(t *testing.T) {
	flags, _ := setupRunFlags()
	args := []string{"--memory=invalid", "img", "cmd"}
	err := flags.Parse(args)
	assert.ErrorContains(t, err, `invalid argument "invalid" for "-m, --memory" flag`)

	_, hostconfig, _ := mustParse(t, "--memory=1G")
	assert.Check(t, is.Equal(int64(1073741824), hostconfig.Memory))
}

func TestParseWithMemorySwap(t *testing.T) {
	flags, _ := setupRunFlags()
	args := []string{"--memory-swap=invalid", "img", "cmd"}
	err := flags.Parse(args)
	assert.ErrorContains(t, err, `invalid argument "invalid" for "--memory-swap" flag`)

	_, hostconfig, _ := mustParse(t, "--memory-swap=1G")
	assert.Check(t, is.Equal(int64(1073741824), hostconfig.MemorySwap))

	_, hostconfig, _ = mustParse(t, "--memory-swap=-1")
	assert.Check(t, is.Equal(int64(-1), hostconfig.MemorySwap))
}

func TestParseHostname(t *testing.T) {
	validHostnames := map[string]string{
		"hostname":    "hostname",
		"host-name":   "host-name",
		"hostname123": "hostname123",
		"123hostname": "123hostname",
		"hostname-of-63-bytes-long-should-be-valid-and-without-any-error": "hostname-of-63-bytes-long-should-be-valid-and-without-any-error",
	}
	hostnameWithDomain := "--hostname=hostname.domainname"
	hostnameWithDomainTld := "--hostname=hostname.domainname.tld"
	for hostname, expectedHostname := range validHostnames {
		if config, _, _ := mustParse(t, fmt.Sprintf("--hostname=%s", hostname)); config.Hostname != expectedHostname {
			t.Fatalf("Expected the config to have 'hostname' as %q, got %q", expectedHostname, config.Hostname)
		}
	}
	if config, _, _ := mustParse(t, hostnameWithDomain); config.Hostname != "hostname.domainname" || config.Domainname != "" {
		t.Fatalf("Expected the config to have 'hostname' as hostname.domainname, got %q", config.Hostname)
	}
	if config, _, _ := mustParse(t, hostnameWithDomainTld); config.Hostname != "hostname.domainname.tld" || config.Domainname != "" {
		t.Fatalf("Expected the config to have 'hostname' as hostname.domainname.tld, got %q", config.Hostname)
	}
}

func TestParseHostnameDomainname(t *testing.T) {
	validDomainnames := map[string]string{
		"domainname":    "domainname",
		"domain-name":   "domain-name",
		"domainname123": "domainname123",
		"123domainname": "123domainname",
		"domainname-63-bytes-long-should-be-valid-and-without-any-errors": "domainname-63-bytes-long-should-be-valid-and-without-any-errors",
	}
	for domainname, expectedDomainname := range validDomainnames {
		if config, _, _ := mustParse(t, "--domainname="+domainname); config.Domainname != expectedDomainname {
			t.Fatalf("Expected the config to have 'domainname' as %q, got %q", expectedDomainname, config.Domainname)
		}
	}
	if config, _, _ := mustParse(t, "--hostname=some.prefix --domainname=domainname"); config.Hostname != "some.prefix" || config.Domainname != "domainname" {
		t.Fatalf("Expected the config to have 'hostname' as 'some.prefix' and 'domainname' as 'domainname', got %q and %q", config.Hostname, config.Domainname)
	}
	if config, _, _ := mustParse(t, "--hostname=another-prefix --domainname=domainname.tld"); config.Hostname != "another-prefix" || config.Domainname != "domainname.tld" {
		t.Fatalf("Expected the config to have 'hostname' as 'another-prefix' and 'domainname' as 'domainname.tld', got %q and %q", config.Hostname, config.Domainname)
	}
}

func TestParseWithExpose(t *testing.T) {
	invalids := map[string]string{
		":":                   "invalid port format for --expose: :",
		"8080:9090":           "invalid port format for --expose: 8080:9090",
		"/tcp":                "invalid range format for --expose: /tcp, error: empty string specified for ports",
		"/udp":                "invalid range format for --expose: /udp, error: empty string specified for ports",
		"NaN/tcp":             `invalid range format for --expose: NaN/tcp, error: strconv.ParseUint: parsing "NaN": invalid syntax`,
		"NaN-NaN/tcp":         `invalid range format for --expose: NaN-NaN/tcp, error: strconv.ParseUint: parsing "NaN": invalid syntax`,
		"8080-NaN/tcp":        `invalid range format for --expose: 8080-NaN/tcp, error: strconv.ParseUint: parsing "NaN": invalid syntax`,
		"1234567890-8080/tcp": `invalid range format for --expose: 1234567890-8080/tcp, error: strconv.ParseUint: parsing "1234567890": value out of range`,
	}
	valids := map[string][]nat.Port{
		"8080/tcp":      {"8080/tcp"},
		"8080/udp":      {"8080/udp"},
		"8080/ncp":      {"8080/ncp"},
		"8080-8080/udp": {"8080/udp"},
		"8080-8082/tcp": {"8080/tcp", "8081/tcp", "8082/tcp"},
	}
	for expose, expectedError := range invalids {
		if _, _, _, err := parseRun([]string{fmt.Sprintf("--expose=%v", expose), "img", "cmd"}); err == nil || err.Error() != expectedError {
			t.Fatalf("Expected error '%v' with '--expose=%v', got '%v'", expectedError, expose, err)
		}
	}
	for expose, exposedPorts := range valids {
		config, _, _, err := parseRun([]string{fmt.Sprintf("--expose=%v", expose), "img", "cmd"})
		if err != nil {
			t.Fatal(err)
		}
		if len(config.ExposedPorts) != len(exposedPorts) {
			t.Fatalf("Expected %v exposed port, got %v", len(exposedPorts), len(config.ExposedPorts))
		}
		for _, port := range exposedPorts {
			if _, ok := config.ExposedPorts[port]; !ok {
				t.Fatalf("Expected %v, got %v", exposedPorts, config.ExposedPorts)
			}
		}
	}
	// Merge with actual published port
	config, _, _, err := parseRun([]string{"--publish=80", "--expose=80-81/tcp", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.ExposedPorts) != 2 {
		t.Fatalf("Expected 2 exposed ports, got %v", config.ExposedPorts)
	}
	ports := []nat.Port{"80/tcp", "81/tcp"}
	for _, port := range ports {
		if _, ok := config.ExposedPorts[port]; !ok {
			t.Fatalf("Expected %v, got %v", ports, config.ExposedPorts)
		}
	}
}

func TestParseDevice(t *testing.T) {
	skip.If(t, runtime.GOOS != "linux") // Windows and macOS validate server-side
	valids := map[string]container.DeviceMapping{
		"/dev/snd": {
			PathOnHost:        "/dev/snd",
			PathInContainer:   "/dev/snd",
			CgroupPermissions: "rwm",
		},
		"/dev/snd:rw": {
			PathOnHost:        "/dev/snd",
			PathInContainer:   "/dev/snd",
			CgroupPermissions: "rw",
		},
		"/dev/snd:/something": {
			PathOnHost:        "/dev/snd",
			PathInContainer:   "/something",
			CgroupPermissions: "rwm",
		},
		"/dev/snd:/something:rw": {
			PathOnHost:        "/dev/snd",
			PathInContainer:   "/something",
			CgroupPermissions: "rw",
		},
	}
	for device, deviceMapping := range valids {
		_, hostconfig, _, err := parseRun([]string{fmt.Sprintf("--device=%v", device), "img", "cmd"})
		if err != nil {
			t.Fatal(err)
		}
		if len(hostconfig.Devices) != 1 {
			t.Fatalf("Expected 1 devices, got %v", hostconfig.Devices)
		}
		if hostconfig.Devices[0] != deviceMapping {
			t.Fatalf("Expected %v, got %v", deviceMapping, hostconfig.Devices)
		}
	}

}

func TestParseNetworkConfig(t *testing.T) {
	tests := []struct {
		name        string
		flags       []string
		expected    map[string]*networktypes.EndpointSettings
		expectedCfg container.HostConfig
		expectedErr string
	}{
		{
			name:        "single-network-legacy",
			flags:       []string{"--network", "net1"},
			expected:    map[string]*networktypes.EndpointSettings{},
			expectedCfg: container.HostConfig{NetworkMode: "net1"},
		},
		{
			name:        "single-network-advanced",
			flags:       []string{"--network", "name=net1"},
			expected:    map[string]*networktypes.EndpointSettings{},
			expectedCfg: container.HostConfig{NetworkMode: "net1"},
		},
		{
			name: "single-network-legacy-with-options",
			flags: []string{
				"--ip", "172.20.88.22",
				"--ip6", "2001:db8::8822",
				"--link", "foo:bar",
				"--link", "bar:baz",
				"--link-local-ip", "169.254.2.2",
				"--link-local-ip", "fe80::169:254:2:2",
				"--network", "name=net1",
				"--network-alias", "web1",
				"--network-alias", "web2",
			},
			expected: map[string]*networktypes.EndpointSettings{
				"net1": {
					IPAMConfig: &networktypes.EndpointIPAMConfig{
						IPv4Address:  "172.20.88.22",
						IPv6Address:  "2001:db8::8822",
						LinkLocalIPs: []string{"169.254.2.2", "fe80::169:254:2:2"},
					},
					Links:   []string{"foo:bar", "bar:baz"},
					Aliases: []string{"web1", "web2"},
				},
			},
			expectedCfg: container.HostConfig{NetworkMode: "net1"},
		},
		{
			name: "multiple-network-advanced-mixed",
			flags: []string{
				"--ip", "172.20.88.22",
				"--ip6", "2001:db8::8822",
				"--link", "foo:bar",
				"--link", "bar:baz",
				"--link-local-ip", "169.254.2.2",
				"--link-local-ip", "fe80::169:254:2:2",
				"--network", "name=net1,driver-opt=field1=value1",
				"--network-alias", "web1",
				"--network-alias", "web2",
				"--network", "net2",
				"--network", "name=net3,alias=web3,driver-opt=field3=value3,ip=172.20.88.22,ip6=2001:db8::8822",
			},
			expected: map[string]*networktypes.EndpointSettings{
				"net1": {
					DriverOpts: map[string]string{"field1": "value1"},
					IPAMConfig: &networktypes.EndpointIPAMConfig{
						IPv4Address:  "172.20.88.22",
						IPv6Address:  "2001:db8::8822",
						LinkLocalIPs: []string{"169.254.2.2", "fe80::169:254:2:2"},
					},
					Links:   []string{"foo:bar", "bar:baz"},
					Aliases: []string{"web1", "web2"},
				},
				"net2": {},
				"net3": {
					DriverOpts: map[string]string{"field3": "value3"},
					IPAMConfig: &networktypes.EndpointIPAMConfig{
						IPv4Address: "172.20.88.22",
						IPv6Address: "2001:db8::8822",
					},
					Aliases: []string{"web3"},
				},
			},
			expectedCfg: container.HostConfig{NetworkMode: "net1"},
		},
		{
			name:  "single-network-advanced-with-options",
			flags: []string{"--network", "name=net1,alias=web1,alias=web2,driver-opt=field1=value1,driver-opt=field2=value2,ip=172.20.88.22,ip6=2001:db8::8822"},
			expected: map[string]*networktypes.EndpointSettings{
				"net1": {
					DriverOpts: map[string]string{
						"field1": "value1",
						"field2": "value2",
					},
					IPAMConfig: &networktypes.EndpointIPAMConfig{
						IPv4Address: "172.20.88.22",
						IPv6Address: "2001:db8::8822",
					},
					Aliases: []string{"web1", "web2"},
				},
			},
			expectedCfg: container.HostConfig{NetworkMode: "net1"},
		},
		{
			name:        "multiple-networks",
			flags:       []string{"--network", "net1", "--network", "name=net2"},
			expected:    map[string]*networktypes.EndpointSettings{"net1": {}, "net2": {}},
			expectedCfg: container.HostConfig{NetworkMode: "net1"},
		},
		{
			name:        "conflict-network",
			flags:       []string{"--network", "duplicate", "--network", "name=duplicate"},
			expectedErr: `network "duplicate" is specified multiple times`,
		},
		{
			name:        "conflict-options-alias",
			flags:       []string{"--network", "name=net1,alias=web1", "--network-alias", "web1"},
			expectedErr: `conflicting options: cannot specify both --network-alias and per-network alias`,
		},
		{
			name:        "conflict-options-ip",
			flags:       []string{"--network", "name=net1,ip=172.20.88.22,ip6=2001:db8::8822", "--ip", "172.20.88.22"},
			expectedErr: `conflicting options: cannot specify both --ip and per-network IPv4 address`,
		},
		{
			name:        "conflict-options-ip6",
			flags:       []string{"--network", "name=net1,ip=172.20.88.22,ip6=2001:db8::8822", "--ip6", "2001:db8::8822"},
			expectedErr: `conflicting options: cannot specify both --ip6 and per-network IPv6 address`,
		},
		// case is skipped as it fails w/o any change
		//
		//{
		//	name:        "invalid-mixed-network-types",
		//	flags:       []string{"--network", "name=host", "--network", "net1"},
		//	expectedErr: `conflicting options: cannot attach both user-defined and non-user-defined network-modes`,
		//},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, hConfig, nwConfig, err := parseRun(tc.flags)

			if tc.expectedErr != "" {
				assert.Error(t, err, tc.expectedErr)
				return
			}

			assert.NilError(t, err)
			assert.DeepEqual(t, hConfig.NetworkMode, tc.expectedCfg.NetworkMode)
			assert.DeepEqual(t, nwConfig.EndpointsConfig, tc.expected)
		})
	}
}

func TestParseModes(t *testing.T) {
	// pid ko
	flags, copts := setupRunFlags()
	args := []string{"--pid=container:", "img", "cmd"}
	assert.NilError(t, flags.Parse(args))
	_, err := parse(flags, copts, runtime.GOOS)
	assert.ErrorContains(t, err, "--pid: invalid PID mode")

	// pid ok
	_, hostconfig, _, err := parseRun([]string{"--pid=host", "img", "cmd"})
	assert.NilError(t, err)
	if !hostconfig.PidMode.Valid() {
		t.Fatalf("Expected a valid PidMode, got %v", hostconfig.PidMode)
	}

	// uts ko
	_, _, _, err = parseRun([]string{"--uts=container:", "img", "cmd"}) //nolint:dogsled
	assert.ErrorContains(t, err, "--uts: invalid UTS mode")

	// uts ok
	_, hostconfig, _, err = parseRun([]string{"--uts=host", "img", "cmd"})
	assert.NilError(t, err)
	if !hostconfig.UTSMode.Valid() {
		t.Fatalf("Expected a valid UTSMode, got %v", hostconfig.UTSMode)
	}
}

func TestRunFlagsParseShmSize(t *testing.T) {
	// shm-size ko
	flags, _ := setupRunFlags()
	args := []string{"--shm-size=a128m", "img", "cmd"}
	expectedErr := `invalid argument "a128m" for "--shm-size" flag:`
	err := flags.Parse(args)
	assert.ErrorContains(t, err, expectedErr)

	// shm-size ok
	_, hostconfig, _, err := parseRun([]string{"--shm-size=128m", "img", "cmd"})
	assert.NilError(t, err)
	if hostconfig.ShmSize != 134217728 {
		t.Fatalf("Expected a valid ShmSize, got %d", hostconfig.ShmSize)
	}
}

func TestParseRestartPolicy(t *testing.T) {
	invalids := map[string]string{
		"always:2:3":         "invalid restart policy format: maximum retry count must be an integer",
		"on-failure:invalid": "invalid restart policy format: maximum retry count must be an integer",
	}
	valids := map[string]container.RestartPolicy{
		"": {},
		"always": {
			Name:              "always",
			MaximumRetryCount: 0,
		},
		"on-failure:1": {
			Name:              "on-failure",
			MaximumRetryCount: 1,
		},
	}
	for restart, expectedError := range invalids {
		if _, _, _, err := parseRun([]string{fmt.Sprintf("--restart=%s", restart), "img", "cmd"}); err == nil || err.Error() != expectedError {
			t.Fatalf("Expected an error with message '%v' for %v, got %v", expectedError, restart, err)
		}
	}
	for restart, expected := range valids {
		_, hostconfig, _, err := parseRun([]string{fmt.Sprintf("--restart=%v", restart), "img", "cmd"})
		if err != nil {
			t.Fatal(err)
		}
		if hostconfig.RestartPolicy != expected {
			t.Fatalf("Expected %v, got %v", expected, hostconfig.RestartPolicy)
		}
	}
}

func TestParseRestartPolicyAutoRemove(t *testing.T) {
	expected := "Conflicting options: --restart and --rm"
	_, _, _, err := parseRun([]string{"--rm", "--restart=always", "img", "cmd"}) //nolint:dogsled
	if err == nil || err.Error() != expected {
		t.Fatalf("Expected error %v, but got none", expected)
	}
}

func TestParseHealth(t *testing.T) {
	checkOk := func(args ...string) *container.HealthConfig {
		config, _, _, err := parseRun(args)
		if err != nil {
			t.Fatalf("%#v: %v", args, err)
		}
		return config.Healthcheck
	}
	checkError := func(expected string, args ...string) {
		config, _, _, err := parseRun(args)
		if err == nil {
			t.Fatalf("Expected error, but got %#v", config)
		}
		if err.Error() != expected {
			t.Fatalf("Expected %#v, got %#v", expected, err)
		}
	}
	health := checkOk("--no-healthcheck", "img", "cmd")
	if health == nil || len(health.Test) != 1 || health.Test[0] != "NONE" {
		t.Fatalf("--no-healthcheck failed: %#v", health)
	}

	health = checkOk("--health-cmd=/check.sh -q", "img", "cmd")
	if len(health.Test) != 2 || health.Test[0] != "CMD-SHELL" || health.Test[1] != "/check.sh -q" {
		t.Fatalf("--health-cmd: got %#v", health.Test)
	}
	if health.Timeout != 0 {
		t.Fatalf("--health-cmd: timeout = %s", health.Timeout)
	}

	checkError("--no-healthcheck conflicts with --health-* options",
		"--no-healthcheck", "--health-cmd=/check.sh -q", "img", "cmd")

	health = checkOk("--health-timeout=2s", "--health-retries=3", "--health-interval=4.5s", "--health-start-period=5s", "img", "cmd")
	if health.Timeout != 2*time.Second || health.Retries != 3 || health.Interval != 4500*time.Millisecond || health.StartPeriod != 5*time.Second {
		t.Fatalf("--health-*: got %#v", health)
	}
}

func TestParseLoggingOpts(t *testing.T) {
	// logging opts ko
	if _, _, _, err := parseRun([]string{"--log-driver=none", "--log-opt=anything", "img", "cmd"}); err == nil || err.Error() != "invalid logging opts for driver none" {
		t.Fatalf("Expected an error with message 'invalid logging opts for driver none', got %v", err)
	}
	// logging opts ok
	_, hostconfig, _, err := parseRun([]string{"--log-driver=syslog", "--log-opt=something", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if hostconfig.LogConfig.Type != "syslog" || len(hostconfig.LogConfig.Config) != 1 {
		t.Fatalf("Expected a 'syslog' LogConfig with one config, got %v", hostconfig.RestartPolicy)
	}
}

func TestParseEnvfileVariables(t *testing.T) {
	e := "open nonexistent: no such file or directory"
	if runtime.GOOS == "windows" {
		e = "open nonexistent: The system cannot find the file specified."
	}
	// env ko
	if _, _, _, err := parseRun([]string{"--env-file=nonexistent", "img", "cmd"}); err == nil || err.Error() != e {
		t.Fatalf("Expected an error with message '%s', got %v", e, err)
	}
	// env ok
	config, _, _, err := parseRun([]string{"--env-file=testdata/valid.env", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Env) != 1 || config.Env[0] != "ENV1=value1" {
		t.Fatalf("Expected a config with [ENV1=value1], got %v", config.Env)
	}
	config, _, _, err = parseRun([]string{"--env-file=testdata/valid.env", "--env=ENV2=value2", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Env) != 2 || config.Env[0] != "ENV1=value1" || config.Env[1] != "ENV2=value2" {
		t.Fatalf("Expected a config with [ENV1=value1 ENV2=value2], got %v", config.Env)
	}
}

func TestParseEnvfileVariablesWithBOMUnicode(t *testing.T) {
	// UTF8 with BOM
	config, _, _, err := parseRun([]string{"--env-file=testdata/utf8.env", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	env := []string{"FOO=BAR", "HELLO=" + string([]byte{0xe6, 0x82, 0xa8, 0xe5, 0xa5, 0xbd}), "BAR=FOO"}
	if len(config.Env) != len(env) {
		t.Fatalf("Expected a config with %d env variables, got %v: %v", len(env), len(config.Env), config.Env)
	}
	for i, v := range env {
		if config.Env[i] != v {
			t.Fatalf("Expected a config with [%s], got %v", v, []byte(config.Env[i]))
		}
	}

	// UTF16 with BOM
	e := "invalid env file"
	if _, _, _, err := parseRun([]string{"--env-file=testdata/utf16.env", "img", "cmd"}); err == nil || !strings.Contains(err.Error(), e) {
		t.Fatalf("Expected an error with message '%s', got %v", e, err)
	}
	// UTF16BE with BOM
	if _, _, _, err := parseRun([]string{"--env-file=testdata/utf16be.env", "img", "cmd"}); err == nil || !strings.Contains(err.Error(), e) {
		t.Fatalf("Expected an error with message '%s', got %v", e, err)
	}
}

func TestParseLabelfileVariables(t *testing.T) {
	e := "open nonexistent: no such file or directory"
	if runtime.GOOS == "windows" {
		e = "open nonexistent: The system cannot find the file specified."
	}
	// label ko
	if _, _, _, err := parseRun([]string{"--label-file=nonexistent", "img", "cmd"}); err == nil || err.Error() != e {
		t.Fatalf("Expected an error with message '%s', got %v", e, err)
	}
	// label ok
	config, _, _, err := parseRun([]string{"--label-file=testdata/valid.label", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Labels) != 1 || config.Labels["LABEL1"] != "value1" {
		t.Fatalf("Expected a config with [LABEL1:value1], got %v", config.Labels)
	}
	config, _, _, err = parseRun([]string{"--label-file=testdata/valid.label", "--label=LABEL2=value2", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Labels) != 2 || config.Labels["LABEL1"] != "value1" || config.Labels["LABEL2"] != "value2" {
		t.Fatalf("Expected a config with [LABEL1:value1 LABEL2:value2], got %v", config.Labels)
	}
}

func TestParseEntryPoint(t *testing.T) {
	config, _, _, err := parseRun([]string{"--entrypoint=anything", "cmd", "img"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Entrypoint) != 1 && config.Entrypoint[0] != "anything" {
		t.Fatalf("Expected entrypoint 'anything', got %v", config.Entrypoint)
	}
}

func TestValidateDevice(t *testing.T) {
	skip.If(t, runtime.GOOS != "linux") // Windows and macOS validate server-side
	valid := []string{
		"/home",
		"/home:/home",
		"/home:/something/else",
		"/with space",
		"/home:/with space",
		"relative:/absolute-path",
		"hostPath:/containerPath:r",
		"/hostPath:/containerPath:rw",
		"/hostPath:/containerPath:mrw",
	}
	invalid := map[string]string{
		"":        "bad format for path: ",
		"./":      "./ is not an absolute path",
		"../":     "../ is not an absolute path",
		"/:../":   "../ is not an absolute path",
		"/:path":  "path is not an absolute path",
		":":       "bad format for path: :",
		"/tmp:":   " is not an absolute path",
		":test":   "bad format for path: :test",
		":/test":  "bad format for path: :/test",
		"tmp:":    " is not an absolute path",
		":test:":  "bad format for path: :test:",
		"::":      "bad format for path: ::",
		":::":     "bad format for path: :::",
		"/tmp:::": "bad format for path: /tmp:::",
		":/tmp::": "bad format for path: :/tmp::",
		"path:ro": "ro is not an absolute path",
		"path:rr": "rr is not an absolute path",
		"a:/b:ro": "bad mode specified: ro",
		"a:/b:rr": "bad mode specified: rr",
	}

	for _, path := range valid {
		if _, err := validateDevice(path, runtime.GOOS); err != nil {
			t.Fatalf("ValidateDevice(`%q`) should succeed: error %q", path, err)
		}
	}

	for path, expectedError := range invalid {
		if _, err := validateDevice(path, runtime.GOOS); err == nil {
			t.Fatalf("ValidateDevice(`%q`) should have failed validation", path)
		} else {
			if err.Error() != expectedError {
				t.Fatalf("ValidateDevice(`%q`) error should contain %q, got %q", path, expectedError, err.Error())
			}
		}
	}
}

func TestParseSystemPaths(t *testing.T) {
	tests := []struct {
		doc                       string
		in, out, masked, readonly []string
	}{
		{
			doc: "not set",
			in:  []string{},
			out: []string{},
		},
		{
			doc: "not set, preserve other options",
			in: []string{
				"seccomp=unconfined",
				"apparmor=unconfined",
				"label=user:USER",
				"foo=bar",
			},
			out: []string{
				"seccomp=unconfined",
				"apparmor=unconfined",
				"label=user:USER",
				"foo=bar",
			},
		},
		{
			doc:      "unconfined",
			in:       []string{"systempaths=unconfined"},
			out:      []string{},
			masked:   []string{},
			readonly: []string{},
		},
		{
			doc:      "unconfined and other options",
			in:       []string{"foo=bar", "bar=baz", "systempaths=unconfined"},
			out:      []string{"foo=bar", "bar=baz"},
			masked:   []string{},
			readonly: []string{},
		},
		{
			doc: "unknown option",
			in:  []string{"foo=bar", "systempaths=unknown", "bar=baz"},
			out: []string{"foo=bar", "systempaths=unknown", "bar=baz"},
		},
	}

	for _, tc := range tests {
		securityOpts, maskedPaths, readonlyPaths := parseSystemPaths(tc.in)
		assert.DeepEqual(t, securityOpts, tc.out)
		assert.DeepEqual(t, maskedPaths, tc.masked)
		assert.DeepEqual(t, readonlyPaths, tc.readonly)
	}
}

func TestConvertToStandardNotation(t *testing.T) {
	valid := map[string][]string{
		"20:10/tcp":               {"target=10,published=20"},
		"40:30":                   {"40:30"},
		"20:20 80:4444":           {"20:20", "80:4444"},
		"1500:2500/tcp 1400:1300": {"target=2500,published=1500", "1400:1300"},
		"1500:200/tcp 90:80/tcp":  {"published=1500,target=200", "target=80,published=90"},
	}

	invalid := [][]string{
		{"published=1500,target:444"},
		{"published=1500,444"},
		{"published=1500,target,444"},
	}

	for key, ports := range valid {
		convertedPorts, err := convertToStandardNotation(ports)

		if err != nil {
			assert.NilError(t, err)
		}
		assert.DeepEqual(t, strings.Split(key, " "), convertedPorts)
	}

	for _, ports := range invalid {
		if _, err := convertToStandardNotation(ports); err == nil {
			t.Fatalf("ConvertToStandardNotation(`%q`) should have failed conversion", ports)
		}
	}
}
