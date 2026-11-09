package artifacts

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nektos/act/pkg/common"
)

func TestListenAddress(t *testing.T) {
	outbound := "127.0.0.1"
	if ip := common.GetOutboundIP(); ip != nil {
		outbound = ip.String()
	}

	tests := []struct {
		name      string
		goos      string
		network   string
		services  bool
		set       bool
		requested string
		want      string
	}{
		{name: "linux host network", goos: "linux", network: "host", want: "127.0.0.1"},
		{name: "linux default network", goos: "linux", network: "", requested: "ignored", want: "127.0.0.1"},
		{name: "linux bridge", goos: "linux", network: "bridge", want: outbound},
		{name: "linux service containers", goos: "linux", network: "host", services: true, want: outbound},
		{name: "darwin host network", goos: "darwin", network: "host", want: outbound},
		{name: "windows host network", goos: "windows", network: "host", want: outbound},
		{name: "explicit address", goos: "linux", network: "host", set: true, requested: "10.1.2.3", want: "10.1.2.3"},
		{name: "explicit loopback on a bridge", goos: "linux", network: "bridge", set: true, requested: "127.0.0.1", want: "127.0.0.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ListenAddress(tc.goos, tc.network, tc.services, tc.requested, tc.set)
			assert.Equal(t, tc.want, got)
		})
	}
}
