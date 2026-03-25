package datastore

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/stretchr/testify/require"
)

func TestGCSExists(t *testing.T) {
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: "test-bucket",
				Name:       "objects/testnet/file.txt",
			},
			Content: []byte("inside the file"),
		},
	})
	defer server.Stop()

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	exists, err := store.Exists(context.Background(), "file.txt")
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = store.Exists(context.Background(), "missing-file.txt")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestGCSSize(t *testing.T) {
	content := []byte("inside the file")
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: "test-bucket",
				Name:       "objects/testnet/file.txt",
			},
			Content: content,
		},
	})
	defer server.Stop()

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	size, err := store.Size(context.Background(), "file.txt")
	require.NoError(t, err)
	require.Equal(t, int64(len(content)), size)

	_, err = store.Size(context.Background(), "missing-file.txt")
	require.ErrorIs(t, err, os.ErrNotExist)
}

type writerToRecorder struct {
	io.WriterTo
	total int64
}

func (r *writerToRecorder) WriteTo(w io.Writer) (int64, error) {
	count, err := r.WriterTo.WriteTo(w)
	r.total += count
	return count, err
}

func TestGCSPutFile(t *testing.T) {
	server := fakestorage.NewServer([]fakestorage.Object{})
	defer server.Stop()
	server.CreateBucketWithOpts(fakestorage.CreateBucketOpts{
		Name:                  "test-bucket",
		VersioningEnabled:     false,
		DefaultEventBasedHold: false,
	})

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	content := []byte("inside the file")
	writerTo := &writerToRecorder{
		WriterTo: bytes.NewReader(content),
	}
	err = store.PutFile(context.Background(), "file.txt", writerTo, nil)
	require.NoError(t, err)
	require.Equal(t, int64(len(content)), writerTo.total)

	reader, err := store.GetFile(context.Background(), "file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, content)

	metadata, err := store.GetFileMetadata(context.Background(), "file.txt")
	require.NoError(t, err)
	require.Equal(t, map[string]string(nil), metadata)

	otherContent := []byte("other text")
	writerTo = &writerToRecorder{
		WriterTo: bytes.NewReader(otherContent),
	}
	err = store.PutFile(context.Background(), "file.txt", writerTo, nil)
	require.NoError(t, err)
	require.Equal(t, int64(len(otherContent)), writerTo.total)

	reader, err = store.GetFile(context.Background(), "file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, otherContent)

	metadata, err = store.GetFileMetadata(context.Background(), "file.txt")
	require.NoError(t, err)
	require.Equal(t, map[string]string(nil), metadata)
}

func TestGCSPutFileIfNotExists(t *testing.T) {
	existingContent := []byte("inside the file")
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: "test-bucket",
				Name:       "objects/testnet/file.txt",
			},
			Content: existingContent,
		},
	})
	defer server.Stop()

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	newContent := []byte("overwrite the file")
	writerTo := &writerToRecorder{
		WriterTo: bytes.NewReader(newContent),
	}
	ok, err := store.PutFileIfNotExists(context.Background(), "file.txt", writerTo, nil)
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, int64(len(newContent)), writerTo.total)

	reader, err := store.GetFile(context.Background(), "file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, existingContent)

	metadata, err := store.GetFileMetadata(context.Background(), "file.txt")
	require.NoError(t, err)
	require.Equal(t, map[string]string(nil), metadata)

	writerTo = &writerToRecorder{
		WriterTo: bytes.NewReader(newContent),
	}
	ok, err = store.PutFileIfNotExists(context.Background(), "other-file.txt", writerTo, nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(len(newContent)), writerTo.total)

	reader, err = store.GetFile(context.Background(), "other-file.txt")
	require.NoError(t, err)
	requireReaderContentEquals(t, reader, newContent)

	metadata, err = store.GetFileMetadata(context.Background(), "other-file.txt")
	require.NoError(t, err)
	require.Equal(t, map[string]string(nil), metadata)
}

func TestGCSGetFileLastModified(t *testing.T) {
	server := fakestorage.NewServer([]fakestorage.Object{})
	defer server.Stop()
	server.CreateBucketWithOpts(fakestorage.CreateBucketOpts{
		Name:                  "test-bucket",
		VersioningEnabled:     false,
		DefaultEventBasedHold: false,
	})

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	content := []byte("inside the file")
	writerTo := &writerToRecorder{
		WriterTo: bytes.NewReader(content),
	}
	err = store.PutFile(context.Background(), "file.txt", writerTo, nil)
	require.NoError(t, err)

	lastModified, err := store.GetFileLastModified(context.Background(), "file.txt")
	require.NoError(t, err)
	require.NotZero(t, lastModified)
}

