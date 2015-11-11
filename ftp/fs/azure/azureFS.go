// Package azureFS implements fs.FileProvider
// and handles Azure blob storage
package azureFS

//"github.com/Azure/azure-sdk-for-go/storage"
import (
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/storage"
	log "github.com/Sirupsen/logrus"
	"github.com/mindflavor/ftpserver2/ftp/fs"
	"github.com/mindflavor/ftpserver2/ftp/fs/azure/azureBlob"
	"github.com/mindflavor/ftpserver2/ftp/fs/azure/azureContainer"
	"github.com/mindflavor/ftpserver2/identity"
)

type azureFS struct {
	id                   identity.Identity
	client               storage.BlobStorageClient
	currentRealDirectory string
}

func (pfs *azureFS) String() string {
	return fmt.Sprintf("id:%s, currentRealDirectory: %s", pfs.id, pfs.currentRealDirectory)
}

// New initializes a new fs.FileProvider with a specific Azure account and key
func New(account, secret string) (fs.FileProvider, error) {
	cli, err := storage.NewClient(account, secret, storage.DefaultBaseURL, storage.DefaultAPIVersion, true)
	if err != nil {
		return nil, err
	}

	return &azureFS{
		id:                   nil,
		client:               cli.GetBlobService(),
		currentRealDirectory: "",
	}, nil
}

func (pfs *azureFS) Identity() identity.Identity {
	return pfs.id
}
func (pfs *azureFS) SetIdentity(identity identity.Identity) {
	pfs.id = identity
}

func (pfs *azureFS) CurrentDirectory() string {
	return "/" + pfs.currentRealDirectory
}

func (pfs *azureFS) List() ([]fs.File, error) {
	if pfs.CurrentDirectory() == "/" {
		// list containers

		// TODO
		// we should check for more than 1000 entries
		lcParams := storage.ListContainersParameters{MaxResults: 1000}
		lbr, err := pfs.client.ListContainers(lcParams)
		if err != nil {
			return nil, err
		}

		cnts := make([]fs.File, len(lbr.Containers))

		for i, item := range lbr.Containers {
			cnts[i] = azureContainer.New(item.Name, parseAzureTime(item.Properties.LastModified), pfs.client)
		}

		return cnts, nil
	}

	// files!
	lbParams := storage.ListBlobsParameters{MaxResults: 1000}
	lbr, err := pfs.client.ListBlobs(pfs.currentRealDirectory, lbParams)
	if err != nil {
		return nil, err
	}

	blobs := make([]fs.File, len(lbr.Blobs))

	for i, item := range lbr.Blobs {
		blobs[i] = azureBlob.New(item.Name, pfs.currentRealDirectory, item.Properties.ContentLength, parseAzureTime(item.Properties.LastModified), 0666, pfs.client)
	}

	return blobs, nil
}

func (pfs *azureFS) Get(filename string) (fs.File, error) {
	fullpath := filename
	if fullpath[0] != '/' {
		fullpath = "/" + pfs.currentRealDirectory + "/" + filename
	}

	toks := splitAndCleanPath(fullpath)
	log.WithFields(log.Fields{"pfs": pfs, "filename": filename, "fullpath": fullpath, "toks": toks}).Debug("azureFS::azureFS::Get called")

	if len(toks) == 0 { // root
		return azureContainer.New("", time.Now(), pfs.client), nil
	}
	if len(toks) == 1 { // containter
		return azureContainer.New(filename, time.Now(), pfs.client), nil
	}

	// else blob
	return azureBlob.New(toks[1], toks[0], 0, time.Now(), 0666, pfs.client), nil
}

func (pfs *azureFS) New(filename string, isDirectory bool) (fs.File, error) {
	fullpath := filename
	if fullpath[0] != '/' {
		fullpath = "/" + pfs.currentRealDirectory + "/" + filename
	}

	log.WithFields(log.Fields{"pfs": pfs, "filename": filename, "fullpath": fullpath, "isDirectory": isDirectory}).Debug("azureFS::azureFS::New called")

	toks := splitAndCleanPath(fullpath)

	if len(toks) == 1 { // container
		return azureContainer.New(filename, time.Now(), pfs.client), nil
	}

	return azureBlob.New(toks[1], toks[0], 0, time.Now(), 0666, pfs.client), nil
}

