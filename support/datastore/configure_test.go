package datastore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConfigureDatastore(t *testing.T) {

	var defaultCfg = DataStoreConfig{
		Type: "test",
		Schema: DataStoreSchema{
			LedgersPerFile:    1000,
			FilesPerPartition: 10,
		},
		NetworkPassphrase: "passphrase",
		Compression:       "xyz",
	}

	var expectedManifest = DatastoreManifest{
		NetworkPassphrase: "passphrase",
		Version:           "1.0",
		Compression:       "xyz",
		LedgersPerFile:    1000,
		FilesPerPartition: 10,
	}
	configJSON, err := json.Marshal(expectedManifest)
	require.NoError(t, err)

	t.Run("creates new manifest", func(t *testing.T) {
		mockDataStore := new(MockDataStore)
		ctx := context.Background()

		mockDataStore.On("GetFile", ctx, manifestFilename).Return(nil, int64(0), os.ErrNotExist).Once()
		mockDataStore.On("PutFileIfNotExists", ctx, manifestFilename, bytes.NewReader(configJSON),
			mock.Anything).Return(true, nil).Once()

		manifest, ok, err := PublishConfig(ctx, mockDataStore, defaultCfg)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, expectedManifest, manifest)

		mockDataStore.AssertExpectations(t)
	})

	t.Run("uses existing manifest", func(t *testing.T) {
		mockDataStore := new(MockDataStore)
		ctx := context.Background()

		mockDataStore.On("GetFile", ctx, manifestFilename).
			Return(io.NopCloser(bytes.NewReader(configJSON)), int64(-1), nil).Once()

		manifest, ok, err := PublishConfig(ctx, mockDataStore, defaultCfg)
		require.NoError(t, err)
		require.False(t, ok)
		require.Equal(t, expectedManifest, manifest)

		mockDataStore.AssertExpectations(t)
	})

	t.Run("returns error if PutFile fails", func(t *testing.T) {
		mockDataStore := new(MockDataStore)
		ctx := context.Background()

		mockDataStore.On("GetFile", ctx, manifestFilename).Return(nil, int64(0), os.ErrNotExist).Once()
		mockDataStore.On("PutFileIfNotExists", ctx, manifestFilename, bytes.NewReader(configJSON), mock.Anything).
			Return(false, errors.New("boom")).Once()

		_, ok, err := PublishConfig(ctx, mockDataStore, defaultCfg)
		require.Error(t, err)
		require.False(t, ok)
		require.Contains(t, err.Error(), "boom")

		mockDataStore.AssertExpectations(t)
	})
}

