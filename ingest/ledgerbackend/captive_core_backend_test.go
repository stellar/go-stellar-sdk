package ledgerbackend

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TODO: test frame decoding
// TODO: test from static base64-encoded data

type stellarCoreRunnerMock struct {
	mock.Mock
}

func (m *stellarCoreRunnerMock) context() context.Context {
	a := m.Called()
	return a.Get(0).(context.Context)
}

func (m *stellarCoreRunnerMock) catchup(from, to uint32) error {
	a := m.Called(from, to)
	return a.Error(0)
}

func (m *stellarCoreRunnerMock) runFrom(from uint32) error {
	a := m.Called(from)
	return a.Error(0)
}

func (m *stellarCoreRunnerMock) getMetaPipe() (<-chan metaResult, bool) {
	a := m.Called()
	return a.Get(0).(<-chan metaResult), a.Bool(1)
}

func (m *stellarCoreRunnerMock) getProcessExitError() (error, bool) {
	a := m.Called()
	return a.Error(0), a.Bool(1)
}

func (m *stellarCoreRunnerMock) close() error {
	a := m.Called()
	return a.Error(0)
}

func buildLedgerCloseMeta(header testLedgerHeader) xdr.LedgerCloseMeta {
	opResults := []xdr.OperationResult{}
	opMeta := []xdr.OperationMeta{}

	tmpHash, _ := hex.DecodeString("cde54da3901f5b9c0331d24fbb06ac9c5c5de76de9fb2d4a7b86c09e46f11d8c")
	var hash [32]byte
	copy(hash[:], tmpHash)

	var ledgerHash [32]byte
	if header.hash != "" {
		tmpHash, err := hex.DecodeString(header.hash)
		if err != nil {
			panic(err)
		}
		copy(ledgerHash[:], tmpHash)
	}

	var previousLedgerHash [32]byte
	if header.hash != "" {
		tmpHash, err := hex.DecodeString(header.previousLedgerHash)
		if err != nil {
			panic(err)
		}
		copy(previousLedgerHash[:], tmpHash)
	}

	source := xdr.MustAddress("GAEJJMDDCRYF752PKIJICUVL7MROJBNXDV2ZB455T7BAFHU2LCLSE2LW")
	return xdr.LedgerCloseMeta{
		V: 0,
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Hash: ledgerHash,
				Header: xdr.LedgerHeader{
					LedgerSeq:          xdr.Uint32(header.sequence),
					PreviousLedgerHash: previousLedgerHash,
				},
			},
			TxSet: xdr.TransactionSet{
				Txs: []xdr.TransactionEnvelope{
					{
						Type: xdr.EnvelopeTypeEnvelopeTypeTx,
						V1: &xdr.TransactionV1Envelope{
							Tx: xdr.Transaction{
								SourceAccount: source.ToMuxedAccount(),
								Fee:           xdr.Uint32(header.sequence),
							},
						},
					},
				},
			},
			TxProcessing: []xdr.TransactionResultMeta{
				{
					Result: xdr.TransactionResultPair{
						TransactionHash: xdr.Hash(hash),
						Result: xdr.TransactionResult{
							FeeCharged: xdr.Int64(header.sequence),
							Result: xdr.TransactionResultResult{
								Code:    xdr.TransactionResultCodeTxSuccess,
								Results: &opResults,
							},
						},
					},
					TxApplyProcessing: xdr.TransactionMeta{
						Operations: &opMeta,
					},
				},
			},
		},
	}

}

type testLedgerHeader struct {
	sequence           uint32
	hash               string
	previousLedgerHash string
}

func TestCaptiveNew(t *testing.T) {
	storagePath := t.TempDir()

	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header["User-Agent"][0]
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executablePath := "/etc/stellar-core"
	networkPassphrase := network.PublicNetworkPassphrase
	historyURLs := []string{server.URL}

	captiveStellarCore, err := NewCaptive(
		CaptiveCoreConfig{
			BinaryPath:            executablePath,
			NetworkPassphrase:     networkPassphrase,
			HistoryArchiveURLs:    historyURLs,
			StoragePath:           storagePath,
			UserAgent:             "uatest",
			CoreProtocolVersionFn: func(string) (uint, error) { return 23, nil },
		},
	)

	assert.NoError(t, err)
	assert.Equal(t, uint32(0), captiveStellarCore.nextLedger)
	assert.NotNil(t, captiveStellarCore.archive)
	_, err = captiveStellarCore.archive.BucketExists(historyarchive.EmptyXdrArrayHash())
	assert.NoError(t, err)
	assert.Equal(t, "uatest", userAgent)
}

