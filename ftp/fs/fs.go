// Package fs exposes the required
// interface needed by the FTP Server in order
// to use the fs as backend file system
package fs

import (
	"io"
	"time"

	"github.com/mindflavor/ftpserver2/identity"
)

// File is the minimum interface needed to
// be supported as file (or directory)
type File interface {
	Name() string
	Path() string
	FullPath() string
	Size() int64
	IsDirectory() bool
	ModTime() time.Time

	Read(startPosition int64) (io.ReadCloser, error)
	Write() (io.WriteCloser, error)

	Delete() error

	Clone() File

	Mode() string
}

// FileProvider represents the
// file system handle. It should
// store the current directory
// to allow fs traversing with relative paths
type FileProvider interface {
	Identity() identity.Identity
	SetIdentity(identity identity.Identity)
	Clone() FileProvider
	New(name string, isDirectory bool) (File, error)
	Get(filename string) (File, error)
	List() ([]File, error)
	CurrentDirectory() string
	ChangeDirectory(path string) error
	CreateDirectory(name string) error
	RemoveDirectory(name string) error
}
