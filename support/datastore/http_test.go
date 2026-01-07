package datastore

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mockHTTPServer holds test data and simulates HTTP responses
type mockHTTPServer struct {
	files map[string]mockHTTPFile
}

type mockHTTPFile struct {
	content      string
	lastModified time.Time
	headers      map[string]string
	exists       bool
}

func (s *mockHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Check for authentication header if required
	if strings.HasPrefix(path, "private/") {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	file, exists := s.files[path]
	if !exists || !file.exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Set custom headers
	for key, value := range file.headers {
		w.Header().Set(key, value)
	}

	// Set standard headers
	w.Header().Set("Content-Length", strconv.Itoa(len(file.content)))
	w.Header().Set("Last-Modified", file.lastModified.Format(http.TimeFormat))

	switch r.Method {
	case http.MethodHead:
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(file.content))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func setupMockServer() (*httptest.Server, *mockHTTPServer) {
	now := time.Now()
	mockServer := &mockHTTPServer{
		files: map[string]mockHTTPFile{
			"test.txt": {
				content:      "Hello, World!",
				lastModified: now,
				headers:      map[string]string{"Content-Type": "text/plain"},
				exists:       true,
			},
			"data/file.json": {
				content:      `{"key": "value"}`,
				lastModified: now.Add(-time.Hour),
				headers:      map[string]string{"Content-Type": "application/json"},
				exists:       true,
			},
			"private/secret.txt": {
				content:      "secret data",
				lastModified: now,
				headers:      map[string]string{"Content-Type": "text/plain"},
				exists:       true,
			},
		},
	}

	server := httptest.NewServer(mockServer)
	return server, mockServer
}

func TestNewHTTPDataStore(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := DataStoreConfig{
			Type: "HTTP",
			Params: map[string]string{
				"base_url": "https://example.com/",
			},
		}

		ds, err := NewHTTPDataStore(config)
		require.NoError(t, err)
		require.NotNil(t, ds)

		httpDS := ds.(*HTTPDataStore)
		require.Equal(t, "https://example.com/", httpDS.baseURL)
		require.Empty(t, httpDS.headers)
	})

	t.Run("config with path in base_url", func(t *testing.T) {
		config := DataStoreConfig{
			Type: "HTTP",
			Params: map[string]string{
				"base_url": "https://example.com/data/exports",
			},
		}

		ds, err := NewHTTPDataStore(config)
		require.NoError(t, err)

		httpDS := ds.(*HTTPDataStore)
		require.Equal(t, "https://example.com/data/exports/", httpDS.baseURL)
	})

	t.Run("config with custom headers", func(t *testing.T) {
		config := DataStoreConfig{
			Type: "HTTP",
			Params: map[string]string{
				"base_url":             "https://example.com/",
				"header_Authorization": "Bearer token",
				"header_X-API-Key":     "key123",
			},
		}

		ds, err := NewHTTPDataStore(config)
		require.NoError(t, err)

		httpDS := ds.(*HTTPDataStore)
		require.Equal(t, "Bearer token", httpDS.headers["Authorization"])
		require.Equal(t, "key123", httpDS.headers["X-API-Key"])
	})

	t.Run("missing base_url", func(t *testing.T) {
		config := DataStoreConfig{
			Type:   "HTTP",
			Params: map[string]string{},
		}

		_, err := NewHTTPDataStore(config)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no base_url")
	})
}

func TestHTTPDataStore_GetFile(t *testing.T) {
	server, _ := setupMockServer()
	defer server.Close()

	config := DataStoreConfig{
		Type: "HTTP",
		Params: map[string]string{
			"base_url": server.URL + "/",
		},
	}

	ds, err := NewHTTPDataStore(config)
	require.NoError(t, err)

	t.Run("get existing file", func(t *testing.T) {
		reader, err := ds.GetFile(context.Background(), "test.txt")
		require.NoError(t, err)
		defer reader.Close()

		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, "Hello, World!", string(content))
	})

	t.Run("get file with path", func(t *testing.T) {
		reader, err := ds.GetFile(context.Background(), "data/file.json")
		require.NoError(t, err)
		defer reader.Close()

		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, `{"key": "value"}`, string(content))
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := ds.GetFile(context.Background(), "nonexistent.txt")
		require.Error(t, err)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestHTTPDataStore_WithCustomHeaders(t *testing.T) {
	server, _ := setupMockServer()
	defer server.Close()

	config := DataStoreConfig{
		Type: "HTTP",
		Params: map[string]string{
			"base_url":             server.URL + "/",
			"header_Authorization": "Bearer test-token",
		},
	}

	ds, err := NewHTTPDataStore(config)
	require.NoError(t, err)

	t.Run("access private file with auth", func(t *testing.T) {
		reader, err := ds.GetFile(context.Background(), "private/secret.txt")
		require.NoError(t, err)
		defer reader.Close()

		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, "secret data", string(content))
	})
}

