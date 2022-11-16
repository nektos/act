package container

import (
	"errors"
	"os"
	"syscall"
)

func getSysProcAttr(cmdLine string, tty bool) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CmdLine: cmdLine, CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func openPty() (*os.File, *os.File, error) {
	return nil, nil, errors.New("Unsupported")
}
