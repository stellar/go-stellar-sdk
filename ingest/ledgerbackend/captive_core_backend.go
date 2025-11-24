package ledgerbackend

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/stellar/go-stellar-sdk/clients/stellarcore"
	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/support/storage"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const minProtocolVersionSupported uint = 23

// Ensure CaptiveStellarCore implements LedgerBackend
var _ LedgerBackend = (*CaptiveStellarCore)(nil)

// ErrCannotStartFromGenesis is returned when attempting to prepare a range from ledger 1
var ErrCannotStartFromGenesis = errors.New("CaptiveCore is unable to start from ledger 1, start from ledger 2")

// ErrCannotCatchupAheadLatestCheckpoint is returned when attempting to prepare a bounded range where the ending ledger
// is ahead of the latest history archive snapshot.
var ErrCannotCatchupAheadLatestCheckpoint = errors.New("CaptiveCore is unable to catchup ahead of latest checkpoint")

func (c *CaptiveStellarCore) roundDownToFirstReplayAfterCheckpointStart(ledger uint32) uint32 {
	r := c.checkpointManager.GetCheckpointRange(ledger)
	if r.Low <= 1 {
		// Stellar-Core doesn't stream ledger 1
		return 2
	}
	// All other checkpoints start at the next multiple of 64
	return r.Low
}

// CaptiveStellarCore is a ledger backend that starts internal Stellar-Core
// subprocess responsible for streaming ledger data. It provides better decoupling
// than DatabaseBackend but requires some extra init time.
//
// It operates in two modes:
//   - When a BoundedRange is prepared it starts Stellar-Core in catchup mode that
//     replays ledgers in memory. This is very fast but requires Stellar-Core to
//     keep ledger state in RAM. It requires around 3GB of RAM as of August 2020.
//   - When a UnboundedRange is prepared it runs Stellar-Core catchup mode to
//     sync with the first ledger and then runs it in a normal mode. This
//     requires the configAppendPath to be provided because a quorum set needs to
//     be selected.
//
// When running CaptiveStellarCore will create a temporary folder to store
// bucket files and other temporary files. The folder is removed when Close is
// called.
//
// The communication is performed via filesystem pipe which is created in a
// temporary folder.
//
// Currently BoundedRange requires a full-trust on history archive. This issue is
// being fixed in Stellar-Core.
//
// While using BoundedRanges is straightforward there are a few gotchas connected
// to UnboundedRanges:
//   - PrepareRange takes more time because all ledger entries must be stored on
//     disk instead of RAM.
//   - If GetLedger is not called frequently (every 5 sec. on average) the
//     Stellar-Core process can go out of sync with the network. This happens
//     because there is no buffering of communication pipe and CaptiveStellarCore
//     has a very small internal buffer and Stellar-Core will not close the new
//     ledger if it's not read.
//
// Except for the Close function, CaptiveStellarCore is not thread-safe and should
// not be accessed by multiple go routines. Close is thread-safe and can be called
// from another go routine. Once Close is called it will interrupt and cancel any
// pending operations.
//
// Requires Stellar-Core v13.2.0+.
type CaptiveStellarCore struct {
	archive           historyarchive.ArchiveInterface
	checkpointManager historyarchive.CheckpointManager

	// cancel is the CancelFunc for context which controls the lifetime of a CaptiveStellarCore instance.
	// Once it is invoked CaptiveStellarCore will not be able to stream ledgers from Stellar Core or
	// spawn new instances of Stellar Core.
	cancel context.CancelFunc

	stellarCoreRunner stellarCoreRunnerInterface
	// stellarCoreLock protects access to stellarCoreRunner. When the read lock
	// is acquired stellarCoreRunner can be accessed. When the write lock is acquired
	// stellarCoreRunner can be updated.
	stellarCoreLock sync.RWMutex

	// For testing
	stellarCoreRunnerFactory func() stellarCoreRunnerInterface

	// cachedMeta keeps that ledger data of the last fetched ledger. Updated in GetLedger().
	cachedMeta *xdr.LedgerCloseMeta

	// ledgerSequenceLock mutex is used to protect the member variables used in the
	// read-only GetLatestLedgerSequence method from concurrent write operations.
	// This is required when GetLatestLedgerSequence is called from other goroutine
	// such as writing Prometheus metric captive_stellar_core_latest_ledger.
	ledgerSequenceLock sync.RWMutex

	prepared           *Range  // non-nil if any range is prepared
	closed             bool    // False until the core is closed
	nextLedger         uint32  // next ledger expected, error w/ restart if not seen
	lastLedger         *uint32 // end of current segment if offline, nil if online
	previousLedgerHash *string

	config                   CaptiveCoreConfig
	captiveCoreStartDuration prometheus.Summary
	captiveCoreNewDBCounter  prometheus.Counter
	stellarCoreClient        *stellarcore.Client
	captiveCoreVersion       string // Updates when captive-core restarts
}

