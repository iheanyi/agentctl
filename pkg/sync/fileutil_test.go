package sync

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAtomicWriteFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "atomic-write-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("writes file successfully", func(t *testing.T) {
		path := filepath.Join(tmpDir, "test.json")
		data := []byte(`{"key": "value"}`)

		err := AtomicWriteFile(path, data, 0644)
		if err != nil {
			t.Fatalf("AtomicWriteFile failed: %v", err)
		}

		// Verify content
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if string(content) != string(data) {
			t.Errorf("Content mismatch: got %q, want %q", string(content), string(data))
		}

		// Verify permissions
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}
		if info.Mode().Perm() != 0644 {
			t.Errorf("Permissions mismatch: got %o, want %o", info.Mode().Perm(), 0644)
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		path := filepath.Join(tmpDir, "nested", "dir", "test.json")
		data := []byte(`{"nested": true}`)

		err := AtomicWriteFile(path, data, 0644)
		if err != nil {
			t.Fatalf("AtomicWriteFile failed: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if string(content) != string(data) {
			t.Errorf("Content mismatch: got %q, want %q", string(content), string(data))
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "overwrite.json")

		// Write initial content
		if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
			t.Fatalf("Failed to write initial file: %v", err)
		}

		// Overwrite with atomic write
		newData := []byte("new content")
		if err := AtomicWriteFile(path, newData, 0644); err != nil {
			t.Fatalf("AtomicWriteFile failed: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if string(content) != string(newData) {
			t.Errorf("Content mismatch: got %q, want %q", string(content), string(newData))
		}
	})

	t.Run("no temp files left on success", func(t *testing.T) {
		path := filepath.Join(tmpDir, "noleftover.json")
		data := []byte(`{"clean": true}`)

		if err := AtomicWriteFile(path, data, 0644); err != nil {
			t.Fatalf("AtomicWriteFile failed: %v", err)
		}

		// Check for leftover temp files
		entries, err := os.ReadDir(tmpDir)
		if err != nil {
			t.Fatalf("Failed to read dir: %v", err)
		}

		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".tmp-") {
				t.Errorf("Leftover temp file found: %s", entry.Name())
			}
		}
	})
}

func TestFileLock(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "filelock-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("lock and unlock", func(t *testing.T) {
		path := filepath.Join(tmpDir, "test.json")
		lock := NewFileLock(path)

		if err := lock.Lock(); err != nil {
			t.Fatalf("Lock failed: %v", err)
		}

		// Verify lock file was created
		lockPath := path + ".lock"
		if _, err := os.Stat(lockPath); os.IsNotExist(err) {
			t.Error("Lock file should exist")
		}

		if err := lock.Unlock(); err != nil {
			t.Fatalf("Unlock failed: %v", err)
		}
	})

	t.Run("trylock returns false when locked", func(t *testing.T) {
		path := filepath.Join(tmpDir, "trylock.json")
		lock1 := NewFileLock(path)
		lock2 := NewFileLock(path)

		// Acquire first lock
		if err := lock1.Lock(); err != nil {
			t.Fatalf("Lock1 failed: %v", err)
		}
		defer lock1.Unlock()

		// Try to acquire second lock
		acquired, err := lock2.TryLock()
		if err != nil {
			t.Fatalf("TryLock error: %v", err)
		}
		if acquired {
			lock2.Unlock()
			t.Error("TryLock should return false when lock is held")
		}
	})

	t.Run("trylock returns true when not locked", func(t *testing.T) {
		path := filepath.Join(tmpDir, "trylock-free.json")
		lock := NewFileLock(path)

		acquired, err := lock.TryLock()
		if err != nil {
			t.Fatalf("TryLock error: %v", err)
		}
		if !acquired {
			t.Error("TryLock should return true when lock is free")
		}
		lock.Unlock()
	})

	t.Run("concurrent access protection", func(t *testing.T) {
		// Skip in CI - flock behavior can be unreliable in containerized environments
		// with certain filesystems (overlay, tmpfs, etc.)
		if os.Getenv("CI") != "" {
			t.Skip("Skipping concurrent lock test in CI - flock unreliable in containers")
		}

		path := filepath.Join(tmpDir, "concurrent.json")
		counter := 0
		iterations := 10

		var wg sync.WaitGroup
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				lock := NewFileLock(path)
				if err := lock.Lock(); err != nil {
					t.Errorf("Lock failed: %v", err)
					return
				}
				defer lock.Unlock()

				// Simulate work with the protected resource
				val := counter
				time.Sleep(time.Millisecond)
				counter = val + 1
			}()
		}

		wg.Wait()

		if counter != iterations {
			t.Errorf("Counter should be %d, got %d (race condition detected)", iterations, counter)
		}
	})

	t.Run("unlock when not locked is safe", func(t *testing.T) {
		path := filepath.Join(tmpDir, "never-locked.json")
		lock := NewFileLock(path)

		// Should not panic or error
		if err := lock.Unlock(); err != nil {
			t.Errorf("Unlock on non-locked should not error: %v", err)
		}
	})
}

