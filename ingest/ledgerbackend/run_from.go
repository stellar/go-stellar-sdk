package ledgerbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stellar/go-stellar-sdk/protocols/stellarcore"
	"github.com/stellar/go-stellar-sdk/support/log"
)

type runFromStream struct {
	dir                     workingDir
	from                    uint32
	coreCmdFactory          coreCmdFactory
	log                     *log.Entry
	captiveCoreNewDBCounter prometheus.Counter
}

func newRunFromStream(r *stellarCoreRunner, from uint32, captiveCoreNewDBCounter prometheus.Counter) (runFromStream, error) {
	// We only use ephemeral directories on windows because there is
	// no way to terminate captive core gracefully on windows.
	// Having an ephemeral directory ensures that it is wiped out
	// whenever we terminate captive core
	dir, err := newWorkingDir(r, runtime.GOOS == "windows")
	if err != nil {
		return runFromStream{}, err
	}
	return runFromStream{
		dir:                     dir,
		from:                    from,
		coreCmdFactory:          newCoreCmdFactory(r, dir),
		log:                     r.log,
		captiveCoreNewDBCounter: captiveCoreNewDBCounter,
	}, nil
}

func (s runFromStream) getWorkingDir() workingDir {
	return s.dir
}

func (s runFromStream) offlineInfo(ctx context.Context) (stellarcore.InfoResponse, error) {
	cmd, err := s.coreCmdFactory.newCmd(ctx, stellarCoreRunnerModeOnline, false, "offline-info")
	if err != nil {
		return stellarcore.InfoResponse{}, fmt.Errorf("error creating offline-info cmd: %w", err)
	}
	output, err := cmd.Output()
	if err != nil {
		return stellarcore.InfoResponse{}, fmt.Errorf("error executing offline-info cmd: %w", err)
	}
	var info stellarcore.InfoResponse
	err = json.Unmarshal(output, &info)
	if err != nil {
		return stellarcore.InfoResponse{}, fmt.Errorf("invalid output of offline-info cmd: %w", err)
	}
	return info, nil
}

func (s runFromStream) start(ctx context.Context) (cmd cmdI, captiveCorePipe pipe, returnErr error) {
	var err error
	var createNewDB bool
	defer func() {
		if returnErr != nil && createNewDB {
			// if we could not start captive core remove the new db we created
			s.dir.remove()
		}
	}()

	// Check if on-disk core DB exists and what's the LCL there. If not what
	// we need remove storage dir and start from scratch.
	var info stellarcore.InfoResponse
	info, err = s.offlineInfo(ctx)
	if err != nil {
		s.log.Infof("Error running offline-info: %v, removing existing storage-dir contents", err)
		createNewDB = true
	} else if info.Info.Ledger.Num <= 2 || uint32(info.Info.Ledger.Num) > s.from {
		s.log.Infof("Unexpected LCL in Stellar-Core DB: %d (want: %d), removing existing storage-dir contents", info.Info.Ledger.Num, s.from)
		createNewDB = true
	}

	if createNewDB {
		if s.captiveCoreNewDBCounter != nil {
			s.captiveCoreNewDBCounter.Inc()
		}
		if err = s.dir.remove(); err != nil {
			return nil, pipe{}, fmt.Errorf("error removing existing storage-dir contents: %w", err)
		}

		cmd, err = s.coreCmdFactory.newCmd(ctx, stellarCoreRunnerModeOnline, true, "new-db")
		if err != nil {
			return nil, pipe{}, fmt.Errorf("error creating command: %w", err)
		}

		if err = cmd.Run(); err != nil {
			return nil, pipe{}, fmt.Errorf("error initializing core db: %w", err)
		}

		if s.from < 3 {
			// If the from is < 3, the caller wants ledger 2, to get that from core 'run'
			// we don't run catchup to set LCL, instead core db should be empty, with new db state with LCL=1
			// and instead we set CATCHUP_COMPLETE=true, which will trigger core to emit ledger 2 first
			s.getWorkingDir().enableCoreCatchupComplete()
		} else {
			// Do a quick catch-up in the empty db to set the LCL in core to be our expected starting point.
			cmd, err = s.coreCmdFactory.newCmd(ctx, stellarCoreRunnerModeOnline, true, "catchup", fmt.Sprintf("%d/0", s.from-1))
			if err != nil {
				return nil, pipe{}, fmt.Errorf("error creating command: %w", err)
			}

			if err = cmd.Run(); err != nil {
				return nil, pipe{}, fmt.Errorf("error running stellar-core catchup: %w", err)
			}
		}
	}

	cmd, err = s.coreCmdFactory.newCmd(
		ctx,
		stellarCoreRunnerModeOnline,
		true,
		"run",
		"--metadata-output-stream", s.coreCmdFactory.getPipeName(),
	)
	if err != nil {
		return nil, pipe{}, fmt.Errorf("error creating command: %w", err)
	}

	captiveCorePipe, err = s.coreCmdFactory.startCaptiveCore(cmd)
	if err != nil {
		return nil, pipe{}, fmt.Errorf("error starting `stellar-core run` subprocess: %w", err)
	}

	return cmd, captiveCorePipe, nil
}
