// Package azureBlob implements the fs interfaces
// for the local file system
package azureBlob

import (
	"fmt"
	"io"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/mindflavor/ftpserver2/ftp/fs"
)

type azureBlob struct {
	name    string
	path    string
	size    int64
	modTime time.Time
	mode    os.FileMode
	client  storage.BlobStorageClient
}

// New initializes a new fs.File with the
// specified parameters.
func New(name string, path string, size int64, modTime time.Time, mode os.FileMode, client storage.BlobStorageClient) fs.File {
	log.WithFields(log.Fields{"name": name, "path": path, "size": size, "modTime": modTime, "mode": mode}).Debug("azureBlob::New called")

	return &azureBlob{
		name:    name,
		path:    path,
		size:    size,
		modTime: modTime,
		mode:    mode,
		client:  client,
	}
}

func (b *azureBlob) String() string {
	return fmt.Sprintf("{name=%s, path=%s, size=%d, mode=%s, modTime=%s}", b.name, b.path, b.size, b.mode, b.modTime)
}

func (b *azureBlob) Name() string {
	return b.name
}

func (b *azureBlob) Path() string {
	return b.path
}

func (b *azureBlob) FullPath() string {
	return b.path + "/" + b.name
}

func (b *azureBlob) Size() int64 {
	return b.size
}

func (b *azureBlob) IsDirectory() bool {
	return false
}

func (b *azureBlob) ModTime() time.Time {
	return b.modTime
}

func (b *azureBlob) Mode() string {
	return b.mode.String()
}

func (b *azureBlob) Read(startPosition int64) (io.ReadCloser, error) {
	log.WithFields(log.Fields{"b": b, "startPosition": startPosition}).Debug("azureBlob::azureBlob::Read called")
	return b.client.GetBlob(b.path, b.name)
}

func (b *azureBlob) Write() (io.WriteCloser, error) {
	log.WithFields(log.Fields{"b": b}).Debug("azureBlob::azureBlob::Write called")
	return NewBlockBlobWriter(b)
}

func (b *azureBlob) Clone() fs.File {
	return &azureBlob{
		name:    b.name,
		path:    b.path,
		size:    b.size,
		modTime: b.modTime,
		mode:    b.mode,
		client:  b.client,
	}
}

func (b azureBlob) Delete() error {
	return b.client.DeleteBlob(b.path, b.name, nil)
}
