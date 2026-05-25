package ledgerbackend

import (
	"context"
	"errors"
	"iter"

	"github.com/stellar/go-stellar-sdk/support/datastore"
	"github.com/stellar/go-stellar-sdk/support/log"
)

// LedgerStream is an interchangeable ingestion source — captive-core,
// buffered-storage (GCS/S3/…), RPC, and so on. Streaming is the only
// operation: an implementation owns its own setup and teardown, so there is no
// PrepareRange/GetLedger/Close for the consumer to sequence or misuse, and no
// locking for the consumer to reason about (a stream is consumed by a single
// goroutine).
type LedgerStream interface {
	// RawLedgers yields the raw XDR bytes of each ledger in ledgerRange, in
	// order. The source is set up on the first pull and fully torn down when
	// iteration ends — completion, an early break, an error, or ctx
	// cancellation. Each yielded slice is BORROWED: it is valid only until the
	// next iteration step, so copy it if you need to retain it. Cancel a blocked
	// stream via ctx.
	RawLedgers(ctx context.Context, ledgerRange Range) iter.Seq2[[]byte, error]
}

// rawReader is the per-backend machinery one RawLedgers iteration drives: it
// prepares the range, reads each ledger's raw frame, and releases all
// resources. The read returns a borrow valid only until the next read; because
// a stream exclusively owns its backend on a single goroutine, the read needs
// no lock (the only thing a backend's read lock guards is a concurrent Close,
// which can't happen here — a blocked read is cancelled via ctx instead).
type rawReader struct {
	prepare func(ctx context.Context, ledgerRange Range) error
	read    func(ctx context.Context, sequence uint32) ([]byte, error)
	close   func() error
}

// streamRaw is the shared skeleton behind every LedgerStream.RawLedgers: build
// the reader, prepare the range, yield each ledger's raw borrow in order, and
// close on exit (completion, break, error, or ctx cancellation). The teardown
// error is logged rather than returned — by the time close runs the caller's
// loop has already ended. The build func (and the reader it returns) is the
// only backend-specific part.
func streamRaw(
	ctx context.Context,
	ledgerRange Range,
	logger *log.Entry,
	name string,
	build func() (rawReader, error),
) iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		rr, err := build()
		if err != nil {
			yield(nil, err)
			return
		}
		defer func() {
			if cerr := rr.close(); cerr != nil {
				logger.WithError(cerr).Warnf("%s stream: error closing backend", name)
			}
		}()

		if err := rr.prepare(ctx, ledgerRange); err != nil {
			yield(nil, err)
			return
		}
		for seq := ledgerRange.from; ; seq++ {
			raw, err := rr.read(ctx, seq)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(raw, nil) {
				return
			}
			if ledgerRange.bounded && seq == ledgerRange.to {
				return
			}
		}
	}
}

// bufferedStorageStream is a LedgerStream backed by a DataStore (GCS, S3, …).
// Each RawLedgers call opens the datastore, runs a BufferedStorageBackend over
// the range, and tears both down when iteration ends.
var _ LedgerStream = (*bufferedStorageStream)(nil)

type bufferedStorageStream struct {
	config   BufferedStorageBackendConfig
	dsConfig datastore.DataStoreConfig
	log      *log.Entry

	// openStore creates the datastore + schema; overridable for tests. nil →
	// built from dsConfig.
	openStore func(context.Context) (datastore.DataStore, datastore.DataStoreSchema, error)
}

// NewBufferedStorageStream returns a LedgerStream that streams raw ledgers from
// the datastore described by dsConfig, tuned by cfg. The stream owns the
// datastore lifecycle: it is created when iteration begins and closed when
// iteration ends. If logger is nil a default logger is used; teardown errors
// are logged at Warn, since they cannot be returned once iteration has ended.
func NewBufferedStorageStream(
	cfg BufferedStorageBackendConfig,
	dsConfig datastore.DataStoreConfig,
	logger *log.Entry,
) LedgerStream {
	if logger == nil {
		logger = log.New()
	}
	return &bufferedStorageStream{config: cfg, dsConfig: dsConfig, log: logger}
}

func (s *bufferedStorageStream) open(ctx context.Context) (datastore.DataStore, datastore.DataStoreSchema, error) {
	if s.openStore != nil {
		return s.openStore(ctx)
	}
	ds, err := datastore.NewDataStore(ctx, s.dsConfig)
	if err != nil {
		return nil, datastore.DataStoreSchema{}, err
	}
	schema, err := datastore.LoadSchema(ctx, ds, s.dsConfig)
	if err != nil {
		if cerr := ds.Close(); cerr != nil {
			s.log.WithError(cerr).Warn("buffered storage stream: error closing datastore after schema load failure")
		}
		return nil, datastore.DataStoreSchema{}, err
	}
	return ds, schema, nil
}

