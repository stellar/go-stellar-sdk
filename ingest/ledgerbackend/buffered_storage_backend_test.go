package ledgerbackend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/support/collections/heap"
	"github.com/stellar/go-stellar-sdk/support/compressxdr"
	"github.com/stellar/go-stellar-sdk/support/datastore"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var partitionSize = uint32(64000)
var ledgerPerFileCount = uint32(1)

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

func createBufferedStorageBackendConfigForTesting() BufferedStorageBackendConfig {
	param := make(map[string]string)
	param["destination_bucket_path"] = "testURL"

	return BufferedStorageBackendConfig{
		BufferSize: 100,
		NumWorkers: 5,
		RetryLimit: 3,
		RetryWait:  time.Microsecond,
	}
}

func createBufferedStorageBackendForTesting() BufferedStorageBackend {
	config := createBufferedStorageBackendConfigForTesting()

	dataStore := new(datastore.MockDataStore)
	return BufferedStorageBackend{
		config:    config,
		dataStore: dataStore,
		schema: datastore.DataStoreSchema{
			LedgersPerFile:    ledgerPerFileCount,
			FilesPerPartition: partitionSize,
			FileExtension:     "zstd",
		},
	}
}

func createMockdataStore(t *testing.T, start, end, partitionSize, count uint32) *datastore.MockDataStore {
	mockDataStore := new(datastore.MockDataStore)
	partition := count*partitionSize - 1

	schema := datastore.DataStoreSchema{
		LedgersPerFile:    count,
		FilesPerPartition: partitionSize,
		FileExtension:     "zstd",
	}

	start = schema.GetSequenceNumberStartBoundary(start)
	end = schema.GetSequenceNumberEndBoundary(end)
	for i := start; i <= end; i = i + count {
		var objectName string
		var readCloser io.ReadCloser
		if count > 1 {
			endFileSeq := i + count - 1
			readCloser = createLCMBatchReader(i, endFileSeq, count)
			objectName = fmt.Sprintf("FFFFFFFF--0-%d/%08X--%d-%d.xdr.zstd", partition, math.MaxUint32-i, i, endFileSeq)
		} else {
			readCloser = createLCMBatchReader(i, i, count)
			objectName = fmt.Sprintf("FFFFFFFF--0-%d/%08X--%d.xdr.zstd", partition, math.MaxUint32-i, i)
		}
		mockDataStore.On("GetFile", mock.Anything, objectName).Return(readCloser, int64(-1), nil).Times(1)
	}

	t.Cleanup(func() {
		mockDataStore.AssertExpectations(t)
	})

	return mockDataStore
}

func createLCMForTesting(start, end uint32) []xdr.LedgerCloseMeta {
	var lcmArray []xdr.LedgerCloseMeta
	for i := start; i <= end; i++ {
		lcmArray = append(lcmArray, createLedgerCloseMeta(i))
	}

	return lcmArray
}

func createTestLedgerCloseMetaBatch(startSeq, endSeq, count uint32) xdr.LedgerCloseMetaBatch {
	var ledgerCloseMetas []xdr.LedgerCloseMeta
	for i := uint32(0); i < count; i++ {
		ledgerCloseMetas = append(ledgerCloseMetas, createLedgerCloseMeta(startSeq+uint32(i)))
	}
	return xdr.LedgerCloseMetaBatch{
		StartSequence:    xdr.Uint32(startSeq),
		EndSequence:      xdr.Uint32(endSeq),
		LedgerCloseMetas: ledgerCloseMetas,
	}
}

func decodeLedgerCloseMetaBatch(data []byte) (xdr.LedgerCloseMetaBatch, error) {
	var batch xdr.LedgerCloseMetaBatch
	err := xdr.SafeUnmarshal(data, &batch)
	return batch, err
}

func createLCMBatchReader(start, end, count uint32) io.ReadCloser {
	testData := createTestLedgerCloseMetaBatch(start, end, count)
	encoder := compressxdr.NewXDREncoder(compressxdr.DefaultCompressor, testData)
	var buf bytes.Buffer
	encoder.WriteTo(&buf)
	capturedBuf := buf.Bytes()
	reader := bytes.NewReader(capturedBuf)
	return io.NopCloser(reader)
}

