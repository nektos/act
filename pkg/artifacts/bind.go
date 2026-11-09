package artifacts

import (
	"github.com/nektos/act/pkg/common"
)

// ListenAddress is the address the artifact server should bind to.
// When requestedSet is true, requested is used unchanged.
// Otherwise the server binds to loopback when job containers share the host
// network namespace, which is a Linux engine using the host network and no
// service-container network. Every other setup binds to the machine's outbound
// address, because those containers cannot reach the host's loopback interface.
func ListenAddress(goos, network string, serviceContainers bool, requested string, requestedSet bool) string {
	if requestedSet {
		return requested
	}
	if goos == "linux" && !serviceContainers && (network == "" || network == "host") {
		return "127.0.0.1"
	}
	if ip := common.GetOutboundIP(); ip != nil {
		return ip.String()
	}
	return "127.0.0.1"
}
