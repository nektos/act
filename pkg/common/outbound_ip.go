package common

import (
	"net"
	"sort"
	"strings"
)

// GetOutboundIP returns an outbound IP address of this machine.
// It tries to access the internet and returns the local IP address of the connection.
// If the machine cannot access the internet, it returns a preferred IP address from network interfaces.
// It returns nil if no IP address is found.
func GetOutboundIP() net.IP {
	// See https://stackoverflow.com/a/37382208
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		return conn.LocalAddr().(*net.UDPAddr).IP
	}

	// So the machine cannot access the internet. Pick an IP address from network interfaces.
	if ifs, err := net.Interfaces(); err == nil {
		type IP struct {
			net.IP
			net.Interface
		}
		var ips []IP
		for _, i := range ifs {
			if addrs, err := i.Addrs(); err == nil {
				for _, addr := range addrs {
					var ip net.IP
					switch v := addr.(type) {
					case *net.IPNet:
						ip = v.IP
					case *net.IPAddr:
						ip = v.IP
					}
					if ip.IsGlobalUnicast() {
						ips = append(ips, IP{ip, i})
					}
				}
			}
		}
		if len(ips) > 1 {
			sort.Slice(ips, func(i, j int) bool {
				ifi := ips[i].Interface
				ifj := ips[j].Interface

				// ethernet is preferred
				if vi, vj := strings.HasPrefix(ifi.Name, "e"), strings.HasPrefix(ifj.Name, "e"); vi != vj {
					return vi
				}

				ipi := ips[i].IP
				ipj := ips[j].IP

				// IPv4 is preferred
				if vi, vj := ipi.To4() != nil, ipj.To4() != nil; vi != vj {
					return vi
				}

				// en0 is preferred to en1
				if ifi.Name != ifj.Name {
					return ifi.Name < ifj.Name
				}

				// fallback
				return ipi.String() < ipj.String()
			})
			return ips[0].IP
		}
	}

	return nil
}