func TestCompareManifests(t *testing.T) {
	with := func(base DatastoreManifest, modify func(*DatastoreManifest)) DatastoreManifest {
		copy := base
		modify(&copy)
		return copy
	}

	base := DatastoreManifest{
		NetworkPassphrase: "test-passphrase",
		Version:           "1.0",
		Compression:       "zstd",
		LedgersPerFile:    1000,
		FilesPerPartition: 10,
	}

	tests := []struct {
		name     string
		expected DatastoreManifest
		actual   DatastoreManifest
		wantErr  string
	}{
		{
			name:     "match",
			expected: base,
			actual:   base,
			wantErr:  "",
		},
		{
			name:     "network passphrase mismatch",
			expected: base,
			actual:   with(base, func(m *DatastoreManifest) { m.NetworkPassphrase = "wrong" }),
			wantErr:  "The local config does not match the manifest stored in the datastore. Details: networkPassphrase: local=\"test-passphrase\", datastore=\"wrong\"",
		},
		{
			name:     "version mismatch",
			expected: base,
			actual:   with(base, func(m *DatastoreManifest) { m.Version = "2.0" }),
			wantErr:  "The local config does not match the manifest stored in the datastore. Details: version: local=\"1.0\", datastore=\"2.0\"",
		},
		{
			name:     "compression mismatch",
			expected: base,
			actual:   with(base, func(m *DatastoreManifest) { m.Compression = "gzip" }),
			wantErr:  "The local config does not match the manifest stored in the datastore. Details: compression: local=\"zstd\", datastore=\"gzip\"",
		},
		{
			name:     "ledgersPerFile mismatch",
			expected: base,
			actual:   with(base, func(m *DatastoreManifest) { m.LedgersPerFile = 500 }),
			wantErr:  "The local config does not match the manifest stored in the datastore. Details: ledgersPerFile: local=1000, datastore=500",
		},
		{
			name:     "filesPerPartition mismatch",
			expected: base,
			actual:   with(base, func(m *DatastoreManifest) { m.FilesPerPartition = 5 }),
			wantErr:  "The local config does not match the manifest stored in the datastore. Details: filesPerPartition: local=10, datastore=5",
		},
		{
			name:     "multiple mismatches",
			expected: base,
			actual:   with(base, func(m *DatastoreManifest) { m.Version = "2.0"; m.Compression = "gzip" }),
			wantErr:  "The local config does not match the manifest stored in the datastore. Details: version: local=\"1.0\", datastore=\"2.0\"; compression: local=\"zstd\", datastore=\"gzip\"",
		},
		{
			name:     "empty expected manifest",
			expected: DatastoreManifest{},
			actual:   base,
			wantErr:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := compareManifests(tt.expected, tt.actual)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func TestLoadSchema(t *testing.T) {
	ctx := context.Background()
	defaultCfg := DataStoreConfig{
		Type:              "test",
		NetworkPassphrase: "passphrase",
		Schema: DataStoreSchema{
			LedgersPerFile:    1000,
			FilesPerPartition: 10,
		},
		Compression: "gzip",
	}
	validManifest := DatastoreManifest{
		NetworkPassphrase: "passphrase",
		Version:           "1.0",
		Compression:       "gzip",
		LedgersPerFile:    1000,
		FilesPerPartition: 10,
	}

	validManifestBytes, err := json.Marshal(validManifest)
	require.NoError(t, err)

	// Manifest file exists and is valid (happy path)
	t.Run("Manifest found and valid", func(t *testing.T) {
		mockOS := new(MockDataStore)
		mockOS.On("GetFile", ctx, manifestFilename).Return(io.NopCloser(bytes.NewReader(validManifestBytes)), int64(-1), nil).Once()
		mockOS.On("ListFilePaths", ctx, ListFileOptions{}).Return(nil, nil)
		schema, err := LoadSchema(ctx, mockOS, defaultCfg)
		require.NoError(t, err)
		require.NotNil(t, schema)
		require.Equal(t, uint32(1000), schema.LedgersPerFile)
		require.Equal(t, uint32(10), schema.FilesPerPartition)
		mockOS.AssertExpectations(t)
	})

	// Manifest file not found (backward compatibility), fallback to config
	t.Run("Manifest not found", func(t *testing.T) {
		mockOS := new(MockDataStore)
		mockOS.On("GetFile", ctx, manifestFilename).Return(nil, int64(0), os.ErrNotExist).Once()
		mockOS.On("ListFilePaths", ctx, ListFileOptions{}).Return(nil, nil)

		schema, err := LoadSchema(ctx, mockOS, defaultCfg)
		require.NoError(t, err)
		require.NotNil(t, schema)
		require.Equal(t, uint32(1000), schema.LedgersPerFile)
		require.Equal(t, uint32(10), schema.FilesPerPartition)
		mockOS.AssertExpectations(t)
	})

	t.Run("Manifest found but invalid JSON", func(t *testing.T) {
		mockOS := new(MockDataStore)
		mockOS.On("GetFile", ctx, manifestFilename).Return(io.NopCloser(bytes.NewReader([]byte(`{"invalid": "json"`))), int64(-1), nil).Once()
		mockOS.On("ListFilePaths", ctx, ListFileOptions{}).Return(nil, nil)

		schema, err := LoadSchema(ctx, mockOS, defaultCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid JSON in manifest file")
		require.EqualValues(t, DataStoreSchema{}, schema)
		mockOS.AssertExpectations(t)
	})

	t.Run("Manifest found but validation fails", func(t *testing.T) {
		mockOS := new(MockDataStore)
		invalidManifestBytes, err := json.Marshal(DatastoreManifest{
			LedgersPerFile:    500,
			FilesPerPartition: 5,
		})
		require.NoError(t, err)

		mockOS.On("GetFile", ctx, manifestFilename).Return(io.NopCloser(bytes.NewReader(invalidManifestBytes)), int64(-1), nil).Once()
		mockOS.On("ListFilePaths", ctx, ListFileOptions{}).Return(nil, nil)

		schema, err := LoadSchema(ctx, mockOS, defaultCfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "datastore config mismatch")
		require.EqualValues(t, DataStoreSchema{}, schema)
		mockOS.AssertExpectations(t)
	})

	t.Run("Manifest not found, and incomplete config", func(t *testing.T) {
		mockOS := new(MockDataStore)
		mockOS.On("GetFile", ctx, manifestFilename).Return(nil, int64(0), os.ErrNotExist).Once()
		mockOS.On("ListFilePaths", ctx, ListFileOptions{}).Return(nil, nil)

		schema, err := LoadSchema(ctx, mockOS, DataStoreConfig{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "datastore manifest is missing and local config is incomplete; ledgersPerFile and filesPerPartition must be set")
		require.EqualValues(t, DataStoreSchema{}, schema)
		mockOS.AssertExpectations(t)
	})
}

func TestGetLedgerFileExtension(t *testing.T) {
	type tc struct {
		name        string
		files       []string
		listErr     error
		ExpectedExt string
		ExpectedErr error
	}

	cases := []tc{
		{
			name:        "returns zst when first matching schema file is .xdr.zst",
			files:       []string{".config.json", "misc/ignore.txt", "ledger/FFFFFFFF--0.xdr.zst"},
			ExpectedExt: "zst",
			ExpectedErr: nil,
		},
		{
			name:        "returns zstd when first matching schema file is .xdr.zstd",
			files:       []string{".config.json", "something.bin", "ledger/FFFFFFFE--0-999.xdr.zstd"},
			ExpectedExt: "zstd",
			ExpectedErr: nil,
		},
		{
			name:        "ignores manifest and non-matching files and returns ErrNoLedgerFiles",
			files:       []string{".config.json", "random/2025-08-01-0001.zst", "ABCDEF--bad.xdr.gz"},
			ExpectedExt: "",
			ExpectedErr: ErrNoLedgerFiles,
		},
		{
			name:        "no files returns ErrNoLedgerFiles",
			files:       []string{},
			ExpectedExt: "",
			ExpectedErr: ErrNoLedgerFiles,
		},
		{
			name:        "propagates underlying list error",
			files:       []string{},
			listErr:     fmt.Errorf("boom"),
			ExpectedExt: "",
			ExpectedErr: fmt.Errorf("failed to list ledger files: boom"),
		},
		{
			name:        "works with nested paths including partition dir",
			files:       []string{"part/DEADBEEF--0-999/DEADBEEF--0-999.xdr.zst"},
			ExpectedExt: "zst",
			ExpectedErr: nil,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ds := new(MockDataStore)
			ds.On("ListFilePaths", mock.Anything, ListFileOptions{}).Return(tt.files, tt.listErr).Once()

			ext, err := GetLedgerFileExtension(context.Background(), ds)
			require.Equal(t, tt.ExpectedExt, ext)

			if tt.ExpectedErr != nil {
				if tt.listErr != nil {
					require.ErrorContains(t, err, tt.ExpectedErr.Error())
				} else {
					require.ErrorIs(t, err, tt.ExpectedErr)
				}
			} else {
				require.NoError(t, err)
			}
			ds.AssertExpectations(t)
		})
	}
}