func TestNewBufferedStorageBackend(t *testing.T) {
	config := createBufferedStorageBackendConfigForTesting()
	mockDataStore := new(datastore.MockDataStore)
	bsb, err := NewBufferedStorageBackend(config, mockDataStore, datastore.DataStoreSchema{
		LedgersPerFile:    1,
		FilesPerPartition: 64000,
	})

	assert.NoError(t, err)
	assert.Equal(t, bsb.dataStore, mockDataStore)
	assert.Equal(t, uint32(1), bsb.schema.LedgersPerFile)
	assert.Equal(t, uint32(64000), bsb.schema.FilesPerPartition)
	assert.Equal(t, uint32(100), bsb.config.BufferSize)
	assert.Equal(t, uint32(5), bsb.config.NumWorkers)
	assert.Equal(t, uint32(3), bsb.config.RetryLimit)
	assert.Equal(t, time.Microsecond, bsb.config.RetryWait)
}

func TestNewLedgerBuffer(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(7)
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 2
	bsb.config.BufferSize = 5
	ledgerRange := BoundedRange(startLedger, endLedger)
	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	ledgerBuffer, err := bsb.newLedgerBuffer(ledgerRange)
	assert.Eventually(t, func() bool { return len(ledgerBuffer.ledgerQueue) == 5 }, time.Second*5, time.Millisecond*50)
	assert.NoError(t, err)

	latestSeq, err := ledgerBuffer.getLatestLedgerSequence()
	assert.NoError(t, err)
	assert.Equal(t, uint32(7), latestSeq)
	assert.Equal(t, ledgerRange, ledgerBuffer.ledgerRange)
}

func TestNewLedgerBufferSizeLessThanRangeSize(t *testing.T) {
	startLedger := uint32(10)
	endLedger := uint32(30)
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 2
	bsb.config.BufferSize = 10
	ledgerRange := BoundedRange(startLedger, endLedger)
	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	ledgerBuffer, err := bsb.newLedgerBuffer(ledgerRange)
	assert.Eventually(t, func() bool { return len(ledgerBuffer.ledgerQueue) == 10 }, time.Second*1, time.Millisecond*50)
	assert.NoError(t, err)

	for i := startLedger; i <= endLedger; i++ {
		compressed, err := ledgerBuffer.getFromLedgerQueue(context.Background())
		assert.NoError(t, err)
		batchBytes, err := ledgerBuffer.decompress(compressed)
		assert.NoError(t, err)
		lcm, err := decodeLedgerCloseMetaBatch(batchBytes)
		assert.NoError(t, err)
		assert.Equal(t, xdr.Uint32(i), lcm.StartSequence)
	}
	assert.Equal(t, ledgerRange, ledgerBuffer.ledgerRange)
}

func TestNewLedgerBufferSizeLargerThanRangeSize(t *testing.T) {
	startLedger := uint32(1)
	endLedger := uint32(15)
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 2
	bsb.config.BufferSize = 100
	ledgerRange := BoundedRange(startLedger, endLedger)
	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	ledgerBuffer, err := bsb.newLedgerBuffer(ledgerRange)
	assert.Eventually(t, func() bool { return len(ledgerBuffer.ledgerQueue) == 15 }, time.Second*1, time.Millisecond*50)
	assert.NoError(t, err)

	for i := startLedger; i <= endLedger; i++ {
		compressed, err := ledgerBuffer.getFromLedgerQueue(context.Background())
		assert.NoError(t, err)
		batchBytes, err := ledgerBuffer.decompress(compressed)
		assert.NoError(t, err)
		lcm, err := decodeLedgerCloseMetaBatch(batchBytes)
		assert.NoError(t, err)
		assert.Equal(t, xdr.Uint32(i), lcm.StartSequence)
	}
	assert.Equal(t, ledgerRange, ledgerBuffer.ledgerRange)
}

func TestBSBGetLatestLedgerSequence(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)
	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 3 }, time.Second*5, time.Millisecond*50)

	latestSeq, err := bsb.GetLatestLedgerSequence(ctx)
	assert.NoError(t, err)

	assert.Equal(t, uint32(5), latestSeq)
}