func TestCreateBackup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("creates timestamped backup", func(t *testing.T) {
		path := filepath.Join(tmpDir, "config.json")
		originalContent := []byte(`{"version": 1}`)

		if err := os.WriteFile(path, originalContent, 0644); err != nil {
			t.Fatalf("Failed to write original: %v", err)
		}

		backupPath, err := CreateBackup(path)
		if err != nil {
			t.Fatalf("CreateBackup failed: %v", err)
		}

		if backupPath == "" {
			t.Fatal("Backup path should not be empty")
		}

		// Verify backup exists
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("Backup file should exist")
		}

		// Verify backup content
		backupContent, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("Failed to read backup: %v", err)
		}
		if string(backupContent) != string(originalContent) {
			t.Errorf("Backup content mismatch: got %q, want %q", string(backupContent), string(originalContent))
		}

		// Verify backup filename format
		if !strings.Contains(backupPath, BackupSuffix) {
			t.Errorf("Backup path should contain %s: %s", BackupSuffix, backupPath)
		}
	})

	t.Run("returns empty for non-existent file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "nonexistent.json")

		backupPath, err := CreateBackup(path)
		if err != nil {
			t.Fatalf("CreateBackup should not error for non-existent file: %v", err)
		}

		if backupPath != "" {
			t.Errorf("Backup path should be empty for non-existent file: %s", backupPath)
		}
	})
}

func TestCreateSimpleBackup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "simple-backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("creates simple .bak backup", func(t *testing.T) {
		path := filepath.Join(tmpDir, "config.json")
		originalContent := []byte(`{"simple": true}`)

		if err := os.WriteFile(path, originalContent, 0644); err != nil {
			t.Fatalf("Failed to write original: %v", err)
		}

		backupPath, err := CreateSimpleBackup(path)
		if err != nil {
			t.Fatalf("CreateSimpleBackup failed: %v", err)
		}

		expectedPath := path + BackupSuffix
		if backupPath != expectedPath {
			t.Errorf("Backup path mismatch: got %q, want %q", backupPath, expectedPath)
		}

		backupContent, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("Failed to read backup: %v", err)
		}
		if string(backupContent) != string(originalContent) {
			t.Errorf("Backup content mismatch")
		}
	})

	t.Run("overwrites previous backup", func(t *testing.T) {
		path := filepath.Join(tmpDir, "overwrite.json")

		// Create first version
		if err := os.WriteFile(path, []byte("v1"), 0644); err != nil {
			t.Fatalf("Failed to write v1: %v", err)
		}
		if _, err := CreateSimpleBackup(path); err != nil {
			t.Fatalf("First backup failed: %v", err)
		}

		// Create second version
		if err := os.WriteFile(path, []byte("v2"), 0644); err != nil {
			t.Fatalf("Failed to write v2: %v", err)
		}
		backupPath, err := CreateSimpleBackup(path)
		if err != nil {
			t.Fatalf("Second backup failed: %v", err)
		}

		// Backup should contain v2
		backupContent, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("Failed to read backup: %v", err)
		}
		if string(backupContent) != "v2" {
			t.Errorf("Backup should contain v2, got %q", string(backupContent))
		}
	})
}