// CaptiveCoreConfig contains all the parameters required to create a CaptiveStellarCore instance
type CaptiveCoreConfig struct {
	// BinaryPath is the file path to the Stellar Core binary
	BinaryPath string
	// NetworkPassphrase is the Stellar network passphrase used by captive core when connecting to the Stellar network
	NetworkPassphrase string
	// HistoryArchiveURLs are a list of history archive urls
	HistoryArchiveURLs []string
	// UserAgent is the value of `User-Agent` header that will be send along http archive requests.
	UserAgent string
	Toml      *CaptiveCoreToml

	// Optional fields

	// CheckpointFrequency is the number of ledgers between checkpoints
	// if unset, DefaultCheckpointFrequency will be used
	CheckpointFrequency uint32
	// Log is an (optional) custom logger which will capture any output from the Stellar Core process.
	// If Log is omitted then all output will be printed to stdout.
	Log *log.Entry
	// Context is the (optional) context which controls the lifetime of a CaptiveStellarCore instance. Once the context is done
	// the CaptiveStellarCore instance will not be able to stream ledgers from Stellar Core or spawn new
	// instances of Stellar Core. If Context is omitted CaptiveStellarCore will default to using context.Background.
	Context context.Context
	// StoragePath is the (optional) base path passed along to Core's
	// BUCKET_DIR_PATH which specifies where various bucket data should be
	// stored. We always append /captive-core to this directory, since we clean
	// it up entirely on shutdown.
	StoragePath string

	// CoreProtocolVersionFn is a function that returns the protocol version of the stellar-core binary.
	CoreProtocolVersionFn CoreProtocolVersionFunc

	// CoreBuildVersionFn is a function that returns the build version of the stellar-core binary.
	CoreBuildVersionFn CoreBuildVersionFunc
}

// NewCaptive returns a new CaptiveStellarCore instance.
func NewCaptive(config CaptiveCoreConfig) (*CaptiveStellarCore, error) {
	// Here we set defaults in the config. Because config is not a pointer this code should
	// not mutate the original CaptiveCoreConfig instance which was passed into NewCaptive()

	// Log Captive Core straight to stdout by default
	if config.Log == nil {
		config.Log = log.New()
		config.Log.SetOutput(os.Stdout)
		config.Log.SetLevel(logrus.InfoLevel)
	}

	if config.CoreProtocolVersionFn == nil {
		config.CoreProtocolVersionFn = CoreProtocolVersion
	}

	if config.CoreBuildVersionFn == nil {
		config.CoreBuildVersionFn = CoreBuildVersion
	}

	protocolVersion, err := config.CoreProtocolVersionFn(config.BinaryPath)
	if err != nil {
		return nil, fmt.Errorf("error determining stellar-core protocol version: %w", err)
	}

	if protocolVersion < minProtocolVersionSupported {
		return nil, fmt.Errorf("stellar-core version not supported. Installed stellar-core version is at protocol %d, but minimum "+
			"required version is %d. Please upgrade stellar-core to a version that supports protocol version %d or higher",
			protocolVersion, minProtocolVersionSupported, minProtocolVersionSupported)
	}

	parentCtx := config.Context
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	var cancel context.CancelFunc
	config.Context, cancel = context.WithCancel(parentCtx)

	archivePool, err := historyarchive.NewArchivePool(
		config.HistoryArchiveURLs,
		historyarchive.ArchiveOptions{
			Logger:              config.Log,
			NetworkPassphrase:   config.NetworkPassphrase,
			CheckpointFrequency: config.CheckpointFrequency,
			ConnectOptions: storage.ConnectOptions{
				Context:   config.Context,
				UserAgent: config.UserAgent,
			},
		},
	)

	if err != nil {
		cancel()
		return nil, errors.Wrap(err, "Error connecting to ALL history archives.")
	}

	c := &CaptiveStellarCore{
		archive:           archivePool,
		cancel:            cancel,
		config:            config,
		checkpointManager: historyarchive.NewCheckpointManager(config.CheckpointFrequency),
	}

	c.stellarCoreRunnerFactory = func() stellarCoreRunnerInterface {
		c.setCoreVersion()
		return newStellarCoreRunner(config, c.captiveCoreNewDBCounter)
	}

	if config.Toml != nil && config.Toml.HTTPPort != 0 {
		c.stellarCoreClient = &stellarcore.Client{
			HTTP: &http.Client{
				Timeout: 2 * time.Second,
			},
			URL: fmt.Sprintf("http://localhost:%d", config.Toml.HTTPPort),
		}
	}
	c.setCoreVersion()
	return c, nil
}