func (pfs *azureFS) Clone() fs.FileProvider {
	return &azureFS{
		id:                   pfs.id,
		client:               pfs.client,
		currentRealDirectory: pfs.currentRealDirectory,
	}
}

func (pfs *azureFS) ChangeDirectory(path string) error {
	fullpath := path

	if len(path) == 0 {
		pfs.currentRealDirectory = ""
		log.WithFields(log.Fields{"pfs": pfs, "path": path}).Debug("azureFS::azureFS::ChangeDirectory changed to root /")
		return nil
	}

	if fullpath[0] != '/' {
		fullpath = "/" + pfs.currentRealDirectory + "/" + path
	}

	log.WithFields(log.Fields{"pfs": pfs, "path": path, "fullpath": fullpath}).Debug("azureFS::azureFS::ChangeDirectory called")

	toks := splitAndCleanPath(fullpath)

	if len(toks) == 0 {
		pfs.currentRealDirectory = ""
		log.WithFields(log.Fields{"pfs": pfs, "path": path, "len(toks)": len(toks), "toks": toks}).Debug("azureFS::azureFS::ChangeDirectory changed to root /")
		return nil
	}

	if toks[len(toks)-1] == ".." { // strip .. folder
		if len(toks) == 1 { // root
			pfs.currentRealDirectory = ""
			log.WithFields(log.Fields{"pfs": pfs, "path": path, "len(toks)": len(toks), "toks": toks}).Debug("azureFS::azureFS::ChangeDirectory changed to root /")
			return nil
		}
		toks = toks[:len(toks)-2]
	}

	if len(toks) == 1 {
		pfs.currentRealDirectory = toks[0]
		log.WithFields(log.Fields{"pfs": pfs, "path": path, "len(toks)": len(toks), "toks": toks}).Debug("azureFS::azureFS::ChangeDirectory changed to container")
		return nil
	}

	log.WithFields(log.Fields{"pfs": pfs, "path": path, "len(toks)": len(toks), "toks": toks}).Warn("azureFS::azureFS::ChangeDirectory azure storage does not support nested containers")
	return fmt.Errorf("cannot change directory: azure storage supports only root level container")
}

func (pfs *azureFS) CreateDirectory(path string) error {
	fullpath := path
	if fullpath[0] != '/' {
		fullpath = "/" + pfs.currentRealDirectory + "/" + path
	}

	log.WithFields(log.Fields{"pfs": pfs, "path": path, "fullpath": fullpath}).Debug("azureFS::azureFS::CreateDirectory called")

	toks := splitAndCleanPath(fullpath)

	if len(toks) > 1 {
		return fmt.Errorf("cannot nest subdirectories in azure storage")
	}

	return pfs.client.CreateContainer(path, storage.ContainerAccessTypePrivate)

}

func (pfs *azureFS) RemoveDirectory(path string) error {
	fullpath := path
	if fullpath[0] != '/' {
		fullpath = "/" + pfs.currentRealDirectory + "/" + path
	}

	log.WithFields(log.Fields{"pfs": pfs, "path": path, "fullpath": fullpath}).Debug("azureFS::azureFS::RemoveDirectory called")

	toks := splitAndCleanPath(fullpath)

	if len(toks) > 1 {
		return fmt.Errorf("there are no subdirectories in Azure storage")
	}

	return pfs.client.DeleteContainer(path)
}

func parseAzureTime(tToParse string) time.Time {
	log.WithFields(log.Fields{"tToParse": tToParse}).Debug("azureFS::parseAzureTime called")
	t, _ := time.Parse(time.RFC1123, tToParse)
	return t
}

func splitAndCleanPath(s string) []string {
	var toks []string
	for _, item := range strings.Split(s, "/") {
		if item != "" {
			toks = append(toks, item)
		}
	}

	return toks
}
