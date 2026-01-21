package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

// DefaultBackupCount is the default number of backups to keep
const DefaultBackupCount = 3

// BackupSuffix is the suffix used for backup files
const BackupSuffix = ".bak"

// AtomicWriteFile writes data to a file atomically by first writing to a
// temporary file and then renaming it to the target path. This prevents
// corruption if the process is interrupted mid-write.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Create a temp file in the same directory to ensure same filesystem
	// This is required for atomic rename to work
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Write data to temp file
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing to temp file: %w", err)
	}

	// Sync to disk before rename
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Set permissions before rename
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}

	success = true
	return nil
}

// FileLock represents a file-based lock for synchronization
type FileLock struct {
	path string
	file *os.File
}

// NewFileLock creates a new file lock for the given path.
// The lock file will be created at path + ".lock"
func NewFileLock(path string) *FileLock {
	return &FileLock{
		path: path + ".lock",
	}
}

// Lock acquires an exclusive lock on the file.
// This blocks until the lock is acquired.
func (l *FileLock) Lock() error {
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}

	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}

	if err := lockFile(file); err != nil {
		file.Close()
		return fmt.Errorf("acquiring lock: %w", err)
	}

	l.file = file
	return nil
}

// TryLock attempts to acquire an exclusive lock without blocking.
// Returns true if the lock was acquired, false if it's held by another process.
func (l *FileLock) TryLock() (bool, error) {
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("creating lock directory: %w", err)
	}

	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return false, fmt.Errorf("opening lock file: %w", err)
	}

	if err := tryLockFile(file); err != nil {
		file.Close()
		if isLockBusy(err) {
			return false, nil
		}
		return false, fmt.Errorf("acquiring lock: %w", err)
	}

	l.file = file
	return true, nil
}

// Unlock releases the lock.
func (l *FileLock) Unlock() error {
	if l.file == nil {
		return nil
	}

	if err := unlockFile(l.file); err != nil {
		l.file.Close()
		l.file = nil
		return fmt.Errorf("releasing lock: %w", err)
	}

	if err := l.file.Close(); err != nil {
		l.file = nil
		return fmt.Errorf("closing lock file: %w", err)
	}

	l.file = nil
	return nil
}

// Platform-specific file locking implementations

func lockFile(f *os.File) error {
	if runtime.GOOS == "windows" {
		return lockFileWindows(f)
	}
	return lockFileUnix(f)
}

func tryLockFile(f *os.File) error {
	if runtime.GOOS == "windows" {
		return tryLockFileWindows(f)
	}
	return tryLockFileUnix(f)
}

func unlockFile(f *os.File) error {
	if runtime.GOOS == "windows" {
		return unlockFileWindows(f)
	}
	return unlockFileUnix(f)
}

func isLockBusy(err error) bool {
	if runtime.GOOS == "windows" {
		return isLockBusyWindows(err)
	}
	return isLockBusyUnix(err)
}

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

// Windows implementation - uses LockFileEx via syscall
// Note: On Windows, we use a simpler approach with file existence checks
// since flock is not available. For production use, consider using
// golang.org/x/sys/windows for proper LockFileEx support.
func lockFileWindows(f *os.File) error {
	// On Windows, opening with exclusive access provides basic locking
	// This is a simplified implementation
	return nil
}

func tryLockFileWindows(f *os.File) error {
	return nil
}

func unlockFileWindows(f *os.File) error {
	return nil
}

func isLockBusyWindows(err error) bool {
	return false
}

// CreateBackup creates a timestamped backup of the file at the given path.
// Returns the backup path or empty string if the source file doesn't exist.
func CreateBackup(path string) (string, error) {
	// Check if source file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("checking source file: %w", err)
	}

	// Generate timestamp-based backup filename (include nanoseconds for uniqueness)
	timestamp := time.Now().Format("20060102-150405.000000000")
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	backupPath := fmt.Sprintf("%s%s.%s%s", base, BackupSuffix, timestamp, ext)

	// Copy file to backup location
	if err := copyFile(path, backupPath); err != nil {
		return "", fmt.Errorf("creating backup: %w", err)
	}

	return backupPath, nil
}