func (c *CaptiveStellarCore) coreSyncedMetric() float64 {
	if c.stellarCoreClient == nil {
		return -2
	}

	info, err := c.stellarCoreClient.Info(c.config.Context)
	if err != nil {
		return -1
	}

	if info.IsSynced() {
		return 1
	} else {
		return 0
	}
}

func (c *CaptiveStellarCore) coreVersionMetric() float64 {
	if c.stellarCoreClient == nil {
		return -2
	}

	info, err := c.stellarCoreClient.Info(c.config.Context)
	if err != nil {
		return -1
	}

	return float64(info.Info.ProtocolVersion)
}

func (c *CaptiveStellarCore) setCoreVersion() {
	var err error
	c.captiveCoreVersion, err = c.config.CoreBuildVersionFn(c.config.BinaryPath)
	if err != nil {
		c.config.Log.Errorf("Failed to set stellar-core version: %s", err)
	}
	c.config.Log.Infof("stellar-core version: %s", c.captiveCoreVersion)
}

func (c *CaptiveStellarCore) GetCoreVersion() string {
	return c.captiveCoreVersion
}

func (c *CaptiveStellarCore) registerMetrics(registry *prometheus.Registry, namespace string) {
	coreSynced := prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: "ingest", Name: "captive_stellar_core_synced",
			Help: "1 if sync, 0 if not synced, -1 if unable to connect or HTTP server disabled.",
		},
		c.coreSyncedMetric,
	)
	supportedProtocolVersion := prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: "ingest", Name: "captive_stellar_core_supported_protocol_version",
			Help: "determines the supported version of the protocol by Captive-Core",
		},
		c.coreVersionMetric,
	)
	latestLedger := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: "ingest", Name: "captive_stellar_core_latest_ledger",
		Help: "sequence number of the latest ledger available in the ledger backend",
	},
		func() float64 {
			latest, err := c.GetLatestLedgerSequence(context.Background())
			if err != nil {
				return 0
			}
			return float64(latest)
		},
	)
	c.captiveCoreStartDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:  namespace,
		Subsystem:  "ingest",
		Name:       "captive_stellar_core_start_duration_seconds",
		Help:       "duration of start up time when running captive core on an unbounded range, sliding window = 10m",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})
	c.captiveCoreNewDBCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "ingest",
		Name:      "captive_stellar_core_new_db",
		Help:      "counter for the number of times we start up captive core with a new buckets db, sliding window = 10m",
	})

	registry.MustRegister(
		coreSynced,
		supportedProtocolVersion,
		latestLedger,
		c.captiveCoreStartDuration,
		c.captiveCoreNewDBCounter,
	)
}

func (c *CaptiveStellarCore) getLatestCheckpointSequence() (uint32, error) {
	has, err := c.archive.GetRootHAS()
	if err != nil {
		return 0, errors.Wrap(err, "error getting root HAS")
	}

	return has.CurrentLedger, nil
}