func TestBSBGetLedger_SingleLedgerPerFile(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	lcmArray := createLCMForTesting(startLedger, endLedger)
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 3 }, time.Second*5, time.Millisecond*50)

	lcm, err := bsb.GetLedger(ctx, uint32(3))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[0], lcm)
	lcm, err = bsb.GetLedger(ctx, uint32(4))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[1], lcm)
	lcm, err = bsb.GetLedger(ctx, uint32(5))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[2], lcm)
}

// TestBufferedStorageStream exercises the LedgerStream API end to end over a
// mocked datastore (via the openStore seam): RawLedgers builds a backend,
// prepares the range, and yields each ledger's raw bytes (the lock-free
// getLedgerRaw borrow) in order.
func TestBufferedStorageStream(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	lcmArray := createLCMForTesting(startLedger, endLedger)
	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	// The stream owns the datastore lifecycle and closes it on teardown.
	mockDataStore.On("Close").Return(nil).Once()
	schema := datastore.DataStoreSchema{
		LedgersPerFile:    ledgerPerFileCount,
		FilesPerPartition: partitionSize,
		FileExtension:     "zstd",
	}

	s := &bufferedStorageStream{
		config: createBufferedStorageBackendConfigForTesting(),
		log:    log.New(),
		openStore: func(context.Context) (datastore.DataStore, datastore.DataStoreSchema, error) {
			return mockDataStore, schema, nil
		},
	}

	seq := startLedger
	for raw, err := range s.RawLedgers(ctx, BoundedRange(startLedger, endLedger)) {
		require.NoError(t, err)
		require.NotEmpty(t, raw)

		// raw is a borrow; decode it to verify it's the expected ledger.
		var lcm xdr.LedgerCloseMeta
		require.NoError(t, xdr.SafeUnmarshal(raw, &lcm))
		assert.Equal(t, lcmArray[seq-startLedger], lcm)
		seq++
	}
	assert.Equal(t, endLedger+1, seq, "stream should yield every ledger in range")
}

// TestBSBGetLedger_IdempotentReRequest verifies that requesting the
// most-recently-served sequence again returns the same ledger without
// erroring or advancing state, matching CaptiveStellarCore's behavior.
func TestBSBGetLedger_IdempotentReRequest(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	lcmArray := createLCMForTesting(startLedger, endLedger)
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 3 }, time.Second*5, time.Millisecond*50)

	// First call serves the ledger and advances state.
	first, err := bsb.GetLedger(ctx, startLedger)
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[0], first)

	// Second call for the same sequence returns the same ledger without erroring.
	second, err := bsb.GetLedger(ctx, startLedger)
	assert.NoError(t, err)
	assert.Equal(t, first, second)

	// Forward progress still works after re-requests.
	next, err := bsb.GetLedger(ctx, startLedger+1)
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[1], next)
}

func TestCloudStorageGetLedger_MultipleLedgerPerFile(t *testing.T) {
	startLedger := uint32(6)
	endLedger := uint32(17)
	lcmArray := createLCMForTesting(startLedger, endLedger)
	bsb := createBufferedStorageBackendForTesting()
	ctx := context.Background()
	ledgerRange := BoundedRange(startLedger, endLedger)

	bsb.schema.LedgersPerFile = 4
	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, 4)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 4 }, time.Second*5, time.Millisecond*50)

	for i := 0; i <= int(endLedger-startLedger); i++ {
		lcm, err := bsb.GetLedger(ctx, startLedger+uint32(i))
		assert.NoError(t, err)
		assert.Equal(t, lcmArray[i], lcm)
	}
}

func TestBSBGetLedger_ErrorPreceedingLedger(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	lcmArray := createLCMForTesting(startLedger, endLedger)
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 3 }, time.Second*5, time.Millisecond*50)

	lcm, err := bsb.GetLedger(ctx, uint32(3))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[0], lcm)

	_, err = bsb.GetLedger(ctx, uint32(2))
	assert.EqualError(t, err, "requested sequence 2 precedes current LedgerRange [3, 5]")
}

func TestBSBGetLedger_NotPrepared(t *testing.T) {
	bsb := createBufferedStorageBackendForTesting()
	ctx := context.Background()

	_, err := bsb.GetLedger(ctx, uint32(3))
	assert.EqualError(t, err, "session is not prepared, call PrepareRange first")
}

