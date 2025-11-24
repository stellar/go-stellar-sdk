package historyarchive

import (
	"io"
	"os"
	"path"

	lru "github.com/hashicorp/golang-lru"
	log "github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/support/storage"
)

type CacheOptions struct {
	Cache bool

	Path     string
	MaxFiles uint
	Log      *log.Entry
}

type ArchiveBucketCache struct {
	path string
	lru  *lru.Cache
	log  *log.Entry
}

// MakeArchiveBucketCache creates a cache on the disk at the given path that
// acts as an LRU cache, mimicking a particular upstream.
func MakeArchiveBucketCache(opts CacheOptions) (*ArchiveBucketCache, error) {
	log_ := opts.Log
	if opts.Log == nil {
		log_ = log.WithField("subservice", "fs-cache")
	}
	log_ = log_.
		WithField("path", opts.Path).
		WithField("cap", opts.MaxFiles)

	if _, err := os.Stat(opts.Path); err == nil || os.IsExist(err) {
		log_.Warnf("Cache directory already exists, removing")
		os.RemoveAll(opts.Path)
	}

	backend := &ArchiveBucketCache{
		path: opts.Path,
		log:  log_,
	}

	cache, err := lru.NewWithEvict(int(opts.MaxFiles), backend.onEviction)
	if err != nil {
		return &ArchiveBucketCache{}, err
	}
	backend.lru = cache

	log_.Info("Bucket cache initialized")
	return backend, nil
}

// GetFile retrieves the file contents from the local cache if present.
// Otherwise, it returns the same result as the upstream, adding that result
// into the local cache if possible. It returns a 3-tuple of a reader (which may
// be nil on an error), an indication of whether or not it was *found* in the
// cache, and any error.
func (abc *ArchiveBucketCache) GetFile(
	filepath string,
	upstream storage.Storage,
) (io.ReadCloser, bool, error) {
	L := abc.log.WithField("key", filepath)
	localPath := path.Join(abc.path, filepath)

	// If the lockfile exists, we should defer to the remote source but *not*
	// update the cache, as it means there's an in-progress sync of the same
	// file.
	_, statErr := os.Stat(NameLockfile(localPath))
	if statErr == nil || os.IsExist(statErr) {
		L.Info("Incomplete file in on-disk cache: deferring")
		reader, err := upstream.GetFile(filepath)
		return reader, false, err
	} else if _, ok := abc.lru.Get(localPath); !ok {
		L.Info("File does not exist in the cache: downloading")

		// Since it's not on-disk, pull it from the remote backend, shove it
		// into the cache, and write it to disk.
		remote, err := upstream.GetFile(filepath)
		if err != nil {
			return remote, false, err
		}

		local, err := abc.createLocal(filepath)
		if err != nil {
			// If there's some local FS error, we can still continue with the
			// remote version, so just log it and continue.
			L.WithError(err).Warn("Creating cache file failed")
			return remote, false, nil
		}

		return teeReadCloser(remote, local, func() error {
			L.Debug("Download complete: removing lockfile")
			return os.Remove(NameLockfile(localPath))
		}), false, nil
	}

	L.Info("Found file in cache")
	// The cache claims it exists, so just give it a read and send it.
	local, err := os.Open(localPath)
	if err != nil {
		// Uh-oh, the cache and the disk are not in sync somehow? Let's evict
		// this value and try again (recurse) w/ the remote version.
		L.WithError(err).Warn("Opening cached file failed")
		abc.lru.Remove(localPath)
		return abc.GetFile(filepath, upstream)
	}

	return local, true, nil
}

func (abc *ArchiveBucketCache) Exists(filepath string) bool {
	localPath := path.Join(abc.path, filepath)

	// First, check if the file exists in the cache.
	if abc.lru.Contains(localPath) {
		return true
	}

	// If it doesn't, it may still exist on the disk which is still a cheaper
	// check than going upstream.
	//
	// Note that this means the cache and disk are out of sync (perhaps due to
	// other archives using the same cache location) so we can update it. This
	// situation is well-handled by `GetFile`.
	_, statErr := os.Stat(localPath)
	if statErr == nil || os.IsExist(statErr) {
		abc.lru.Add(localPath, struct{}{})
		return true
	}

	return false
}

// Close purges the cache and cleans up the filesystem.
func (abc *ArchiveBucketCache) Close() error {
	abc.lru.Purge()
	return os.RemoveAll(abc.path)
}

// Evict removes a file from the cache and the filesystem.
func (abc *ArchiveBucketCache) Evict(filepath string) {
	log.WithField("key", filepath).Info("Evicting file from the disk")
	abc.lru.Remove(path.Join(abc.path, filepath))
}

func (abc *ArchiveBucketCache) onEviction(key, value interface{}) {
	path := key.(string)
	os.Remove(NameLockfile(path))           // just in case
	if err := os.Remove(path); err != nil { // best effort removal
		abc.log.WithError(err).
			WithField("key", path).
			Warn("Removal failed after cache eviction")
	}
}

func (abc *ArchiveBucketCache) createLocal(filepath string) (*os.File, error) {
	localPath := path.Join(abc.path, filepath)
	if err := os.MkdirAll(path.Dir(localPath), 0755 /* drwxr-xr-x */); err != nil {
		return nil, err
	}

	local, err := os.Create(localPath) /* mode -rw-rw-rw- */
	if err != nil {
		return nil, err
	}
	_, err = os.Create(NameLockfile(localPath))
	if err != nil {
		return nil, err
	}

	abc.lru.Add(localPath, struct{}{}) // just use the cache as an array
	return local, nil
}

func NameLockfile(file string) string {
	return file + ".lock"
}

// The below is a helper interface so that we can use io.TeeReader to write
// data locally immediately as we read it remotely.

type trc struct {
	io.Reader
	close  func() error
	closed bool // prevents a double-close
}

func (t trc) Close() error {
	if t.closed {
		return nil
	}

	return t.close()
}

func teeReadCloser(r io.ReadCloser, w io.WriteCloser, onClose func() error) io.ReadCloser {
	closer := trc{
		Reader: io.TeeReader(r, w),
		closed: false,
	}
	closer.close = func() error {
		if closer.closed {
			return nil
		}

		// Always run all closers, but return the first error
		err1 := r.Close()
		err2 := w.Close()
		err3 := onClose()

		closer.closed = true
		if err1 != nil {
			return err1
		} else if err2 != nil {
			return err2
		}
		return err3
	}

	return closer
}