func TestCaptiveNewUnsupportedProtocolVersion(t *testing.T) {
	storagePath := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executablePath := "/etc/stellar-core"
	networkPassphrase := network.PublicNetworkPassphrase
	historyURLs := []string{server.URL}

	_, err := NewCaptive(
		CaptiveCoreConfig{
			BinaryPath:            executablePath,
			NetworkPassphrase:     networkPassphrase,
			HistoryArchiveURLs:    historyURLs,
			StoragePath:           storagePath,
			UserAgent:             "uatest",
			CoreProtocolVersionFn: func(string) (uint, error) { return 22, nil },
		},
	)

	assert.EqualError(t, err, "stellar-core version not supported. Installed stellar-core version is at protocol 22, but minimum required version is 23. Please upgrade stellar-core to a version that supports protocol version 23 or higher")
}

func TestCaptivePrepareRange(t *testing.T) {
	metaChan := make(chan metaResult, 100)

	// Core will actually start with the last checkpoint before the from ledger
	// and then rewind to the `from` ledger.
	for i := 64; i <= 100; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}

	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("catchup", uint32(100), uint32(200)).Return(nil).Once()
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	cancelCalled := false
	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
		cancel: context.CancelFunc(func() {
			cancelCalled = true
		}),
	}
	captiveBackend.registerMetrics(prometheus.NewRegistry(), "test")

	err := captiveBackend.PrepareRange(ctx, BoundedRange(100, 200))
	assert.NoError(t, err)
	mockRunner.On("close").Return(nil).Once()
	err = captiveBackend.Close()
	assert.NoError(t, err)
	assert.True(t, cancelCalled)
	mockRunner.AssertExpectations(t)
	mockArchive.AssertExpectations(t)

	assert.Equal(t, uint64(0), getStartDurationMetric(captiveBackend).GetSampleCount())
}

func TestCaptivePrepareRangeCrash(t *testing.T) {
	metaChan := make(chan metaResult)
	close(metaChan)
	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("catchup", uint32(100), uint32(200)).Return(nil).Once()
	mockRunner.On("getProcessExitError").Return(errors.New("exit code -1"), true)
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("close").Return(nil).Once()
	mockRunner.On("context").Return(ctx)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, BoundedRange(100, 200))
	assert.EqualError(t, err, "Error fast-forwarding to 100: stellar core exited unexpectedly: exit code -1")
	mockRunner.AssertExpectations(t)
	mockArchive.AssertExpectations(t)
}

func TestCaptivePrepareRangeTerminated(t *testing.T) {
	metaChan := make(chan metaResult, 100)

	// Core will actually start with the last checkpoint before the from ledger
	// and then rewind to the `from` ledger.
	for i := 64; i <= 100; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}
	close(metaChan)
	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("catchup", uint32(100), uint32(200)).Return(nil).Once()
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, BoundedRange(100, 200))
	assert.NoError(t, err)
	mockRunner.AssertExpectations(t)
	mockArchive.AssertExpectations(t)
}

func TestCaptivePrepareRangeCloseNotFullyTerminated(t *testing.T) {
	metaChan := make(chan metaResult, 100)
	for i := 64; i <= 100; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("catchup", uint32(100), uint32(200)).Return(nil).Twice()
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)
	mockRunner.On("close").Return(nil)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, BoundedRange(100, 200))
	assert.NoError(t, err)

	// Simulates a long (but graceful) shutdown...
	cancel()

	err = captiveBackend.PrepareRange(ctx, BoundedRange(100, 200))
	assert.NoError(t, err)

	mockRunner.AssertExpectations(t)
	mockArchive.AssertExpectations(t)
}

func TestCaptivePrepareRange_ErrClosingSession(t *testing.T) {
	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("close").Return(fmt.Errorf("transient error"))
	mockRunner.On("getProcessExitError").Return(nil, false)
	mockRunner.On("context").Return(ctx)

	captiveBackend := &CaptiveStellarCore{
		nextLedger:        300,
		stellarCoreRunner: mockRunner,
	}

	err := captiveBackend.PrepareRange(ctx, BoundedRange(100, 200))
	assert.EqualError(t, err, "error starting prepare range: error closing existing session: transient error")

	err = captiveBackend.PrepareRange(ctx, UnboundedRange(64))
	assert.EqualError(t, err, "error starting prepare range: error closing existing session: transient error")

	mockRunner.AssertExpectations(t)
}

