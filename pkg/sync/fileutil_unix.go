//go:build !windows

package sync

import (
	"os"
	"syscall"
)

// Unix implementation using flock

func lockFileUnix(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

func tryLockFileUnix(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func unlockFileUnix(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

func isLockBusyUnix(err error) bool {
	return err == syscall.EWOULDBLOCK || err == syscall.EAGAIN
}

func lockFile(f *os.File) error    { return lockFileUnix(f) }
func tryLockFile(f *os.File) error { return tryLockFileUnix(f) }
func unlockFile(f *os.File) error  { return unlockFileUnix(f) }
func isLockBusy(err error) bool    { return isLockBusyUnix(err) }
