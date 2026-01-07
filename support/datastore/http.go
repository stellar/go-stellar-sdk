package datastore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/stellar/go-stellar-sdk/support/log"
)

// HTTPDataStore implements DataStore for HTTP(S) endpoints.
// This is designed for read-only access to publicly available files.
type HTTPDataStore struct {
	client  *http.Client
	baseURL string
	headers map[string]string
}

// NewHTTPDataStore creates a new HTTP-based DataStore for read-only access to files.
//
// The datastoreConfig.Params map supports the following keys:
//
//   - "base_url" (required): The base HTTP or HTTPS URL for the datastore.
//     Must include the scheme (http:// or https://) and will have a trailing
//     slash appended if not present.
//
//   - "timeout" (optional): HTTP client timeout as a duration string (e.g., "30s", "1m").
//     Defaults to 30 seconds if not specified. Parsed using time.ParseDuration.
//
//   - "header_<name>" (optional): Custom HTTP headers to include in all requests.
//     The header name is derived by stripping the "header_" prefix from the key.
//     For example, "header_Authorization" sets the "Authorization" header.
//
// Example TOML configuration:
//
//	[datastore]
//	type = "HTTP"
//
//	[datastore.params]
//	base_url = "https://example.com/data/"
//	timeout = "60s"
//	header_Authorization = "Bearer token123"
//	header_X-Custom-Header = "custom-value"
func NewHTTPDataStore(datastoreConfig DataStoreConfig) (DataStore, error) {
	baseURL, ok := datastoreConfig.Params["base_url"]
	if !ok {
		return nil, errors.New("invalid HTTP config, no base_url")
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, errors.New("base_url must use http or https scheme")
	}

	if !strings.HasSuffix(baseURL, "/") {
		baseURL = baseURL + "/"
	}

	timeout := 30 * time.Second
	if timeoutStr, ok := datastoreConfig.Params["timeout"]; ok {
		parsedTimeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
		timeout = parsedTimeout
	}

	headers := make(map[string]string)
	for key, value := range datastoreConfig.Params {
		if strings.HasPrefix(key, "header_") {
			headerName := strings.TrimPrefix(key, "header_")
			headers[headerName] = value
		}
	}

	client := &http.Client{
		Timeout: timeout,
	}

	return &HTTPDataStore{
		client:  client,
		baseURL: baseURL,
		headers: headers,
	}, nil
}

func (h *HTTPDataStore) buildURL(filePath string) string {
	return h.baseURL + filePath
}

func (h *HTTPDataStore) addHeaders(req *http.Request) {
	for key, value := range h.headers {
		req.Header.Set(key, value)
	}
}

func (h *HTTPDataStore) checkHTTPStatus(resp *http.Response, filePath string) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return os.ErrNotExist
	default:
		return fmt.Errorf("HTTP error %d for file %s", resp.StatusCode, filePath)
	}
}

func (h *HTTPDataStore) doHeadRequest(ctx context.Context, filePath string) (*http.Response, error) {
	requestURL := h.buildURL(filePath)
	req, err := http.NewRequestWithContext(ctx, "HEAD", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HEAD request for %s: %w", filePath, err)
	}
	h.addHeaders(req)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HEAD request failed for %s: %w", filePath, err)
	}

	if err := h.checkHTTPStatus(resp, filePath); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return resp, nil
}

// GetFileMetadata retrieves basic metadata for a file via HTTP HEAD request.
func (h *HTTPDataStore) GetFileMetadata(ctx context.Context, filePath string) (map[string]string, error) {
	resp, err := h.doHeadRequest(ctx, filePath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	metadata := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			metadata[strings.ToLower(key)] = values[0]
		}
	}

	return metadata, nil
}

// GetFileLastModified retrieves the last modified time from HTTP headers.
func (h *HTTPDataStore) GetFileLastModified(ctx context.Context, filePath string) (time.Time, error) {
	metadata, err := h.GetFileMetadata(ctx, filePath)
	if err != nil {
		return time.Time{}, err
	}

	if lastModified, ok := metadata["last-modified"]; ok {
		return http.ParseTime(lastModified)
	}

	return time.Time{}, errors.New("last-modified header not found")
}

// GetFile downloads a file from the HTTP endpoint.
func (h *HTTPDataStore) GetFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	requestURL := h.buildURL(filePath)
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request for %s: %w", filePath, err)
	}
	h.addHeaders(req)

	resp, err := h.client.Do(req)
	if err != nil {
		log.Debugf("Error retrieving file '%s': %v", filePath, err)
		return nil, fmt.Errorf("GET request failed for %s: %w", filePath, err)
	}

	if err := h.checkHTTPStatus(resp, filePath); err != nil {
		resp.Body.Close()
		return nil, err
	}

	log.Debugf("File retrieved successfully: %s", filePath)
	return resp.Body, nil
}

// PutFile is not supported for HTTP datastore.
func (h *HTTPDataStore) PutFile(ctx context.Context, path string, in io.WriterTo, metaData map[string]string) error {
	return errors.New("HTTP datastore is read-only, PutFile not supported")
}

// PutFileIfNotExists is not supported for HTTP datastore.
func (h *HTTPDataStore) PutFileIfNotExists(ctx context.Context, path string, in io.WriterTo, metaData map[string]string) (bool, error) {
	return false, errors.New("HTTP datastore is read-only, PutFileIfNotExists not supported")
}

// Exists checks if a file exists by making a HEAD request.
func (h *HTTPDataStore) Exists(ctx context.Context, filePath string) (bool, error) {
	resp, err := h.doHeadRequest(ctx, filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer resp.Body.Close()

	return true, nil
}

// Size retrieves the file size from Content-Length header.
func (h *HTTPDataStore) Size(ctx context.Context, filePath string) (int64, error) {
	metadata, err := h.GetFileMetadata(ctx, filePath)
	if err != nil {
		return 0, err
	}

	if contentLength, ok := metadata["content-length"]; ok {
		size, err := strconv.ParseInt(contentLength, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid content-length header: %s", contentLength)
		}
		return size, nil
	}

	return 0, errors.New("content-length header not found")
}

// ListFilePaths is not supported for HTTP datastore.
func (h *HTTPDataStore) ListFilePaths(ctx context.Context, options ListFileOptions) ([]string, error) {
	return nil, errors.New("HTTP datastore does not support listing files")
}

// Close does nothing for HTTPDataStore as it does not maintain a persistent connection.
func (h *HTTPDataStore) Close() error {
	return nil
}