func TestRotateBackups(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rotate-backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("removes old backups", func(t *testing.T) {
		path := filepath.Join(tmpDir, "rotate.json")
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(path, ext)

		// Create 5 backup files with different timestamps
		timestamps := []string{"20240101-120000", "20240101-120001", "20240101-120002", "20240101-120003", "20240101-120004"}
		for _, ts := range timestamps {
			backupPath := base + BackupSuffix + "." + ts + ext
			if err := os.WriteFile(backupPath, []byte(ts), 0644); err != nil {
				t.Fatalf("Failed to create backup %s: %v", ts, err)
			}
		}

		// Rotate to keep only 2
		if err := RotateBackups(path, 2); err != nil {
			t.Fatalf("RotateBackups failed: %v", err)
		}

		// List remaining backups
		backups, err := ListBackups(path)
		if err != nil {
			t.Fatalf("ListBackups failed: %v", err)
		}

		if len(backups) != 2 {
			t.Errorf("Should have 2 backups, got %d: %v", len(backups), backups)
		}

		// Verify oldest were removed
		for _, backup := range backups {
			if strings.Contains(backup, "120000") || strings.Contains(backup, "120001") || strings.Contains(backup, "120002") {
				t.Errorf("Old backup should have been removed: %s", backup)
			}
		}
	})

	t.Run("handles fewer backups than keep count", func(t *testing.T) {
		path := filepath.Join(tmpDir, "few.json")

		// Create only 2 backups
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}

		// Create 2 timestamped backups
		for i := 0; i < 2; i++ {
			if _, err := CreateBackup(path); err != nil {
				t.Fatalf("CreateBackup failed: %v", err)
			}
			time.Sleep(time.Millisecond * 10) // Ensure different timestamps
		}

		// Rotate with keep count of 5 (more than we have)
		if err := RotateBackups(path, 5); err != nil {
			t.Fatalf("RotateBackups failed: %v", err)
		}

		backups, err := ListBackups(path)
		if err != nil {
			t.Fatalf("ListBackups failed: %v", err)
		}

		if len(backups) != 2 {
			t.Errorf("Should still have 2 backups, got %d", len(backups))
		}
	})

	t.Run("handles non-existent directory", func(t *testing.T) {
		path := filepath.Join(tmpDir, "nonexistent-dir", "file.json")

		// Should not error
		if err := RotateBackups(path, 3); err != nil {
			t.Errorf("RotateBackups should not error for non-existent dir: %v", err)
		}
	})
}

