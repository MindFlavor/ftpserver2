// Package azureFS implements fs.FileProvider
// and handles Azure blob storage
package azureFS

//"github.com/Azure/azure-sdk-for-go/storage"
import (
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/storage"
	log "github.com/sirupsen/logrus"
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
	toks := splitAndCleanPath(pfs.currentRealDirectory)
	var lbParams storage.ListBlobsParameters
	if len(toks) == 1 {
		lbParams = storage.ListBlobsParameters{MaxResults: 1000, Delimiter: "/"}
	} else {
		lbParams = storage.ListBlobsParameters{MaxResults: 1000, Prefix: strings.Join(toks[1:], "/") + "/", Delimiter: "/"}
	}
	lbr, err := pfs.client.ListBlobs(toks[0], lbParams)
	if err != nil {
		return nil, err
	}

	blobs := make([]fs.File, len(lbr.Blobs))

	for i, item := range lbr.Blobs {
		toks := splitAndCleanPath(item.Name)
		blobs[i] = azureBlob.New(toks[len(toks)-1], pfs.currentRealDirectory, item.Properties.ContentLength, parseAzureTime(item.Properties.LastModified), 0666, pfs.client)
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
	props, err := pfs.client.GetBlobProperties(toks[0], strings.Join(toks[1:], "/"))
	if err != nil {
		return nil, err
	}
	return azureBlob.New(strings.Join(toks[1:], "/"), toks[0], props.ContentLength, parseAzureTime(props.LastModified), 0666, pfs.client), nil
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

	return azureBlob.New(strings.Join(toks[1:], "/"), toks[0], 0, time.Now(), 0666, pfs.client), nil
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

	// Container
	if len(toks) == 1 {
		exists, err := pfs.client.ContainerExists(toks[0])
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("cannot change directory: container not found")
		}
		pfs.currentRealDirectory = toks[0]
		log.WithFields(log.Fields{"pfs": pfs, "path": path, "len(toks)": len(toks), "toks": toks}).Debug("azureFS::azureFS::ChangeDirectory changed to container")
		return nil
	}

	// Then, Blob
	exists, err := pfs.client.BlobExists(toks[0], strings.Join(toks[1:], "/"))
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("cannot change directory: blob not found")
	}
	pfs.currentRealDirectory = strings.Join(toks, "/")

	log.WithFields(log.Fields{"pfs": pfs, "path": path, "len(toks)": len(toks), "toks": toks}).Debug("azureFS::azureFS::ChangeDirectory changed to blob")

	return nil
}

func (pfs *azureFS) CreateDirectory(path string) error {
	fullpath := path
	if fullpath[0] != '/' {
		fullpath = "/" + pfs.currentRealDirectory + "/" + path
	}

	log.WithFields(log.Fields{"pfs": pfs, "path": path, "fullpath": fullpath}).Debug("azureFS::azureFS::CreateDirectory called")

	toks := splitAndCleanPath(fullpath)

	if _, err := pfs.client.CreateContainerIfNotExists(toks[0], storage.ContainerAccessTypePrivate); err != nil {
		return err
	}

	// Container only
	if len(toks) == 1 {
		return nil
	}

	// Blob
	return pfs.client.CreateBlockBlob(toks[0], strings.Join(toks[1:], "/"))
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
