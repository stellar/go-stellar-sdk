package ledgerbackend

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/stellar/go-stellar-sdk/protocols/stellarcore"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/support/log"
)

func TestCloseOffline(t *testing.T) {
	captiveCoreToml, err := NewCaptiveCoreToml(CaptiveCoreTomlParams{})
	assert.NoError(t, err)

	runner := newStellarCoreRunner(CaptiveCoreConfig{
		BinaryPath:         "/usr/bin/stellar-core",
		HistoryArchiveURLs: []string{"http://localhost"},
		Log:                log.New(),
		Context:            context.Background(),
		Toml:               captiveCoreToml,
		StoragePath:        "/tmp/captive-core",
	}, nil)

	cmdMock := simpleCommandMock()
	cmdMock.On("Wait").Return(nil)

	newDBCmdMock := simpleCommandMock()
	newDBCmdMock.On("Run").Return(nil)

	// Replace system calls with a mock
	scMock := &mockSystemCaller{}
	defer scMock.AssertExpectations(t)
	scMock.On("stat", mock.Anything).Return(isDirImpl(true), nil)
	scMock.On("writeFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"new-db",
	).Return(newDBCmdMock)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"catchup",
		"200/101",
		"--metadata-output-stream",
		"fd:3",
	).Return(cmdMock)
	scMock.On("removeAll", mock.Anything).Return(nil).Once()
	runner.systemCaller = scMock

	assert.NoError(t, runner.catchup(100, 200))
	assert.NoError(t, runner.close())
}

func TestCloseOnline(t *testing.T) {
	captiveCoreToml, err := NewCaptiveCoreToml(CaptiveCoreTomlParams{})
	assert.NoError(t, err)

	captiveCoreToml.AddExamplePubnetValidators()

	runner := newStellarCoreRunner(CaptiveCoreConfig{
		BinaryPath:         "/usr/bin/stellar-core",
		HistoryArchiveURLs: []string{"http://localhost"},
		Log:                log.New(),
		Context:            context.Background(),
		Toml:               captiveCoreToml,
		StoragePath:        "/tmp/captive-core",
	}, nil)

	cmdMock := simpleCommandMock()
	cmdMock.On("Wait").Return(nil)

	offlineInfoCmdMock := simpleCommandMock()
	infoResponse := stellarcore.InfoResponse{}
	infoResponse.Info.Ledger.Num = 100
	infoResponseBytes, err := json.Marshal(infoResponse)
	offlineInfoCmdMock.On("Output").Return(infoResponseBytes, nil)
	offlineInfoCmdMock.On("Wait").Return(nil)

	// Replace system calls with a mock
	scMock := &mockSystemCaller{}
	defer scMock.AssertExpectations(t)
	scMock.On("stat", mock.Anything).Return(isDirImpl(true), nil)
	scMock.On("writeFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"offline-info",
	).Return(offlineInfoCmdMock)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"run",
		"--metadata-output-stream",
		"fd:3",
	).Return(cmdMock)
	runner.systemCaller = scMock

	assert.NoError(t, runner.runFrom(100))
	assert.NoError(t, runner.close())
}

