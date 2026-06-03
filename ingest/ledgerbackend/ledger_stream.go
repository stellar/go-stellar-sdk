package ledgerbackend

import (
	"context"
	"errors"
	"iter"
	"time"

	"github.com/prometheus/client_golang/prometheus"

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
	//
	// Pass WithStreamMetrics to instrument the invocation. The collectors are
	// registered on the supplied registry, so a registry instruments a single
	// RawLedgers call; instrumenting a second call on the same stream+registry
	// panics on duplicate registration. Uninstrumented calls are reusable.
	RawLedgers(ctx context.Context, ledgerRange Range, opts ...StreamOption) iter.Seq2[[]byte, error]
}

// StreamOption configures a single RawLedgers invocation.
type StreamOption func(*streamConfig)

type streamConfig struct {
	registry  *prometheus.Registry
	namespace string
}

// WithStreamMetrics instruments a RawLedgers invocation: it records
// ledger_fetch_duration_seconds — and, for captive-core, the
// captive_stellar_core_* suite — on registry under namespace, matching the
// metric names WithMetrics emits on the GetLedger path. The collectors are
// registered on the call, so a given registry instruments one RawLedgers call;
// instrumenting a second call against the same registry panics on duplicate
// registration. Create a new stream (with a fresh registry) to instrument
// another run.
func WithStreamMetrics(registry *prometheus.Registry, namespace string) StreamOption {
	return func(c *streamConfig) {
		c.registry = registry
		c.namespace = namespace
	}
}

// streamMetrics parses opts and, when a registry was supplied, registers and
// returns the fetch-duration summary. It also returns the config so a backend
// can register its own suite (captive-core). With no registry it returns a nil
// summary and the fetch timing is skipped.
func streamMetrics(opts []StreamOption) (prometheus.Summary, streamConfig) {
	var cfg streamConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.registry == nil {
		return nil, cfg
	}
	return newLedgerFetchDurationSummary(cfg.registry, cfg.namespace), cfg
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
//
// When fetchDuration is non-nil, each successful read is timed into it — the
// streaming analog of the metricsLedgerBackend.GetLedger observation, recorded
// at the single point all three backends funnel their reads through.
func streamRaw(
	ctx context.Context,
	ledgerRange Range,
	logger *log.Entry,
	name string,
	fetchDuration prometheus.Summary,
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
			// Only sample the clock when instrumented — this is the per-ledger
			// hot path and time.Now() is wasted work when metrics are off.
			var startTime time.Time
			if fetchDuration != nil {
				startTime = time.Now()
			}
			raw, err := rr.read(ctx, seq)
			if err != nil {
				yield(nil, err)
				return
			}
			if fetchDuration != nil {
				fetchDuration.Observe(time.Since(startTime).Seconds())
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
//
// Pass WithStreamMetrics to RawLedgers to record ledger_fetch_duration_seconds.
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

func (s *bufferedStorageStream) RawLedgers(ctx context.Context, ledgerRange Range, opts ...StreamOption) iter.Seq2[[]byte, error] {
	fetchDuration, _ := streamMetrics(opts)
	return streamRaw(ctx, ledgerRange, s.log, "buffered storage", fetchDuration, func() (rawReader, error) {
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
//
// Pass WithStreamMetrics to RawLedgers to record ledger_fetch_duration_seconds
// and the captive_stellar_core_* suite (registered on the core that call
// builds, matching WithMetrics on a captive backend).
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

func (s *captiveCoreStream) RawLedgers(ctx context.Context, ledgerRange Range, opts ...StreamOption) iter.Seq2[[]byte, error] {
	fetchDuration, cfg := streamMetrics(opts)
	return streamRaw(ctx, ledgerRange, s.log, "captive-core", fetchDuration, func() (rawReader, error) {
		c, err := s.newCaptive()
		if err != nil {
			return rawReader{}, err
		}
		// Register the captive suite on this core via the backend's own
		// mechanism — the same call WithMetrics makes — before PrepareRange,
		// which is where the start-duration summary and new-db counter are
		// observed. The gauges read this core live on the scrape goroutine.
		if cfg.registry != nil {
			c.registerMetrics(cfg.registry, cfg.namespace)
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
//
// Pass WithStreamMetrics to RawLedgers to record ledger_fetch_duration_seconds.
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

func (s *rpcStream) RawLedgers(ctx context.Context, ledgerRange Range, opts ...StreamOption) iter.Seq2[[]byte, error] {
	fetchDuration, _ := streamMetrics(opts)
	return streamRaw(ctx, ledgerRange, s.log, "rpc", fetchDuration, func() (rawReader, error) {
		b := s.newRPC()
		return rawReader{
			prepare: b.PrepareRange,
			read:    b.fetchSequence,
			close:   b.Close,
		}, nil
	})
}
