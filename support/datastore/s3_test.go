package datastore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

// mockS3Object stores object data and metadata within the mock server.
type mockS3Object struct {
	body         []byte
	metadata     map[string]string
	crc32c       string
	lastModified time.Time
}

// mockS3Server is our mock S3 server, holding an in-memory "bucket".
type mockS3Server struct {
	mu      sync.Mutex
	objects map[string]mockS3Object
}

// ServeHTTP is the core logic of the mock server, handling all incoming HTTP requests.
func (s *mockS3Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pathParts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)

	switch r.Method {
	case http.MethodHead:
		// See https://docs.aws.amazon.com/AmazonS3/latest/API/API_HeadObject.html
		s.handleHeadRequest(w, pathParts)
	case http.MethodGet:
		// See https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetObject.html
		s.handleGetRequest(w, r, pathParts)
	case http.MethodPut:
		// See https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html
		s.handlePutRequest(w, r, pathParts)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *mockS3Server) handleHeadRequest(w http.ResponseWriter, pathParts []string) {
	// Handle HeadObject: A request with a key.
	key := pathParts[1]
	obj, exists := s.objects[key]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	s.setObjectHeaders(w, obj)
	w.WriteHeader(http.StatusOK)
}

func (s *mockS3Server) handleGetRequest(w http.ResponseWriter, r *http.Request, pathParts []string) {
	// Check for query param for ListObjectsV2
	if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
		// Minimal ListObjectsV2 XML response
		prefix := r.URL.Query().Get("prefix")
		maxKeys := 1000
		if mk := r.URL.Query().Get("max-keys"); mk != "" {
			if v, err := strconv.Atoi(mk); err == nil {
				maxKeys = v
			}
		}
		// Collect and sort keys matching prefix
		keys := make([]string, 0)
		for k := range s.objects {
			if strings.HasPrefix(k, prefix) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)

		// Apply start-after
		startAfter := r.URL.Query().Get("start-after")
		if startAfter != "" {
			filtered := make([]string, 0, len(keys))
			// binary search to first index with key > boundary
			i := sort.Search(len(keys), func(i int) bool { return keys[i] > startAfter })
			if i < len(keys) {
				filtered = keys[i:]
			}
			keys = filtered
		}

		if len(keys) > maxKeys {
			keys = keys[:maxKeys]
		}
		// Build XML response
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		b.WriteString(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`)
		b.WriteString(`<IsTruncated>false</IsTruncated>`)
		for _, k := range keys {
			b.WriteString(`<Contents><Key>`)
			b.WriteString(k)
			b.WriteString(`</Key></Contents>`)
		}
		b.WriteString(`</ListBucketResult>`)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(b.String()))
		return
	}

	if len(pathParts) < 2 {
		http.Error(w, "Invalid path: Key is required for GET", http.StatusBadRequest)
		return
	}

	key := pathParts[1]
	obj, exists := s.objects[key]
	if !exists {
		s.writeS3NoSuchKeyError(w)
		return
	}

	s.setObjectHeaders(w, obj)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(obj.body)
}

func (s *mockS3Server) handlePutRequest(w http.ResponseWriter, r *http.Request, pathParts []string) {
	if len(pathParts) < 2 {
		http.Error(w, "Invalid path: Key is required for PUT", http.StatusBadRequest)
		return
	}

	key := pathParts[1]

	// Handle conditional put
	if r.Header.Get("If-None-Match") == "*" {
		if _, exists := s.objects[key]; exists {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	metadata := s.extractMetadata(r.Header)
	crc32c := r.Header.Get("x-amz-checksum-crc32c")
	s.objects[key] = mockS3Object{
		body:         body,
		metadata:     metadata,
		crc32c:       crc32c,
		lastModified: time.Now(),
	}
	w.WriteHeader(http.StatusOK)
}

func (s *mockS3Server) setObjectHeaders(w http.ResponseWriter, obj mockS3Object) {
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(obj.body)))
	for k, v := range obj.metadata {
		w.Header().Set("x-amz-meta-"+k, v)
	}
	if obj.crc32c != "" {
		w.Header().Set("x-amz-checksum-crc32c", obj.crc32c)
	}
	if !obj.lastModified.IsZero() {
		w.Header().Set("Last-Modified", obj.lastModified.UTC().Format(http.TimeFormat))
	}
}

func (s *mockS3Server) writeS3NoSuchKeyError(w http.ResponseWriter) {
	const s3NoSuchKeyError = `<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(s3NoSuchKeyError))
}