func TestCaptivePrepareRange_ErrGettingRootHAS(t *testing.T) {
	ctx := context.Background()
	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{}, errors.New("transient error"))

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
	}

	err := captiveBackend.PrepareRange(ctx, BoundedRange(100, 200))
	assert.EqualError(t, err, "error starting prepare range: opening subprocess: error getting latest checkpoint sequence: error getting root HAS: transient error")

	err = captiveBackend.PrepareRange(ctx, UnboundedRange(100))
	assert.EqualError(t, err, "error starting prepare range: opening subprocess: error calculating ledger and hash for stellar-core run: error getting latest checkpoint sequence: error getting root HAS: transient error")

	mockArchive.AssertExpectations(t)
}

func TestCaptivePrepareRange_FromIsAheadOfRootHAS(t *testing.T) {
	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(64),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, BoundedRange(100, 200))
	assert.ErrorIs(t, err, ErrCannotCatchupAheadLatestCheckpoint)
	assert.ErrorContains(t, err, "to sequence: 200 is greater than max available in history archives: 64")

	err = captiveBackend.PrepareRange(ctx, UnboundedRange(193))
	assert.EqualError(t, err, "error starting prepare range: opening subprocess: error calculating ledger and hash for stellar-core run: trying to start online mode too far (latest checkpoint=64), only two checkpoints in the future allowed")

	metaChan := make(chan metaResult, 100)
	meta := buildLedgerCloseMeta(testLedgerHeader{sequence: 100})
	metaChan <- metaResult{
		LedgerCloseMeta: &meta,
	}
	mockRunner.On("runFrom", uint32(99)).Return(nil).Once()
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)

	assert.NoError(t, captiveBackend.PrepareRange(ctx, UnboundedRange(100)))

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestCaptivePrepareRange_ToIsAheadOfRootHAS(t *testing.T) {
	mockRunner := &stellarCoreRunnerMock{}
	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(192),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(context.Background(), BoundedRange(100, 200))
	assert.ErrorIs(t, err, ErrCannotCatchupAheadLatestCheckpoint)
	assert.ErrorContains(t, err, "to sequence: 200 is greater than max available in history archives: 192")

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestCaptivePrepareRange_ErrCatchup(t *testing.T) {
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("catchup", uint32(100), uint32(192)).Return(errors.New("transient error")).Once()

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(192),
		}, nil)

	ctx := context.Background()
	cancelCalled := false
	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		cancel: context.CancelFunc(func() {
			cancelCalled = true
		}),
	}

	err := captiveBackend.PrepareRange(ctx, BoundedRange(100, 192))
	assert.EqualError(t, err, "error starting prepare range: opening subprocess: error running stellar-core: transient error")

	// make sure we can Close without errors
	assert.NoError(t, captiveBackend.Close())
	assert.True(t, cancelCalled)

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestCaptivePrepareRangeUnboundedRange_ErrRunFrom(t *testing.T) {
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("runFrom", uint32(127)).Return(errors.New("transient error")).Once()

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(127),
		}, nil)

	ctx := context.Background()
	cancelCalled := false
	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
		cancel: context.CancelFunc(func() {
			cancelCalled = true
		}),
	}
	captiveBackend.registerMetrics(prometheus.NewRegistry(), "test")

	err := captiveBackend.PrepareRange(ctx, UnboundedRange(128))
	assert.EqualError(t, err, "error starting prepare range: opening subprocess: error running stellar-core: transient error")

	assert.Equal(t, uint64(0), getStartDurationMetric(captiveBackend).GetSampleCount())

	// make sure we can Close without errors
	assert.NoError(t, captiveBackend.Close())
	assert.True(t, cancelCalled)

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func getStartDurationMetric(captiveCore *CaptiveStellarCore) *dto.Summary {
	value := &dto.Metric{}
	err := captiveCore.captiveCoreStartDuration.Write(value)
	if err != nil {
		panic(err)
	}
	return value.GetSummary()
}

