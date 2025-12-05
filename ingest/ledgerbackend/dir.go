package ledgerbackend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stellar/go-stellar-sdk/support/log"
)

type workingDir struct {
	ephemeral    bool
	path         string
	log          *log.Entry
	toml         *CaptiveCoreToml
	systemCaller systemCaller
}

func newWorkingDir(r *stellarCoreRunner, ephemeral bool) (workingDir, error) {
	var path string
	if ephemeral {
		path = filepath.Join(r.storagePath, "captive-core-"+createRandomHexString(8))
	} else {
		path = filepath.Join(r.storagePath, "captive-core")
	}

	// create clone of caller's toml config to avoid potential side effects if the config needs to
	// be modified internally such as for enabling CATCHUP_COMPLETE
	clonedToml, err := r.toml.clone()
	if err != nil {
		return workingDir{}, fmt.Errorf("failed to clone toml config: %w", err)
	}

	return workingDir{
		ephemeral:    ephemeral,
		path:         path,
		log:          r.log,
		toml:         clonedToml,
		systemCaller: r.systemCaller,
	}, nil
}

func (w workingDir) createIfNotExists() error {
	info, err := w.systemCaller.stat(w.path)
	if os.IsNotExist(err) {
		innerErr := w.systemCaller.mkdirAll(w.path, os.FileMode(int(0755))) // rwx|rx|rx
		if innerErr != nil {
			return fmt.Errorf("failed to create storage directory (%s): %w", w.path, innerErr)
		}
	} else if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", w.path)
	} else if err != nil {
		return fmt.Errorf("error accessing storage directory (%s): %w", w.path, err)
	}

	return nil
}

func (w workingDir) writeConf(mode stellarCoreRunnerMode) error {
	text, err := generateConfig(w.toml, mode)
	if err != nil {
		return err
	}

	w.log.Debugf("captive core config file contents:\n%s", string(text))
	return w.systemCaller.writeFile(w.getConfFileName(), text, 0644)
}

func (w workingDir) cleanup(coreExitError error) error {
	if w.ephemeral || (coreExitError != nil && !errors.Is(coreExitError, context.Canceled)) {
		return w.remove()
	}
	return nil
}

func (w workingDir) remove() error {
	return w.systemCaller.removeAll(w.path)
}

// enableCoreCatchupComplete sets CATCHUP_COMPLETE=true in the captive core toml configuration
func (w workingDir) enableCoreCatchupComplete() {
	w.toml.CatchupComplete = true
}

func generateConfig(captiveCoreToml *CaptiveCoreToml, mode stellarCoreRunnerMode) ([]byte, error) {
	if mode == stellarCoreRunnerModeOffline {
		var err error
		captiveCoreToml, err = captiveCoreToml.CatchupToml()
		if err != nil {
			return nil, fmt.Errorf("could not generate catch up config: %w", err)
		}
	}

	if !captiveCoreToml.QuorumSetIsConfigured() {
		return nil, fmt.Errorf("captive-core config file does not define any quorum set")
	}

	text, err := captiveCoreToml.Marshal()
	if err != nil {
		return nil, fmt.Errorf("could not marshal captive core config: %w", err)
	}
	return text, nil
}

func (w workingDir) getConfFileName() string {
	joinedPath := filepath.Join(w.path, "stellar-core.conf")

	// Given that `storagePath` can be anything, we need the full, absolute path
	// here so that everything Core needs is created under the storagePath
	// subdirectory.
	//
	// If the path *can't* be absolutely resolved (bizarre), we can still try
	// recovering by using the path the user specified directly.
	path, err := filepath.Abs(joinedPath)
	if err != nil {
		w.log.Warnf("Failed to resolve %s as an absolute path: %s", joinedPath, err)
		return joinedPath
	}
	return path
}