func (c *CaptiveStellarCore) openOfflineReplaySubprocess(from, to uint32) error {
	latestCheckpointSequence, err := c.getLatestCheckpointSequence()
	if err != nil {
		return errors.Wrap(err, "error getting latest checkpoint sequence")
	}

	if to > latestCheckpointSequence {
		return errors.Wrapf(ErrCannotCatchupAheadLatestCheckpoint,
			"to sequence: %d is greater than max available in history archives: %d",
			to,
			latestCheckpointSequence,
		)
	}

	stellarCoreRunner := c.stellarCoreRunnerFactory()
	if err = stellarCoreRunner.catchup(from, to); err != nil {
		return errors.Wrap(err, "error running stellar-core")
	}
	c.stellarCoreRunner = stellarCoreRunner

	// The next ledger should be the first ledger of the checkpoint containing
	// the requested ledger
	ran := BoundedRange(from, to)
	c.ledgerSequenceLock.Lock()
	defer c.ledgerSequenceLock.Unlock()

	c.prepared = &ran
	c.nextLedger = c.roundDownToFirstReplayAfterCheckpointStart(from)
	c.lastLedger = &to
	c.previousLedgerHash = nil

	return nil
}

func (c *CaptiveStellarCore) openOnlineReplaySubprocess(ctx context.Context, from uint32) error {
	runFrom, err := c.runFromParams(ctx, from)
	if err != nil {
		return errors.Wrap(err, "error calculating ledger and hash for stellar-core run")
	}

	stellarCoreRunner := c.stellarCoreRunnerFactory()
	if err = stellarCoreRunner.runFrom(runFrom); err != nil {
		return errors.Wrap(err, "error running stellar-core")
	}
	c.stellarCoreRunner = stellarCoreRunner

	// In the online mode we update nextLedger after streaming the first ledger.
	// This is to support versions before and after/including v17.1.0 that
	// introduced minimal persistent DB.
	c.ledgerSequenceLock.Lock()
	defer c.ledgerSequenceLock.Unlock()

	c.nextLedger = 0
	ran := UnboundedRange(from)
	c.prepared = &ran
	c.lastLedger = nil
	c.previousLedgerHash = nil

	return nil
}

// runFromParams receives a ledger sequence and calculates the required values to call stellar-core run with --start-ledger and --start-hash
func (c *CaptiveStellarCore) runFromParams(ctx context.Context, from uint32) (uint32, error) {
	if from == 1 {
		// Trying to start-from 1 results in an error from Stellar-Core:
		// Target ledger 1 is not newer than last closed ledger 1 - nothing to do
		// TODO maybe we can fix it by generating 1st ledger meta
		// like GenesisLedgerStateReader?
		return 0, ErrCannotStartFromGenesis
	}

	if from <= c.checkpointManager.GetCheckpoint(0) {
		// The line below is to support a special case for streaming ledger 2
		// that works for all other ledgers <= 63 (fast-forward).
		// We can't set from=2 because Stellar-Core will not allow starting from 1.
		// To solve this we start from 3 and exploit the fast that Stellar-Core
		// will stream data from 2 for the first checkpoint.
		from = 3
	}

	latestCheckpointSequence, err := c.getLatestCheckpointSequence()
	if err != nil {
		return 0, errors.Wrap(err, "error getting latest checkpoint sequence")
	}

	// We don't allow starting the online mode starting with more than two
	// checkpoints from now. Such requests are likely buggy.
	// We should allow only one checkpoint here but sometimes there are up to a
	// minute delays when updating root HAS by stellar-core.
	twoCheckPointsLength := (c.checkpointManager.GetCheckpoint(0) + 1) * 2
	maxLedger := latestCheckpointSequence + twoCheckPointsLength
	if from > maxLedger {
		return 0, errors.Errorf(
			"trying to start online mode too far (latest checkpoint=%d), only two checkpoints in the future allowed",
			latestCheckpointSequence,
		)
	}

	runFrom := from - 1
	return runFrom, nil
}

// nextExpectedSequence returns nextLedger (if currently set) or start of
// prepared range. Otherwise it returns 0.
// This is done because `nextLedger` is 0 between the moment Stellar-Core is
// started and streaming the first ledger (in such case we return first ledger
// in requested range).
func (c *CaptiveStellarCore) nextExpectedSequence() uint32 {
	if c.nextLedger == 0 && c.prepared != nil {
		return c.prepared.from
	}
	return c.nextLedger
}

