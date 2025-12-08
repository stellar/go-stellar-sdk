package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/support/compressxdr"
	"github.com/stellar/go-stellar-sdk/support/datastore"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestDefaultBSBConfigs(t *testing.T) {
	smallConfig := ledgerbackend.BufferedStorageBackendConfig{
		RetryLimit: 5,
		RetryWait:  30 * time.Second,
		BufferSize: 100,
		NumWorkers: 10,
	}

	largeConfig := ledgerbackend.BufferedStorageBackendConfig{
		RetryLimit: 5,
		RetryWait:  30 * time.Second,
		BufferSize: 10,
		NumWorkers: 2,
	}

	assert.Equal(t, DefaultBufferedStorageBackendConfig(1), smallConfig)
	assert.Equal(t, DefaultBufferedStorageBackendConfig(64), largeConfig)
	assert.Equal(t, DefaultBufferedStorageBackendConfig(512), largeConfig)
}

func TestBSBProducerFn(t *testing.T) {
	startLedger := uint32(2)
	endLedger := uint32(3)
	ctx := context.Background()
	ledgerRange := ledgerbackend.BoundedRange(startLedger, endLedger)
	mockDataStore := createMockdataStore(t, startLedger, endLedger, 64000)
	dsConfig := datastore.DataStoreConfig{}
	pubConfig := PublisherConfig{
		DataStoreConfig:       dsConfig,
		BufferedStorageConfig: DefaultBufferedStorageBackendConfig(1),
	}

	// inject the mock datastore using the package private testing factory override
	datastoreFactory = func(ctx context.Context, datastoreConfig datastore.DataStoreConfig) (datastore.DataStore, error) {
		assert.Equal(t, datastoreConfig, dsConfig)
		return mockDataStore, nil
	}

	expectedLcmSeqWasPublished := []bool{false, false}

	appCallback := func(lcm xdr.LedgerCloseMeta) error {
		if lcm.MustV0().LedgerHeader.Header.LedgerSeq == 2 {
			if expectedLcmSeqWasPublished[0] {
				assert.Fail(t, "producer fn had multiple callback invocations for same lcm")
			}
			expectedLcmSeqWasPublished[0] = true
		}
		if lcm.MustV0().LedgerHeader.Header.LedgerSeq == 3 {
			if expectedLcmSeqWasPublished[1] {
				assert.Fail(t, "producer fn had multiple callback invocations for same lcm")
			}
			expectedLcmSeqWasPublished[1] = true
		}
		return nil
	}

	assert.Nil(t, ApplyLedgerMetadata(ledgerRange, pubConfig, ctx, appCallback))
	assert.Equal(t, expectedLcmSeqWasPublished, []bool{true, true}, "producer fn did not invoke callback for all expected lcm")
}

func TestBSBProducerFnDataStoreError(t *testing.T) {
	ctx := context.Background()
	ledgerRange := ledgerbackend.BoundedRange(uint32(2), uint32(3))
	pubConfig := PublisherConfig{
		DataStoreConfig:       datastore.DataStoreConfig{},
		BufferedStorageConfig: ledgerbackend.BufferedStorageBackendConfig{},
	}

	datastoreFactory = func(ctx context.Context, datastoreConfig datastore.DataStoreConfig) (datastore.DataStore, error) {
		return &datastore.MockDataStore{}, errors.New("uhoh")
	}

	appCallback := func(lcm xdr.LedgerCloseMeta) error {
		return nil
	}

	assert.ErrorContains(t,
		ApplyLedgerMetadata(ledgerRange, pubConfig, ctx, appCallback),
		"failed to create datastore:")
}

func TestBSBProducerFnConfigError(t *testing.T) {
	ctx := context.Background()
	ledgerRange := ledgerbackend.BoundedRange(uint32(2), uint32(3))
	pubConfig := PublisherConfig{
		DataStoreConfig:       datastore.DataStoreConfig{},
		BufferedStorageConfig: ledgerbackend.BufferedStorageBackendConfig{},
	}
	mockDataStore := new(datastore.MockDataStore)
	appCallback := func(lcm xdr.LedgerCloseMeta) error {
		return nil
	}
	mockDataStore.On("GetFile", mock.Anything, ".config.json").
		Return(io.NopCloser(bytes.NewReader(configManifestJSON(t))), nil).Once()
	mockDataStore.On("ListFilePaths", mock.Anything, datastore.ListFileOptions{}).Return(nil, nil)

	datastoreFactory = func(_ context.Context, _ datastore.DataStoreConfig) (datastore.DataStore, error) {
		return mockDataStore, nil
	}
	assert.ErrorContains(t,
		ApplyLedgerMetadata(ledgerRange, pubConfig, ctx, appCallback),
		"failed to create buffered storage backend")
	mockDataStore.AssertExpectations(t)
}

func TestBSBProducerFnInvalidRange(t *testing.T) {
	ctx := context.Background()
	pubConfig := PublisherConfig{
		DataStoreConfig:       datastore.DataStoreConfig{},
		BufferedStorageConfig: DefaultBufferedStorageBackendConfig(1),
	}
	mockDataStore := new(datastore.MockDataStore)
	mockDataStore.On("GetFile", mock.Anything, ".config.json").
		Return(io.NopCloser(bytes.NewReader(configManifestJSON(t))), nil).Once()
	mockDataStore.On("ListFilePaths", mock.Anything, datastore.ListFileOptions{}).Return(nil, nil)

	appCallback := func(lcm xdr.LedgerCloseMeta) error {
		return nil
	}

	datastoreFactory = func(_ context.Context, _ datastore.DataStoreConfig) (datastore.DataStore, error) {
		return mockDataStore, nil
	}

	assert.ErrorContains(t,
		ApplyLedgerMetadata(ledgerbackend.BoundedRange(uint32(3), uint32(2)), pubConfig, ctx, appCallback),
		"invalid end value for bounded range, must be greater than start")
	mockDataStore.AssertExpectations(t)
}

