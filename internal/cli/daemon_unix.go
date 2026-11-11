//go:build !windows

package cli

import "syscall"

// daemonSysProcAttr detaches the daemon from the controlling terminal on Unix.
func daemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