func TestCloseOnlineWithError(t *testing.T) {
	captiveCoreToml, err := NewCaptiveCoreToml(CaptiveCoreTomlParams{})
	assert.NoError(t, err)

	captiveCoreToml.AddExamplePubnetValidators()

	runner := newStellarCoreRunner(CaptiveCoreConfig{
		BinaryPath:         "/usr/bin/stellar-core",
		HistoryArchiveURLs: []string{"http://localhost"},
		Log:                log.New(),
		Context:            context.Background(),
		Toml:               captiveCoreToml,
		StoragePath:        "/tmp/captive-core",
	}, nil)

	cmdMock := simpleCommandMock()
	cmdMock.On("Wait").Return(errors.New("wait error"))

	offlineInfoCmdMock := simpleCommandMock()
	infoResponse := stellarcore.InfoResponse{}
	infoResponse.Info.Ledger.Num = 100
	infoResponseBytes, err := json.Marshal(infoResponse)
	offlineInfoCmdMock.On("Output").Return(infoResponseBytes, nil)
	offlineInfoCmdMock.On("Wait").Return(nil)

	// Replace system calls with a mock
	scMock := &mockSystemCaller{}
	defer scMock.AssertExpectations(t)
	scMock.On("stat", mock.Anything).Return(isDirImpl(true), nil)
	scMock.On("writeFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"offline-info",
	).Return(offlineInfoCmdMock)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"run",
		"--metadata-output-stream",
		"fd:3",
	).Return(cmdMock)
	scMock.On("removeAll", mock.Anything).Return(nil).Once()
	runner.systemCaller = scMock

	assert.NoError(t, runner.runFrom(100))

	// Wait with calling close until r.processExitError is set to Wait() error
	for {
		err, _ := runner.getProcessExitError()
		if err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.NoError(t, runner.close())
}

func TestCloseConcurrency(t *testing.T) {
	captiveCoreToml, err := NewCaptiveCoreToml(CaptiveCoreTomlParams{})
	assert.NoError(t, err)

	captiveCoreToml.AddExamplePubnetValidators()

	runner := newStellarCoreRunner(CaptiveCoreConfig{
		BinaryPath:         "/usr/bin/stellar-core",
		HistoryArchiveURLs: []string{"http://localhost"},
		Log:                log.New(),
		Context:            context.Background(),
		Toml:               captiveCoreToml,
		StoragePath:        "/tmp/captive-core",
	}, nil)

	cmdMock := simpleCommandMock()
	cmdMock.On("Wait").Return(errors.New("wait error")).WaitUntil(time.After(time.Millisecond * 300))
	defer cmdMock.AssertExpectations(t)

	newDBCmdMock := simpleCommandMock()
	newDBCmdMock.On("Run").Return(nil)

	// Replace system calls with a mock
	scMock := &mockSystemCaller{}
	defer scMock.AssertExpectations(t)
	scMock.On("stat", mock.Anything).Return(isDirImpl(true), nil)
	scMock.On("writeFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"new-db",
	).Return(newDBCmdMock)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"catchup",
		"200/101",
		"--metadata-output-stream",
		"fd:3",
	).Return(cmdMock)
	scMock.On("removeAll", mock.Anything).Return(nil).Once()
	runner.systemCaller = scMock

	assert.NoError(t, runner.catchup(100, 200))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, runner.close())
			err, exited := runner.getProcessExitError()
			assert.True(t, exited)
			assert.Error(t, err)
		}()
	}

	wg.Wait()
}

func TestRunFromUseDBLedgersMatch(t *testing.T) {
	captiveCoreToml, err := NewCaptiveCoreToml(CaptiveCoreTomlParams{})
	assert.NoError(t, err)

	captiveCoreToml.AddExamplePubnetValidators()

	runner := newStellarCoreRunner(CaptiveCoreConfig{
		BinaryPath:         "/usr/bin/stellar-core",
		HistoryArchiveURLs: []string{"http://localhost"},
		Log:                log.New(),
		Context:            context.Background(),
		Toml:               captiveCoreToml,
		StoragePath:        "/tmp/captive-core",
	}, createNewDBCounter())

	cmdMock := simpleCommandMock()
	cmdMock.On("Wait").Return(nil)

	offlineInfoCmdMock := simpleCommandMock()
	infoResponse := stellarcore.InfoResponse{}
	infoResponse.Info.Ledger.Num = 100
	infoResponseBytes, err := json.Marshal(infoResponse)
	assert.NoError(t, err)
	offlineInfoCmdMock.On("Output").Return(infoResponseBytes, nil)
	offlineInfoCmdMock.On("Wait").Return(nil)

	// Replace system calls with a mock
	scMock := &mockSystemCaller{}
	defer scMock.AssertExpectations(t)
	scMock.On("stat", mock.Anything).Return(isDirImpl(true), nil)
	scMock.On("writeFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"offline-info",
	).Return(offlineInfoCmdMock)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"run",
		"--metadata-output-stream",
		"fd:3",
	).Return(cmdMock)
	// removeAll not called
	runner.systemCaller = scMock

	assert.NoError(t, runner.runFrom(100))
	assert.NoError(t, runner.close())

	assert.Equal(t, float64(0), getNewDBCounterMetric(runner))
}