func (s *mockS3Server) extractMetadata(headers http.Header) map[string]string {
	metadata := make(map[string]string)
	for k, v := range headers {
		if strings.HasPrefix(strings.ToLower(k), "x-amz-meta-") {
			metaKey := strings.TrimPrefix(strings.ToLower(k), "x-amz-meta-")
			metadata[metaKey] = v[0]
		}
	}
	return metadata
}

// setupTestS3DataStore is a helper function that initializes the mock server and an S3DataStore instance.
func setupTestS3DataStore(t *testing.T, ctx context.Context, bucketPath string, initObjects map[string]mockS3Object) (DataStore, func()) {
	t.Helper()
	mockServer := &mockS3Server{
		objects: make(map[string]mockS3Object),
	}
	// Initialize the mock server with provided objects.
	for key, obj := range initObjects {
		// Ensure lastModified is set if not already set
		if obj.lastModified.IsZero() {
			obj.lastModified = time.Now()
		}
		mockServer.objects[key] = obj
	}
	server := httptest.NewServer(mockServer)

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-west-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("KEY", "SECRET", "")),
	)
	require.NoError(t, err)

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(server.URL)
		o.UsePathStyle = true
	})

	store, err := FromS3Client(ctx, client, bucketPath)
	require.NoError(t, err)

	teardown := func() {
		server.Close()
	}

	return store, teardown
}

func TestS3ListFilePaths(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet/", map[string]mockS3Object{
		"objects/testnet-1/a": {body: []byte("1")},
		"objects/testnet/a":   {body: []byte("1")},
		"objects/testnet/b":   {body: []byte("1")},
		"objects/testnet/c":   {body: []byte("1")},
	})
	defer teardown()

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, paths)
}

func TestS3ListFilePaths_NoPrefix(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket", map[string]mockS3Object{
		"a": {body: []byte("1")},
		"b": {body: []byte("1")},
		"c": {body: []byte("1")},
	})
	defer teardown()

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "c"}, paths)

	paths, err = store.ListFilePaths(context.Background(), ListFileOptions{Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, paths)

	paths, err = store.ListFilePaths(context.Background(), ListFileOptions{StartAfter: "a"})
	require.NoError(t, err)
	require.Equal(t, []string{"b", "c"}, paths)
}

func TestS3ListFilePaths_WithPrefix(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", map[string]mockS3Object{
		"objects/testnet/a/x": {body: []byte("1")},
		"objects/testnet/a/y": {body: []byte("1")},
		"objects/testnet/b/z": {body: []byte("1")},
	})
	defer teardown()

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{Prefix: "a", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, []string{"a/x", "a/y"}, paths)
}

func TestS3ListFilePaths_LimitDefaultAndCap(t *testing.T) {
	ctx := context.Background()
	init := map[string]mockS3Object{}
	for i := 0; i < 1200; i++ {
		key := fmt.Sprintf("objects/testnet/%04d", i)
		init[key] = mockS3Object{body: []byte("1")}
	}
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", init)
	defer teardown()

	// limit <= 0 defaults to 1000
	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{})
	require.NoError(t, err)
	require.Equal(t, 1000, len(paths))

	// limit > 1000 is capped at 1000
	paths, err = store.ListFilePaths(context.Background(), ListFileOptions{Limit: 5000})
	require.NoError(t, err)
	require.Equal(t, 1000, len(paths))
}

