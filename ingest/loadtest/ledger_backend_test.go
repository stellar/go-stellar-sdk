package loadtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/xdr"
)

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