func TestBSBProducerFnGetLedgerError(t *testing.T) {
	ctx := context.Background()
	pubConfig := PublisherConfig{
		DataStoreConfig:       datastore.DataStoreConfig{},
		BufferedStorageConfig: DefaultBufferedStorageBackendConfig(1),
	}
	// we don't want to let buffer do real retries, force the first error to propagate
	pubConfig.BufferedStorageConfig.RetryLimit = 0
	mockDataStore := new(datastore.MockDataStore)
	mockDataStore.On("GetFile", mock.Anything, ".config.json").
		Return(io.NopCloser(bytes.NewReader(configManifestJSON(t))), nil).Once()

	mockDataStore.On("GetFile", mock.Anything, "FFFFFFFD--2.xdr.zst").Return(nil, os.ErrNotExist).Once()
	// since buffer is multi-worker async, it may get to this on other worker, but not deterministic,
	// don't assert on it
	mockDataStore.On("GetFile", mock.Anything, "FFFFFFFC--3.xdr.zst").Return(makeSingleLCMBatch(3), nil).Maybe()
	mockDataStore.On("ListFilePaths", mock.Anything, datastore.ListFileOptions{}).Return(nil, nil)

	appCallback := func(lcm xdr.LedgerCloseMeta) error {
		return nil
	}

	datastoreFactory = func(_ context.Context, _ datastore.DataStoreConfig) (datastore.DataStore, error) {
		return mockDataStore, nil
	}
	assert.ErrorContains(t,
		ApplyLedgerMetadata(ledgerbackend.BoundedRange(uint32(2), uint32(3)), pubConfig, ctx, appCallback),
		"error getting ledger")

	mockDataStore.AssertExpectations(t)
}

func configManifestJSON(t *testing.T) []byte {
	var expectedManifest = datastore.DatastoreManifest{
		NetworkPassphrase: "passphrase",
		Version:           "1.0",
		Compression:       "xyz",
		LedgersPerFile:    1,
		FilesPerPartition: 1,
	}
	configJSON, err := json.Marshal(expectedManifest)
	require.NoError(t, err)
	return configJSON
}

func TestBSBProducerFnCallbackError(t *testing.T) {
	ctx := context.Background()
	pubConfig := PublisherConfig{
		DataStoreConfig:       datastore.DataStoreConfig{},
		BufferedStorageConfig: DefaultBufferedStorageBackendConfig(1),
	}
	mockDataStore := createMockdataStore(t, 2, 3, 64000)

	appCallback := func(lcm xdr.LedgerCloseMeta) error {
		return errors.New("uhoh")
	}

	datastoreFactory = func(_ context.Context, _ datastore.DataStoreConfig) (datastore.DataStore, error) {
		return mockDataStore, nil
	}
	assert.ErrorContains(t,
		ApplyLedgerMetadata(ledgerbackend.BoundedRange(uint32(2), uint32(3)), pubConfig, ctx, appCallback),
		"received an error from callback invocation")
}

func createMockdataStore(t *testing.T, start, end, partitionSize uint32) *datastore.MockDataStore {
	mockDataStore := new(datastore.MockDataStore)

	var expectedManifest = datastore.DatastoreManifest{
		NetworkPassphrase: "passphrase",
		Version:           "1.0",
		Compression:       "xyz",
		LedgersPerFile:    1,
		FilesPerPartition: partitionSize,
	}
	configJSON, err := json.Marshal(expectedManifest)
	require.NoError(t, err)

	mockDataStore.On("GetFile", mock.Anything, ".config.json").
		Return(io.NopCloser(bytes.NewReader(configJSON)), nil).Once()
	mockDataStore.On("ListFilePaths", mock.Anything, datastore.ListFileOptions{}).Return(nil, nil)

	partition := partitionSize - 1
	for i := start; i <= end; i++ {
		objectName := fmt.Sprintf("FFFFFFFF--0-%d/%08X--%d.xdr.zst", partition, math.MaxUint32-i, i)
		mockDataStore.On("GetFile", mock.Anything, objectName).Return(makeSingleLCMBatch(i), nil).Once()
	}

	t.Cleanup(func() {
		mockDataStore.AssertExpectations(t)
	})

	return mockDataStore
}

func makeSingleLCMBatch(seq uint32) io.ReadCloser {
	lcm := xdr.LedgerCloseMetaBatch{
		StartSequence: xdr.Uint32(seq),
		EndSequence:   xdr.Uint32(seq),
		LedgerCloseMetas: []xdr.LedgerCloseMeta{
			createLedgerCloseMeta(seq),
		},
	}
	encoder := compressxdr.NewXDREncoder(compressxdr.DefaultCompressor, lcm)
	var buf bytes.Buffer
	encoder.WriteTo(&buf)
	capturedBuf := buf.Bytes()
	reader := bytes.NewReader(capturedBuf)
	return io.NopCloser(reader)
}

func createLedgerCloseMeta(ledgerSeq uint32) xdr.LedgerCloseMeta {
	return xdr.LedgerCloseMeta{
		V: int32(0),
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(ledgerSeq),
				},
			},
			TxSet:              xdr.TransactionSet{},
			TxProcessing:       nil,
			UpgradesProcessing: nil,
			ScpInfo:            nil,
		},
		V1: nil,
	}
}