func TestBSBGetLedger_SequenceNotInBatch(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 3 }, time.Second*5, time.Millisecond*50)

	_, err := bsb.GetLedger(ctx, uint32(2))
	assert.EqualError(t, err, "requested sequence 2 precedes current LedgerRange [3, 5]")

	_, err = bsb.GetLedger(ctx, uint32(6))
	assert.EqualError(t, err, "requested sequence 6 beyond current LedgerRange [3, 5]")
}

func TestBSBPrepareRange(t *testing.T) {
	startLedger := uint32(2)
	endLedger := uint32(3)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 2 }, time.Second*5, time.Millisecond*50)

	assert.NotNil(t, bsb.prepared)

	// check alreadyPrepared
	err := bsb.PrepareRange(ctx, ledgerRange)
	assert.NoError(t, err)
	assert.NotNil(t, bsb.prepared)
}

func TestBSBIsPrepared_Bounded(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 3 }, time.Second*5, time.Millisecond*50)

	ok, err := bsb.IsPrepared(ctx, ledgerRange)
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = bsb.IsPrepared(ctx, BoundedRange(2, 4))
	assert.NoError(t, err)
	assert.False(t, ok)

	ok, err = bsb.IsPrepared(ctx, UnboundedRange(3))
	assert.NoError(t, err)
	assert.False(t, ok)

	ok, err = bsb.IsPrepared(ctx, UnboundedRange(2))
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestBSBIsPrepared_Unbounded(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(8)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 2
	bsb.config.BufferSize = 5
	ledgerRange := UnboundedRange(3)
	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 5 }, time.Second*5, time.Millisecond*50)

	ok, err := bsb.IsPrepared(ctx, ledgerRange)
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = bsb.IsPrepared(ctx, BoundedRange(3, 4))
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = bsb.IsPrepared(ctx, BoundedRange(2, 4))
	assert.NoError(t, err)
	assert.False(t, ok)

	ok, err = bsb.IsPrepared(ctx, UnboundedRange(4))
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = bsb.IsPrepared(ctx, UnboundedRange(2))
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestBSBClose(t *testing.T) {
	startLedger := uint32(2)
	endLedger := uint32(3)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 2 }, time.Second*5, time.Millisecond*50)

	err := bsb.Close()
	assert.NoError(t, err)
	assert.Equal(t, true, bsb.closed)

	_, err = bsb.GetLatestLedgerSequence(ctx)
	assert.EqualError(t, err, "BufferedStorageBackend is closed; cannot GetLatestLedgerSequence")

	_, err = bsb.GetLedger(ctx, 3)
	assert.EqualError(t, err, "BufferedStorageBackend is closed; cannot GetLedger")

	err = bsb.PrepareRange(ctx, ledgerRange)
	assert.EqualError(t, err, "BufferedStorageBackend is closed; cannot PrepareRange")

	_, err = bsb.IsPrepared(ctx, ledgerRange)
	assert.EqualError(t, err, "BufferedStorageBackend is closed; cannot IsPrepared")
}

func TestLedgerBufferInvariant(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(6)
	ctx := context.Background()
	lcmArray := createLCMForTesting(startLedger, endLedger)
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 2
	bsb.config.BufferSize = 2
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 2 }, time.Second*5, time.Millisecond*50)

	// Buffer should have hit the BufferSize limit
	assert.Equal(t, 2, len(bsb.ledgerBuffer.ledgerQueue))

	lcm, err := bsb.GetLedger(ctx, uint32(3))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[0], lcm)
	lcm, err = bsb.GetLedger(ctx, uint32(4))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[1], lcm)

	// Buffer should fill up with remaining ledgers
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 2 }, time.Second*5, time.Millisecond*50)
	assert.Equal(t, 2, len(bsb.ledgerBuffer.ledgerQueue))

	lcm, err = bsb.GetLedger(ctx, uint32(5))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[2], lcm)

	// Buffer should only have the final ledger
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 1 }, time.Second*5, time.Millisecond*50)
	assert.Equal(t, 1, len(bsb.ledgerBuffer.ledgerQueue))

	lcm, err = bsb.GetLedger(ctx, uint32(6))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[3], lcm)

	// Buffer should be empty
	assert.Equal(t, 0, len(bsb.ledgerBuffer.ledgerQueue))
}

