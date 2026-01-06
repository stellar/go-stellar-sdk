package datastore

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilesystemExists(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	// Create a test file
	content := []byte("test content")
	err = os.WriteFile(filepath.Join(dir, "file.txt"), content, 0600)
	require.NoError(t, err)

	exists, err := store.Exists(context.Background(), "file.txt")
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = store.Exists(context.Background(), "missing-file.txt")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestFilesystemSize(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	content := []byte("inside the file")
	err = os.WriteFile(filepath.Join(dir, "file.txt"), content, 0600)
	require.NoError(t, err)

	size, err := store.Size(context.Background(), "file.txt")
	require.NoError(t, err)
	require.Equal(t, int64(len(content)), size)

	_, err = store.Size(context.Background(), "missing-file.txt")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestFilesystemPutFile(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	content := []byte("inside the file")
	writerTo := bytes.NewReader(content)
	err = store.PutFile(context.Background(), "file.txt", writerTo, nil)
	require.NoError(t, err)

	reader, err := store.GetFile(context.Background(), "file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, content)

	metadata, err := store.GetFileMetadata(context.Background(), "file.txt")
	require.NoError(t, err)
	require.Equal(t, map[string]string{}, metadata)

	// Test overwriting
	otherContent := []byte("other text")
	writerTo = bytes.NewReader(otherContent)
	err = store.PutFile(context.Background(), "file.txt", writerTo, nil)
	require.NoError(t, err)

	reader, err = store.GetFile(context.Background(), "file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, otherContent)
}

func TestFilesystemPutFileCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	content := []byte("nested file content")
	writerTo := bytes.NewReader(content)
	err = store.PutFile(context.Background(), "a/b/c/file.txt", writerTo, nil)
	require.NoError(t, err)

	reader, err := store.GetFile(context.Background(), "a/b/c/file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, content)
}

func TestFilesystemPutFileIfNotExists(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	existingContent := []byte("existing content")
	err = os.WriteFile(filepath.Join(dir, "file.txt"), existingContent, 0600)
	require.NoError(t, err)

	// Attempt to overwrite - should fail
	newContent := []byte("new content")
	writerTo := bytes.NewReader(newContent)
	ok, err := store.PutFileIfNotExists(context.Background(), "file.txt", writerTo, nil)
	require.NoError(t, err)
	require.False(t, ok)

	// Verify content unchanged
	reader, err := store.GetFile(context.Background(), "file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, existingContent)

	// Create new file - should succeed
	writerTo = bytes.NewReader(newContent)
	ok, err = store.PutFileIfNotExists(context.Background(), "other-file.txt", writerTo, nil)
	require.NoError(t, err)
	require.True(t, ok)

	reader, err = store.GetFile(context.Background(), "other-file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, newContent)
}

func TestFilesystemGetFileLastModified(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	content := []byte("inside the file")
	writerTo := bytes.NewReader(content)
	err = store.PutFile(context.Background(), "file.txt", writerTo, nil)
	require.NoError(t, err)

	lastModified, err := store.GetFileLastModified(context.Background(), "file.txt")
	require.NoError(t, err)
	require.NotZero(t, lastModified)
}

func TestFilesystemGetNonExistentFile(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	// Create a different file
	content := []byte("inside the file")
	err = os.WriteFile(filepath.Join(dir, "file.txt"), content, 0600)
	require.NoError(t, err)

	_, err = store.GetFile(context.Background(), "other-file.txt")
	require.ErrorIs(t, err, os.ErrNotExist)

	metadata, err := store.GetFileMetadata(context.Background(), "other-file.txt")
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Nil(t, metadata)
}

func TestFilesystemListFilePaths(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	// Create test files
	for _, name := range []string{"a", "b", "c"} {
		err = os.WriteFile(filepath.Join(dir, name), []byte("1"), 0600)
		require.NoError(t, err)
	}

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, paths)
}

func TestFilesystemListFilePaths_WithPrefix(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	// Create directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "b"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "x"), []byte("1"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "y"), []byte("1"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b", "z"), []byte("1"), 0600))

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{Prefix: "a", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, []string{"a/x", "a/y"}, paths)
}

func TestFilesystemListFilePaths_LimitDefaultAndCap(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	// Create 1200 files
	for i := 0; i < 1200; i++ {
		err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%04d", i)), []byte("1"), 0600)
		require.NoError(t, err)
	}

	// Default limit should cap at 1000
	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{})
	require.NoError(t, err)
	require.Equal(t, 1000, len(paths))

	// Explicit limit over 1000 should also cap at 1000
	paths, err = store.ListFilePaths(context.Background(), ListFileOptions{Limit: 5000})
	require.NoError(t, err)
	require.Equal(t, 1000, len(paths))
}

func TestFilesystemListFilePaths_StartAfter_Basic(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	for i := 0; i < 10; i++ {
		err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%04d", i)), []byte("x"), 0600)
		require.NoError(t, err)
	}

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
		StartAfter: "0005",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"0006", "0007", "0008", "0009"}, paths)
}

func TestFilesystemListFilePaths_StartAfter_WithPrefix(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "b"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "0001"), []byte(""), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "0002"), []byte(""), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b", "0002"), []byte(""), 0600))

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
		Prefix:     "a/",
		StartAfter: "a/0001",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a/0002"}, paths)
}

func TestFilesystemListFilePaths_StartAfter_EqualsLastKey(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	for _, name := range []string{"0000", "0001", "0002"} {
		err = os.WriteFile(filepath.Join(dir, name), []byte(""), 0600)
		require.NoError(t, err)
	}

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
		StartAfter: "0002",
	})
	require.NoError(t, err)
	require.Empty(t, paths)
}

func TestFilesystemListFilePaths_StartAfter_BeforeFirstKey(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	for _, name := range []string{"0001", "0002", "0003"} {
		err = os.WriteFile(filepath.Join(dir, name), []byte(""), 0600)
		require.NoError(t, err)
	}

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
		StartAfter: "0000",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"0001", "0002", "0003"}, paths)
}

func TestFilesystemListFilePaths_StartAfter_BetweenKeys(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	for _, name := range []string{"0002", "0004", "0006"} {
		err = os.WriteFile(filepath.Join(dir, name), []byte(""), 0600)
		require.NoError(t, err)
	}

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
		StartAfter: "0003",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"0004", "0006"}, paths)
}

func TestFilesystemListFilePaths_StartAfter_WithLimit(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFilesystemDataStoreWithPath(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	for i := 0; i < 10; i++ {
		err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%04d", i)), []byte("x"), 0600)
		require.NoError(t, err)
	}

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
		StartAfter: "0004",
		Limit:      3,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"0005", "0006", "0007"}, paths)
}

func TestNewFilesystemDataStore(t *testing.T) {
	dir := t.TempDir()

	config := DataStoreConfig{
		Type: "Filesystem",
		Params: map[string]string{
			"destination_path": dir,
		},
	}

	store, err := NewDataStore(context.Background(), config)
	require.NoError(t, err)
	require.NotNil(t, store)
	require.NoError(t, store.Close())
}

func TestNewFilesystemDataStore_MissingDestinationPath(t *testing.T) {
	config := DataStoreConfig{
		Type:   "Filesystem",
		Params: map[string]string{},
	}

	_, err := NewDataStore(context.Background(), config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no destination_path")
}