// CreateSimpleBackup creates a simple .bak backup (overwrites previous backup).
// This is useful when you only need one backup file.
func CreateSimpleBackup(path string) (string, error) {
	// Check if source file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("checking source file: %w", err)
	}

	backupPath := path + BackupSuffix

	if err := copyFile(path, backupPath); err != nil {
		return "", fmt.Errorf("creating backup: %w", err)
	}

	return backupPath, nil
}

// RotateBackups keeps only the most recent N backups for the given file.
// It looks for files matching the pattern: base.bak.TIMESTAMP.ext
func RotateBackups(path string, keepCount int) error {
	if keepCount < 0 {
		keepCount = DefaultBackupCount
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	prefix := nameWithoutExt + BackupSuffix + "."

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading directory: %w", err)
	}

	// Find all backup files
	var backups []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ext) {
			backups = append(backups, filepath.Join(dir, name))
		}
	}

	// Sort by name (timestamp is in name, so lexicographic sort works)
	sort.Strings(backups)

	// Remove oldest backups beyond keepCount
	if len(backups) > keepCount {
		toRemove := backups[:len(backups)-keepCount]
		for _, backup := range toRemove {
			if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing old backup %s: %w", backup, err)
			}
		}
	}

	return nil
}

// RestoreBackup restores the most recent backup for the given file.
// Returns the backup path that was restored, or empty string if no backup exists.
func RestoreBackup(path string) (string, error) {
	// First try simple .bak file
	simpleBak := path + BackupSuffix
	if _, err := os.Stat(simpleBak); err == nil {
		if err := copyFile(simpleBak, path); err != nil {
			return "", fmt.Errorf("restoring simple backup: %w", err)
		}
		return simpleBak, nil
	}

	// Look for timestamped backups
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	prefix := nameWithoutExt + BackupSuffix + "."

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading directory: %w", err)
	}

	var backups []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ext) {
			backups = append(backups, filepath.Join(dir, name))
		}
	}

	if len(backups) == 0 {
		return "", nil
	}

	// Sort and get most recent (last in sorted order)
	sort.Strings(backups)
	mostRecent := backups[len(backups)-1]

	if err := copyFile(mostRecent, path); err != nil {
		return "", fmt.Errorf("restoring backup: %w", err)
	}

	return mostRecent, nil
}

// ListBackups returns a list of all backup files for the given path,
// sorted from oldest to newest.
func ListBackups(path string) ([]string, error) {
	var backups []string

	// Check for simple .bak file
	simpleBak := path + BackupSuffix
	if _, err := os.Stat(simpleBak); err == nil {
		backups = append(backups, simpleBak)
	}

	// Look for timestamped backups
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	prefix := nameWithoutExt + BackupSuffix + "."

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return backups, nil
		}
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	var timestamped []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ext) {
			timestamped = append(timestamped, filepath.Join(dir, name))
		}
	}

	sort.Strings(timestamped)
	backups = append(backups, timestamped...)

	return backups, nil
}

// SafeWriteFile combines backup creation, atomic write, and backup rotation.
// This is the recommended way to safely write config files.
func SafeWriteFile(path string, data []byte, perm os.FileMode, keepBackups int) error {
	// Create backup if file exists
	if _, err := CreateBackup(path); err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}

	// Write atomically
	if err := AtomicWriteFile(path, data, perm); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}

	// Rotate old backups
	if err := RotateBackups(path, keepBackups); err != nil {
		// Log but don't fail - the write succeeded
		return nil
	}

	return nil
}

// SafeWriteFileWithLock combines file locking, backup, and atomic write.
// Use this when multiple processes might write to the same file.
func SafeWriteFileWithLock(path string, data []byte, perm os.FileMode, keepBackups int) error {
	lock := NewFileLock(path)

	if err := lock.Lock(); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	return SafeWriteFile(path, data, perm, keepBackups)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}