func TestS3ListFilePaths_StartAfter(t *testing.T) {
	ctx := context.Background()

	t.Run("basic start-after (no Prefix)", func(t *testing.T) {
		init := map[string]mockS3Object{}
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("objects/testnet/%04d", i)
			init[key] = mockS3Object{body: []byte("x")}
		}
		// decoy outside the prefix directory style
		init["objects/testnet-foo/0000"] = mockS3Object{body: []byte("x")}

		store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", init)
		defer teardown()

		paths, err := store.ListFilePaths(ctx, ListFileOptions{
			StartAfter: "0005",
		})
		require.NoError(t, err)
		require.Equal(t, []string{"0006", "0007", "0008", "0009"}, paths, "should start strictly after 0005 and trim prefix")
	})

	t.Run("with Prefix directory and start-after inside it", func(t *testing.T) {
		init := map[string]mockS3Object{
			"objects/testnet/a/0001": {body: []byte("x")},
			"objects/testnet/a/0002": {body: []byte("x")},
			"objects/testnet/b/0001": {body: []byte("x")}, // different subdir; should be filtered by Prefix
			// decoy outside the prefix directory style
			"objects/testnet-foo/a/0001": {body: []byte("x")},
		}
		store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", init)
		defer teardown()

		paths, err := store.ListFilePaths(ctx, ListFileOptions{
			Prefix:     "a/",     // ensure we only list under a/
			StartAfter: "a/0001", // start strictly after this key
		})
		require.NoError(t, err)
		require.Equal(t, []string{"a/0002"}, paths)
	})

	t.Run("start-after equals last key -> empty", func(t *testing.T) {
		init := map[string]mockS3Object{
			"objects/testnet/0000": {body: []byte("x")},
			"objects/testnet/0001": {body: []byte("x")},
			"objects/testnet/0002": {body: []byte("x")},
			// decoy outside the prefix directory style
			"objects/testnet-foo/0001": {body: []byte("x")},
		}
		store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", init)
		defer teardown()

		paths, err := store.ListFilePaths(ctx, ListFileOptions{
			StartAfter: "0002",
		})
		require.NoError(t, err)
		require.Empty(t, paths)
	})

	t.Run("start-after before first key -> all returned", func(t *testing.T) {
		init := map[string]mockS3Object{
			"objects/testnet/0001": {body: []byte("x")},
			"objects/testnet/0002": {body: []byte("x")},
			"objects/testnet/0003": {body: []byte("x")},
			// decoy outside the prefix directory style
			"objects/testnet-foo/0001": {body: []byte("x")},
		}
		store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", init)
		defer teardown()

		paths, err := store.ListFilePaths(ctx, ListFileOptions{
			StartAfter: "0000",
		})
		require.NoError(t, err)
		require.Equal(t, []string{"0001", "0002", "0003"}, paths)
	})

	t.Run("start-after missing-but-between keys -> starts at next greater", func(t *testing.T) {
		init := map[string]mockS3Object{
			"objects/testnet/0002": {body: []byte("x")},
			"objects/testnet/0004": {body: []byte("x")},
			"objects/testnet/0006": {body: []byte("x")},
			// decoy outside the prefix directory style
			"objects/testnet-foo/0001": {body: []byte("x")},
		}
		store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", init)
		defer teardown()

		paths, err := store.ListFilePaths(ctx, ListFileOptions{
			StartAfter: "0003", // not present, should start at 0004
		})
		require.NoError(t, err)
		require.Equal(t, []string{"0004", "0006"}, paths)
	})

	t.Run("respects limit together with start-after", func(t *testing.T) {
		init := map[string]mockS3Object{}
		for i := 0; i < 10; i++ {
			init[fmt.Sprintf("objects/testnet/%04d", i)] = mockS3Object{body: []byte("x")}
		}
		store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", init)
		defer teardown()

		paths, err := store.ListFilePaths(ctx, ListFileOptions{
			StartAfter: "0004",
			Limit:      3, // only the next 3 after 0004
		})
		require.NoError(t, err)
		require.Equal(t, []string{"0005", "0006", "0007"}, paths)
	})
}