func (c *CaptiveStellarCore) startPreparingRange(ctx context.Context, ledgerRange Range) (bool, error) {
	c.stellarCoreLock.Lock()
	defer c.stellarCoreLock.Unlock()

	if c.isPrepared(ledgerRange) {
		return true, nil
	}

	if c.stellarCoreRunner != nil {
		if err := c.stellarCoreRunner.close(); err != nil {
			return false, errors.Wrap(err, "error closing existing session")
		}
	}

	var err error
	if ledgerRange.bounded {
		err = c.openOfflineReplaySubprocess(ledgerRange.from, ledgerRange.to)
	} else {
		err = c.openOnlineReplaySubprocess(ctx, ledgerRange.from)
	}
	if err != nil {
		return false, errors.Wrap(err, "opening subprocess")
	}

	return false, nil
}

// PrepareRange prepares the given range (including from and to) to be loaded.
// Captive stellar-core backend needs to initialize Stellar-Core state to be
// able to stream ledgers.
// Stellar-Core mode depends on the provided ledgerRange:
//   - For BoundedRange it will start Stellar-Core in catchup mode.
//   - For UnboundedRange it will first catchup to starting ledger and then run
//     it normally (including connecting to the Stellar network).
//
// Please note that using a BoundedRange, currently, requires a full-trust on
// history archive. This issue is being fixed in Stellar-Core.
func (c *CaptiveStellarCore) PrepareRange(ctx context.Context, ledgerRange Range) error {
	startTime := time.Now()
	if alreadyPrepared, err := c.startPreparingRange(ctx, ledgerRange); err != nil {
		return errors.Wrap(err, "error starting prepare range")
	} else if alreadyPrepared {
		return nil
	}

	var reportedStartTime bool
	// the prepared range might be below ledgerRange.from so we
	// need to seek ahead until we reach ledgerRange.from
	for seq := c.prepared.from; seq <= ledgerRange.from; seq++ {
		_, err := c.GetLedger(ctx, seq)
		if err != nil {
			return errors.Wrapf(err, "Error fast-forwarding to %d", ledgerRange.from)
		}
		if !reportedStartTime {
			reportedStartTime = true
			if c.captiveCoreStartDuration != nil && !ledgerRange.bounded {
				c.captiveCoreStartDuration.Observe(time.Since(startTime).Seconds())
			}
		}
	}

	return nil
}

// IsPrepared returns true if a given ledgerRange is prepared.
func (c *CaptiveStellarCore) IsPrepared(ctx context.Context, ledgerRange Range) (bool, error) {
	c.stellarCoreLock.RLock()
	defer c.stellarCoreLock.RUnlock()

	return c.isPrepared(ledgerRange), nil
}

func (c *CaptiveStellarCore) isPrepared(ledgerRange Range) bool {
	if c.closed {
		return false
	}

	if c.stellarCoreRunner == nil || c.stellarCoreRunner.context().Err() != nil {
		return false
	}

	if _, exited := c.stellarCoreRunner.getProcessExitError(); exited {
		return false
	}

	lastLedger := uint32(0)
	if c.lastLedger != nil {
		lastLedger = *c.lastLedger
	}

	cachedLedger := uint32(0)
	if c.cachedMeta != nil {
		cachedLedger = c.cachedMeta.LedgerSequence()
	}

	if c.prepared == nil {
		return false
	}

	if lastLedger == 0 {
		return c.nextExpectedSequence() <= ledgerRange.from || cachedLedger == ledgerRange.from
	}

	// From now on: lastLedger != 0 so current range is bounded

	if ledgerRange.bounded {
		return (c.nextExpectedSequence() <= ledgerRange.from || cachedLedger == ledgerRange.from) &&
			lastLedger >= ledgerRange.to
	}

	// Requested range is unbounded but current one is bounded
	return false
}

