package ledgerbackend

import (
	"bytes"
	"crypto/sha1"
	"io"
	"os"
	"sync"
	"time"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/support/log"
)

type hash []byte

func (h hash) Equals(other hash) bool {
	return bytes.Equal(h, other)
}

type fileWatcher struct {
	pathToFile string
	duration   time.Duration
	onChange   func()
	exit       <-chan struct{}
	log        *log.Entry
	hashFile   func(string) (hash, error)
	lastHash   hash
}

func hashFile(filename string) (hash, error) {
	f, err := os.Open(filename)
	if err != nil {
		return hash{}, errors.Wrapf(err, "unable to open %v", f)
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return hash{}, errors.Wrapf(err, "unable to copy %v into buffer", f)
	}

	return h.Sum(nil), nil
}

func newFileWatcher(runner *stellarCoreRunner) (*fileWatcher, error) {
	return newFileWatcherWithOptions(runner, hashFile, 10*time.Second)
}

func newFileWatcherWithOptions(
	runner *stellarCoreRunner,
	hashFile func(string) (hash, error),
	tickerDuration time.Duration,
) (*fileWatcher, error) {
	hashResult, err := hashFile(runner.executablePath)
	if err != nil {
		return nil, errors.Wrap(err, "could not hash captive core binary")
	}

	once := &sync.Once{}
	return &fileWatcher{
		pathToFile: runner.executablePath,
		duration:   tickerDuration,
		onChange: func() {
			once.Do(func() {
				runner.log.Warnf("detected new version of captive core binary %s , aborting session.", runner.executablePath)
				if err := runner.close(); err != nil {
					runner.log.Warnf("could not close captive core %v", err)
				}
			})
		},
		exit:     runner.ctx.Done(),
		log:      runner.log,
		hashFile: hashFile,
		lastHash: hashResult,
	}, nil
}

func (f *fileWatcher) loop() {
	ticker := time.NewTicker(f.duration)

	for {
		select {
		case <-f.exit:
			ticker.Stop()
			return
		case <-ticker.C:
			if f.fileChanged() {
				f.onChange()
			}
		}
	}
}

func (f *fileWatcher) fileChanged() bool {
	hashResult, err := f.hashFile(f.pathToFile)
	if err != nil {
		f.log.Warnf("could not hash contents of %s: %v", f.pathToFile, err)
		return false
	}

	if !f.lastHash.Equals(hashResult) {
		f.log.Infof(
			"detected update to %s. previous file hash was %v current hash is %v",
			f.pathToFile,
			f.lastHash,
			hashResult,
		)
		return true
	}
	return false
}