func TestS3Exists(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", map[string]mockS3Object{})
	defer teardown()

	content := []byte("inside the file")
	err := store.PutFile(ctx, "file.txt", bytes.NewReader(content), nil)
	require.NoError(t, err)

	exists, err := store.Exists(ctx, "file.txt")
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = store.Exists(ctx, "missing-file.txt")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestS3Size(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", map[string]mockS3Object{})
	defer teardown()

	content := []byte("inside the file")
	err := store.PutFile(ctx, "file.txt", bytes.NewReader(content), nil)
	require.NoError(t, err)

	size, err := store.Size(ctx, "file.txt")
	require.NoError(t, err)
	require.Equal(t, int64(len(content)), size)

	_, err = store.Size(ctx, "missing-file.txt")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestS3PutFile(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", map[string]mockS3Object{})
	defer teardown()

	content := []byte("inside the file")
	writerTo := &writerToRecorder{
		WriterTo: bytes.NewReader(content),
	}
	err := store.PutFile(ctx, "file.txt", writerTo, nil)
	require.NoError(t, err)
	require.Equal(t, int64(len(content)), writerTo.total)

	reader, err := store.GetFile(ctx, "file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, content)

	metadata, err := store.GetFileMetadata(ctx, "file.txt")
	require.NoError(t, err)
	require.Equal(t, map[string]string(nil), metadata)

	otherContent := []byte("other text")
	writerTo = &writerToRecorder{
		WriterTo: bytes.NewReader(otherContent),
	}
	err = store.PutFile(ctx, "file.txt", writerTo, nil)
	require.NoError(t, err)
	require.Equal(t, int64(len(otherContent)), writerTo.total)

	reader, err = store.GetFile(ctx, "file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, otherContent)

	metadata, err = store.GetFileMetadata(ctx, "file.txt")
	require.NoError(t, err)
	require.Equal(t, map[string]string(nil), metadata)
}

func TestS3GetFileLastModified(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", map[string]mockS3Object{})
	defer teardown()

	content := []byte("inside the file")
	err := store.PutFile(ctx, "file.txt", bytes.NewReader(content), nil)
	require.NoError(t, err)

	lastModified, err := store.GetFileLastModified(context.Background(), "file.txt")
	require.NoError(t, err)
	require.NotZero(t, lastModified)
}

func TestS3PutFileIfNotExists(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", map[string]mockS3Object{})
	defer teardown()

	existingContent := []byte("inside the file")
	err := store.PutFile(ctx, "file.txt", bytes.NewReader(existingContent), nil)
	require.NoError(t, err)

	newContent := []byte("overwrite the file")
	writerTo := &writerToRecorder{
		WriterTo: bytes.NewReader(newContent),
	}
	ok, err := store.PutFileIfNotExists(ctx, "file.txt", writerTo, nil)
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, int64(len(newContent)), writerTo.total)

	reader, err := store.GetFile(ctx, "file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, existingContent)

	metadata, err := store.GetFileMetadata(ctx, "file.txt")
	require.NoError(t, err)
	require.Equal(t, map[string]string(nil), metadata)

	writerTo = &writerToRecorder{
		WriterTo: bytes.NewReader(newContent),
	}
	ok, err = store.PutFileIfNotExists(ctx, "other-file.txt", writerTo, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(len(newContent)), writerTo.total)

	reader, err = store.GetFile(ctx, "other-file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, newContent)

	metadata, err = store.GetFileMetadata(ctx, "other-file.txt")
	require.NoError(t, err)
	require.Equal(t, map[string]string(nil), metadata)
}

func TestS3PutFileWithMetadata(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", map[string]mockS3Object{})
	defer teardown()

	metadataObj := MetaData{
		StartLedger:          1234,
		EndLedger:            1234,
		StartLedgerCloseTime: 1234,
		EndLedgerCloseTime:   1234,
		NetworkPassPhrase:    "testnet",
		CompressionType:      "zstd",
		ProtocolVersion:      21,
		CoreVersion:          "v1.2.3",
		Version:              "1.0.0",
	}

	content := []byte("inside the file")
	writerTo := &writerToRecorder{
		WriterTo: bytes.NewReader(content),
	}
	err := store.PutFile(ctx, "file.txt", writerTo, metadataObj.ToMap())
	require.NoError(t, err)
	require.Equal(t, int64(len(content)), writerTo.total)

	metadata, err := store.GetFileMetadata(ctx, "file.txt")
	require.NoError(t, err)
	require.Equal(t, metadataObj.ToMap(), metadata)

	modifiedMetadataObj := MetaData{
		StartLedger:          5678,
		EndLedger:            6789,
		StartLedgerCloseTime: 1622547800,
		EndLedgerCloseTime:   1622548900,
		NetworkPassPhrase:    "mainnet",
		CompressionType:      "gzip",
		ProtocolVersion:      23,
		CoreVersion:          "v1.4.0",
		Version:              "2.0.0",
	}

	otherContent := []byte("other text")
	writerTo = &writerToRecorder{
		WriterTo: bytes.NewReader(otherContent),
	}
	err = store.PutFile(ctx, "file.txt", writerTo, modifiedMetadataObj.ToMap())
	require.NoError(t, err)
	require.Equal(t, int64(len(otherContent)), writerTo.total)

	metadata, err = store.GetFileMetadata(ctx, "file.txt")
	require.NoError(t, err)
	require.Equal(t, modifiedMetadataObj.ToMap(), metadata)
}

func TestS3PutFileIfNotExistsWithMetadata(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", map[string]mockS3Object{})
	defer teardown()

	metadataObj := MetaData{
		StartLedger:          1234,
		EndLedger:            1234,
		StartLedgerCloseTime: 1234,
		EndLedgerCloseTime:   1234,
		NetworkPassPhrase:    "testnet",
		CompressionType:      "zstd",
		ProtocolVersion:      21,
		CoreVersion:          "v1.2.3",
		Version:              "1.0.0",
	}

	newContent := []byte("overwrite the file")
	writerTo := &writerToRecorder{
		WriterTo: bytes.NewReader(newContent),
	}
	ok, err := store.PutFileIfNotExists(ctx, "file.txt", writerTo, metadataObj.ToMap())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(len(newContent)), writerTo.total)

	metadata, err := store.GetFileMetadata(ctx, "file.txt")
	require.NoError(t, err)
	require.Equal(t, metadataObj.ToMap(), metadata)

	modifiedMetadataObj := MetaData{
		StartLedger:          5678,
		EndLedger:            6789,
		StartLedgerCloseTime: 1622547800,
		EndLedgerCloseTime:   1622548900,
		NetworkPassPhrase:    "mainnet",
		CompressionType:      "gzip",
		ProtocolVersion:      23,
		CoreVersion:          "v1.4.0",
		Version:              "2.0.0",
	}

	writerTo = &writerToRecorder{
		WriterTo: bytes.NewReader(newContent),
	}
	ok, err = store.PutFileIfNotExists(ctx, "file.txt", writerTo, modifiedMetadataObj.ToMap())
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, int64(len(newContent)), writerTo.total)

	metadata, err = store.GetFileMetadata(ctx, "file.txt")
	require.NoError(t, err)
	require.Equal(t, metadataObj.ToMap(), metadata)
}

func TestS3GetNonExistentFile(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", map[string]mockS3Object{})
	defer teardown()

	_, err := store.GetFile(ctx, "other-file.txt")
	require.ErrorIs(t, err, os.ErrNotExist)

	metadata, err := store.GetFileMetadata(ctx, "other-file.txt")
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Equal(t, map[string]string(nil), metadata)
}

func TestS3GetFileValidatesCRC32C(t *testing.T) {
	ctx := context.Background()
	store, teardown := setupTestS3DataStore(t, ctx, "test-bucket/objects/testnet", map[string]mockS3Object{
		"objects/testnet/file.txt": {
			body:         []byte("hello"),
			metadata:     map[string]string{},
			crc32c:       "VLn+tw==", // invalid CRC32C for the content
			lastModified: time.Now(),
		}})
	defer teardown()

	reader, err := store.GetFile(ctx, "file.txt")
	require.NoError(t, err)
	var buf bytes.Buffer
	_, err = io.Copy(&buf, reader)
	require.EqualError(t, err, "checksum did not match: algorithm CRC32C, expect VLn+tw==, actual mnG7TA==")
}