// GetLedger will block until the ledger is available in the backend
// (even for UnboundedRange), then return it's LedgerCloseMeta.
//
// Call PrepareRange first to instruct the backend which ledgers to fetch.
// CaptiveStellarCore requires PrepareRange call first to initialize Stellar-Core.
// Requesting a ledger on non-prepared backend will return an error.
//
// Please note that requesting a ledger sequence far after current
// ledger will block the execution for a long time.
//
// Because ledger data is streamed from Stellar-Core sequentially, users should
// request sequences in a non-decreasing order. If the requested sequence number
// is less than the last requested sequence number, an error will be returned.
//
// This function behaves differently for bounded and unbounded ranges:
//   - BoundedRange: After getting the last ledger in a range this method will
//     also Close() the backend.
func (c *CaptiveStellarCore) GetLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error) {
	c.stellarCoreLock.RLock()
	defer c.stellarCoreLock.RUnlock()

	if c.cachedMeta != nil && sequence == c.cachedMeta.LedgerSequence() {
		// GetLedger can be called multiple times using the same sequence, ex. to create
		// change and transaction readers. If we have this ledger buffered, let's return it.
		return *c.cachedMeta, nil
	}

	if c.closed {
		return xdr.LedgerCloseMeta{}, errors.New("stellar-core is no longer usable")
	}

	if c.prepared == nil {
		return xdr.LedgerCloseMeta{}, errors.New("session is not prepared, call PrepareRange first")
	}

	if c.stellarCoreRunner == nil {
		return xdr.LedgerCloseMeta{}, errors.New("stellar-core cannot be nil, call PrepareRange first")
	}

	if sequence < c.nextExpectedSequence() {
		return xdr.LedgerCloseMeta{}, errors.Errorf(
			"requested ledger %d is behind the captive core stream (expected=%d)",
			sequence,
			c.nextExpectedSequence(),
		)
	}

	if c.lastLedger != nil && sequence > *c.lastLedger {
		return xdr.LedgerCloseMeta{}, errors.Errorf(
			"reading past bounded range (requested sequence=%d, last ledger in range=%d)",
			sequence,
			*c.lastLedger,
		)
	}

	ch, ok := c.stellarCoreRunner.getMetaPipe()
	if !ok {
		return xdr.LedgerCloseMeta{}, errors.New("stellar-core is not running, call PrepareRange first")
	}

	// Now loop along the range until we find the ledger we want.
	for {
		select {
		case <-ctx.Done():
			return xdr.LedgerCloseMeta{}, ctx.Err()
		case result, ok := <-ch:
			found, ledger, err := c.handleMetaPipeResult(sequence, result, ok)
			if found || err != nil {
				return ledger, err
			}
		}
	}
}

func (c *CaptiveStellarCore) handleMetaPipeResult(sequence uint32, result metaResult, ok bool) (bool, xdr.LedgerCloseMeta, error) {
	if err := c.checkMetaPipeResult(result, ok); err != nil {
		c.stellarCoreRunner.close()
		return false, xdr.LedgerCloseMeta{}, err
	}

	seq := result.LedgerCloseMeta.LedgerSequence()
	// If we got something unexpected; close and reset
	if c.nextLedger != 0 && seq != c.nextLedger {
		c.stellarCoreRunner.close()
		return false, xdr.LedgerCloseMeta{}, errors.Errorf(
			"unexpected ledger sequence (expected=%d actual=%d)",
			c.nextLedger,
			seq,
		)
	} else if c.nextLedger == 0 && seq > c.prepared.from {
		// First stream ledger is greater than prepared.from
		c.stellarCoreRunner.close()
		return false, xdr.LedgerCloseMeta{}, errors.Errorf(
			"unexpected ledger sequence (expected=<=%d actual=%d)",
			c.prepared.from,
			seq,
		)
	}

	newPreviousLedgerHash := result.LedgerCloseMeta.PreviousLedgerHash().HexString()
	if c.previousLedgerHash != nil && *c.previousLedgerHash != newPreviousLedgerHash {
		// We got something unexpected; close and reset
		c.stellarCoreRunner.close()
		return false, xdr.LedgerCloseMeta{}, errors.Errorf(
			"unexpected previous ledger hash for ledger %d (expected=%s actual=%s)",
			seq,
			*c.previousLedgerHash,
			newPreviousLedgerHash,
		)
	}

	c.ledgerSequenceLock.Lock()
	c.nextLedger = result.LedgerSequence() + 1
	c.ledgerSequenceLock.Unlock()

	currentLedgerHash := result.LedgerCloseMeta.LedgerHash().HexString()
	c.previousLedgerHash = &currentLedgerHash

	// Update cache with the latest value because we incremented nextLedger.
	c.cachedMeta = result.LedgerCloseMeta

	if seq == sequence {
		// If we got the _last_ ledger in a segment, close before returning.
		if c.lastLedger != nil && *c.lastLedger == seq {
			if err := c.stellarCoreRunner.close(); err != nil {
				return false, xdr.LedgerCloseMeta{}, errors.Wrap(err, "error closing session")
			}
		}
		return true, *c.cachedMeta, nil
	}

	return false, xdr.LedgerCloseMeta{}, nil
}

