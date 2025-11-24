package datastore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"

	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/support/url"
)

// GCSDataStore implements DataStore for GCS
type GCSDataStore struct {
	client *storage.Client
	bucket *storage.BucketHandle
	prefix string
}

func NewGCSDataStore(ctx context.Context, dataStoreConfig DataStoreConfig) (DataStore, error) {
	destinationBucketPath, ok := dataStoreConfig.Params["destination_bucket_path"]
	if !ok {
		return nil, errors.New("invalid GCS config, no destination_bucket_path")
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return FromGCSClient(ctx, client, destinationBucketPath)
}

func FromGCSClient(ctx context.Context, client *storage.Client, bucketPath string) (DataStore, error) {
	// append the gcs:// scheme to enable usage of the url package reliably to
	// get parse bucket name which is first path segment as URL.Host
	gcsBucketURL := fmt.Sprintf("gcs://%s", bucketPath)
	parsed, err := url.Parse(gcsBucketURL)
	if err != nil {
		return nil, err
	}

	// Inside gcs, all paths start _without_ the leading /
	prefix := strings.TrimPrefix(parsed.Path, "/")
	bucketName := parsed.Host

	log.Debugf("creating GCS client for bucket: %s, prefix: %s", bucketName, prefix)
	// Check the bucket exists
	bucket := client.Bucket(bucketName)
	if _, err := bucket.Attrs(ctx); err != nil {
		return nil, fmt.Errorf("failed to retrieve bucket attributes: %w", err)
	}

	return &GCSDataStore{client: client, bucket: bucket, prefix: prefix}, nil
}

func (b GCSDataStore) GetFileAttrs(ctx context.Context, filePath string) (*storage.ObjectAttrs, error) {
	filePath = path.Join(b.prefix, filePath)
	attrs, err := b.bucket.Object(filePath).Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, os.ErrNotExist
		}
	}
	return attrs, nil
}

// GetFileMetadata retrieves the metadata for the specified file in the GCS bucket.
func (b GCSDataStore) GetFileMetadata(ctx context.Context, filePath string) (map[string]string, error) {
	attrs, err := b.GetFileAttrs(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return attrs.Metadata, nil
}

// GetFileLastModified retrieves the last modified time of a file in the GCS bucket.
func (b GCSDataStore) GetFileLastModified(ctx context.Context, filePath string) (time.Time, error) {
	attrs, err := b.GetFileAttrs(ctx, filePath)
	if err != nil {
		return time.Time{}, err
	}
	return attrs.Updated, nil
}

// GetFile retrieves a file from the GCS bucket.
func (b GCSDataStore) GetFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	filePath = path.Join(b.prefix, filePath)
	// setting ReadCompressed(true) will avoid transcoding of compressed files by including
	// an "Accept-Encoding: gzip" header in the request:
	// https://github.com/googleapis/google-cloud-go/blob/main/storage/http_client.go#L1307-L1309
	// https://cloud.google.com/storage/docs/transcoding#decompressive_transcoding
	// This will ensure that the reader performs CRC validation upon finishing the download:
	// https://pkg.go.dev/cloud.google.com/go/storage#Reader
	r, err := b.bucket.Object(filePath).ReadCompressed(true).NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, os.ErrNotExist
		}
		if gcsError, ok := err.(*googleapi.Error); ok {
			log.Debugf("GCS error: %s %s", gcsError.Message, gcsError.Body)
		}
		return nil, fmt.Errorf("error retrieving file %s: %w", filePath, err)
	}
	log.Debugf("File retrieved successfully: %s", filePath)
	return r, nil
}

// PutFileIfNotExists uploads a file to GCS only if it doesn't already exist.
func (b GCSDataStore) PutFileIfNotExists(ctx context.Context, filePath string, in io.WriterTo, metaData map[string]string) (bool, error) {
	err := b.putFile(ctx, filePath, in, &storage.Conditions{DoesNotExist: true}, metaData)
	if err != nil {
		if gcsError, ok := err.(*googleapi.Error); ok {
			switch gcsError.Code {
			case http.StatusPreconditionFailed:
				log.Debugf("Precondition failed: %s already exists in the bucket", filePath)
				return false, nil // Treat as success
			default:
				log.Debugf("GCS error: %s %s", gcsError.Message, gcsError.Body)
			}
		}
		return false, fmt.Errorf("error uploading file %s: %w", filePath, err)
	}
	log.Debugf("File uploaded successfully: %s", filePath)
	return true, nil
}

