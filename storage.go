package delta

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
)

type objectStorage interface {
	put(key string, data []byte) error
	putObject(key string, data []byte) error
	putIfAbsent(name string, data []byte) error
	listPrefix(prefix string, startAfter string) ([]string, error)
	read(name string) ([]byte, error)
	keyExists(key string) (bool, error)
}

type fileObjectStorage struct {
	deltaBaseDir string
}

const (
	tempDir        = "_temp"
	logDir         = "_delta_log"
	logExt         = ".log"
	dataExt        = ".data"
	lastCheckPoint = "_last_checkpoint"
	checkpointExt  = ".checkpoint"
	pageSize       = 4 * 1024
)

func NewFileObjectStorage(deltaBaseDir string) fileObjectStorage {
	return fileObjectStorage{
		deltaBaseDir: deltaBaseDir,
	}
}

// this method writes file to the storage. Simpler than the put() method
// may result in partial writes
// used for delta data files
func (fos *fileObjectStorage) putObject(key string, data []byte) error {
	path := path.Join(fos.deltaBaseDir, key)

	// check if parent dirs exists if not then create
	parentDir := filepath.Dir(path)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	log.Printf("putObject write at: %v", path)

	file, err := os.OpenFile(path, os.O_EXCL|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	err = fos.writeData(data, file, path)
	if err != nil {
		return err
	}

	err = file.Sync()
	if err != nil {
		errRemove := os.Remove(path)
		assert(errRemove == nil, "error removing data file")
		return err
	}

	err = file.Close()
	if err != nil {
		errRemove := os.Remove(path)
		assert(errRemove == nil, "error removing data file")
		return err
	}

	return nil
}

func (fos *fileObjectStorage) putIfAbsent(key string, data []byte) error {
	// create temp txn file
	txnDir := path.Dir(key)
	tmpPath := path.Join(fos.deltaBaseDir, txnDir, tempDir, uuid.NewString())
	finalPath := path.Join(fos.deltaBaseDir, key) // Full path including table name

	log.Printf("putIfAbsent finalpath: %v", finalPath)

	// Ensure parent directories exist for both temp and final paths
	dirsToCreate := []string{
		path.Dir(tmpPath),   // temp file's parent dir
		path.Dir(finalPath), // final file's parent dir
	}

	for _, dir := range dirsToCreate {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	log.Printf("Writing to temp path: %s", tmpPath)
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		fmt.Printf("error in creating the tmpFile:%v", err)
		return err
	}

	err = fos.writeData(data, tmpFile, tmpPath)
	if err != nil {
		return err
	}

	err = tmpFile.Sync()
	if err != nil {
		assert(os.Remove(tmpPath) == nil, "Failed to remove temp file")
		return err
	}

	err = tmpFile.Close()
	if err != nil {
		assert(os.Remove(tmpPath) == nil, "Failed to remove temp file")
		return err
	}

	//https://rcrowley.org/2010/01/06/things-unix-can-do-atomically.html
	err = os.Link(tmpPath, finalPath)
	if err != nil {
		log.Printf("Failed to link file: %v", err)
		assert(os.Remove(tmpPath) == nil, "Failed to remove temp file")
		return err
	}

	// remove the temp txn file
	err = os.Remove(tmpPath)
	if err != nil {
		return err
	}

	return nil
}

// This is a simplified version of headObject() that returns a response object with response headers with empty body
// Can be used to check if a key exists without reading the full file content and is more efficient than doing a list operation
func (fos *fileObjectStorage) keyExists(key string) (bool, error) {
	baseDir := filepath.Dir(key)
	suffix := filepath.Base(key)

	txnDir := filepath.Join(fos.deltaBaseDir, baseDir)

	// directory exists?
	if _, err := os.Stat(txnDir); os.IsNotExist(err) {
		//return empty list
		return false, nil
	}

	dir, err := os.Open(txnDir)
	if err != nil {
		return false, err
	}
	defer dir.Close()

	for {
		var files []os.FileInfo
		files, err := dir.Readdir(100)
		if err != nil && err != io.EOF {
			return false, err
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if file.Name() == suffix {
				return true, nil
			}
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return false, err
		}
	}
	return false, nil
}

// this method ensures atomic overwrite (if file exists) else creates a new file
// safer for concurrent operations and prevents partial writes
// used to write/overwrite delta metadata files
func (fos *fileObjectStorage) put(key string, data []byte) error {
	txnDir := path.Dir(key)
	tmpPath := path.Join(fos.deltaBaseDir, txnDir, tempDir, uuid.NewString())
	finalPath := path.Join(fos.deltaBaseDir, key) // Full path including table name

	log.Printf("put finalpath: %v", finalPath)

	// Ensure parent directories exist for both temp and final paths
	dirsToCreate := []string{
		path.Dir(tmpPath),   // temp file's parent dir
		path.Dir(finalPath), // final file's parent dir
	}

	for _, dir := range dirsToCreate {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	log.Printf("Writing to temp path: %s", tmpPath)
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		fmt.Printf("error in creating the tmpFile:%v", err)
		return err
	}

	err = fos.writeData(data, tmpFile, tmpPath)
	if err != nil {
		return err
	}

	err = tmpFile.Sync()
	if err != nil {
		assert(os.Remove(tmpPath) == nil, "Failed to remove temp file")
		return err
	}

	err = tmpFile.Close()
	if err != nil {
		assert(os.Remove(tmpPath) == nil, "Failed to remove temp file")
		return err
	}

	//https://rcrowley.org/2010/01/06/things-unix-can-do-atomically.html
	//rename atomic within the same file system
	err = os.Rename(tmpPath, finalPath)
	if err != nil {
		log.Printf("Failed to link file: %v", err)
		assert(os.Remove(tmpPath) == nil, "Failed to remove temp file")
		return err
	}

	return nil

}

func (*fileObjectStorage) writeData(data []byte, tmpFile *os.File, tmpPath string) error {
	written := 0
	for written < len(data) {
		toWrite := min(pageSize+written, len(data))
		n, err := tmpFile.Write(data[written:toWrite])
		if err != nil {
			errRemove := os.Remove(tmpPath)
			assert(errRemove == nil, "error removing data file")
			return err
		}
		written += n
	}
	return nil
}

// this method list files within a prefix (directory), not the absolute path.
func (fos *fileObjectStorage) listPrefix(prefix string, startAfter string) ([]string, error) {
	// todo: table name
	txnDir := filepath.Join(fos.deltaBaseDir, prefix)
	log.Printf("listPrefix txnDir: %s", txnDir)

	// directory exists?
	if _, err := os.Stat(txnDir); os.IsNotExist(err) {
		//return empty list
		return []string{}, nil
	}

	dir, err := os.Open(txnDir)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	// Readdirnames is expected to return the files in OS aka ls -l order
	// Assuming txn logs are always created monotonically, sorting should not be required
	// FIX: Use Readdir to get FileInfo objects as with tempDir the dir order of Readdir is not maintained
	var filteredFiles []string
	var startFound bool

	if strings.Compare(startAfter, "") == 0 {
		startFound = true
	}

	for {
		var files []os.FileInfo
		files, err := dir.Readdir(100)
		// log.Printf("files:%v", files)
		if err != nil && err != io.EOF {
			return nil, err
		}

		// sort by creation date
		sort.Slice(
			files,
			func(i, j int) bool { return files[i].ModTime().Before(files[j].ModTime()) },
		)
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			log.Printf("file: %s \n", file.Name())
			if !startFound {
				if file.Name() == startAfter {
					startFound = true
				}
				continue
			}
			filteredFiles = append(filteredFiles, file.Name())
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}
	}
	// err = dir.Close()
	return filteredFiles, err
}

func (fos *fileObjectStorage) read(name string) ([]byte, error) {
	file := filepath.Join(fos.deltaBaseDir, name)
	return os.ReadFile(file)
}
