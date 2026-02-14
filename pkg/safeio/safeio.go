package safeio

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

// DefaultMaxReadSize is the default max file size read.
const DefaultMaxReadSize = 1 << 20 // 1MB

// DefaultBackupCount is the default number of backups to keep.
const DefaultBackupCount = 3

// BackupSuffix is the suffix used for backup files.
const BackupSuffix = ".bak"

// Reader provides safe file reading with size limits.
type Reader struct {
	MaxSize int64
}

// NewReader creates a new Reader with the provided max size.
// If maxSize <= 0, DefaultMaxReadSize is used.
func NewReader(maxSize int64) *Reader {
	if maxSize <= 0 {
		maxSize = DefaultMaxReadSize
	}
	return &Reader{MaxSize: maxSize}
}

// ReadFile reads a file with a size check to avoid memory exhaustion.
func (r *Reader) ReadFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > r.MaxSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), r.MaxSize)
	}
	return os.ReadFile(path)
}

// DefaultReader is the package-level default safe reader.
var DefaultReader = NewReader(DefaultMaxReadSize)

// SafeReadFile reads a file using DefaultReader.
func SafeReadFile(path string) ([]byte, error) {
	return DefaultReader.ReadFile(path)
}

// AtomicWriteFile writes data atomically using a temp file + rename.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing to temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}

	success = true
	return nil
}

// FileLock represents a file-based lock for synchronization.
type FileLock struct {
	path string
	file *os.File
}

// NewFileLock creates a lock at path + ".lock".
func NewFileLock(path string) *FileLock {
	return &FileLock{path: path + ".lock"}
}

// Lock acquires the lock.
func (l *FileLock) Lock() error {
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}
	if err := lockFile(f); err != nil {
		_ = f.Close()
		return fmt.Errorf("acquiring lock: %w", err)
	}

	l.file = f
	return nil
}

// Unlock releases the lock.
func (l *FileLock) Unlock() error {
	if l.file == nil {
		return nil
	}
	if err := unlockFile(l.file); err != nil {
		_ = l.file.Close()
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

func lockFile(f *os.File) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

func unlockFile(f *os.File) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// CreateBackup creates a timestamped backup of path.
// Returns empty backup path if source does not exist.
func CreateBackup(path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("checking source file: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405.000000000")
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	backupPath := fmt.Sprintf("%s%s.%s%s", base, BackupSuffix, timestamp, ext)

	if err := copyFile(path, backupPath); err != nil {
		return "", fmt.Errorf("creating backup: %w", err)
	}
	return backupPath, nil
}

// RotateBackups keeps only the newest keepCount timestamped backups.
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

	var backups []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ext) {
			backups = append(backups, filepath.Join(dir, name))
		}
	}

	sort.Strings(backups)
	if len(backups) > keepCount {
		for _, backup := range backups[:len(backups)-keepCount] {
			if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing old backup %s: %w", backup, err)
			}
		}
	}
	return nil
}

// SafeWriteFile applies backup + atomic write + rotation.
func SafeWriteFile(path string, data []byte, perm os.FileMode, keepBackups int) error {
	if _, err := CreateBackup(path); err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}
	if err := AtomicWriteFile(path, data, perm); err != nil {
		return fmt.Errorf("atomic write: %w", err)
	}
	if err := RotateBackups(path, keepBackups); err != nil {
		// Non-fatal: primary write succeeded.
		return nil
	}
	return nil
}

// SafeWriteFileWithLock applies lock + SafeWriteFile.
func SafeWriteFileWithLock(path string, data []byte, perm os.FileMode, keepBackups int) error {
	lock := NewFileLock(path)
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() { _ = lock.Unlock() }()
	return SafeWriteFile(path, data, perm, keepBackups)
}

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