func TestRunFromUseDBLedgersBehind(t *testing.T) {
	captiveCoreToml, err := NewCaptiveCoreToml(CaptiveCoreTomlParams{})
	assert.NoError(t, err)

	captiveCoreToml.AddExamplePubnetValidators()

	runner := newStellarCoreRunner(CaptiveCoreConfig{
		BinaryPath:         "/usr/bin/stellar-core",
		HistoryArchiveURLs: []string{"http://localhost"},
		Log:                log.New(),
		Context:            context.Background(),
		Toml:               captiveCoreToml,
		StoragePath:        "/tmp/captive-core",
	}, createNewDBCounter())

	newDBCmdMock := simpleCommandMock()
	newDBCmdMock.On("Run").Return(nil)

	catchupCmdMock := simpleCommandMock()
	catchupCmdMock.On("Run").Return(nil)

	cmdMock := simpleCommandMock()
	cmdMock.On("Wait").Return(nil)

	offlineInfoCmdMock := simpleCommandMock()
	infoResponse := stellarcore.InfoResponse{}
	infoResponse.Info.Ledger.Num = 90 // runner is 10 ledgers behind
	infoResponseBytes, err := json.Marshal(infoResponse)
	assert.NoError(t, err)
	offlineInfoCmdMock.On("Output").Return(infoResponseBytes, nil)
	offlineInfoCmdMock.On("Wait").Return(nil)

	// Replace system calls with a mock
	scMock := &mockSystemCaller{}
	defer scMock.AssertExpectations(t)
	// Storage dir is not removed because ledgers do not match
	scMock.On("stat", mock.Anything).Return(isDirImpl(true), nil)
	scMock.On("writeFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"offline-info",
	).Return(offlineInfoCmdMock)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"run",
		"--metadata-output-stream",
		"fd:3",
	).Return(cmdMock)
	runner.systemCaller = scMock

	assert.NoError(t, runner.runFrom(100))
	assert.NoError(t, runner.close())

	assert.Equal(t, float64(0), getNewDBCounterMetric(runner))
}

func createNewDBCounter() prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "test", Subsystem: "captive_core", Name: "new_db_counter",
	})
}

func getNewDBCounterMetric(runner *stellarCoreRunner) float64 {
	value := &dto.Metric{}
	err := runner.captiveCoreNewDBCounter.Write(value)
	if err != nil {
		panic(err)
	}
	return value.GetCounter().GetValue()
}

