package ingest

import (
	"github.com/stretchr/testify/mock"

	"github.com/stellar/go-stellar-sdk/xdr"
)

var _ ChangeReader = (*MockChangeReader)(nil)

type MockChangeReader struct {
	mock.Mock
}

func (m *MockChangeReader) Read() (Change, error) {
	args := m.Called()
	return args.Get(0).(Change), args.Error(1)
}

func (m *MockChangeReader) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockChangeReader) VerifyBucketList(expectedHash xdr.Hash) error {
	args := m.Called(expectedHash)
	return args.Error(0)
}
