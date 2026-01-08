package datastore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stellar/go-stellar-sdk/support/log"
)

const (
	defaultDirPerms  os.FileMode = 0755
	defaultFilePerms os.FileMode = 0644
)

var _ DataStore = &FilesystemDataStore{}

// FilesystemDataStore implements DataStore for local filesystem storage.
//
// Note: This implementation is not recommended for production use. It is
// intended for development and testing purposes only.
//
// This implementation does not support storing metadata. The metaData
// parameter in PutFile and PutFileIfNotExists is ignored, and GetFileMetadata
// always returns an empty map.
//
// Concurrent writes to the same file path are not safe and may result in
// data corruption. Callers must ensure proper synchronization when writing
// to the same path from multiple processes.
type FilesystemDataStore struct {
	basePath string
}

// NewFilesystemDataStore creates a new FilesystemDataStore from configuration.
func NewFilesystemDataStore(ctx context.Context, datastoreConfig DataStoreConfig) (DataStore, error) {
	destinationPath, ok := datastoreConfig.Params["destination_path"]
	if !ok {
		return nil, errors.New("invalid Filesystem config, no destination_path")
	}

	return NewFilesystemDataStoreWithPath(destinationPath)
}

// NewFilesystemDataStoreWithPath creates a FilesystemDataStore with the given base path.
func NewFilesystemDataStoreWithPath(basePath string) (DataStore, error) {
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	log.Debugf("Creating Filesystem datastore at: %s", absPath)

	return &FilesystemDataStore{
		basePath: absPath,
	}, nil
}

// fullPath returns the full filesystem path for a given relative path.
func (f *FilesystemDataStore) fullPath(path string) string {
	return filepath.Join(f.basePath, path)
}

// Exists checks if a file exists in the filesystem.
func (f *FilesystemDataStore) Exists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(f.fullPath(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Size returns the size of a file in bytes.
func (f *FilesystemDataStore) Size(ctx context.Context, path string) (int64, error) {
	info, err := os.Stat(f.fullPath(path))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, os.ErrNotExist
		}
		return 0, err
	}
	return info.Size(), nil
}

// GetFileLastModified returns the last modification time of a file.
func (f *FilesystemDataStore) GetFileLastModified(ctx context.Context, path string) (time.Time, error) {
	info, err := os.Stat(f.fullPath(path))
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, os.ErrNotExist
		}
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// GetFile returns a reader for the file at the given path.
func (f *FilesystemDataStore) GetFile(ctx context.Context, path string) (io.ReadCloser, error) {
	file, err := os.Open(f.fullPath(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("error opening file %s: %w", path, err)
	}
	log.Debugf("File retrieved successfully: %s", path)
	return file, nil
}

// GetFileMetadata returns an empty map as filesystem storage does not support metadata.
func (f *FilesystemDataStore) GetFileMetadata(ctx context.Context, path string) (map[string]string, error) {
	if _, err := os.Stat(f.fullPath(path)); os.IsNotExist(err) {
		return nil, os.ErrNotExist
	}
	return map[string]string{}, nil
}

// PutFile writes a file to the filesystem.
func (f *FilesystemDataStore) PutFile(ctx context.Context, path string, in io.WriterTo, metaData map[string]string) error {
	fullPath := f.fullPath(path)

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, defaultDirPerms); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write the data file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}

	if _, err := in.WriteTo(file); err != nil {
		file.Close()
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close file %s: %w", path, err)
	}

	log.Debugf("File written successfully: %s", path)
	return nil
}

// PutFileIfNotExists writes a file only if it doesn't already exist.
func (f *FilesystemDataStore) PutFileIfNotExists(
	ctx context.Context, path string, in io.WriterTo, metaData map[string]string,
) (bool, error) {
	fullPath := f.fullPath(path)

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, defaultDirPerms); err != nil {
		return false, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Use O_CREATE|O_EXCL for atomic check-and-create
	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, defaultFilePerms)
	if err != nil {
		if os.IsExist(err) {
			log.Debugf("File already exists: %s", path)
			return false, nil
		}
		return false, fmt.Errorf("failed to create file %s: %w", path, err)
	}

	if _, err := in.WriteTo(file); err != nil {
		file.Close()
		os.Remove(fullPath) // Clean up on error
		return false, fmt.Errorf("failed to write file %s: %w", path, err)
	}

	if err := file.Close(); err != nil {
		return false, fmt.Errorf("failed to close file %s: %w", path, err)
	}

	log.Debugf("File written successfully: %s", path)
	return true, nil
}

// ListFilePaths lists file paths matching the given options.
// Results are returned in lexicographical order (matching GCS/S3 behavior).
func (f *FilesystemDataStore) ListFilePaths(ctx context.Context, options ListFileOptions) ([]string, error) {
	limit := options.Limit
	if limit <= 0 || limit > listFilePathsMaxLimit {
		limit = listFilePathsMaxLimit
	}

	var files []string
	err := filepath.WalkDir(f.basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check for context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get path relative to basePath and normalize to forward slashes
		relPath, err := filepath.Rel(f.basePath, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		// Apply prefix filter
		if options.Prefix != "" && !strings.HasPrefix(relPath, options.Prefix) {
			return nil
		}

		// Apply StartAfter filter (WalkDir walks in lexical order)
		if options.StartAfter != "" && relPath <= options.StartAfter {
			return nil
		}

		files = append(files, relPath)

		// Stop early if we've reached the limit
		if len(files) >= int(limit) {
			return filepath.SkipAll
		}

		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return nil, err
	}

	return files, nil
}

// Close is a no-op for FilesystemDataStore as it doesn't maintain persistent connections.
func (f *FilesystemDataStore) Close() error {
	return nil
}
