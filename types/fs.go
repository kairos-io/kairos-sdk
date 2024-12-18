package types

import (
	"io/fs"
	"os"
	"time"
)

// KairosFS is our interface for methods that need an FS
type KairosFS interface {
	Open(name string) (fs.File, error)
	Chmod(name string, mode os.FileMode) error
	Create(name string) (*os.File, error)
	Mkdir(name string, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
	Lstat(name string) (os.FileInfo, error)
	RemoveAll(path string) error
	ReadFile(filename string) ([]byte, error)
	Readlink(name string) (string, error)
	RawPath(name string) (string, error)
	ReadDir(dirname string) ([]fs.DirEntry, error)
	Remove(name string) error
	OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error
	Rename(oldpath, newpath string) error
	Truncate(name string, size int64) error
	Chown(name string, uid, git int) error
	Chtimes(name string, atime, mtime time.Time) error
	Glob(pattern string) ([]string, error)
	Lchown(name string, uid, git int) error
	Link(oldname, newname string) error
	PathSeparator() rune
	Symlink(oldname, newname string) error
}
