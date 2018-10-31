// Package azureContainer implements fs.File
// but is specific for Azure containers
package azureContainer

import (
	"fmt"
	"io"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/mindflavor/ftpserver2/ftp/fs"
)

type azureContainer struct {
	name    string
	modTime time.Time
	client  storage.BlobStorageClient
}

// New initializes a new fs.File with the
// specified parameters.
func New(name string, modTime time.Time, client storage.BlobStorageClient) fs.File {
	log.WithFields(log.Fields{"name": name, "modTime": modTime}).Debug("azureContainer::New called")
	return &azureContainer{
		name:    name,
		modTime: modTime,
		client:  client,
	}
}

func (p *azureContainer) Name() string {
	return p.name
}

func (p *azureContainer) Path() string {
	return "/" + p.name
}

func (p *azureContainer) FullPath() string {
	return p.Path()
}

func (p *azureContainer) Size() int64 {
	return 0
}

func (p *azureContainer) IsDirectory() bool {
	return true
}

func (p *azureContainer) ModTime() time.Time {
	return p.modTime
}

func (p *azureContainer) Mode() string {
	return "drwxrwsrwx"
}

func (p *azureContainer) Read(startPosition int64) (io.ReadCloser, error) {
	return nil, fmt.Errorf("azure container is not readable")
}

func (p *azureContainer) Write() (io.WriteCloser, error) {
	return nil, fmt.Errorf("azure container is not writeable")
}

func (p *azureContainer) Clone() fs.File {
	return &azureContainer{
		name:    p.name,
		modTime: p.modTime,
		client:  p.client,
	}
}

func (p *azureContainer) Delete() error {
	// should check if empty first? nah :)
	return p.client.DeleteContainer(p.name)
}
