package container

import (
	"errors"
	"os"
	"syscall"
)

func getSysProcAttr(cmdLine string, tty bool) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Rfork: syscall.RFNOTEG,
	}
}

func openPty() (*os.File, *os.File, error) {
	return nil, nil, errors.New("Unsupported")
}
