package ledgerbackend

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/stellar/go-stellar-sdk/xdr"
)

var _ LedgerBackend = (*MockDatabaseBackend)(nil)

type MockDatabaseBackend struct {
	mock.Mock
}

func (m *MockDatabaseBackend) GetLatestLedgerSequence(ctx context.Context) (uint32, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint32), args.Error(1)
}

func (m *MockDatabaseBackend) PrepareRange(ctx context.Context, ledgerRange Range) error {
	args := m.Called(ctx, ledgerRange)
	return args.Error(0)
}

func (m *MockDatabaseBackend) IsPrepared(ctx context.Context, ledgerRange Range) (bool, error) {
	args := m.Called(ctx, ledgerRange)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabaseBackend) GetLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error) {
	args := m.Called(ctx, sequence)
	return args.Get(0).(xdr.LedgerCloseMeta), args.Error(1)
}

func (m *MockDatabaseBackend) Close() error {
	args := m.Called()
	return args.Error(0)
}