func TestCaptivePrepareRangeUnboundedRange_ReuseSession(t *testing.T) {
	metaChan := make(chan metaResult, 100)

	// Core will actually start with the last checkpoint before the from ledger
	// and then rewind to the `from` ledger.
	for i := 2; i <= 65; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}

	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("runFrom", uint32(64)).Return(nil).Once()
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)
	mockRunner.On("getProcessExitError").Return(nil, false)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(129),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}
	captiveBackend.registerMetrics(prometheus.NewRegistry(), "test")

	err := captiveBackend.PrepareRange(ctx, UnboundedRange(65))
	assert.NoError(t, err)

	assert.Equal(t, uint64(1), getStartDurationMetric(captiveBackend).GetSampleCount())
	assert.Greater(t, getStartDurationMetric(captiveBackend).GetSampleSum(), float64(0))

	captiveBackend.nextLedger = 64
	err = captiveBackend.PrepareRange(ctx, UnboundedRange(65))
	assert.NoError(t, err)

	assert.Equal(t, uint64(1), getStartDurationMetric(captiveBackend).GetSampleCount())

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestGetLatestLedgerSequence(t *testing.T) {
	metaChan := make(chan metaResult, 300)

	// Core will actually start with the last checkpoint before the `from` ledger
	// and then rewind to the `from` ledger.
	for i := 2; i <= 200; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}

	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("runFrom", uint32(63)).Return(nil).Once()
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, UnboundedRange(64))
	assert.NoError(t, err)

	latest, err := captiveBackend.GetLatestLedgerSequence(ctx)
	assert.NoError(t, err)
	assert.Equal(t, uint32(200), latest)

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestGetLatestLedgerSequenceRaceCondition(t *testing.T) {
	var fromSeq uint32 = 64
	var toSeq uint32 = 400
	metaChan := make(chan metaResult, toSeq)

	for i := fromSeq; i <= toSeq; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: i})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)
	mockRunner.On("runFrom", mock.Anything).Return(nil)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: toSeq * 2,
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(10),
	}

	ledgerRange := UnboundedRange(fromSeq)
	err := captiveBackend.PrepareRange(ctx, ledgerRange)
	assert.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)

	go func(ctx context.Context) {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, _ = captiveBackend.GetLatestLedgerSequence(ctx)
			}
		}
	}(ctx)

	for i := fromSeq; i < toSeq; i++ {
		_, err = captiveBackend.GetLedger(ctx, i)
		assert.NoError(t, err)
	}

	cancel()

	wg.Wait()
}

func TestCaptiveGetLedger(t *testing.T) {
	tt := assert.New(t)
	metaChan := make(chan metaResult, 300)

	for i := 64; i <= 66; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("catchup", uint32(65), uint32(66)).Return(nil)
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)
	mockRunner.On("getProcessExitError").Return(nil, false)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	// requires PrepareRange
	_, err := captiveBackend.GetLedger(ctx, 64)
	tt.EqualError(err, "session is not prepared, call PrepareRange first")

	ledgerRange := BoundedRange(65, 66)
	tt.False(captiveBackend.isPrepared(ledgerRange), "core is not prepared until explicitly prepared")
	tt.False(captiveBackend.closed)
	err = captiveBackend.PrepareRange(ctx, ledgerRange)
	assert.NoError(t, err)

	tt.True(captiveBackend.isPrepared(ledgerRange))
	tt.False(captiveBackend.closed)

	_, err = captiveBackend.GetLedger(ctx, 64)
	tt.Error(err, "requested ledger 64 is behind the captive core stream (expected=66)")

	// reads value from buffer
	meta, err := captiveBackend.GetLedger(ctx, 65)
	tt.NoError(err)
	tt.Equal(xdr.Uint32(65), meta.V0.LedgerHeader.Header.LedgerSeq)

	// reads value from cachedMeta
	cachedMeta, err := captiveBackend.GetLedger(ctx, 65)
	tt.NoError(err)
	tt.Equal(meta, cachedMeta)

	// next sequence number didn't get consumed
	tt.Equal(uint32(66), captiveBackend.nextLedger)

	mockRunner.On("close").Return(nil).Run(func(args mock.Arguments) {
		cancel()
	}).Once()

	_, err = captiveBackend.GetLedger(ctx, 66)
	tt.NoError(err)

	tt.False(captiveBackend.isPrepared(ledgerRange))
	tt.False(captiveBackend.closed)
	_, err = captiveBackend.GetLedger(ctx, 66)
	tt.NoError(err)

	// core is not closed unless it's explicitly closed
	tt.False(captiveBackend.closed)

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

