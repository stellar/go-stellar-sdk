package loadtest

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// writeLedgersFile writes n synthetic ledgers to a zstd-compressed file and returns its path.
func writeLedgersFile(t *testing.T, n int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), fmt.Sprintf("ledgers-%d.xdr.zst", n))
	file, err := os.Create(path)
	require.NoError(t, err)
	writer, err := zstd.NewWriter(file)
	require.NoError(t, err)
	for range n {
		ledger := xdr.LedgerCloseMeta{V: 0, V0: &xdr.LedgerCloseMetaV0{}}
		require.NoError(t, xdr.MarshalFramed(writer, ledger))
	}
	require.NoError(t, writer.Close())
	require.NoError(t, file.Close())
	return path
}

func TestCountLedgersAcrossFiles(t *testing.T) {
	paths := []string{writeLedgersFile(t, 3), writeLedgersFile(t, 0), writeLedgersFile(t, 5)}

	count, err := countLedgers(paths, 0)
	require.NoError(t, err)
	require.Equal(t, 8, count)

	// MaxLedgersPerFile caps each file independently: min(3,2)+min(0,2)+min(5,2) = 4.
	count, err = countLedgers(paths, 2)
	require.NoError(t, err)
	require.Equal(t, 4, count)
}

func TestCountLedgersPerFile(t *testing.T) {
	paths := []string{writeLedgersFile(t, 3), writeLedgersFile(t, 0), writeLedgersFile(t, 5)}

	counts, err := CountLedgersPerFile(paths, 0)
	require.NoError(t, err)
	require.Equal(t, []FileLedgerCount{
		{Path: paths[0], Ledgers: 3}, {Path: paths[1], Ledgers: 0}, {Path: paths[2], Ledgers: 5},
	}, counts)

	// The per-file cap clamps each file independently.
	counts, err = CountLedgersPerFile(paths, 2)
	require.NoError(t, err)
	require.Equal(t, []FileLedgerCount{
		{Path: paths[0], Ledgers: 2}, {Path: paths[1], Ledgers: 0}, {Path: paths[2], Ledgers: 2},
	}, counts)
}

func TestLedgerReaderEmpty(t *testing.T) {
	reader := newLedgerReader(nil, 0)
	var ledger xdr.LedgerCloseMeta
	require.Equal(t, io.EOF, reader.ReadOne(&ledger))
	require.NoError(t, reader.Close())
}

// v1Ledger builds a minimal, marshalable V1 LedgerCloseMeta with the given number
// of transaction phases and evicted keys.
func v1Ledger(phases, evicted int) xdr.LedgerCloseMeta {
	m := xdr.LedgerCloseMeta{V: 1, V1: &xdr.LedgerCloseMetaV1{
		TxSet: xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{}},
	}}
	for range phases {
		m.V1.TxSet.V1TxSet.Phases = append(m.V1.TxSet.V1TxSet.Phases,
			xdr.TransactionPhase{V: 0, V0Components: &[]xdr.TxSetComponent{}})
	}
	for range evicted {
		m.V1.EvictedKeys = append(m.V1.EvictedKeys,
			xdr.LedgerKey{Type: xdr.LedgerEntryTypeTtl, Ttl: &xdr.LedgerKeyTtl{}})
	}
	return m
}

func TestMergeLedgers(t *testing.T) {
	identity := func(seq uint32) uint32 { return seq }

	// All four merged slices (phases, tx processing, upgrades, evicted keys) share the
	// same append; phases (the transactions) and evicted keys cover the pattern.
	t.Run("appends src onto dst", func(t *testing.T) {
		dst, src := v1Ledger(1, 1), v1Ledger(2, 2)
		require.NoError(t, MergeLedgers(&dst, src, identity))
		require.Len(t, dst.V1.TxSet.V1TxSet.Phases, 3)
		require.Len(t, dst.V1.EvictedKeys, 3)
	})

	t.Run("rejects mismatched versions", func(t *testing.T) {
		dst := v1Ledger(1, 0)
		src := xdr.LedgerCloseMeta{V: 2, V2: &xdr.LedgerCloseMetaV2{
			TxSet: xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{}},
		}}
		require.ErrorContains(t, MergeLedgers(&dst, src, identity), "incompatible")
	})

	t.Run("rejects ledger without a v1 txset", func(t *testing.T) {
		dst := v1Ledger(1, 0)
		dst.V1.TxSet = xdr.GeneralizedTransactionSet{V: 0}
		require.Error(t, MergeLedgers(&dst, v1Ledger(1, 0), identity))
	})
}

