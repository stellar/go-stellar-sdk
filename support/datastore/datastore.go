package datastore

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	manifestFilename = ".config.json"
	Version          = "1.0"
)

// DataStoreConfig defines user-provided configuration used to initialize a DataStore.
type DataStoreConfig struct {
	Type              string            `toml:"type"`
	Params            map[string]string `toml:"params"`
	Schema            DataStoreSchema   `toml:"schema"`
	NetworkPassphrase string
	Compression       string
}

const listFilePathsMaxLimit = 1000

// ListFileOptions controls how ListFilePaths enumerates objects.
type ListFileOptions struct {
	// Prefix filters the results to only include keys that start with this string.
	Prefix string

	// StartAfter specifies the key from which to begin listing. The returned keys will be
	// lexicographically greater than this value.
	StartAfter string

	// Limit restricts the number of keys returned. A value of 0 will use the default limit,
	// and any value above listFilePathsMaxLimit will be automatically capped.
	Limit uint32
}

// DataStore defines an interface for interacting with data storage
type DataStore interface {
	GetFileMetadata(ctx context.Context, path string) (map[string]string, error)
	GetFileLastModified(ctx context.Context, filePath string) (time.Time, error)
	// GetFile retrieves a file and returns its contents as a reader along with
	// the file's size in bytes. Size is -1 if unknown (e.g., chunked transfer).
	GetFile(ctx context.Context, path string) (io.ReadCloser, int64, error)
	PutFile(ctx context.Context, path string, in io.WriterTo, metaData map[string]string) error
	PutFileIfNotExists(ctx context.Context, path string, in io.WriterTo, metaData map[string]string) (bool, error)
	Exists(ctx context.Context, path string) (bool, error)
	Size(ctx context.Context, path string) (int64, error)
	// ListFilePaths lists file paths relative to the datastore prefix in
	// ascending lexicographic order. Directory placeholder objects (keys that
	// are empty or end in "/") are not returned.
	ListFilePaths(ctx context.Context, options ListFileOptions) ([]string, error)
	Close() error
}

// NewDataStore factory, it creates a new DataStore based on the config type
func NewDataStore(ctx context.Context, datastoreConfig DataStoreConfig) (DataStore, error) {
	switch datastoreConfig.Type {
	case "GCS":
		return NewGCSDataStore(ctx, datastoreConfig)
	case "S3":
		return NewS3DataStore(ctx, datastoreConfig)
	case "Filesystem":
		return NewFilesystemDataStore(ctx, datastoreConfig)

	default:
		return nil, fmt.Errorf("invalid datastore type %v, not supported", datastoreConfig.Type)
	}
}

// parsePrefix returns the bucket sub path without the leading URL delimiter
// and at most one trailing slash. Empty, "." or ".." segments are rejected.
func parsePrefix(bucketPath, urlPath string) (string, error) {
	prefix := strings.TrimPrefix(urlPath, "/")
	if prefix == "" {
		return "", nil
	}
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return "", fmt.Errorf("invalid bucket path %q: must not contain empty segments", bucketPath)
	}
	for _, seg := range strings.Split(prefix, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", fmt.Errorf("invalid bucket path %q: must not contain empty, \".\" or \"..\" segments", bucketPath)
		}
	}
	return prefix, nil
}
