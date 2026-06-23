package datastore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

// ledgerFilenameRe is the regular expression that matches filenames produced by
// DataStoreSchema.GetObjectKeyFromSequenceNumber (base name only).
// Examples:
//
//	FFFFFFFF--0.xdr.zstd
//	FFFFFFFE--0-999.xdr.zst
var ledgerFilenameRe = regexp.MustCompile(`^[0-9A-F]{8}--[0-9]+(?:-[0-9]+)?\.xdr\.[A-Za-z0-9._-]+$`)

// DatastoreManifest represents the persisted configuration stored in the object store.
type DatastoreManifest struct {
	NetworkPassphrase string `json:"networkPassphrase"`
	Version           string `json:"version"`
	Compression       string `json:"compression"`
	LedgersPerFile    uint32 `json:"ledgersPerBatch"`
	FilesPerPartition uint32 `json:"batchesPerPartition"`
}

// toDataStoreManifest transforms a user-provided config into a manifest for persistence.
func toDataStoreManifest(cfg DataStoreConfig) DatastoreManifest {
	return DatastoreManifest{
		NetworkPassphrase: cfg.NetworkPassphrase,
		Version:           Version,
		Compression:       cfg.Compression,
		LedgersPerFile:    cfg.Schema.LedgersPerFile,
		FilesPerPartition: cfg.Schema.FilesPerPartition,
	}
}

// createManifest writes a new manifest to the datastore if it doesn't already exist.
func createManifest(ctx context.Context, dataStore DataStore, cfg DataStoreConfig) (DatastoreManifest, bool, error) {
	manifest := toDataStoreManifest(cfg)

	data, err := json.Marshal(manifest)
	if err != nil {
		return DatastoreManifest{}, false, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	ok, err := dataStore.PutFileIfNotExists(ctx, manifestFilename, bytes.NewReader(data), map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return DatastoreManifest{}, false, fmt.Errorf("failed to write manifest file %q: %w", manifestFilename, err)
	}

	return manifest, ok, nil
}

func readManifest(ctx context.Context, dataStore DataStore, filename string) (DatastoreManifest, error) {
	reader, _, err := dataStore.GetFile(ctx, filename)
	if err != nil {
		return DatastoreManifest{}, fmt.Errorf("unable to open manifest file %q: %w", filename, err)
	}
	defer reader.Close()

	var manifest DatastoreManifest
	if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
		return DatastoreManifest{}, fmt.Errorf("invalid JSON in manifest file %q: %w", filename, err)
	}

	return manifest, nil
}

type ConfigMismatchError struct {
	Diffs []string
}

func (e *ConfigMismatchError) Error() string {
	return fmt.Sprintf("The local config does not match the manifest "+
		"stored in the datastore. Details: %s", strings.Join(e.Diffs, "; "))
}

func compareManifests(expected, actual DatastoreManifest) error {
	var diffs []string

	if expected.NetworkPassphrase != "" && expected.NetworkPassphrase != actual.NetworkPassphrase {
		diffs = append(diffs, fmt.Sprintf("networkPassphrase: local=%q, datastore=%q",
			expected.NetworkPassphrase, actual.NetworkPassphrase))
	}
	if expected.Version != "" && expected.Version != actual.Version {
		diffs = append(diffs, fmt.Sprintf("version: local=%q, datastore=%q",
			expected.Version, actual.Version))
	}

	if expected.Compression != "" && expected.Compression != actual.Compression {
		diffs = append(diffs, fmt.Sprintf("compression: local=%q, datastore=%q",
			expected.Compression, actual.Compression))
	}
	if expected.LedgersPerFile != 0 && expected.LedgersPerFile != actual.LedgersPerFile {
		diffs = append(diffs, fmt.Sprintf("ledgersPerFile: local=%d, datastore=%d",
			expected.LedgersPerFile, actual.LedgersPerFile))
	}
	if expected.FilesPerPartition != 0 && expected.FilesPerPartition != actual.FilesPerPartition {
		diffs = append(diffs, fmt.Sprintf("filesPerPartition: local=%d, datastore=%d",
			expected.FilesPerPartition, actual.FilesPerPartition))
	}

	if len(diffs) != 0 {
		return &ConfigMismatchError{
			Diffs: diffs,
		}
	}

	return nil
}

