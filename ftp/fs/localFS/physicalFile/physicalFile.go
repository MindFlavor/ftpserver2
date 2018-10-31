// Package physicalFile implements the fs interfaces
// for the local file system
package physicalFile

import (
	"io"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mindflavor/ftpserver2/ftp/fs"
)

type physicalFile struct {
	name        string
	path        string
	isDirectory bool
	size        int64
	modTime     time.Time
	mode        os.FileMode
}

// New initializes a new fs.File with the
// specified parameters.
func New(name string, path string, isDirectory bool, size int64, modTime time.Time, mode os.FileMode) fs.File {
	return &physicalFile{
		name:        name,
		path:        path,
		isDirectory: isDirectory,
		size:        size,
		modTime:     modTime,
		mode:        mode,
	}
}

func (p physicalFile) Name() string {
	return p.name
}

func (p physicalFile) Path() string {
	return p.path
}

func (p physicalFile) FullPath() string {
	return filepath.Join(p.path, p.name)
}

func (p physicalFile) Size() int64 {
	return p.size
}

func (p physicalFile) IsDirectory() bool {
	return p.isDirectory
}

func (p physicalFile) ModTime() time.Time {
	return p.modTime
}

func (p physicalFile) Mode() string {
	return p.mode.String()
}

func (p physicalFile) Read(startPosition int64) (io.ReadCloser, error) {
	log.WithFields(log.Fields{"p": p}).Debug("localFS::physicalFile::Get called")

	f, err := os.Open(p.FullPath())
	if err != nil {
		return nil, err
	}

	if startPosition != 0 {
		_, err := f.Seek(startPosition, 0)
		if err != nil {
			f.Close()
			return nil, err
		}
	}

	return f, nil
}

func (p physicalFile) Write() (io.WriteCloser, error) {
	log.WithFields(log.Fields{}).Debug("localFS::physicalFile::Write called")

	return os.Create(p.FullPath())
}

func (p physicalFile) Clone() fs.File {
	return &physicalFile{
		name:        p.name,
		path:        p.path,
		isDirectory: p.isDirectory,
		size:        p.size,
		modTime:     p.modTime,
	}
}

func (p physicalFile) Delete() error {
	return os.Remove(p.FullPath())
}
