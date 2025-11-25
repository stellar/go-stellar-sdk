package storage

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/stellar/go-stellar-sdk/support/errors"
)

type Storage interface {
	Exists(path string) (bool, error)
	Size(path string) (int64, error)
	GetFile(path string) (io.ReadCloser, error)
	PutFile(path string, in io.ReadCloser) error
	ListFiles(path string) (chan string, chan error)
	CanListFiles() bool
	Close() error
}

type ConnectOptions struct {
	Context          context.Context
	S3Region         string
	S3Endpoint       string
	UnsignedRequests bool
	GCSEndpoint      string

	// When putting file object to s3 bucket, specify the ACL for the object.
	S3WriteACL string

	// UserAgent is the value of `User-Agent` header. Applicable only for HTTP
	// client.
	UserAgent string

	// Wrap the Storage after connection. For example, to add a caching or
	// introspection layer.
	Wrap func(Storage) (Storage, error)
}

func ConnectBackend(u string, opts ConnectOptions) (Storage, error) {
	if u == "" {
		return nil, errors.New("URL is empty")
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	if opts.Context == nil {
		opts.Context = context.Background()
	}

	pth := parsed.Path
	var backend Storage
	switch parsed.Scheme {
	case "s3":
		// Inside s3, all paths start _without_ the leading /
		pth = strings.TrimPrefix(pth, "/")
		backend, err = NewS3Storage(
			opts.Context,
			parsed.Host,
			pth,
			opts.S3Region,
			opts.S3Endpoint,
			opts.UnsignedRequests,
			opts.S3WriteACL,
		)

	case "gcs":
		// Inside gcs, all paths start _without_ the leading /
		pth = strings.TrimPrefix(pth, "/")
		backend, err = NewGCSBackend(
			opts.Context,
			parsed.Host,
			pth,
			opts.GCSEndpoint,
		)

	case "file":
		pth = path.Join(parsed.Host, pth)
		backend = NewFilesystemStorage(pth)

	case "http", "https":
		backend = NewHttpStorage(opts.Context, parsed, opts.UserAgent)

	default:
		err = errors.New("unknown URL scheme: '" + parsed.Scheme + "'")
	}
	if err == nil && opts.Wrap != nil {
		backend, err = opts.Wrap(backend)
	}
	return backend, err
}

func logReq(r *http.Request) {
	if r == nil {
		return
	}
	logFields := log.Fields{"method": r.Method, "url": r.URL.String()}
	log.WithFields(logFields).Trace("http: Req")
}

func logResp(r *http.Response) {
	if r == nil || r.Request == nil {
		return
	}
	logFields := log.Fields{"method": r.Request.Method, "status": r.Status, "url": r.Request.URL.String()}
	if r.StatusCode >= 200 && r.StatusCode < 400 {
		log.WithFields(logFields).Trace("http: OK")
	} else {
		log.WithFields(logFields).Warn("http: Bad")
	}
}
