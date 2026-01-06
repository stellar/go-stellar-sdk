package datastore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stellar/go-stellar-sdk/support/log"
)

var _ DataStore = &FilesystemDataStore{}

const metadataSuffix = ".metadata.json"

// FilesystemDataStore implements DataStore for local filesystem storage.
type FilesystemDataStore struct {
	basePath      string
	writeMetadata bool
}

// NewFilesystemDataStore creates a new FilesystemDataStore from configuration.
func NewFilesystemDataStore(ctx context.Context, datastoreConfig DataStoreConfig) (DataStore, error) {
	destinationPath, ok := datastoreConfig.Params["destination_path"]
	if !ok {
		return nil, errors.New("invalid Filesystem config, no destination_path")
	}

	// write_metadata defaults to true
	writeMetadata := true
	if val, ok := datastoreConfig.Params["write_metadata"]; ok {
		writeMetadata = val != "false"
	}

	return NewFilesystemDataStoreWithPath(destinationPath, writeMetadata)
}

// NewFilesystemDataStoreWithPath creates a FilesystemDataStore with the given base path.
func NewFilesystemDataStoreWithPath(basePath string, writeMetadata bool) (DataStore, error) {
	// Ensure the base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory %s: %w", basePath, err)
	}

	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	log.Debugf("Creating Filesystem datastore at: %s, writeMetadata: %v", absPath, writeMetadata)

	return &FilesystemDataStore{
		basePath:      absPath,
		writeMetadata: writeMetadata,
	}, nil
}

// fullPath returns the full filesystem path for a given relative path.
func (f *FilesystemDataStore) fullPath(path string) string {
	return filepath.Join(f.basePath, path)
}

// metadataPath returns the path to the metadata sidecar file.
func (f *FilesystemDataStore) metadataPath(path string) string {
	return f.fullPath(path) + metadataSuffix
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

// GetFileMetadata reads metadata from the sidecar JSON file.
func (f *FilesystemDataStore) GetFileMetadata(ctx context.Context, path string) (map[string]string, error) {
	metaPath := f.metadataPath(path)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Check if the main file exists
			if _, mainErr := os.Stat(f.fullPath(path)); os.IsNotExist(mainErr) {
				return nil, os.ErrNotExist
			}
			// Main file exists but no metadata - return empty map
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("error reading metadata file %s: %w", metaPath, err)
	}

	var metadata map[string]string
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("error parsing metadata file %s: %w", metaPath, err)
	}
	return metadata, nil
}

// PutFile writes a file to the filesystem with optional metadata sidecar.
func (f *FilesystemDataStore) PutFile(ctx context.Context, path string, in io.WriterTo, metaData map[string]string) error {
	fullPath := f.fullPath(path)

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write metadata sidecar first if enabled and metadata is provided.
	// This ensures that if the data file exists, metadata is assumed to exist too.
	if f.writeMetadata && len(metaData) > 0 {
		if err := f.writeMetadataFile(path, metaData); err != nil {
			return err
		}
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

	log.Debugf("File uploaded successfully: %s", path)
	return nil
}

// PutFileIfNotExists writes a file only if it doesn't already exist.
func (f *FilesystemDataStore) PutFileIfNotExists(ctx context.Context, path string, in io.WriterTo, metaData map[string]string) (bool, error) {
	fullPath := f.fullPath(path)

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Use O_CREATE|O_EXCL for atomic check-and-create
	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			log.Debugf("File already exists: %s", path)
			return false, nil
		}
		return false, fmt.Errorf("failed to create file %s: %w", path, err)
	}

	// Write content to the file
	buf := &bytes.Buffer{}
	if _, err := in.WriteTo(buf); err != nil {
		file.Close()
		os.Remove(fullPath) // Clean up on error
		return false, fmt.Errorf("failed to write file %s: %w", path, err)
	}

	if _, err := file.Write(buf.Bytes()); err != nil {
		file.Close()
		os.Remove(fullPath) // Clean up on error
		return false, fmt.Errorf("failed to write file %s: %w", path, err)
	}

	if err := file.Close(); err != nil {
		return false, fmt.Errorf("failed to close file %s: %w", path, err)
	}

	// Write metadata sidecar if enabled and metadata is provided
	if f.writeMetadata && len(metaData) > 0 {
		if err := f.writeMetadataFile(path, metaData); err != nil {
			return true, err
		}
	}

	log.Debugf("File uploaded successfully: %s", path)
	return true, nil
}

// writeMetadataFile writes metadata to a sidecar JSON file.
func (f *FilesystemDataStore) writeMetadataFile(path string, metaData map[string]string) error {
	metaPath := f.metadataPath(path)

	data, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata for %s: %w", path, err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file %s: %w", metaPath, err)
	}
	return nil
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

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip metadata sidecar files
		if strings.HasSuffix(d.Name(), metadataSuffix) {
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
		if uint32(len(files)) >= limit {
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
