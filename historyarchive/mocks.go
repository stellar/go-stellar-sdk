package historyarchive

import (
	"github.com/stretchr/testify/mock"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// MockArchive is a mockable archive.
type MockArchive struct {
	mock.Mock
}

func (m *MockArchive) GetLatestLedgerSequence() (uint32, error) {
	a := m.Called()
	return a.Get(0).(uint32), a.Error(1)
}

func (m *MockArchive) GetCheckpointManager() CheckpointManager {
	a := m.Called()
	return a.Get(0).(CheckpointManager)
}

func (m *MockArchive) GetPathHAS(path string) (HistoryArchiveState, error) {
	a := m.Called(path)
	return a.Get(0).(HistoryArchiveState), a.Error(1)
}

func (m *MockArchive) PutPathHAS(path string, has HistoryArchiveState, opts *CommandOptions) error {
	a := m.Called(path, has, opts)
	return a.Error(0)
}

func (m *MockArchive) BucketExists(bucket Hash) (bool, error) {
	a := m.Called(bucket)
	return a.Get(0).(bool), a.Error(1)
}

func (m *MockArchive) BucketSize(bucket Hash) (int64, error) {
	a := m.Called(bucket)
	return a.Get(0).(int64), a.Error(1)
}

func (m *MockArchive) CategoryCheckpointExists(cat string, chk uint32) (bool, error) {
	a := m.Called(cat, chk)
	return a.Get(0).(bool), a.Error(1)
}

func (m *MockArchive) GetLedgerHeader(chk uint32) (xdr.LedgerHeaderHistoryEntry, error) {
	a := m.Called(chk)
	return a.Get(0).(xdr.LedgerHeaderHistoryEntry), a.Error(1)
}

func (m *MockArchive) GetLedgers(start, end uint32) (map[uint32]*Ledger, error) {
	a := m.Called(start, end)
	return a.Get(0).(map[uint32]*Ledger), a.Error(1)
}

func (m *MockArchive) GetRootHAS() (HistoryArchiveState, error) {
	a := m.Called()
	return a.Get(0).(HistoryArchiveState), a.Error(1)
}

func (m *MockArchive) GetCheckpointHAS(chk uint32) (HistoryArchiveState, error) {
	a := m.Called(chk)
	return a.Get(0).(HistoryArchiveState), a.Error(1)
}

func (m *MockArchive) PutCheckpointHAS(chk uint32, has HistoryArchiveState, opts *CommandOptions) error {
	a := m.Called(chk, has, opts)
	return a.Error(0)
}

func (m *MockArchive) PutRootHAS(has HistoryArchiveState, opts *CommandOptions) error {
	a := m.Called(has, opts)
	return a.Error(0)
}

func (m *MockArchive) ListBucket(dp DirPrefix) (chan string, chan error) {
	m.Called(dp)
	panic("Returning channels not implemented")
	return make(chan string), make(chan error)
}

func (m *MockArchive) ListAllBuckets() (chan string, chan error) {
	m.Called()
	panic("Returning channels not implemented")
	return make(chan string), make(chan error)
}

func (m *MockArchive) ListAllBucketHashes() (chan Hash, chan error) {
	m.Called()
	panic("Returning channels not implemented")
	return make(chan Hash), make(chan error)
}

func (m *MockArchive) ListCategoryCheckpoints(cat string, pth string) (chan uint32, chan error) {
	m.Called(cat, pth)
	panic("Returning channels not implemented")
	return make(chan uint32), make(chan error)
}

func (m *MockArchive) GetXdrStreamForHash(hash Hash) (*xdr.Stream, error) {
	a := m.Called(hash)
	return a.Get(0).(*xdr.Stream), a.Error(1)
}

func (m *MockArchive) GetXdrStream(pth string) (*xdr.Stream, error) {
	a := m.Called(pth)
	return a.Get(0).(*xdr.Stream), a.Error(1)
}

func (m *MockArchive) GetStats() []ArchiveStats {
	a := m.Called()
	return a.Get(0).([]ArchiveStats)
}

type MockArchiveStats struct {
	mock.Mock
}

func (m *MockArchiveStats) GetRequests() uint32 {
	a := m.Called()
	return a.Get(0).(uint32)
}

func (m *MockArchiveStats) GetDownloads() uint32 {
	a := m.Called()
	return a.Get(0).(uint32)
}

func (m *MockArchiveStats) GetUploads() uint32 {
	a := m.Called()
	return a.Get(0).(uint32)
}

func (m *MockArchiveStats) GetBackendName() string {
	a := m.Called()
	return a.Get(0).(string)
}

func (m *MockArchiveStats) GetCacheHits() uint32 {
	a := m.Called()
	return a.Get(0).(uint32)
}

func (m *MockArchiveStats) GetCacheBandwidth() uint64 {
	a := m.Called()
	return a.Get(0).(uint64)
}