// TestCaptiveGetLedgerCacheLatestLedger test the following case:
// 1. Prepare Unbounded range.
// 2. GetLedger that is still not in the buffer.
// 3. Get latest ledger in the buffer using GetLedger.
//
// Before 3d97762 this test failed because cachedMeta was only updated when
// the ledger with a requested sequence was reached while streaming meta.
//
// TODO: Not sure this test is really valid or worth it anymore, now that GetLedger is always blocking.
func TestCaptiveGetLedgerCacheLatestLedger(t *testing.T) {
	tt := assert.New(t)
	metaChan := make(chan metaResult, 300)

	for i := 2; i <= 67; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("runFrom", uint32(65)).Return(nil).Once()
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, UnboundedRange(66))
	assert.NoError(t, err)

	// found, _, err := captiveBackend.GetLedger(ctx, 68)
	// tt.NoError(err)
	// tt.False(found)
	// tt.Equal(uint32(67), captiveBackend.cachedMeta.LedgerSequence())
	// tt.Equal(uint32(68), captiveBackend.nextLedger)

	meta, err := captiveBackend.GetLedger(ctx, 67)
	tt.NoError(err)
	tt.Equal(uint32(67), meta.LedgerSequence())

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestCaptiveGetLedger_NextLedgerIsDifferentToLedgerFromBuffer(t *testing.T) {
	metaChan := make(chan metaResult, 100)

	for i := 64; i <= 65; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}
	{
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(68)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}

	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("catchup", uint32(65), uint32(66)).Return(nil)
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)
	mockRunner.On("close").Return(nil)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, BoundedRange(65, 66))
	assert.NoError(t, err)

	_, err = captiveBackend.GetLedger(ctx, 66)
	assert.EqualError(t, err, "unexpected ledger sequence (expected=66 actual=68)")

	// TODO assertions should work - to be fixed in a separate PR.
	// _, err = captiveBackend.GetLedger(ctx, 66)
	// assert.EqualError(t, err, "session is closed, call PrepareRange first")

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestCaptiveGetLedger_NextLedger0RangeFromIsSmallerThanLedgerFromBuffer(t *testing.T) {
	metaChan := make(chan metaResult, 100)

	for i := 66; i <= 66; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}

	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("runFrom", uint32(64)).Return(nil)
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)
	mockRunner.On("close").Return(nil)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, UnboundedRange(65))
	assert.EqualError(t, err, "Error fast-forwarding to 65: unexpected ledger sequence (expected=<=65 actual=66)")

	// TODO assertions should work - to be fixed in a separate PR.
	// prepared, err := captiveBackend.IsPrepared(ctx, UnboundedRange(65))
	// assert.NoError(t, err)
	// assert.False(t, prepared)

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestCaptiveStellarCore_PrepareRangeAfterClose(t *testing.T) {
	storagePath := t.TempDir()

	ctx := context.Background()
	executablePath := "/etc/stellar-core"
	networkPassphrase := network.PublicNetworkPassphrase
	historyURLs := []string{"http://localhost"}

	captiveCoreToml, err := NewCaptiveCoreToml(CaptiveCoreTomlParams{})
	assert.NoError(t, err)

	captiveStellarCore, err := NewCaptive(
		CaptiveCoreConfig{
			BinaryPath:            executablePath,
			NetworkPassphrase:     networkPassphrase,
			HistoryArchiveURLs:    historyURLs,
			Toml:                  captiveCoreToml,
			StoragePath:           storagePath,
			CoreProtocolVersionFn: func(string) (uint, error) { return 23, nil },
		},
	)
	assert.NoError(t, err)

	assert.NoError(t, captiveStellarCore.Close())

	assert.EqualError(
		t,
		captiveStellarCore.PrepareRange(ctx, BoundedRange(65, 66)),
		"error starting prepare range: opening subprocess: error getting latest checkpoint sequence: "+
			"error getting root HAS: Get \"http://localhost/.well-known/stellar-history.json\": context canceled",
	)

	// even if the request to fetch the latest checkpoint succeeds, we should fail at creating the subprocess
	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)
	captiveStellarCore.archive = mockArchive
	assert.EqualError(
		t,
		captiveStellarCore.PrepareRange(ctx, BoundedRange(65, 66)),
		"error starting prepare range: opening subprocess: error running stellar-core: context canceled",
	)
	mockArchive.AssertExpectations(t)
}

