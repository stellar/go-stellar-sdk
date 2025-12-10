package ledgerbackend

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stellar/go-stellar-sdk/support/log"
)

type stellarCoreRunnerInterface interface {
	catchup(from, to uint32) error
	runFrom(from uint32) error
	getMetaPipe() (<-chan metaResult, bool)
	context() context.Context
	getProcessExitError() (error, bool)
	close() error
}

type stellarCoreRunnerMode int

const (
	_ stellarCoreRunnerMode = iota // unset
	stellarCoreRunnerModeOnline
	stellarCoreRunnerModeOffline
)

// stellarCoreRunner uses a named pipe ( https://en.wikipedia.org/wiki/Named_pipe ) to stream ledgers directly
// from Stellar Core
type pipe struct {
	// stellarCoreRunner will be reading ledgers emitted by Stellar Core from the pipe.
	// After the Stellar Core process exits, stellarCoreRunner should eventually close the reader.
	Reader io.ReadCloser
	// stellarCoreRunner is responsible for closing the named pipe file after the Stellar Core process exits.
	// However, only the Stellar Core process will be writing to the pipe. stellarCoreRunner should not
	// write anything to the named pipe file which is why the type of File is io.Closer.
	File io.Closer
}

type executionState struct {
	cmd               cmdI
	workingDir        workingDir
	ledgerBuffer      *bufferedLedgerMetaReader
	pipe              pipe
	wg                sync.WaitGroup
	processExitedLock sync.RWMutex
	processExited     bool
	processExitError  error
	log               *log.Entry
}

type stellarCoreRunner struct {
	executablePath string
	ctx            context.Context
	cancel         context.CancelFunc

	systemCaller systemCaller

	stateLock sync.Mutex
	state     *executionState

	closeOnce sync.Once

	storagePath string
	toml        *CaptiveCoreToml

	captiveCoreNewDBCounter prometheus.Counter

	log *log.Entry
}

func createRandomHexString(n int) string {
	hex := []rune("abcdef1234567890")
	b := make([]rune, n)
	for i := range b {
		b[i] = hex[rand.Intn(len(hex))]
	}
	return string(b)
}

func newStellarCoreRunner(config CaptiveCoreConfig, captiveCoreNewDBCounter prometheus.Counter) *stellarCoreRunner {
	ctx, cancel := context.WithCancel(config.Context)

	runner := &stellarCoreRunner{
		executablePath: config.BinaryPath,
		ctx:            ctx,
		cancel:         cancel,
		storagePath:    config.StoragePath,
		log:            config.Log,
		toml:           config.Toml,

		captiveCoreNewDBCounter: captiveCoreNewDBCounter,
		systemCaller:            realSystemCaller{},
	}

	return runner
}

// context returns the context.Context instance associated with the running captive core instance
func (r *stellarCoreRunner) context() context.Context {
	return r.ctx
}

// runFrom executes the run command with a starting ledger on the captive core subprocess
func (r *stellarCoreRunner) runFrom(from uint32) error {
	stream, err := newRunFromStream(r, from, r.captiveCoreNewDBCounter)
	if err != nil {
		return err
	}
	return r.startMetaStream(stream)
}

// catchup executes the catchup command on the captive core subprocess
func (r *stellarCoreRunner) catchup(from, to uint32) error {
	stream, err := newCatchupStream(r, from, to)
	if err != nil {
		return err
	}
	return r.startMetaStream(stream)
}

type metaStream interface {
	getWorkingDir() workingDir
	start(ctx context.Context) (cmdI, pipe, error)
}

func (r *stellarCoreRunner) startMetaStream(stream metaStream) error {
	r.stateLock.Lock()
	defer r.stateLock.Unlock()

	// check if we have already been closed
	if r.ctx.Err() != nil {
		return r.ctx.Err()
	}

	if r.state != nil {
		return fmt.Errorf("runner already started")
	}

	state := &executionState{
		workingDir: stream.getWorkingDir(),
		log:        r.log,
	}

	cmd, p, err := stream.start(r.ctx)
	if err != nil {
		state.workingDir.cleanup(nil)
		return err
	}

	state.cmd = cmd
	state.pipe = p
	state.ledgerBuffer = newBufferedLedgerMetaReader(state.pipe.Reader)
	go state.ledgerBuffer.start()

	if binaryWatcher, err := newFileWatcher(r); err != nil {
		r.log.Warnf("could not create captive core binary watcher: %v", err)
	} else {
		go binaryWatcher.loop()
	}

	state.wg.Add(1)
	go state.handleExit()

	r.state = state
	return nil
}

func (r *stellarCoreRunner) getExecutionState() *executionState {
	r.stateLock.Lock()
	defer r.stateLock.Unlock()
	return r.state
}

func (state *executionState) handleExit() {
	defer state.wg.Done()

	waitErr := state.cmd.Wait()

	// By closing the pipe file we will send an EOF to the pipe reader used by ledgerBuffer.
	if err := state.pipe.File.Close(); err != nil {
		state.log.WithError(err).Warn("could not close captive core write pipe")
	}

	state.processExitedLock.Lock()
	defer state.processExitedLock.Unlock()
	state.processExited = true
	state.processExitError = waitErr
}

func (state *executionState) getProcessExitError() (error, bool) {
	state.processExitedLock.RLock()
	defer state.processExitedLock.RUnlock()
	return state.processExitError, state.processExited
}

func (state *executionState) cleanup() error {
	// wait for the stellar core process to terminate
	state.wg.Wait()

	// drain meta pipe channel to make sure the ledger buffer goroutine exits
	for range state.ledgerBuffer.getChannel() {

	}

	// now it's safe to close the pipe reader
	// because the ledger buffer is no longer reading from it
	if err := state.pipe.Reader.Close(); err != nil {
		state.log.WithError(err).Warn("could not close captive core read pipe")
	}

	processExitError, _ := state.getProcessExitError()
	return state.workingDir.cleanup(processExitError)
}

// getMetaPipe returns a channel which contains ledgers streamed from the captive core subprocess
func (r *stellarCoreRunner) getMetaPipe() (<-chan metaResult, bool) {
	state := r.getExecutionState()
	if state == nil {
		return nil, false
	}
	return state.ledgerBuffer.getChannel(), true
}

// getProcessExitError returns an exit error (can be nil) of the process and a bool indicating
// if the process has exited yet
// getProcessExitError is thread safe
func (r *stellarCoreRunner) getProcessExitError() (error, bool) {
	state := r.getExecutionState()
	if state == nil {
		return nil, false
	}
	return state.getProcessExitError()
}

// close kills the captive core process if it is still running and performs
// the necessary cleanup on the resources associated with the captive core process
// close is both thread safe and idempotent
func (r *stellarCoreRunner) close() error {
	var closeError error
	r.closeOnce.Do(func() {
		r.cancel()
		state := r.getExecutionState()
		if state != nil {
			closeError = state.cleanup()
		}
	})
	return closeError
}
