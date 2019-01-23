// +build !windows

package ioutils // import "github.com/docker/docker/pkg/ioutils"

import "io/ioutil"

// TempDir on Unix systems is equivalent to ioutil.TempDir.
func TempDir(dir, prefix string) (string, error) {
	return ioutil.TempDir(dir, prefix)
}
