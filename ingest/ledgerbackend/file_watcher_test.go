package ledgerbackend

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/support/log"

	"github.com/stretchr/testify/assert"
)

type mockHash struct {
	sync.Mutex
	t            *testing.T
	expectedPath string
	hashResult   hash
	err          error
	callCount    int
}

func (m *mockHash) setResponse(hashResult hash, err error) {
	m.Lock()
	defer m.Unlock()
	m.hashResult = hashResult
	m.err = err
}

func (m *mockHash) getCallCount() int {
	m.Lock()
	defer m.Unlock()
	return m.callCount
}

func (m *mockHash) hashFile(fp string) (hash, error) {
	m.Lock()
	defer m.Unlock()
	m.callCount++
	assert.Equal(m.t, m.expectedPath, fp)
	return m.hashResult, m.err
}

func createFWFixtures(t *testing.T) (*mockHash, *stellarCoreRunner, *fileWatcher) {
	storagePath := t.TempDir()

	ms := &mockHash{
		hashResult:   hash{},
		expectedPath: "/some/path",
		t:            t,
	}

	captiveCoreToml, err := NewCaptiveCoreToml(CaptiveCoreTomlParams{})
	assert.NoError(t, err)

	runner := newStellarCoreRunner(CaptiveCoreConfig{
		BinaryPath:         "/some/path",
		HistoryArchiveURLs: []string{"http://localhost"},
		Log:                log.New(),
		Context:            context.Background(),
		Toml:               captiveCoreToml,
		StoragePath:        storagePath,
	}, nil)

	fw, err := newFileWatcherWithOptions(runner, ms.hashFile, time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, 1, ms.getCallCount())

	return ms, runner, fw
}

func TestNewFileWatcherError(t *testing.T) {
	storagePath := t.TempDir()

	ms := &mockHash{
		hashResult:   hash{},
		expectedPath: "/some/path",
		t:            t,
	}
	ms.setResponse(hash{}, fmt.Errorf("test error"))

	captiveCoreToml, err := NewCaptiveCoreToml(CaptiveCoreTomlParams{})
	assert.NoError(t, err)

	runner := newStellarCoreRunner(CaptiveCoreConfig{
		BinaryPath:         "/some/path",
		HistoryArchiveURLs: []string{"http://localhost"},
		Log:                log.New(),
		Context:            context.Background(),
		Toml:               captiveCoreToml,
		StoragePath:        storagePath,
	}, nil)

	_, err = newFileWatcherWithOptions(runner, ms.hashFile, time.Millisecond)
	assert.EqualError(t, err, "could not hash captive core binary: test error")
	assert.Equal(t, 1, ms.getCallCount())
}

func TestFileChanged(t *testing.T) {
	ms, _, fw := createFWFixtures(t)

	assert.False(t, fw.fileChanged())
	assert.False(t, fw.fileChanged())
	assert.Equal(t, 3, ms.getCallCount())

	ms.setResponse(hash{}, fmt.Errorf("test error"))
	assert.False(t, fw.fileChanged())
	assert.Equal(t, 4, ms.getCallCount())

	ms.setResponse(ms.hashResult, nil)
	assert.False(t, fw.fileChanged())
	assert.Equal(t, 5, ms.getCallCount())

	ms.setResponse(hash{1}, nil)
	assert.True(t, fw.fileChanged())
	assert.Equal(t, 6, ms.getCallCount())
}

func TestCloseRunnerBeforeFileWatcherLoop(t *testing.T) {
	_, runner, fw := createFWFixtures(t)

	assert.NoError(t, runner.close())

	// loop should exit almost immediately because the runner is closed
	fw.loop()
}

func TestCloseRunnerDuringFileWatcherLoop(t *testing.T) {
	ms, runner, fw := createFWFixtures(t)
	done := make(chan struct{})
	go func() {
		fw.loop()
		close(done)
	}()

	// fw.loop will repeatedly check if the file has changed by calling hash.
	// This test ensures that closing the runner will exit fw.loop so that the goroutine is not leaked.

	closedRunner := false
	for {
		select {
		case <-done:
			assert.True(t, closedRunner)
			return
		default:
			if ms.getCallCount() > 20 {
				runner.close()
				closedRunner = true
			}
		}
	}
}

func TestFileChangesTriggerRunnerClose(t *testing.T) {
	ms, runner, fw := createFWFixtures(t)
	done := make(chan struct{})
	go func() {
		fw.loop()
		close(done)
	}()

	// fw.loop will repeatedly check if the file has changed by calling hash
	// This test ensures that modifying the file will trigger the closing of the runner.
	modifiedFile := false
	for {
		select {
		case <-done:
			assert.True(t, modifiedFile)
			// the runner is closed if and only if runner.ctx.Err() is non-nil
			assert.Error(t, runner.ctx.Err())
			return
		default:
			if ms.getCallCount() > 20 {
				ms.setResponse(hash{1}, nil)
				modifiedFile = true
			}
		}
	}
}
