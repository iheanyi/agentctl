//go:build windows

package cli

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// daemonSysProcAttr detaches the daemon process on Windows.
func daemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
}