type mockLedgerBackend struct {
	mock.Mock
}

func (m *mockLedgerBackend) GetLatestLedgerSequence(ctx context.Context) (sequence uint32, err error) {
	args := m.Called(ctx)
	return args.Get(0).(uint32), args.Error(1)
}

func (m *mockLedgerBackend) GetLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error) {
	args := m.Called(ctx, sequence)
	return args.Get(0).(xdr.LedgerCloseMeta), args.Error(1)
}

func (m *mockLedgerBackend) PrepareRange(ctx context.Context, ledgerRange ledgerbackend.Range) error {
	args := m.Called(ctx, ledgerRange)
	return args.Error(0)
}

func (m *mockLedgerBackend) IsPrepared(ctx context.Context, ledgerRange ledgerbackend.Range) (bool, error) {
	args := m.Called(ctx, ledgerRange)
	return args.Get(0).(bool), args.Error(1)
}

func (m *mockLedgerBackend) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestOptimizedPrepareRange_BoundedContainsMaxBoundedRange(t *testing.T) {
	m := &mockLedgerBackend{}
	r := &LedgerBackend{config: LedgerBackendConfig{LedgerBackend: m}, isCaptiveCore: true}
	ctx := context.Background()

	from := uint32(100)
	ledgerCount := 10
	req := ledgerbackend.BoundedRange(from, 1000)

	m.On("PrepareRange", ctx, ledgerbackend.BoundedRange(from, uint32(109))).
		Return(nil).Once()

	require.NoError(t, r.optimizedPrepareRange(ctx, req, ledgerCount))
	m.AssertExpectations(t)
}

func TestOptimizedPrepareRange_BoundedDoesNotContainMaxBoundedRange(t *testing.T) {
	m := &mockLedgerBackend{}
	r := &LedgerBackend{config: LedgerBackendConfig{LedgerBackend: m}, isCaptiveCore: true}
	ctx := context.Background()

	ledgerCount := 10
	req := ledgerbackend.BoundedRange(105, 107) // does not contain [from=105-(ledgerCount-1)=96, to=114]

	m.On("PrepareRange", ctx, ledgerbackend.BoundedRange(105, 107)).
		Return(nil).Once()

	require.NoError(t, r.optimizedPrepareRange(ctx, req, ledgerCount))
	m.AssertExpectations(t)
}

func TestOptimizedPrepareRange_UnboundedReducedToBounded(t *testing.T) {
	m := &mockLedgerBackend{}
	r := &LedgerBackend{config: LedgerBackendConfig{LedgerBackend: m}, isCaptiveCore: true}
	ctx := context.Background()

	req := ledgerbackend.UnboundedRange(200)
	ledgerCount := 5

	m.On("PrepareRange", ctx, ledgerbackend.BoundedRange(200, 204)).
		Return(nil).Once()

	require.NoError(t, r.optimizedPrepareRange(ctx, req, ledgerCount))
	m.AssertExpectations(t)
}

func TestOptimizedPrepareRange_UnboundedCanotCatchupAfterLatestCheckpoint(t *testing.T) {
	m := &mockLedgerBackend{}
	r := &LedgerBackend{config: LedgerBackendConfig{LedgerBackend: m}, isCaptiveCore: true}
	ctx := context.Background()

	req := ledgerbackend.UnboundedRange(200)
	ledgerCount := 5

	m.On("PrepareRange", ctx, ledgerbackend.BoundedRange(200, 204)).
		Return(
			fmt.Errorf(
				"cannot prepare range: %w",
				ledgerbackend.ErrCannotCatchupAheadLatestCheckpoint,
			),
		).Once()

	m.On("PrepareRange", ctx, req).
		Return(nil).Once()
	require.NoError(t, r.optimizedPrepareRange(ctx, req, ledgerCount))
	m.AssertExpectations(t)
}

func TestOptimizedPrepareRange_UnboundedNotCaptiveCore(t *testing.T) {
	m := &mockLedgerBackend{}
	r := &LedgerBackend{config: LedgerBackendConfig{LedgerBackend: m}, isCaptiveCore: false}
	ctx := context.Background()

	req := ledgerbackend.UnboundedRange(200)
	ledgerCount := 5

	m.On("PrepareRange", ctx, req).
		Return(nil).Once()

	require.NoError(t, r.optimizedPrepareRange(ctx, req, ledgerCount))
	m.AssertExpectations(t)
}