func TestRestoreBackup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restore-backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("restores simple backup", func(t *testing.T) {
		path := filepath.Join(tmpDir, "restore-simple.json")
		originalContent := []byte(`{"original": true}`)
		modifiedContent := []byte(`{"modified": true}`)

		// Create original and backup
		if err := os.WriteFile(path, originalContent, 0644); err != nil {
			t.Fatalf("Failed to write original: %v", err)
		}
		if _, err := CreateSimpleBackup(path); err != nil {
			t.Fatalf("CreateSimpleBackup failed: %v", err)
		}

		// Modify original
		if err := os.WriteFile(path, modifiedContent, 0644); err != nil {
			t.Fatalf("Failed to write modified: %v", err)
		}

		// Restore
		restoredFrom, err := RestoreBackup(path)
		if err != nil {
			t.Fatalf("RestoreBackup failed: %v", err)
		}

		if restoredFrom == "" {
			t.Error("restoredFrom should not be empty")
		}

		// Verify restored content
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read restored file: %v", err)
		}
		if string(content) != string(originalContent) {
			t.Errorf("Restored content mismatch: got %q, want %q", string(content), string(originalContent))
		}
	})

	t.Run("restores most recent timestamped backup", func(t *testing.T) {
		path := filepath.Join(tmpDir, "restore-timestamped.json")

		// Create file and multiple backups
		if err := os.WriteFile(path, []byte("v1"), 0644); err != nil {
			t.Fatalf("Failed to write v1: %v", err)
		}
		if _, err := CreateBackup(path); err != nil {
			t.Fatalf("First backup failed: %v", err)
		}

		time.Sleep(time.Millisecond * 10)

		if err := os.WriteFile(path, []byte("v2"), 0644); err != nil {
			t.Fatalf("Failed to write v2: %v", err)
		}
		if _, err := CreateBackup(path); err != nil {
			t.Fatalf("Second backup failed: %v", err)
		}

		// Modify to v3
		if err := os.WriteFile(path, []byte("v3"), 0644); err != nil {
			t.Fatalf("Failed to write v3: %v", err)
		}

		// Restore should get v2 (most recent backup)
		_, err := RestoreBackup(path)
		if err != nil {
			t.Fatalf("RestoreBackup failed: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read restored: %v", err)
		}
		if string(content) != "v2" {
			t.Errorf("Should restore v2, got %q", string(content))
		}
	})

	t.Run("returns empty when no backup exists", func(t *testing.T) {
		path := filepath.Join(tmpDir, "no-backup.json")

		if err := os.WriteFile(path, []byte("no backup"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}

		restoredFrom, err := RestoreBackup(path)
		if err != nil {
			t.Fatalf("RestoreBackup should not error: %v", err)
		}

		if restoredFrom != "" {
			t.Errorf("restoredFrom should be empty, got %q", restoredFrom)
		}
	})
}

func TestListBackups(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "list-backup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("lists all backup types", func(t *testing.T) {
		path := filepath.Join(tmpDir, "list.json")

		// Create original file
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}

		// Create simple backup
		if _, err := CreateSimpleBackup(path); err != nil {
			t.Fatalf("CreateSimpleBackup failed: %v", err)
		}

		// Create timestamped backups
		for i := 0; i < 3; i++ {
			if _, err := CreateBackup(path); err != nil {
				t.Fatalf("CreateBackup %d failed: %v", i, err)
			}
			time.Sleep(time.Millisecond * 10)
		}

		backups, err := ListBackups(path)
		if err != nil {
			t.Fatalf("ListBackups failed: %v", err)
		}

		// Should have simple backup + 3 timestamped
		if len(backups) != 4 {
			t.Errorf("Should have 4 backups, got %d: %v", len(backups), backups)
		}
	})

	t.Run("returns empty for file without backups", func(t *testing.T) {
		path := filepath.Join(tmpDir, "no-backups.json")

		backups, err := ListBackups(path)
		if err != nil {
			t.Fatalf("ListBackups failed: %v", err)
		}

		if len(backups) != 0 {
			t.Errorf("Should have 0 backups, got %d", len(backups))
		}
	})

	t.Run("sorted from oldest to newest", func(t *testing.T) {
		path := filepath.Join(tmpDir, "sorted.json")
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(path, ext)

		// Create backups out of order
		timestamps := []string{"20240103-120000", "20240101-120000", "20240102-120000"}
		for _, ts := range timestamps {
			backupPath := base + BackupSuffix + "." + ts + ext
			if err := os.WriteFile(backupPath, []byte(ts), 0644); err != nil {
				t.Fatalf("Failed to create backup: %v", err)
			}
		}

		backups, err := ListBackups(path)
		if err != nil {
			t.Fatalf("ListBackups failed: %v", err)
		}

		// Verify sorted order
		expectedOrder := []string{"20240101-120000", "20240102-120000", "20240103-120000"}
		for i, backup := range backups {
			if !strings.Contains(backup, expectedOrder[i]) {
				t.Errorf("Backup %d should contain %s, got %s", i, expectedOrder[i], backup)
			}
		}
	})
}

func TestSafeWriteFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "safe-write-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("writes with backup", func(t *testing.T) {
		path := filepath.Join(tmpDir, "safe.json")
		originalContent := []byte(`{"version": 1}`)
		newContent := []byte(`{"version": 2}`)

		// Create original
		if err := os.WriteFile(path, originalContent, 0644); err != nil {
			t.Fatalf("Failed to write original: %v", err)
		}

		// Safe write
		if err := SafeWriteFile(path, newContent, 0644, 3); err != nil {
			t.Fatalf("SafeWriteFile failed: %v", err)
		}

		// Verify new content
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if string(content) != string(newContent) {
			t.Errorf("Content mismatch")
		}

		// Verify backup was created
		backups, err := ListBackups(path)
		if err != nil {
			t.Fatalf("ListBackups failed: %v", err)
		}
		if len(backups) == 0 {
			t.Error("Backup should have been created")
		}
	})

	t.Run("creates new file without error", func(t *testing.T) {
		path := filepath.Join(tmpDir, "new-safe.json")
		content := []byte(`{"new": true}`)

		if err := SafeWriteFile(path, content, 0644, 3); err != nil {
			t.Fatalf("SafeWriteFile failed for new file: %v", err)
		}

		// Verify content
		written, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if string(written) != string(content) {
			t.Errorf("Content mismatch")
		}
	})
}

func TestSafeWriteFileWithLock(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "safe-write-lock-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("concurrent writes are serialized", func(t *testing.T) {
		path := filepath.Join(tmpDir, "concurrent.json")
		iterations := 5

		// Create initial file
		if err := os.WriteFile(path, []byte("0"), 0644); err != nil {
			t.Fatalf("Failed to write initial: %v", err)
		}

		var wg sync.WaitGroup
		for i := 1; i <= iterations; i++ {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				content := []byte(string(rune('0' + val)))
				if err := SafeWriteFileWithLock(path, content, 0644, 3); err != nil {
					t.Errorf("SafeWriteFileWithLock failed: %v", err)
				}
			}(i)
		}

		wg.Wait()

		// File should contain one of the written values
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if len(content) != 1 {
			t.Errorf("Content should be single character, got %q", string(content))
		}
	})
}

func TestCopyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "copy-file-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("copies file content and permissions", func(t *testing.T) {
		src := filepath.Join(tmpDir, "source.txt")
		dst := filepath.Join(tmpDir, "dest.txt")
		content := []byte("test content")

		if err := os.WriteFile(src, content, 0640); err != nil {
			t.Fatalf("Failed to write source: %v", err)
		}

		if err := copyFile(src, dst); err != nil {
			t.Fatalf("copyFile failed: %v", err)
		}

		dstContent, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("Failed to read dest: %v", err)
		}
		if string(dstContent) != string(content) {
			t.Errorf("Content mismatch")
		}

		dstInfo, err := os.Stat(dst)
		if err != nil {
			t.Fatalf("Failed to stat dest: %v", err)
		}
		if dstInfo.Mode().Perm() != 0640 {
			t.Errorf("Permissions mismatch: got %o, want %o", dstInfo.Mode().Perm(), 0640)
		}
	})
}

// Benchmarks

func BenchmarkAtomicWriteFile(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-atomic")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "bench.json")
	data := []byte(`{"benchmark": true, "data": "some test content here"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := AtomicWriteFile(path, data, 0644); err != nil {
			b.Fatalf("AtomicWriteFile failed: %v", err)
		}
	}
}

func BenchmarkSafeWriteFile(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-safe")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "bench.json")
	data := []byte(`{"benchmark": true, "data": "some test content here"}`)

	// Create initial file
	if err := os.WriteFile(path, data, 0644); err != nil {
		b.Fatalf("Failed to write initial: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := SafeWriteFile(path, data, 0644, 3); err != nil {
			b.Fatalf("SafeWriteFile failed: %v", err)
		}
	}
}

func BenchmarkFileLock(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-lock")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "bench.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lock := NewFileLock(path)
		if err := lock.Lock(); err != nil {
			b.Fatalf("Lock failed: %v", err)
		}
		if err := lock.Unlock(); err != nil {
			b.Fatalf("Unlock failed: %v", err)
		}
	}
}
