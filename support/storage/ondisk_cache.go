package storage

import (
	"io"
	"os"
	"path"

	lru "github.com/hashicorp/golang-lru"
	"github.com/stellar/go-stellar-sdk/support/log"
)

// OnDiskCache fronts another storage with a local filesystem cache. Its
// thread-safe, meaning you can be actively caching a file and retrieve it at
// the same time without corruption, because retrieval will wait for the fetch.
type OnDiskCache struct {
	Storage
	dir      string
	maxFiles int
	lru      *lru.Cache

	log *log.Entry
}

// MakeOnDiskCache wraps an Storage with a local filesystem cache in
// `dir`. If dir is blank, a temporary directory will be created. If `maxFiles`
// is zero, a default (90 days of ledgers) is used.
func MakeOnDiskCache(upstream Storage, dir string, maxFiles uint) (Storage, error) {
	if dir == "" {
		tmp, err := os.MkdirTemp(os.TempDir(), "stellar-horizon-*")
		if err != nil {
			return nil, err
		}
		dir = tmp
	}
	if maxFiles == 0 {
		// A guess at a reasonable number of checkpoints. This is 90 days of
		// ledgers. (90*86_400)/(5*64) = 24_300
		maxFiles = 24_300
	}

	backendLog := log.
		WithField("subservice", "fs-cache").
		WithField("path", dir).
		WithField("size", maxFiles)
	backendLog.Info("Filesystem cache configured")

	backend := &OnDiskCache{
		Storage:  upstream,
		dir:      dir,
		maxFiles: int(maxFiles),
		log:      backendLog,
	}

	cache, err := lru.NewWithEvict(int(maxFiles), backend.onEviction)
	if err != nil {
		return nil, err
	}

	backend.lru = cache
	return backend, nil
}

// GetFile retrieves the file contents from the local cache if present.
// Otherwise, it returns the same result that the wrapped backend returns and
// adds that result into the local cache, if possible.
func (b *OnDiskCache) GetFile(filepath string) (io.ReadCloser, error) {
	L := b.log.WithField("key", filepath)
	localPath := path.Join(b.dir, filepath)

	// If the lockfile exists, we should defer to the remote source but *not*
	// update the cache, as it means there's an in-progress sync of the same
	// file.
	_, statErr := os.Stat(NameLockfile(localPath))
	if statErr == nil {
		L.Debug("incomplete file in cache on disk")
		L.Debug("retrieving file from remote backend")
		return b.Storage.GetFile(filepath)
	} else if _, ok := b.lru.Get(localPath); !ok {
		// If it doesn't exist in the cache, it might still exist on the disk if
		// we've restarted from an existing directory.
		local, err := os.Open(localPath)
		if err == nil {
			L.Debug("found file on disk but not in cache, adding")
			b.lru.Add(localPath, struct{}{})
			return local, nil
		}

		L.Debug("retrieving file from remote backend")

		// Since it's not on-disk, pull it from the remote backend, shove it
		// into the cache, and write it to disk.
		remote, err := b.Storage.GetFile(filepath)
		if err != nil {
			return remote, err
		}

		local, err = b.createLocal(filepath)
		if err != nil {
			// If there's some local FS error, we can still continue with the
			// remote version, so just log it and continue.
			L.WithError(err).Error("caching ledger failed")
			return remote, nil
		}

		return teeReadCloser(remote, local, func() error {
			return os.Remove(NameLockfile(localPath))
		}), nil
	}

	// The cache claims it exists, so just give it a read and send it.
	local, err := os.Open(localPath)
	if err != nil {
		// Uh-oh, the cache and the disk are not in sync somehow? Let's evict
		// this value and try again (recurse) w/ the remote version.
		L.WithError(err).Warn("opening cached ledger failed")
		b.lru.Remove(localPath)
		return b.GetFile(filepath)
	}

	L.Debug("Found file in cache")
	return local, nil
}

