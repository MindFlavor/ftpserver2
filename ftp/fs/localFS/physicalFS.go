// Package localFS implements the fs interfaces
// for the local file system
package localFS

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/mindflavor/ftpserver2/ftp/fs"
	"github.com/mindflavor/ftpserver2/ftp/fs/localFS/physicalFile"
	"github.com/mindflavor/ftpserver2/identity"
)

type physicalFS struct {
	homeRealDirectory    string
	currentRealDirectory string
	identity             identity.Identity
}

// New initializes a new FileProvider with a specific homepath.
// Homepath is the root of the FS so it will appear as "/"
func New(homepath string) (fs.FileProvider, error) {
	return &physicalFS{
		homeRealDirectory:    homepath,
		currentRealDirectory: homepath,
		identity:             nil,
	}, nil
}

func (pfs *physicalFS) Identity() identity.Identity {
	return pfs.identity
}
func (pfs *physicalFS) SetIdentity(identity identity.Identity) {
	pfs.identity = identity
}

func (pfs *physicalFS) CurrentDirectory() string {
	// remove (hide) homeDirectory from path
	home := strings.Split(pfs.homeRealDirectory, string(os.PathSeparator))
	lHome := len(home)

	remote := strings.Split(pfs.currentRealDirectory, string(os.PathSeparator))
	lRemote := len(remote)

	rel := remote[lHome:]

	log.WithFields(log.Fields{"home": home, "lHome": lHome, "physicalFS": pfs, "remote": remote, "lRemote": lRemote, "rel": rel}).Debug("localFS::physicalFS::CurrentDirectory before joining rem with /")

	//	*nix only format
	return "/" + strings.Join(rel, "/")
}

func (pfs *physicalFS) List() ([]fs.File, error) {
	items, err := ioutil.ReadDir(pfs.currentRealDirectory)

	if err != nil {
		return nil, err
	}

	var files []fs.File

	for _, item := range items {
		files = append(files, physicalFile.New(item.Name(), pfs.currentRealDirectory, item.IsDir(), item.Size(), item.ModTime(), item.Mode()))
	}

	return files, nil
}

func (pfs *physicalFS) Get(filename string) (fs.File, error) {
	var fullpath string
	if filename[0] == '/' {
		fullpath = filepath.Join(pfs.homeRealDirectory, filename)
	} else {
		fullpath = filepath.Join(pfs.currentRealDirectory, filename)
	}

	log.WithFields(log.Fields{"pfs": pfs, "filename": filename, "fullpath": fullpath}).Debug("localFS::physicalFS::Get called")
	f, err := os.Stat(path.Clean(fullpath))

	if err != nil {
		return nil, err
	}

	return physicalFile.New(filepath.Base(fullpath), filepath.Dir(fullpath), f.IsDir(), f.Size(), f.ModTime(), f.Mode()), nil
}

func (pfs *physicalFS) New(name string, isDirectory bool) (fs.File, error) {
	fullpath := filepath.Join(pfs.currentRealDirectory, name)

	log.WithFields(log.Fields{"pfs": pfs, "name": name, "fullpath": fullpath, "isDirectory": isDirectory}).Debug("localFS::physicalFS::New called")

	createMode := os.FileMode(0770)

	if isDirectory {
		// try to create it and else fail
		err := os.Mkdir(fullpath, createMode)
		if err != nil {
			return nil, err
		}
	}
	pfile := physicalFile.New(name, pfs.currentRealDirectory, isDirectory, 0, time.Now(), createMode)

	if !isDirectory {
		// create an empty file
		w, err := pfile.Write()
		if err != nil {
			return nil, err
		}
		w.Close()
	}

	return pfile, nil
}

func (pfs *physicalFS) Clone() fs.FileProvider {
	return &physicalFS{
		homeRealDirectory:    pfs.homeRealDirectory,
		currentRealDirectory: pfs.currentRealDirectory,
	}
}

func (pfs *physicalFS) ChangeDirectory(path string) error {
	var tmpDir string

	if pfs.homeRealDirectory == pfs.currentRealDirectory && path == ".." {
		return nil
	}

	if path[0] == '/' {
		// absolute path, join with homeDirectory
		tmpDir = filepath.Join(pfs.homeRealDirectory, path)
	} else {
		// relative path, join with currentDirectory
		tmpDir = filepath.Join(pfs.currentRealDirectory, path)
	}

	log.WithFields(log.Fields{"physicalFS": pfs, "tmpDir": tmpDir}).Debug("localFS::physicalFS::ChangeDirectory before testing tmpDir")

	// check existence and availability
	stat, err := os.Stat(tmpDir)
	if err != nil {
		log.WithFields(log.Fields{"physicalFS": pfs, "tmpDir": tmpDir}).Info("localFS::physicalFS::ChangeDirectory requested invalid directory")
		return err
	}

	if !stat.IsDir() {
		log.WithFields(log.Fields{"physicalFS": pfs, "tmpDir": tmpDir}).Info("localFS::physicalFS::ChangeDirectory requested entry is not a directory")
		return fmt.Errorf("%s requested entry is not a directory", path)
	}

	pfs.currentRealDirectory = tmpDir
	log.WithFields(log.Fields{"physicalFS": pfs}).Debug("localFS::physicalFS::ChangeDirectory before finish")
	return nil
}

func (pfs *physicalFS) CreateDirectory(name string) error {
	createMode := os.FileMode(0770)
	return os.MkdirAll(filepath.Join(pfs.currentRealDirectory, name), createMode)
}

func (pfs *physicalFS) RemoveDirectory(name string) error {
	return os.Remove(filepath.Join(pfs.currentRealDirectory, name))
}