func TestLedgerBufferClose(t *testing.T) {
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 1
	bsb.config.BufferSize = 5
	ledgerRange := UnboundedRange(3)

	mockDataStore := new(datastore.MockDataStore)
	partition := ledgerPerFileCount*partitionSize - 1

	objectName := fmt.Sprintf("FFFFFFFF--0-%d/%08X--%d.xdr.zstd", partition, math.MaxUint32-3, 3)
	afterPrepareRange := make(chan struct{})
	mockDataStore.On("GetFile", mock.Anything, objectName).Return(io.NopCloser(&bytes.Buffer{}), int64(-1), context.Canceled).Run(func(args mock.Arguments) {
		<-afterPrepareRange
		go bsb.ledgerBuffer.close()
	}).Once()

	t.Cleanup(func() {
		mockDataStore.AssertExpectations(t)
	})

	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	close(afterPrepareRange)

	bsb.ledgerBuffer.wg.Wait()

	_, err := bsb.GetLedger(ctx, 3)
	assert.EqualError(t, err, "failed getting next ledger batch from queue: context canceled")
}

func TestLedgerBufferBoundedObjectNotFound(t *testing.T) {
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 1
	bsb.config.BufferSize = 5
	ledgerRange := BoundedRange(3, 5)

	mockDataStore := new(datastore.MockDataStore)
	partition := ledgerPerFileCount*partitionSize - 1

	objectName := fmt.Sprintf("FFFFFFFF--0-%d/%08X--%d.xdr.zstd", partition, math.MaxUint32-3, 3)
	mockDataStore.On("GetFile", mock.Anything, objectName).Return(io.NopCloser(&bytes.Buffer{}), int64(0), os.ErrNotExist).Once()
	t.Cleanup(func() {
		mockDataStore.AssertExpectations(t)
	})

	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))

	bsb.ledgerBuffer.wg.Wait()

	_, err := bsb.GetLedger(ctx, 3)
	assert.ErrorContains(t, err, "ledger object containing sequence 3 is missing")
	assert.ErrorContains(t, err, objectName)
	assert.ErrorContains(t, err, "file does not exist")
}

func TestLedgerBufferUnboundedObjectNotFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 1
	bsb.config.BufferSize = 5
	ledgerRange := UnboundedRange(3)

	mockDataStore := new(datastore.MockDataStore)
	partition := ledgerPerFileCount*partitionSize - 1

	objectName := fmt.Sprintf("FFFFFFFF--0-%d/%08X--%d.xdr.zstd", partition, math.MaxUint32-3, 3)
	iteration := &atomic.Int32{}
	cancelAfter := int32(bsb.config.RetryLimit) + 2
	mockDataStore.On("GetFile", mock.Anything, objectName).Return(io.NopCloser(&bytes.Buffer{}), int64(0), os.ErrNotExist).Run(func(args mock.Arguments) {
		if iteration.Load() >= cancelAfter {
			cancel()
		}
		iteration.Add(1)
	})
	t.Cleanup(func() {
		mockDataStore.AssertExpectations(t)
	})

	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))

	_, err := bsb.GetLedger(ctx, 3)
	assert.EqualError(t, err, "failed getting next ledger batch from queue: context canceled")
	assert.GreaterOrEqual(t, iteration.Load(), cancelAfter)
	assert.NoError(t, bsb.Close())
}

func TestLedgerBufferRetryLimit(t *testing.T) {
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 1
	bsb.config.BufferSize = 5
	ledgerRange := UnboundedRange(3)

	mockDataStore := new(datastore.MockDataStore)
	partition := ledgerPerFileCount*partitionSize - 1

	objectName := fmt.Sprintf("FFFFFFFF--0-%d/%08X--%d.xdr.zstd", partition, math.MaxUint32-3, 3)
	mockDataStore.On("GetFile", mock.Anything, objectName).
		Return(io.NopCloser(&bytes.Buffer{}), int64(-1), fmt.Errorf("transient error")).
		Times(int(bsb.config.RetryLimit) + 1)
	t.Cleanup(func() {
		mockDataStore.AssertExpectations(t)
	})

	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(context.Background(), ledgerRange))

	bsb.ledgerBuffer.wg.Wait()

	_, err := bsb.GetLedger(context.Background(), 3)
	assert.ErrorContains(t, err, "failed getting next ledger batch from queue")
	assert.ErrorContains(t, err, "maximum retries exceeded for downloading object containing sequence 3")
	assert.ErrorContains(t, err, objectName)
	assert.ErrorContains(t, err, "transient error")
}

