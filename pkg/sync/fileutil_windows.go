//go:build windows

package sync

import (
	"os"

	"golang.org/x/sys/windows"
)

// Windows implementation using LockFileEx (replaces the previous no-op
// stubs, which provided no actual cross-process locking).

func lockFileWindows(f *os.File) error {
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK,
		0, 1, 0, new(windows.Overlapped),
	)
}

func tryLockFileWindows(f *os.File) error {
	return windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, new(windows.Overlapped),
	)
}

func unlockFileWindows(f *os.File) error {
	return windows.UnlockFileEx(
		windows.Handle(f.Fd()),
		0, 1, 0, new(windows.Overlapped),
	)
}

func isLockBusyWindows(err error) bool {
	return err == windows.ERROR_LOCK_VIOLATION || err == windows.ERROR_SHARING_VIOLATION
}

func lockFile(f *os.File) error    { return lockFileWindows(f) }
func tryLockFile(f *os.File) error { return tryLockFileWindows(f) }
func unlockFile(f *os.File) error  { return unlockFileWindows(f) }
func isLockBusy(err error) bool    { return isLockBusyWindows(err) }