// PublishConfig ensures that a datastore manifest exists and matches the provided configuration.
// If the manifest is missing, it creates one. Returns the manifest, whether it was created, and any error encountered.
func PublishConfig(ctx context.Context, dataStore DataStore, cfg DataStoreConfig) (DatastoreManifest, bool, error) {
	manifest, err := readManifest(ctx, dataStore, manifestFilename)
	if err == nil {
		// Validate that the existing manifest matches the provided config
		if err = compareManifests(toDataStoreManifest(cfg), manifest); err != nil {
			return manifest, false, fmt.Errorf(
				"datastore config mismatch: %w. If the difference is in schema settings, "+
					"either remove the schema section from your local config or update it to match the datastore", err)
		}
		return manifest, false, nil
	}

	// failed for a reason other than not existing
	if !errors.Is(err, os.ErrNotExist) {
		return DatastoreManifest{}, false, fmt.Errorf("failed to read manifest: %w", err)
	}

	createdManifest, created, err := createManifest(ctx, dataStore, cfg)
	if err != nil {
		return DatastoreManifest{}, false, fmt.Errorf("failed to create manifest: %w", err)
	}

	return createdManifest, created, nil
}

// LoadSchema reads the datastore manifest from the given DataStore and returns its schema configuration.
func LoadSchema(ctx context.Context, dataStore DataStore, cfg DataStoreConfig) (DataStoreSchema, error) {
	fileExt, err := GetLedgerFileExtension(ctx, dataStore)
	if err != nil && !errors.Is(err, ErrNoLedgerFiles) {
		return DataStoreSchema{}, fmt.Errorf("unable to determine ledger file extension from data store: %w", err)
	}

	manifest, err := readManifest(ctx, dataStore, manifestFilename)
	if err != nil {
		// If the manifest is missing, fall back to using cfg values
		if cfg.Schema.LedgersPerFile == 0 || cfg.Schema.FilesPerPartition == 0 {
			return DataStoreSchema{}, errors.New("datastore manifest is missing and local config is incomplete; ledgersPerFile and filesPerPartition must be set")
		}

		if errors.Is(err, os.ErrNotExist) {
			return DataStoreSchema{
				LedgersPerFile:    cfg.Schema.LedgersPerFile,
				FilesPerPartition: cfg.Schema.FilesPerPartition,
				FileExtension:     fileExt,
			}, nil
		}
		// return any other error reading manifest
		return DataStoreSchema{}, fmt.Errorf("failed to read manifest: %w", err)
	}

	// If manifest exists, validate against cfg
	if err := compareManifests(toDataStoreManifest(cfg), manifest); err != nil {
		return DataStoreSchema{}, fmt.Errorf(
			"datastore config mismatch: %w. If the difference is in schema settings, "+
				"either remove the schema section from your local config or update it to match the datastore", err)
	}

	return DataStoreSchema{
		LedgersPerFile:    manifest.LedgersPerFile,
		FilesPerPartition: manifest.FilesPerPartition,
		FileExtension:     fileExt,
	}, nil
}

var ErrNoLedgerFiles = errors.New("no ledger files found")

func GetLedgerFileExtension(ctx context.Context, dataStore DataStore) (string, error) {
	files, err := dataStore.ListFilePaths(ctx, ListFileOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list ledger files: %w", err)
	}

	// Note: The file may be inside a partition directory; we only check the base name here.
	for _, file := range files {
		base := filepath.Base(file)
		if ledgerFilenameRe.MatchString(base) {
			// Extract the extension after the last dot using filepath.Ext
			// e.g., "...xdr.zst" -> ".zst" -> "zst"
			ext := strings.TrimPrefix(filepath.Ext(base), ".")
			if ext != "" {
				return ext, nil
			}
		}
	}

	return "", ErrNoLedgerFiles
}