// createSizedMockDataStore reports each object's real compressed size instead
// of -1, which is what GCS, S3 and the filesystem store all do — that is the
// branch taking io.ReadFull and compressedPool.Get.
func createSizedMockDataStore(t *testing.T, start, end, partitionSize, count uint32) *datastore.MockDataStore {
	return newMockDataStore(t, start, end, partitionSize, count, true)
}

func newMockDataStore(t *testing.T, start, end, partitionSize, count uint32, reportSize bool) *datastore.MockDataStore {
	mockDataStore := new(datastore.MockDataStore)
	partition := count*partitionSize - 1
	schema := datastore.DataStoreSchema{
		LedgersPerFile:    count,
		FilesPerPartition: partitionSize,
		FileExtension:     "zstd",
	}
	start = schema.GetSequenceNumberStartBoundary(start)
	end = schema.GetSequenceNumberEndBoundary(end)
	for i := start; i <= end; i = i + count {
		var objectName string
		var payload []byte
		if count > 1 {
			endFileSeq := i + count - 1
			payload = createLCMBatchBytes(i, endFileSeq, count)
			objectName = fmt.Sprintf("FFFFFFFF--0-%d/%08X--%d-%d.xdr.zstd", partition, math.MaxUint32-i, i, endFileSeq)
		} else {
			payload = createLCMBatchBytes(i, i, count)
			objectName = fmt.Sprintf("FFFFFFFF--0-%d/%08X--%d.xdr.zstd", partition, math.MaxUint32-i, i)
		}
		size := int64(-1)
		if reportSize {
			size = int64(len(payload))
		}
		mockDataStore.On("GetFile", mock.Anything, objectName).
			Return(io.NopCloser(bytes.NewReader(payload)), size, nil).Times(1)
	}
	t.Cleanup(func() { mockDataStore.AssertExpectations(t) })
	return mockDataStore
}

func createLCMBatchBytes(start, end, count uint32) []byte {
	testData := createTestLedgerCloseMetaBatch(start, end, count)
	encoder := compressxdr.NewXDREncoder(compressxdr.DefaultCompressor, testData)
	var buf bytes.Buffer
	if _, err := encoder.WriteTo(&buf); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// TestBufferBytesDepth covers the byte cap across its regimes in one table: off,
// binding, and tighter than a single object. Every row drains the full range in
// order — a cap throttles, it never drops, reorders or stalls — and pins the
// resulting depth.
func TestBufferBytesDepth(t *testing.T) {
	for _, tc := range []struct {
		name        string
		numWorkers  uint32
		bufferSize  uint32
		bufferBytes int64
		sized       bool
		wantPeak    int64
	}{
		{"off: prior depth of BufferSize+1", 2, 5, 0, false, 6},
		{"off, sized store: prior depth", 2, 5, 0, true, 6},
		{"one byte: floors at the minimum depth, never 0", 2, 50, 1, true, 2},
		{"one byte, unsized store", 2, 50, 1, false, 2},
		{"generous cap does not bind", 3, 8, 1 << 20, true, 9},
		// The regime production runs in: strictly between the worker floor and
		// the object bound. Objects are ~46 bytes of pool capacity here.
		{"intermediate cap sets the depth", 2, 50, 200, true, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			startLedger, endLedger := uint32(3), uint32(30)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			lcmArray := createLCMForTesting(startLedger, endLedger)
			bsb := createBufferedStorageBackendForTesting()
			bsb.config.NumWorkers = tc.numWorkers
			bsb.config.BufferSize = tc.bufferSize
			bsb.config.BufferBytes = tc.bufferBytes
			if tc.sized {
				bsb.dataStore = createSizedMockDataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
			} else {
				bsb.dataStore = createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
			}

			require.NoError(t, bsb.PrepareRange(ctx, BoundedRange(startLedger, endLedger)))
			defer bsb.Close()

			var peak int64
			for i, seq := 0, startLedger; seq <= endLedger; i, seq = i+1, seq+1 {
				lcm, err := bsb.GetLedger(ctx, seq)
				require.NoError(t, err, "ledger %d", seq)
				assert.Equal(t, lcmArray[i], lcm)
				if d := bsb.ledgerBuffer.outstanding.Load(); d > peak {
					peak = d
				}
			}
			assert.Equal(t, tc.wantPeak, peak, "steady-state depth")
		})
	}
}

