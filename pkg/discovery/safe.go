package discovery

import (
	"fmt"
	"os"
)

// MaxFileSize is the default maximum file size to read (1MB)
const MaxFileSize = 1 << 20 // 1MB

// SafeReader provides safe file reading with size limits to prevent memory exhaustion
type SafeReader struct {
	MaxSize int64
}

// NewSafeReader creates a new SafeReader with the default max size
func NewSafeReader() *SafeReader {
	return &SafeReader{MaxSize: MaxFileSize}
}

// ReadFile reads a file with size checking to prevent memory exhaustion
func (r *SafeReader) ReadFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > r.MaxSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), r.MaxSize)
	}
	return os.ReadFile(path)
}

// DefaultReader is a package-level SafeReader for convenience
var DefaultReader = NewSafeReader()

// SafeReadFile reads a file using the default SafeReader
func SafeReadFile(path string) ([]byte, error) {
	return DefaultReader.ReadFile(path)
}
