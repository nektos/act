// +build !linux

package archive // import "github.com/docker/docker/pkg/archive"

func getWhiteoutConverter(format WhiteoutFormat) tarWhiteoutConverter {
	return nil
}
