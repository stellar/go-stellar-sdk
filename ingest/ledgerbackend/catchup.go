package ledgerbackend

import (
	"context"
	"fmt"

	"github.com/stellar/go-stellar-sdk/support/log"
)

type catchupStream struct {
	dir            workingDir
	from           uint32
	to             uint32
	coreCmdFactory coreCmdFactory
	log            *log.Entry
}

func newCatchupStream(r *stellarCoreRunner, from, to uint32) (catchupStream, error) {
	// We want to use ephemeral directories in running the catchup command
	// (used for the reingestion use case) because it's possible to run parallel
	// reingestion where multiple captive cores are active on the same machine.
	// Having ephemeral directories  will ensure that each ingestion worker will
	// have a separate working directory
	dir, err := newWorkingDir(r, true)
	if err != nil {
		return catchupStream{}, err
	}
	return catchupStream{
		dir:            dir,
		from:           from,
		to:             to,
		coreCmdFactory: newCoreCmdFactory(r, dir),
		log:            r.log,
	}, nil
}

func (s catchupStream) getWorkingDir() workingDir {
	return s.dir
}

func (s catchupStream) start(ctx context.Context) (cmdI, pipe, error) {
	var err error
	var cmd cmdI
	var captiveCorePipe pipe

	rangeArg := fmt.Sprintf("%d/%d", s.to, s.to-s.from+1)
	params := []string{"catchup", rangeArg, "--metadata-output-stream", s.coreCmdFactory.getPipeName()}

	cmd, err = s.coreCmdFactory.newCmd(ctx, stellarCoreRunnerModeOffline, true, "new-db")
	if err != nil {
		return nil, pipe{}, fmt.Errorf("error creating command: %w", err)
	}
	if err = cmd.Run(); err != nil {
		return nil, pipe{}, fmt.Errorf("error initializing core db: %w", err)
	}

	cmd, err = s.coreCmdFactory.newCmd(ctx, stellarCoreRunnerModeOffline, true, params...)
	if err != nil {
		return nil, pipe{}, fmt.Errorf("error creating command: %w", err)
	}

	captiveCorePipe, err = s.coreCmdFactory.startCaptiveCore(cmd)
	if err != nil {
		return nil, pipe{}, fmt.Errorf("error starting `stellar-core run` subprocess: %w", err)
	}

	return cmd, captiveCorePipe, nil
}