// Exists shortcuts an existence check by checking if it exists in the cache.
// Otherwise, it returns the same result as the wrapped backend. Note that in
// the latter case, the cache isn't modified.
func (b *OnDiskCache) Exists(filepath string) (bool, error) {
	localPath := path.Join(b.dir, filepath)
	b.log.WithField("key", filepath).Debug("checking existence")

	if _, ok := b.lru.Get(localPath); ok {
		// If the cache says it's there, we can definitively say that this path
		// exists, even if we'd fail to `os.Stat()/Read()/etc.` it locally.
		return true, nil
	}

	return b.Storage.Exists(filepath)
}

// Size will return the size of the file found in the cache if possible.
// Otherwise, it returns the same result as the wrapped backend. Note that in
// the latter case, the cache isn't modified.
func (b *OnDiskCache) Size(filepath string) (int64, error) {
	localPath := path.Join(b.dir, filepath)
	L := b.log.WithField("key", filepath)

	L.Debug("retrieving size")
	if _, ok := b.lru.Get(localPath); ok {
		stats, err := os.Stat(localPath)
		if err == nil {
			L.Debugf("retrieved cached size: %d", stats.Size())
			return stats.Size(), nil
		}

		L.WithError(err).Debug("retrieving size of cached ledger failed")
		b.lru.Remove(localPath) // stale cache?
	}

	return b.Storage.Size(filepath)
}

// PutFile writes to the given `filepath` from the given `in` reader, also
// writing it to the local cache if possible. It returns the same result as the
// wrapped backend.
func (b *OnDiskCache) PutFile(filepath string, in io.ReadCloser) error {
	L := log.WithField("key", filepath)
	L.Debug("putting file")

	// Best effort to tee the upload off to the local cache as well
	local, err := b.createLocal(filepath)
	if err != nil {
		L.WithError(err).Error("failed to put file locally")
	} else {
		// tee upload data into our local file
		in = teeReadCloser(in, local, func() error {
			return os.Remove(NameLockfile(path.Join(b.dir, filepath)))
		})
	}

	return b.Storage.PutFile(filepath, in)
}

// Close purges the cache, then forwards the call to the wrapped backend.
func (b *OnDiskCache) Close() error {
	// We only purge the cache, leaving the filesystem untouched:
	// https://github.com/stellar/go-stellar-sdk/pull/4457#discussion_r929352643
	b.lru.Purge()
	return b.Storage.Close()
}

// Evict removes a file from the cache and the filesystem, but does not affect
// the upstream backend. It isn't part of the `Storage` interface.
func (b *OnDiskCache) Evict(filepath string) {
	log.WithField("key", filepath).Debug("evicting file")
	b.lru.Remove(path.Join(b.dir, filepath))
}

func (b *OnDiskCache) onEviction(key, value interface{}) {
	path := key.(string)
	os.Remove(NameLockfile(path))           // just in case
	if err := os.Remove(path); err != nil { // best effort removal
		b.log.WithError(err).
			WithField("key", path).
			Warn("removal failed after cache eviction")
	}
}

func (b *OnDiskCache) createLocal(filepath string) (*os.File, error) {
	localPath := path.Join(b.dir, filepath)
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

	b.lru.Add(localPath, struct{}{}) // just use the cache as an array
	return local, nil
}

func NameLockfile(file string) string {
	return file + ".lock"
}

// The below is a helper interface so that we can use io.TeeReader to write
// data locally immediately as we read it remotely.

type trc struct {
	io.Reader
	close func() error
}

func (t trc) Close() error {
	return t.close()
}

func teeReadCloser(r io.ReadCloser, w io.WriteCloser, onClose func() error) io.ReadCloser {
	return trc{
		Reader: io.TeeReader(r, w),
		close: func() error {
			// Always run all closers, but return the first error
			err1 := r.Close()
			err2 := w.Close()
			err3 := onClose()

			if err1 != nil {
				return err1
			} else if err2 != nil {
				return err2
			}
			return err3
		},
	}
}