func (c *CaptiveStellarCore) checkMetaPipeResult(result metaResult, ok bool) error {
	// There are 4 error scenarios we check for:
	// 1. User initiated shutdown by canceling the parent context or calling Close().
	// 2. The stellar core process exited unexpectedly with an error message.
	// 3. Some error was encountered while consuming the ledgers emitted by captive core (e.g. parsing invalid xdr)
	// 4. The stellar core process exited unexpectedly without an error message
	if err := c.stellarCoreRunner.context().Err(); err != nil {
		// Case 1 - User initiated shutdown by canceling the parent context or calling Close()
		return err
	}
	if !ok || result.err != nil {
		err, exited := c.stellarCoreRunner.getProcessExitError()
		if exited && err != nil {
			// Case 2 - The stellar core process exited unexpectedly with an error message
			return errors.Wrap(err, "stellar core exited unexpectedly")
		} else if result.err != nil {
			// Case 3 - Some error was encountered while consuming the ledger stream emitted by captive core.
			return result.err
		} else if exited {
			// case 4 - The stellar core process exited unexpectedly without an error message
			return errors.Errorf("stellar core exited unexpectedly")
		} else if !ok {
			// This case should never happen because the ledger buffer channel can only be closed
			// if and only if the process exits or the context is canceled.
			// However, we add this check for the sake of completeness
			return errors.Errorf("meta pipe closed unexpectedly")
		}
	}
	return nil
}

// GetLatestLedgerSequence returns the sequence of the latest ledger available
// in the backend. This method returns an error if not in a session (start with
// PrepareRange).
//
// Note that for UnboundedRange the returned sequence number is not necessarily
// the latest sequence closed by the network. It's always the last value available
// in the backend.
func (c *CaptiveStellarCore) GetLatestLedgerSequence(ctx context.Context) (uint32, error) {
	c.stellarCoreLock.RLock()
	defer c.stellarCoreLock.RUnlock()

	c.ledgerSequenceLock.RLock()
	defer c.ledgerSequenceLock.RUnlock()

	if c.closed {
		return 0, errors.New("stellar-core is no longer usable")
	}
	if c.prepared == nil {
		return 0, errors.New("stellar-core must be prepared, call PrepareRange first")
	}
	if c.stellarCoreRunner == nil {
		return 0, errors.New("stellar-core cannot be nil, call PrepareRange first")
	}
	ch, ok := c.stellarCoreRunner.getMetaPipe()
	if !ok {
		return 0, errors.New("stellar-core is not running, call PrepareRange first")
	}
	if c.lastLedger == nil {
		return c.nextExpectedSequence() - 1 + uint32(len(ch)), nil
	}
	return *c.lastLedger, nil
}

// Close closes existing Stellar-Core process, streaming sessions and removes all
// temporary files. Note, once a CaptiveStellarCore instance is closed it can no longer be used and
// all subsequent calls to PrepareRange(), GetLedger(), etc will fail.
// Close is thread-safe and can be called from another go routine.
func (c *CaptiveStellarCore) Close() error {
	// after the CaptiveStellarCore context is canceled all subsequent calls to PrepareRange() will fail
	c.cancel()

	c.stellarCoreLock.RLock()
	defer c.stellarCoreLock.RUnlock()

	c.closed = true

	if c.stellarCoreRunner != nil {
		return c.stellarCoreRunner.close()
	}
	return nil
}