func TestNegativeBufferBytesRejected(t *testing.T) {
	config := createBufferedStorageBackendConfigForTesting()
	config.BufferBytes = -1
	_, err := NewBufferedStorageBackend(config, new(datastore.MockDataStore), datastore.DataStoreSchema{
		LedgersPerFile: 1, FilesPerPartition: 64000,
	})
	assert.ErrorContains(t, err, "buffer bytes must be >= 0")
}

// TestBufferBytesThroughConstructor pins that the field survives the public
// entry point. Every other test hand-builds the struct, so a constructor that
// silently zeroed BufferBytes would go unnoticed.
func TestBufferBytesThroughConstructor(t *testing.T) {
	config := createBufferedStorageBackendConfigForTesting()
	config.BufferBytes = 4242
	bsb, err := NewBufferedStorageBackend(config, new(datastore.MockDataStore), datastore.DataStoreSchema{
		LedgersPerFile: 1, FilesPerPartition: 64000,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 4242, bsb.config.BufferBytes)
}

// TestCompressedPoolSizedByWorkers pins the pool bound. Get discards only
// undersized buffers, so a pool sized by BufferSize accumulates BufferSize
// tip-sized buffers over a range spanning object growth.
func TestCompressedPoolSizedByWorkers(t *testing.T) {
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 2
	bsb.config.BufferSize = 500
	bsb.dataStore = createMockdataStore(t, 3, 30, partitionSize, ledgerPerFileCount)
	ctx := context.Background()
	require.NoError(t, bsb.PrepareRange(ctx, BoundedRange(3, 30)))
	defer bsb.Close()

	// Sized by NumWorkers, not by the range-clamped BufferSize (28 here).
	assert.EqualValues(t, bsb.config.NumWorkers+1, cap(bsb.ledgerBuffer.compressedPool.ch),
		"pool must track concurrency, not queue depth")

	for seq := uint32(3); seq <= 30; seq++ {
		_, err := bsb.GetLedger(ctx, seq)
		require.NoError(t, err)
	}
}

// TestDepthTracksCurrentObjectSize is the guard for the failure this feature
// exists to prevent: objects grow ~700x across pubnet history, so a depth
// derived from a stale size does not adapt and memory follows the object size
// instead of the budget. Every other test uses one fixture size, so a lastSize
// frozen at the first object passes all of them.
func TestDepthTracksCurrentObjectSize(t *testing.T) {
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 4 // >1, so the pre-arrival floor is a real assertion
	bsb.config.BufferSize = 1000
	bsb.config.BufferBytes = 1200

	newBuf := func() *ledgerBuffer {
		return &ledgerBuffer{
			config:              bsb.config,
			context:             context.Background(),
			ledgerQueue:         make(chan []byte, 8),
			ledgerPriorityQueue: heap.New(func(a, b ledgerBatchObject) bool { return a.startLedger < b.startLedger }, 8),
			currentLedger:       3,
			ledgerRange:         BoundedRange(3, 8),
		}
	}

	lb := newBuf()
	lb.schema.LedgersPerFile = 1
	assert.EqualValues(t, bsb.config.NumWorkers, lb.depthLimit(),
		"before anything arrives, depth is the worker floor")

	require.True(t, lb.storeObject(context.Background(), make([]byte, 10, 100), 3))
	assert.EqualValues(t, 12, lb.depthLimit(), "1200/100")

	require.True(t, lb.storeObject(context.Background(), make([]byte, 10, 400), 4))
	assert.EqualValues(t, 3, lb.depthLimit(),
		"1200/400 — depth must follow the LATEST object, not the first")

	require.True(t, lb.storeObject(context.Background(), make([]byte, 10, 2000), 5))
	assert.EqualValues(t, 1, lb.depthLimit(),
		"an object larger than the whole budget floors the depth at 1")
}