func TestCaptiveGetLedger_ErrReadingMetaResult(t *testing.T) {
	tt := assert.New(t)
	metaChan := make(chan metaResult, 100)

	for i := 64; i <= 65; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}
	metaChan <- metaResult{
		err: fmt.Errorf("unmarshaling error"),
	}

	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("catchup", uint32(65), uint32(66)).Return(nil)
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	ctx, cancel := context.WithCancel(ctx)
	mockRunner.On("context").Return(ctx)
	mockRunner.On("close").Return(nil).Run(func(args mock.Arguments) {
		cancel()
	}).Once()
	mockRunner.On("getProcessExitError").Return(nil, false)

	// even if the request to fetch the latest checkpoint succeeds, we should fail at creating the subprocess
	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, BoundedRange(65, 66))
	assert.NoError(t, err)

	meta, err := captiveBackend.GetLedger(ctx, 65)
	tt.NoError(err)
	tt.Equal(xdr.Uint32(65), meta.V0.LedgerHeader.Header.LedgerSeq)

	tt.False(captiveBackend.closed)

	// try reading from an empty buffer
	_, err = captiveBackend.GetLedger(ctx, 66)
	tt.EqualError(err, "unmarshaling error")

	// not closed even if there is an error getting ledger
	tt.False(captiveBackend.closed)

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestCaptiveGetLedger_ErrClosingAfterLastLedger(t *testing.T) {
	tt := assert.New(t)
	metaChan := make(chan metaResult, 100)

	for i := 64; i <= 66; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}

	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("catchup", uint32(65), uint32(66)).Return(nil)
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)
	mockRunner.On("close").Return(fmt.Errorf("transient error")).Once()

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, BoundedRange(65, 66))
	assert.NoError(t, err)

	_, err = captiveBackend.GetLedger(ctx, 66)
	tt.EqualError(err, "error closing session: transient error")

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestCaptiveAfterClose(t *testing.T) {
	metaChan := make(chan metaResult, 100)

	for i := 64; i <= 66; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}

	mockRunner := &stellarCoreRunnerMock{}
	ctx, cancel := context.WithCancel(context.Background())
	mockRunner.On("catchup", uint32(65), uint32(66)).Return(nil)
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)
	mockRunner.On("close").Return(nil).Once()

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
		cancel:            cancel,
	}

	boundedRange := BoundedRange(65, 66)
	err := captiveBackend.PrepareRange(ctx, boundedRange)
	assert.NoError(t, err)

	assert.NoError(t, captiveBackend.Close())
	assert.True(t, captiveBackend.closed)

	_, err = captiveBackend.GetLedger(ctx, boundedRange.to)
	assert.EqualError(t, err, "stellar-core is no longer usable")

	var prepared bool
	prepared, err = captiveBackend.IsPrepared(ctx, boundedRange)
	assert.False(t, prepared)
	assert.NoError(t, err)

	_, err = captiveBackend.GetLatestLedgerSequence(ctx)
	assert.EqualError(t, err, "stellar-core is no longer usable")

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

func TestGetLedgerBoundsCheck(t *testing.T) {
	metaChan := make(chan metaResult, 100)

	for i := 128; i <= 130; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(i)})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
	}

	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("catchup", uint32(128), uint32(130)).Return(nil).Once()
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(200),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, BoundedRange(128, 130))
	assert.NoError(t, err)

	meta, err := captiveBackend.GetLedger(ctx, 128)
	assert.NoError(t, err)
	assert.Equal(t, uint32(128), meta.LedgerSequence())

	prev := meta
	meta, err = captiveBackend.GetLedger(ctx, 128)
	assert.NoError(t, err)
	assert.Equal(t, prev, meta)

	_, err = captiveBackend.GetLedger(ctx, 64)
	assert.EqualError(t, err, "requested ledger 64 is behind the captive core stream (expected=129)")

	mockArchive.AssertExpectations(t)
	mockRunner.AssertExpectations(t)
}

type GetLedgerTerminatedTestCase struct {
	name               string
	ctx                context.Context
	ledgers            []metaResult
	processExited      bool
	processExitedError error
	expectedError      string
}

func CaptiveGetLedgerTerminatedUnexpectedlyTestCases() []GetLedgerTerminatedTestCase {
	ledger64 := buildLedgerCloseMeta(testLedgerHeader{sequence: uint32(64)})

	return []GetLedgerTerminatedTestCase{
		{
			"stellar core exited unexpectedly without error",
			context.Background(),
			[]metaResult{{LedgerCloseMeta: &ledger64}},
			true,
			nil,
			"stellar core exited unexpectedly",
		},
		{
			"stellar core exited unexpectedly with an error",
			context.Background(),
			[]metaResult{{LedgerCloseMeta: &ledger64}},
			true,
			fmt.Errorf("signal kill"),
			"stellar core exited unexpectedly: signal kill",
		},
		{
			"stellar core exited unexpectedly without error and closed channel",
			context.Background(),
			[]metaResult{{LedgerCloseMeta: &ledger64}},
			true,
			nil,
			"stellar core exited unexpectedly",
		},
		{
			"stellar core exited unexpectedly with an error and closed channel",
			context.Background(),
			[]metaResult{{LedgerCloseMeta: &ledger64}},
			true,
			fmt.Errorf("signal kill"),
			"stellar core exited unexpectedly: signal kill",
		},
		{
			"meta pipe closed unexpectedly",
			context.Background(),
			[]metaResult{{LedgerCloseMeta: &ledger64}},
			false,
			nil,
			"meta pipe closed unexpectedly",
		},
		{
			"Parser error while reading from the pipe resulting in stellar-core exit",
			context.Background(),
			[]metaResult{{LedgerCloseMeta: &ledger64},
				{LedgerCloseMeta: nil, err: errors.New("Parser error")}},
			true,
			nil,
			"Parser error",
		},
		{
			"stellar core exited unexpectedly with an error resulting in meta pipe closed",
			context.Background(),
			[]metaResult{{LedgerCloseMeta: &ledger64},
				{LedgerCloseMeta: &ledger64, err: errors.New("EOF while decoding")}},
			true,
			fmt.Errorf("signal kill"),
			"stellar core exited unexpectedly: signal kill",
		},
	}
}

