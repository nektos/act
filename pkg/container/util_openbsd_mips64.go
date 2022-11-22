package container

import (
	"errors"
	"os"
	"syscall"
)

func getSysProcAttr(cmdLine string, tty bool) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func openPty() (*os.File, *os.File, error) {
	return nil, nil, errors.New("Unsupported")
}
