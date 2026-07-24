//go:build !windows
// +build !windows

package ledgerbackend

import (
	"fmt"
	"os"
)

// Posix-specific methods for the StellarCoreRunner type.

func (c coreCmdFactory) getPipeName() string {
	// The exec.Cmd.ExtraFiles field carries *io.File values that are assigned
	// to child process fds counting from 3, and we'll be passing exactly one
	// fd: the write end of the anonymous pipe below.
	return "fd:3"
}

func (c coreCmdFactory) startCaptiveCore(cmd cmdI) (pipe, error) {
	// First make an anonymous pipe.
	// Note io.File objects close-on-finalization.
	readFile, writeFile, err := os.Pipe()
	if err != nil {
		return pipe{}, fmt.Errorf("error making a pipe: %w", err)
	}
	p := pipe{Reader: readFile, File: writeFile}

	// Add the write-end to the set of inherited file handles. This is defined
	// to be fd 3 on posix platforms.
	cmd.setExtraFiles([]*os.File{writeFile})
	err = cmd.Start()
	if err != nil {
		writeFile.Close()
		readFile.Close()
		return pipe{}, fmt.Errorf("error starting stellar-core: %w", err)
	}

	return p, nil
}

// startChainedCaptiveCore starts cmd writing to the pipe of a previous startCaptiveCore
// call, continuing its metadata stream (we hold the write fd open in between, so no EOF).
func (c coreCmdFactory) startChainedCaptiveCore(cmd cmdI, p pipe) error {
	writeFile, ok := p.File.(*os.File)
	if !ok {
		return fmt.Errorf("expected pipe write end to be *os.File, got %T", p.File)
	}
	cmd.setExtraFiles([]*os.File{writeFile})
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting stellar-core: %w", err)
	}
	return nil
}