func TestCaptiveGetLedgerTerminatedUnexpectedly(t *testing.T) {
	for _, testCase := range CaptiveGetLedgerTerminatedUnexpectedlyTestCases() {
		t.Run(testCase.name, func(t *testing.T) {
			metaChan := make(chan metaResult, 100)

			for _, result := range testCase.ledgers {
				metaChan <- result
			}
			close(metaChan)

			ctx := testCase.ctx
			mockRunner := &stellarCoreRunnerMock{}
			mockRunner.On("catchup", uint32(64), uint32(100)).Return(nil).Once()
			mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
			mockRunner.On("context").Return(ctx)
			mockRunner.On("getProcessExitError").Return(testCase.processExitedError, testCase.processExited)
			mockRunner.On("close").Return(nil).Once()

			mockArchive := &historyarchive.MockArchive{}
			mockArchive.
				On("GetRootHAS").
				Return(historyarchive.HistoryArchiveState{
					CurrentLedger: uint32(200),
				}, nil)

			captiveBackend := &CaptiveStellarCore{
				archive: mockArchive,
				stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
					return mockRunner
				},
				checkpointManager: historyarchive.NewCheckpointManager(64),
			}

			err := captiveBackend.PrepareRange(ctx, BoundedRange(64, 100))
			assert.NoError(t, err)

			meta, err := captiveBackend.GetLedger(ctx, 64)
			assert.NoError(t, err)
			assert.Equal(t, uint32(64), meta.LedgerSequence())

			_, err = captiveBackend.GetLedger(ctx, 65)
			assert.EqualError(t, err, testCase.expectedError)

			mockArchive.AssertExpectations(t)
			mockRunner.AssertExpectations(t)
		})
	}
}

func TestCaptiveRunFromParams(t *testing.T) {
	var tests = []struct {
		from           uint32
		runFrom        uint32
		ledgerArchives uint32
	}{
		// Before and including 1st checkpoint:
		{2, 2, 3},
		{3, 2, 3},
		{3, 2, 3},
		{4, 2, 3},
		{62, 2, 3},
		{63, 2, 3},

		// Starting from 64 we go normal path: between 1st and 2nd checkpoint:
		{64, 63, 64},
		{65, 64, 65},
		{66, 65, 66},
		{126, 125, 126},

		// between 2nd and 3rd checkpoint... and so on.
		{127, 126, 127},
		{128, 127, 128},
		{129, 128, 129},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("from_%d", tc.from), func(t *testing.T) {
			tt := assert.New(t)
			mockArchive := &historyarchive.MockArchive{}
			mockArchive.
				On("GetRootHAS").
				Return(historyarchive.HistoryArchiveState{
					CurrentLedger: uint32(255),
				}, nil)

			captiveBackend := &CaptiveStellarCore{
				archive:           mockArchive,
				checkpointManager: historyarchive.NewCheckpointManager(64),
			}

			ctx := context.Background()
			runFrom, err := captiveBackend.runFromParams(ctx, tc.from)
			tt.NoError(err)
			tt.Equal(tc.runFrom, runFrom, "runFrom")
			mockArchive.AssertExpectations(t)
		})
	}
}

func TestCaptiveIsPrepared(t *testing.T) {
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("context").Return(context.Background()).Maybe()
	mockRunner.On("getProcessExitError").Return(nil, false)

	// c.prepared == nil
	captiveBackend := &CaptiveStellarCore{
		nextLedger: 0,
	}

	result := captiveBackend.isPrepared(UnboundedRange(100))
	assert.False(t, result)

	// c.prepared != nil:
	var tests = []struct {
		nextLedger    uint32
		lastLedger    uint32
		cachedLedger  uint32
		preparedRange Range
		ledgerRange   Range
		result        bool
	}{
		// If nextLedger == 0, prepared range is checked
		{0, 0, 0, UnboundedRange(100), UnboundedRange(100), true},
		{0, 0, 0, UnboundedRange(100), UnboundedRange(99), false},
		{0, 0, 0, UnboundedRange(100), BoundedRange(100, 200), true},

		{100, 0, 0, UnboundedRange(99), UnboundedRange(101), true},
		{101, 0, 100, UnboundedRange(99), UnboundedRange(100), true},
		{100, 200, 0, BoundedRange(99, 200), UnboundedRange(100), false},

		{100, 200, 0, BoundedRange(99, 200), BoundedRange(100, 200), true},
		{100, 200, 0, BoundedRange(99, 200), BoundedRange(100, 201), false},
		{100, 201, 0, BoundedRange(99, 201), BoundedRange(100, 200), true},
		{101, 200, 100, BoundedRange(99, 200), BoundedRange(100, 200), true},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("next_%d_last_%d_cached_%d_range_%v", tc.nextLedger, tc.lastLedger, tc.cachedLedger, tc.ledgerRange), func(t *testing.T) {
			captiveBackend := &CaptiveStellarCore{
				stellarCoreRunner: mockRunner,
				nextLedger:        tc.nextLedger,
				prepared:          &tc.preparedRange,
			}
			if tc.lastLedger > 0 {
				captiveBackend.lastLedger = &tc.lastLedger
			}
			if tc.cachedLedger > 0 {
				meta := buildLedgerCloseMeta(testLedgerHeader{
					sequence: tc.cachedLedger,
				})
				captiveBackend.cachedMeta = &meta
			}

			result := captiveBackend.isPrepared(tc.ledgerRange)
			assert.Equal(t, tc.result, result)
		})
	}
}

