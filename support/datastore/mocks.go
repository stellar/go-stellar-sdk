package datastore

import (
	"context"
	"io"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockDataStore is a mock implementation for the Storage interface.
type MockDataStore struct {
	mock.Mock
}

func (m *MockDataStore) Exists(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Bool(0), args.Error(1)
}

func (m *MockDataStore) Size(ctx context.Context, path string) (int64, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDataStore) GetFileMetadata(ctx context.Context, path string) (map[string]string, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockDataStore) GetFileLastModified(ctx context.Context, filePath string) (time.Time, error) {
	args := m.Called(ctx, filePath)
	if args.Get(0) != nil {
		return args.Get(0).(time.Time), args.Error(1)
	}
	return time.Time{}, args.Error(1)
}

func (m *MockDataStore) GetFile(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	args := m.Called(ctx, path)
	closer := (io.ReadCloser)(nil)
	if args.Get(0) != nil {
		closer = args.Get(0).(io.ReadCloser)
	}
	return closer, args.Get(1).(int64), args.Error(2)
}

func (m *MockDataStore) PutFile(ctx context.Context, path string, in io.WriterTo, metadata map[string]string) error {
	args := m.Called(ctx, path, in, metadata)
	return args.Error(0)
}

func (m *MockDataStore) PutFileIfNotExists(ctx context.Context, path string, in io.WriterTo, metadata map[string]string) (bool, error) {
	args := m.Called(ctx, path, in, metadata)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockDataStore) ListFilePaths(ctx context.Context, options ListFileOptions) ([]string, error) {
	args := m.Called(ctx, options)
	var keys []string
	if v := args.Get(0); v != nil {
		keys = v.([]string)
	}
	return keys, args.Error(1)
}

func (m *MockDataStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDataStore) GetSchema() DataStoreSchema {
	args := m.Called()
	return args.Get(0).(DataStoreSchema)
}

// ensure that the MockClient implements ClientInterface
var _ DataStore = &MockDataStore{}