func TestHTTPDataStore_GetFileMetadata(t *testing.T) {
	server, _ := setupMockServer()
	defer server.Close()

	config := DataStoreConfig{
		Type: "HTTP",
		Params: map[string]string{
			"base_url": server.URL + "/",
		},
	}

	ds, err := NewHTTPDataStore(config)
	require.NoError(t, err)

	t.Run("get metadata for existing file", func(t *testing.T) {
		metadata, err := ds.GetFileMetadata(context.Background(), "test.txt")
		require.NoError(t, err)
		require.Equal(t, "text/plain", metadata["content-type"])
		require.Equal(t, "13", metadata["content-length"])
		require.NotEmpty(t, metadata["last-modified"])
	})

	t.Run("metadata for nonexistent file", func(t *testing.T) {
		_, err := ds.GetFileMetadata(context.Background(), "nonexistent.txt")
		require.Error(t, err)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestHTTPDataStore_GetFileLastModified(t *testing.T) {
	server, _ := setupMockServer()
	defer server.Close()

	config := DataStoreConfig{
		Type: "HTTP",
		Params: map[string]string{
			"base_url": server.URL + "/",
		},
	}

	ds, err := NewHTTPDataStore(config)
	require.NoError(t, err)

	t.Run("get last modified time", func(t *testing.T) {
		lastModified, err := ds.GetFileLastModified(context.Background(), "test.txt")
		require.NoError(t, err)
		require.False(t, lastModified.IsZero())
	})

	t.Run("last modified for nonexistent file", func(t *testing.T) {
		_, err := ds.GetFileLastModified(context.Background(), "nonexistent.txt")
		require.Error(t, err)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestHTTPDataStore_Exists(t *testing.T) {
	server, _ := setupMockServer()
	defer server.Close()

	config := DataStoreConfig{
		Type: "HTTP",
		Params: map[string]string{
			"base_url": server.URL + "/",
		},
	}

	ds, err := NewHTTPDataStore(config)
	require.NoError(t, err)

	t.Run("existing file", func(t *testing.T) {
		exists, err := ds.Exists(context.Background(), "test.txt")
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("nonexistent file", func(t *testing.T) {
		exists, err := ds.Exists(context.Background(), "nonexistent.txt")
		require.NoError(t, err)
		require.False(t, exists)
	})
}

func TestHTTPDataStore_Size(t *testing.T) {
	server, _ := setupMockServer()
	defer server.Close()

	config := DataStoreConfig{
		Type: "HTTP",
		Params: map[string]string{
			"base_url": server.URL + "/",
		},
	}

	ds, err := NewHTTPDataStore(config)
	require.NoError(t, err)

	t.Run("get file size", func(t *testing.T) {
		size, err := ds.Size(context.Background(), "test.txt")
		require.NoError(t, err)
		require.Equal(t, int64(13), size) // "Hello, World!" is 13 bytes
	})

	t.Run("size for nonexistent file", func(t *testing.T) {
		_, err := ds.Size(context.Background(), "nonexistent.txt")
		require.Error(t, err)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestHTTPDataStore_ReadOnlyOperations(t *testing.T) {
	server, _ := setupMockServer()
	defer server.Close()

	config := DataStoreConfig{
		Type: "HTTP",
		Params: map[string]string{
			"base_url": server.URL + "/",
		},
	}

	ds, err := NewHTTPDataStore(config)
	require.NoError(t, err)

	t.Run("PutFile not supported", func(t *testing.T) {
		err := ds.PutFile(context.Background(), "test.txt", nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "read-only")
	})

	t.Run("PutFileIfNotExists not supported", func(t *testing.T) {
		_, err := ds.PutFileIfNotExists(context.Background(), "test.txt", nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "read-only")
	})

	t.Run("ListFilePaths not supported", func(t *testing.T) {
		_, err := ds.ListFilePaths(context.Background(), ListFileOptions{Prefix: "", Limit: 10})
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not support listing")
	})
}

func TestHTTPDataStore_WithPathInBaseURL(t *testing.T) {
	server, _ := setupMockServer()
	defer server.Close()

	config := DataStoreConfig{
		Type: "HTTP",
		Params: map[string]string{
			"base_url": server.URL + "/data/",
		},
	}

	ds, err := NewHTTPDataStore(config)
	require.NoError(t, err)

	t.Run("get file with base URL path", func(t *testing.T) {
		reader, err := ds.GetFile(context.Background(), "file.json")
		require.NoError(t, err)
		defer reader.Close()

		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, `{"key": "value"}`, string(content))
	})
}