// TestCaptiveIsPreparedCoreContextCancelled checks if IsPrepared returns false
// if the stellarCoreRunner.context() is canceled. This can happen when
// stellarCoreRunner was closed, ex. when binary file was updated.
func TestCaptiveIsPreparedCoreContextCancelled(t *testing.T) {
	mockRunner := &stellarCoreRunnerMock{}
	ctx, cancel := context.WithCancel(context.Background())
	mockRunner.On("context").Return(ctx).Maybe()
	mockRunner.On("getProcessExitError").Return(nil, false)

	rang := UnboundedRange(100)
	captiveBackend := &CaptiveStellarCore{
		nextLedger:        100,
		prepared:          &rang,
		stellarCoreRunner: mockRunner,
	}

	result := captiveBackend.isPrepared(UnboundedRange(100))
	assert.True(t, result)

	cancel()

	result = captiveBackend.isPrepared(UnboundedRange(100))
	assert.False(t, result)
}

// TestCaptivePreviousLedgerCheck checks if previousLedgerHash is set in PrepareRange
// and then checked and updated in GetLedger.
func TestCaptivePreviousLedgerCheck(t *testing.T) {
	metaChan := make(chan metaResult, 200)

	h := 3
	for i := 192; i <= 300; i++ {
		meta := buildLedgerCloseMeta(testLedgerHeader{
			sequence:           uint32(i),
			hash:               fmt.Sprintf("%02x00000000000000000000000000000000000000000000000000000000000000", h),
			previousLedgerHash: fmt.Sprintf("%02x00000000000000000000000000000000000000000000000000000000000000", h-1),
		})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}
		h++
	}

	{
		// Write invalid hash
		meta := buildLedgerCloseMeta(testLedgerHeader{
			sequence:           301,
			hash:               "0000000000000000000000000000000000000000000000000000000000000000",
			previousLedgerHash: "0000000000000000000000000000000000000000000000000000000000000000",
		})
		metaChan <- metaResult{
			LedgerCloseMeta: &meta,
		}

	}

	ctx := context.Background()
	mockRunner := &stellarCoreRunnerMock{}
	mockRunner.On("runFrom", uint32(299)).Return(nil).Once()
	mockRunner.On("getMetaPipe").Return((<-chan metaResult)(metaChan), true)
	mockRunner.On("context").Return(ctx)
	mockRunner.On("close").Return(nil).Once()

	mockArchive := &historyarchive.MockArchive{}
	mockArchive.
		On("GetRootHAS").
		Return(historyarchive.HistoryArchiveState{
			CurrentLedger: uint32(255),
		}, nil)

	captiveBackend := &CaptiveStellarCore{
		archive: mockArchive,
		stellarCoreRunnerFactory: func() stellarCoreRunnerInterface {
			return mockRunner
		},
		checkpointManager: historyarchive.NewCheckpointManager(64),
	}

	err := captiveBackend.PrepareRange(ctx, UnboundedRange(300))
	assert.NoError(t, err)

	meta, err := captiveBackend.GetLedger(ctx, 300)
	assert.NoError(t, err)
	assert.NotNil(t, captiveBackend.previousLedgerHash)
	assert.Equal(t, uint32(301), captiveBackend.nextLedger)
	assert.Equal(t, meta.LedgerHash().HexString(), *captiveBackend.previousLedgerHash)

	_, err = captiveBackend.GetLedger(ctx, 301)
	assert.EqualError(t, err, "unexpected previous ledger hash for ledger 301 (expected=6f00000000000000000000000000000000000000000000000000000000000000 actual=0000000000000000000000000000000000000000000000000000000000000000)")

	mockRunner.AssertExpectations(t)
	mockArchive.AssertExpectations(t)
}