// PutFile uploads a file to GCS
func (b GCSDataStore) PutFile(ctx context.Context, filePath string, in io.WriterTo, metaData map[string]string) error {
	err := b.putFile(ctx, filePath, in, nil, metaData) // No conditions for regular PutFile

	if err != nil {
		if gcsError, ok := err.(*googleapi.Error); ok {
			log.Debugf("GCS error: %s %s", gcsError.Message, gcsError.Body)
		}
		return fmt.Errorf("error uploading file %s: %w", filePath, err)
	}
	log.Debugf("File uploaded successfully: %s", filePath)
	return nil
}

// Size retrieves the size of a file in the GCS bucket.
func (b GCSDataStore) Size(ctx context.Context, pth string) (int64, error) {
	pth = path.Join(b.prefix, pth)
	attrs, err := b.bucket.Object(pth).Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		err = os.ErrNotExist
	}
	if err != nil {
		return 0, err
	}
	return attrs.Size, nil
}

// Exists checks if a file exists in the GCS bucket.
func (b GCSDataStore) Exists(ctx context.Context, pth string) (bool, error) {
	_, err := b.Size(ctx, pth)

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return err == nil, err
}

// Close closes the GCS client connection.
func (b GCSDataStore) Close() error {
	return b.client.Close()
}

func (b GCSDataStore) putFile(ctx context.Context, filePath string, in io.WriterTo, conditions *storage.Conditions, metaData map[string]string) error {
	filePath = path.Join(b.prefix, filePath)
	o := b.bucket.Object(filePath)
	if conditions != nil {
		o = o.If(*conditions)
	}
	buf := &bytes.Buffer{}
	if _, err := in.WriteTo(buf); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	w := o.NewWriter(ctx)
	w.Metadata = metaData
	w.SendCRC32C = true
	// we must set CRC32C before invoking w.Write() for the first time
	w.CRC32C = crc32.Checksum(buf.Bytes(), crc32.MakeTable(crc32.Castagnoli))
	if _, err := io.Copy(w, buf); err != nil {
		return fmt.Errorf("failed to put file %s: %w", filePath, err)

	}
	return w.Close()
}

// ListFilePaths lists up to 'limit' file paths under the provided prefix.
// Returned paths are relative to the bucket prefix.
// and ordered lexicographically ascending as provided by the backend.
// If limit <= 0, implementations default to a cap of 1,000; values > 1,000 are capped to 1,000.
func (b GCSDataStore) ListFilePaths(ctx context.Context, options ListFileOptions) ([]string, error) {
	var fullPrefix string

	// When 'prefix' is empty, ensure the base prefix ends with a slash (e.g., "a/b/")
	// so the query returns only objects within that directory, not similarly named paths like "a/b-1".
	if options.Prefix == "" {
		fullPrefix = b.prefix
		if !strings.HasSuffix(fullPrefix, "/") {
			fullPrefix += "/"
		}
	} else {
		// Join the caller-provided prefix with the datastore prefix
		fullPrefix = path.Join(b.prefix, options.Prefix)
	}

	var StartAfter string
	if options.StartAfter != "" {
		StartAfter = path.Join(b.prefix, options.StartAfter)
	}

	query := &storage.Query{
		Prefix:      fullPrefix,
		StartOffset: StartAfter, // inclusive in GCS; we normalize to exclusive below
	}

	// Only request the object name to minimize payload
	query.SetAttrSelection([]string{"Name"})
	it := b.bucket.Objects(ctx, query)

	keys := make([]string, 0)
	// Enforce an effective cap of 1000 total results and default to 1000 if <= 0
	remaining := options.Limit
	if remaining <= 0 || remaining > listFilePathsMaxLimit {
		remaining = listFilePathsMaxLimit
	}
	for {
		if remaining == 0 {
			break
		}
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		// GCS StartOffset is inclusive, so if the key same as StartAfter,
		// skip it so results begin strictly after that key.
		if attrs.Name == StartAfter {
			continue
		}

		// Trim the configured prefix and any leading slash before appending
		relative := strings.TrimPrefix(attrs.Name, b.prefix)
		relative = strings.TrimLeft(relative, "/")
		keys = append(keys, relative)

		remaining--
	}
	return keys, nil
}
