package types

import (
	"io/fs"
	"os"
)

// KairosFS is our interface for methods that need an FS
type KairosFS interface {
	ReadFile(filename string) ([]byte, error)
	Stat(name string) (fs.FileInfo, error)
	Open(name string) (fs.File, error)
	RawPath(name string) (string, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error
}
