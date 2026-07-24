//go:build windows
// +build windows

package ledgerbackend

import (
	"fmt"

	"github.com/Microsoft/go-winio"
)

// Windows-specific methods for the stellarCoreRunner type.

func (c coreCmdFactory) getPipeName() string {
	return fmt.Sprintf(`\\.\pipe\%s`, c.nonce)
}

func (c coreCmdFactory) startCaptiveCore(cmd cmdI) (pipe, error) {
	// First set up the server pipe.
	listener, err := winio.ListenPipe(c.getPipeName(), nil)
	if err != nil {
		return pipe{}, err
	}

	// Then start the process.
	err = cmd.Start()
	if err != nil {
		listener.Close()
		return pipe{}, err
	}

	// Then accept on the server end.
	connection, err := listener.Accept()
	if err != nil {
		listener.Close()
		return pipe{}, err
	}

	return pipe{Reader: connection, File: listener}, nil
}

// startChainedCaptiveCore is unsupported on windows (the named pipe accepts a single
// connection); runFromStream gates on GOOS and never takes the chained path here.
func (c coreCmdFactory) startChainedCaptiveCore(cmd cmdI, p pipe) error {
	return fmt.Errorf("chained metadata streams are not supported on windows")
}