func TestGCSPutFileWithMetadata(t *testing.T) {
	server := fakestorage.NewServer([]fakestorage.Object{})
	defer server.Stop()
	server.CreateBucketWithOpts(fakestorage.CreateBucketOpts{
		Name:                  "test-bucket",
		VersioningEnabled:     false,
		DefaultEventBasedHold: false,
	})

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	// Initial metadata
	metadataObj := MetaData{StartLedger: 1234,
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
	err = store.PutFile(context.Background(), "file.txt", writerTo, metadataObj.ToMap())
	require.NoError(t, err)
	require.Equal(t, int64(len(content)), writerTo.total)

	metadata, err := store.GetFileMetadata(context.Background(), "file.txt")
	require.NoError(t, err)
	require.Equal(t, metadataObj.ToMap(), metadata)

	// Update metadata
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
	err = store.PutFile(context.Background(), "file.txt", writerTo, modifiedMetadataObj.ToMap())
	require.NoError(t, err)
	require.Equal(t, int64(len(otherContent)), writerTo.total)

	metadata, err = store.GetFileMetadata(context.Background(), "file.txt")
	require.NoError(t, err)
	require.Equal(t, modifiedMetadataObj.ToMap(), metadata)
}

func TestGCSPutFileIfNotExistsWithMetadata(t *testing.T) {
	existingContent := []byte("inside the file")
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: "test-bucket",
			},
			Content: existingContent,
		},
	})
	defer server.Stop()

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	metadataObj := MetaData{StartLedger: 1234,
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
	ok, err := store.PutFileIfNotExists(context.Background(), "file.txt", writerTo, metadataObj.ToMap())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(len(newContent)), writerTo.total)

	metadata, err := store.GetFileMetadata(context.Background(), "file.txt")
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
	ok, err = store.PutFileIfNotExists(context.Background(), "file.txt", writerTo, modifiedMetadataObj.ToMap())
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, int64(len(newContent)), writerTo.total)

	metadata, err = store.GetFileMetadata(context.Background(), "file.txt")
	require.NoError(t, err)
	require.Equal(t, metadataObj.ToMap(), metadata)
}

func TestGCSGetNonExistentFile(t *testing.T) {
	existingContent := []byte("inside the file")
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName: "test-bucket",
				Name:       "objects/testnet/file.txt",
			},
			Content: existingContent,
		},
	})
	defer server.Stop()

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	_, err = store.GetFile(context.Background(), "other-file.txt")
	require.ErrorIs(t, err, os.ErrNotExist)

	metadata, err := store.GetFileMetadata(context.Background(), "other-file.txt")
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Equal(t, map[string]string(nil), metadata)
}

func TestGCSGetFileValidatesCRC32C(t *testing.T) {
	// test on a gzipped file so we can verify ReadCompressed()
	// was called correctly
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Name = "file.gz"
	if _, err := zw.Write([]byte("gzipped object data")); err != nil {
		t.Fatalf("creating gzip: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}

	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{
				// set a CRC32C value which doesn't actually match the file contents
				Crc32c:          "mw/l0Q==",
				BucketName:      "test-bucket",
				Name:            "objects/testnet/file.gz",
				ContentEncoding: "gzip",
				ContentType:     "text/plain",
			},
			Content: buf.Bytes(),
		},
	})
	defer server.Stop()

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	reader, err := store.GetFile(context.Background(), "file.gz")
	require.NoError(t, err)
	buf.Reset()
	_, err = io.Copy(&buf, reader)
	require.EqualError(t, err, "storage: bad CRC on read: got 985946173, want 2601510353")
}

func TestGCSListFilePaths(t *testing.T) {
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet-1/a"},
			Content:     []byte("1"),
		},
		{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/a"},
			Content:     []byte("1"),
		},
		{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/b"},
			Content:     []byte("1"),
		},
		{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/c"},
			Content:     []byte("1"),
		},
	})
	defer server.Stop()

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet/")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{Limit: 2})
	require.NoError(t, err)

	require.Equal(t, []string{"a", "b"}, paths)
}

func TestGCSListFilePaths_NoPrefix(t *testing.T) {
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "a"},
			Content:     []byte("1"),
		},
		{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "b"},
			Content:     []byte("1"),
		},
		{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "c"},
			Content:     []byte("1"),
		},
	})
	defer server.Stop()

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

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

func TestGCSListFilePaths_WithPrefix(t *testing.T) {
	server := fakestorage.NewServer([]fakestorage.Object{
		{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/a/x"},
			Content:     []byte("1"),
		},
		{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/a/y"},
			Content:     []byte("1"),
		},
		{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/b/z"},
			Content:     []byte("1"),
		},
	})
	defer server.Stop()

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{Prefix: "a", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, []string{"a/x", "a/y"}, paths)
}