func TestRunFromUseDBLedgersInFront(t *testing.T) {
	captiveCoreToml, err := NewCaptiveCoreToml(CaptiveCoreTomlParams{})
	assert.NoError(t, err)

	captiveCoreToml.AddExamplePubnetValidators()

	runner := newStellarCoreRunner(CaptiveCoreConfig{
		BinaryPath:         "/usr/bin/stellar-core",
		HistoryArchiveURLs: []string{"http://localhost"},
		Log:                log.New(),
		Context:            context.Background(),
		Toml:               captiveCoreToml,
		StoragePath:        "/tmp/captive-core",
	}, createNewDBCounter())

	newDBCmdMock := simpleCommandMock()
	newDBCmdMock.On("Run").Return(nil)

	catchupCmdMock := simpleCommandMock()
	catchupCmdMock.On("Run").Return(nil)

	cmdMock := simpleCommandMock()
	cmdMock.On("Wait").Return(nil)

	offlineInfoCmdMock := simpleCommandMock()
	infoResponse := stellarcore.InfoResponse{}
	infoResponse.Info.Ledger.Num = 110 // runner is 10 ledgers in front
	infoResponseBytes, err := json.Marshal(infoResponse)
	assert.NoError(t, err)
	offlineInfoCmdMock.On("Output").Return(infoResponseBytes, nil)
	offlineInfoCmdMock.On("Wait").Return(nil)

	// Replace system calls with a mock
	scMock := &mockSystemCaller{}
	defer scMock.AssertExpectations(t)
	// Storage dir is removed because ledgers do not match
	scMock.On("removeAll", mock.Anything).Return(nil).Once()
	scMock.On("stat", mock.Anything).Return(isDirImpl(true), nil)
	scMock.On("writeFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"offline-info",
	).Return(offlineInfoCmdMock)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"new-db",
	).Return(newDBCmdMock)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"catchup",
		"99/0",
	).Return(catchupCmdMock)
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"run",
		"--metadata-output-stream",
		"fd:3",
	).Return(cmdMock)
	runner.systemCaller = scMock

	assert.NoError(t, runner.runFrom(100))
	assert.NoError(t, runner.close())
	assert.Equal(t, float64(1), getNewDBCounterMetric(runner))
}

func TestRunFromRequestedSequence2(t *testing.T) {
	captiveCoreToml, err := NewCaptiveCoreToml(CaptiveCoreTomlParams{})
	assert.NoError(t, err)

	captiveCoreToml.AddExamplePubnetValidators()

	runner := newStellarCoreRunner(CaptiveCoreConfig{
		BinaryPath:         "/usr/bin/stellar-core",
		HistoryArchiveURLs: []string{"http://localhost"},
		Log:                log.New(),
		Context:            context.Background(),
		Toml:               captiveCoreToml,
		StoragePath:        "/tmp/captive-core",
	}, nil)

	cmdMock := simpleCommandMock()
	cmdMock.On("Wait").Return(nil)

	offlineInfoCmdMock := simpleCommandMock()
	infoResponse := stellarcore.InfoResponse{}
	infoResponse.Info.Ledger.Num = 100
	infoResponseBytes, err := json.Marshal(infoResponse)
	offlineInfoCmdMock.On("Output").Return(infoResponseBytes, nil)
	offlineInfoCmdMock.On("Wait").Return(nil)

	newDBCmdMock := simpleCommandMock()
	newDBCmdMock.On("Run").Return(nil)

	// Replace system calls with a mock
	scMock := &mockSystemCaller{}
	defer scMock.AssertExpectations(t)

	var writeFileCallCount int
	scMock.On("stat", mock.Anything).Return(isDirImpl(true), nil)
	scMock.On("writeFile", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		writeFileCallCount++
		// The writeFile call after the "run" command should contain CATCHUP_COMPLETE = true
		// First writeFile is for offline-info, second is for new-db, third is for run
		if writeFileCallCount == 3 {
			content := args.Get(1).([]byte)
			assert.Contains(t, string(content), "CATCHUP_COMPLETE = true")
		}
	}).Return(nil)
	scMock.On("removeAll", mock.Anything).Return(nil).Once()
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"offline-info",
	).Return(offlineInfoCmdMock).Once()
	// even though offline-info says LCL is 100, we expect new-db to be run due to requested ledger < 3
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"new-db",
	).Return(newDBCmdMock).Once()
	scMock.On("command",
		runner.ctx,
		"/usr/bin/stellar-core",
		"--conf",
		mock.Anything,
		"--console",
		"run",
		"--metadata-output-stream",
		"fd:3",
	).Return(cmdMock).Once()
	runner.systemCaller = scMock

	assert.NoError(t, runner.runFrom(2))
	assert.NoError(t, runner.close())
}
