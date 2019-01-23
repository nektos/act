// +build !windows

package system // import "github.com/docker/docker/pkg/system"

// InitLCOW does nothing since LCOW is a windows only feature
func InitLCOW(experimental bool) {
}