func TestGCSListFilePaths_LimitDefaultAndCap(t *testing.T) {
	objects := make([]fakestorage.Object, 0, 1200)
	for i := 0; i < 1200; i++ {
		objects = append(objects, fakestorage.Object{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: fmt.Sprintf("objects/testnet/%04d", i)},
			Content:     []byte("1"),
		})
	}
	server := fakestorage.NewServer(objects)
	defer server.Stop()

	store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	paths, err := store.ListFilePaths(context.Background(), ListFileOptions{})
	require.NoError(t, err)
	require.Equal(t, 1000, len(paths))

	paths, err = store.ListFilePaths(context.Background(), ListFileOptions{Limit: 5000})
	require.NoError(t, err)
	require.Equal(t, 1000, len(paths))
}

func TestGCSListFilePaths_StartAfter(t *testing.T) {
	t.Run("basic start-after (no Prefix)", func(t *testing.T) {
		objects := make([]fakestorage.Object, 0, 11)
		for i := 0; i < 10; i++ {
			objects = append(objects, fakestorage.Object{
				ObjectAttrs: fakestorage.ObjectAttrs{
					BucketName: "test-bucket",
					Name:       fmt.Sprintf("objects/testnet/%04d", i)},
				Content: []byte("x"),
			})
		}
		// decoy outside the prefix directory style
		objects = append(objects, fakestorage.Object{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet-foo/0000"},
			Content:     []byte("x"),
		})

		server := fakestorage.NewServer(objects)
		defer server.Stop()

		store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Close() })

		paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
			StartAfter: "0005",
		})
		require.NoError(t, err)
		require.Equal(t, []string{"0006", "0007", "0008", "0009"}, paths,
			"should start strictly after 0005 and trim prefix")
	})

	t.Run("with Prefix directory and start-after inside it", func(t *testing.T) {
		objects := []fakestorage.Object{
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/a/0001"}},
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/a/0002"}},
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/b/0002"}}, // different subdir
			// decoy outside the prefix directory style
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet-foo/0002"}},
		}
		server := fakestorage.NewServer(objects)
		defer server.Stop()

		store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Close() })

		paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
			Prefix:     "a/",
			StartAfter: "a/0001",
		})
		require.NoError(t, err)
		require.Equal(t, []string{"a/0002"}, paths)
	})

	t.Run("start-after equals last key -> empty", func(t *testing.T) {
		objects := []fakestorage.Object{
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/0000"}},
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/0001"}},
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/0002"}},
			// decoy outside the prefix directory style
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet-foo/0002"}},
		}
		server := fakestorage.NewServer(objects)
		defer server.Stop()

		store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Close() })

		paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
			StartAfter: "0002",
		})
		require.NoError(t, err)
		require.Empty(t, paths)
	})

	t.Run("start-after before first key -> all returned", func(t *testing.T) {
		objects := []fakestorage.Object{
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/0001"}},
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/0002"}},
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/0003"}},
			// decoy outside the prefix directory style
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet-foo/0002"}},
		}
		server := fakestorage.NewServer(objects)
		defer server.Stop()

		store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Close() })

		paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
			StartAfter: "0000",
		})
		require.NoError(t, err)
		require.Equal(t, []string{"0001", "0002", "0003"}, paths)
	})

	t.Run("start-after missing-but-between keys -> next greater", func(t *testing.T) {
		objects := []fakestorage.Object{
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/0002"}},
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/0004"}},
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet/0006"}},
			// decoy outside the prefix directory style
			{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet-foo/0002"}},
		}
		server := fakestorage.NewServer(objects)
		defer server.Stop()

		store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Close() })

		paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
			StartAfter: "0003",
		})
		require.NoError(t, err)
		require.Equal(t, []string{"0004", "0006"}, paths)
	})

	t.Run("respects limit together with start-after", func(t *testing.T) {
		objects := make([]fakestorage.Object, 0, 10)
		for i := 0; i < 10; i++ {
			objects = append(objects, fakestorage.Object{
				ObjectAttrs: fakestorage.ObjectAttrs{
					BucketName: "test-bucket",
					Name:       fmt.Sprintf("objects/testnet/%04d", i)},
				Content: []byte("x"),
			})
		}
		// decoy outside the prefix directory style
		objects = append(objects, fakestorage.Object{
			ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "test-bucket", Name: "objects/testnet-foo/0002"},
			Content:     []byte("x"),
		})

		server := fakestorage.NewServer(objects)
		defer server.Stop()

		store, err := FromGCSClient(context.Background(), server.Client(), "test-bucket/objects/testnet")
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Close() })

		paths, err := store.ListFilePaths(context.Background(), ListFileOptions{
			StartAfter: "0004",
			Limit:      3,
		})
		require.NoError(t, err)
		require.Equal(t, []string{"0005", "0006", "0007"}, paths)
	})
}