func (s *bufferedStorageStream) RawLedgers(ctx context.Context, ledgerRange Range) iter.Seq2[[]byte, error] {
	return streamRaw(ctx, ledgerRange, s.log, "buffered storage", func() (rawReader, error) {
		ds, schema, err := s.open(ctx)
		if err != nil {
			return rawReader{}, err
		}
		bsb, err := NewBufferedStorageBackend(s.config, ds, schema)
		if err != nil {
			if cerr := ds.Close(); cerr != nil {
				s.log.WithError(cerr).Warn("buffered storage stream: error closing datastore")
			}
			return rawReader{}, err
		}
		return rawReader{
			prepare: bsb.PrepareRange,
			read:    bsb.getLedgerRaw,
			close:   func() error { return errors.Join(bsb.Close(), ds.Close()) },
		}, nil
	})
}

// captiveCoreStream is a LedgerStream backed by a captive stellar-core process.
// Each RawLedgers call starts a core process for the range and shuts it down
// when iteration ends.
var _ LedgerStream = (*captiveCoreStream)(nil)

type captiveCoreStream struct {
	config CaptiveCoreConfig
	log    *log.Entry

	// newCore builds the backend; overridable for tests. nil → NewCaptive(config).
	newCore func() (*CaptiveStellarCore, error)
}

// NewCaptiveCoreStream returns a LedgerStream backed by captive stellar-core.
// The stream owns the core process lifecycle: it is started when iteration
// begins and closed when iteration ends. If logger is nil a default logger is
// used; teardown errors are logged at Warn.
func NewCaptiveCoreStream(config CaptiveCoreConfig, logger *log.Entry) LedgerStream {
	if logger == nil {
		logger = log.New()
	}
	return &captiveCoreStream{config: config, log: logger}
}

func (s *captiveCoreStream) newCaptive() (*CaptiveStellarCore, error) {
	if s.newCore != nil {
		return s.newCore()
	}
	return NewCaptive(s.config)
}

func (s *captiveCoreStream) RawLedgers(ctx context.Context, ledgerRange Range) iter.Seq2[[]byte, error] {
	return streamRaw(ctx, ledgerRange, s.log, "captive-core", func() (rawReader, error) {
		c, err := s.newCaptive()
		if err != nil {
			return rawReader{}, err
		}
		return rawReader{
			prepare: c.PrepareRange,
			read: func(ctx context.Context, seq uint32) ([]byte, error) {
				if err := c.fetchSequence(ctx, seq); err != nil {
					return nil, err
				}
				return c.cached.Raw, nil
			},
			close: c.Close,
		}, nil
	})
}

// rpcStream is a LedgerStream backed by an RPC server. Each RawLedgers call
// drives a fresh RPCLedgerBackend over the range and closes it when iteration
// ends.
var _ LedgerStream = (*rpcStream)(nil)

type rpcStream struct {
	options RPCLedgerBackendOptions
	log     *log.Entry

	// newBackend builds the backend; overridable for tests. nil →
	// NewRPCLedgerBackend(options).
	newBackend func() *RPCLedgerBackend
}

// NewRPCStream returns a LedgerStream backed by an RPC server. The stream owns
// the backend lifecycle: it is created when iteration begins and closed when
// iteration ends. If logger is nil a default logger is used; teardown errors
// are logged at Warn.
func NewRPCStream(options RPCLedgerBackendOptions, logger *log.Entry) LedgerStream {
	if logger == nil {
		logger = log.New()
	}
	return &rpcStream{options: options, log: logger}
}

func (s *rpcStream) newRPC() *RPCLedgerBackend {
	if s.newBackend != nil {
		return s.newBackend()
	}
	return NewRPCLedgerBackend(s.options)
}

func (s *rpcStream) RawLedgers(ctx context.Context, ledgerRange Range) iter.Seq2[[]byte, error] {
	return streamRaw(ctx, ledgerRange, s.log, "rpc", func() (rawReader, error) {
		b := s.newRPC()
		return rawReader{
			prepare: b.PrepareRange,
			read:    b.fetchSequenceLocked,
			close:   b.Close,
		}, nil
	})
}
