package loadtest

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ErrLoadTestDone indicates that the load test has run to completion.
var ErrLoadTestDone = fmt.Errorf("the load test is done")

// LedgerBackend is used to load test ingestion.
// LedgerBackend will take one or more files of synthetically generated ledgers (see
// services/horizon/internal/integration/generate_ledgers_test.go) and replay
// them to the downstream ingesting system at a configurable rate.
// It is also possible to merge the synthetically generated ledgers with real
// ledgers from the network. To enable the merging behavior, configure the
// LedgerBackend field in LedgerBackendConfig.
type LedgerBackend struct {
	config                LedgerBackendConfig
	mergedLedgersFilePath string
	mergedLedgersStream   *xdr.Stream
	startTime             time.Time
	startLedgerSeq        uint32
	nextLedgerSeq         uint32
	latestLedgerSeq       uint32
	preparedRange         ledgerbackend.Range
	cachedLedger          xdr.LedgerCloseMeta
	done                  bool
	lock                  sync.RWMutex
	isCaptiveCore         bool
}

// LedgerBackendConfig configures LedgerBackend
type LedgerBackendConfig struct {
	// NetworkPassphrase is the passphrase of the Stellar network from where the real ledgers
	// will be obtained
	NetworkPassphrase string
	// LedgerBackend is an optional parameter. When LedgerBackend is configured, ledgers from
	// LedgerBackend will be merged with the synthetic ledgers from LedgersFilePaths.
	LedgerBackend ledgerbackend.LedgerBackend
	// LedgersFilePaths are the files containing the synthetic ledgers that will be replayed,
	// in order, to the downstream ingesting system.
	LedgersFilePaths []string
	// LedgerCloseDuration is the rate at which ledgers will be replayed from LedgerBackend
	LedgerCloseDuration time.Duration
	// MaxLedgersPerFile optionally caps how many ledgers are replayed from each file in
	// LedgersFilePaths. When 0, or greater than the ledgers a file holds, all of its
	// ledgers are replayed.
	MaxLedgersPerFile uint32
}

// NewLedgerBackend constructs an LedgerBackend instance
func NewLedgerBackend(config LedgerBackendConfig) *LedgerBackend {
	var isCaptiveCore bool
	if config.LedgerBackend != nil {
		_, isCaptiveCore = config.LedgerBackend.(*ledgerbackend.CaptiveStellarCore)
	}
	return &LedgerBackend{
		config:        config,
		isCaptiveCore: isCaptiveCore,
	}
}

func (r *LedgerBackend) GetLatestLedgerSequence(ctx context.Context) (uint32, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if r.nextLedgerSeq == 0 {
		return 0, fmt.Errorf("PrepareRange() must be called before GetLatestLedgerSequence()")
	}

	return r.latestLedgerSeq, nil
}

// ledgerReader streams ledgers sequentially across one or more zstd-compressed
// ledger files, transparently advancing to the next file at each EOF. If
// perFileLimit is non-zero, at most that many ledgers are read from each file.
type ledgerReader struct {
	paths        []string
	perFileLimit uint32
	idx          int
	readFromFile uint32
	stream       *xdr.Stream
}

func newLedgerReader(paths []string, perFileLimit uint32) *ledgerReader {
	return &ledgerReader{paths: paths, perFileLimit: perFileLimit, idx: -1}
}

// ReadOne reads the next ledger, returning io.EOF once all files are exhausted.
func (lr *ledgerReader) ReadOne(ledger *xdr.LedgerCloseMeta) error {
	for {
		if lr.stream == nil {
			lr.idx++
			if lr.idx >= len(lr.paths) {
				return io.EOF
			}
			file, err := os.Open(lr.paths[lr.idx])
			if err != nil {
				return fmt.Errorf("could not open ledgers file: %w", err)
			}
			if lr.stream, err = xdr.NewZstdStream(file); err != nil {
				file.Close()
				return fmt.Errorf("could not open zstd stream for ledgers file: %w", err)
			}
			lr.readFromFile = 0
		}
		if lr.perFileLimit == 0 || lr.readFromFile < lr.perFileLimit {
			switch err := lr.stream.ReadOne(ledger); err {
			case nil:
				lr.readFromFile++
				return nil
			case io.EOF: // fall through to advance to the next file
			default:
				return err
			}
		}
		lr.stream.Close() // ReadOne closes the decoder at EOF, but not the underlying file
		lr.stream = nil
	}
}

// Close closes the file currently being read, if any.
func (lr *ledgerReader) Close() error {
	if lr.stream == nil {
		return nil
	}
	err := lr.stream.Close()
	lr.stream = nil
	return err
}

func countReader(reader *ledgerReader) (uint32, error) {
	defer reader.Close()
	var n uint32
	var ledger xdr.LedgerCloseMeta
	for {
		if err := reader.ReadOne(&ledger); err == io.EOF {
			return n, nil
		} else if err != nil {
			return 0, fmt.Errorf("could not get generated ledger: %w", err)
		}
		n++
	}
}

// FileLedgerCount reports how many ledgers a single bundle contributes to a replay.
type FileLedgerCount struct {
	Path    string
	Ledgers uint32
}

// CountLedgersPerFile returns, for each path, the number of ledgers that would be
// replayed under perFileLimit — the same per-file cap PrepareRange applies. Sum the
// counts for the total replay length, or use them as per-bundle segment bounds.
func CountLedgersPerFile(paths []string, perFileLimit uint32) ([]FileLedgerCount, error) {
	counts := make([]FileLedgerCount, len(paths))
	for i, path := range paths {
		n, err := countReader(newLedgerReader([]string{path}, perFileLimit))
		if err != nil {
			return nil, err
		}
		counts[i] = FileLedgerCount{Path: path, Ledgers: n}
	}
	return counts, nil
}

func countLedgers(paths []string, perFileLimit uint32) (int, error) {
	counts, err := CountLedgersPerFile(paths, perFileLimit)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, c := range counts {
		total += int(c.Ledgers)
	}
	return total, nil
}

func (r *LedgerBackend) PrepareRange(ctx context.Context, ledgerRange ledgerbackend.Range) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.done {
		return ErrLoadTestDone
	}
	if r.nextLedgerSeq != 0 {
		if r.isPrepared(ledgerRange) {
			return nil
		}
		return fmt.Errorf("PrepareRange() already called")
	}

	ledgerCount, err := countLedgers(r.config.LedgersFilePaths, r.config.MaxLedgersPerFile)
	if err != nil {
		return fmt.Errorf("could not count ledgers in files: %w", err)
	}
	if ledgerCount == 0 {
		return fmt.Errorf("no ledgers found in files %v", r.config.LedgersFilePaths)
	}
	if ledgerRange.From() > math.MaxUint32-uint32(ledgerCount-1) {
		return fmt.Errorf("ledger range would overflow: from=%d, count=%d", ledgerRange.From(), ledgerCount)
	}
	latestLedgerSeq := ledgerRange.From() + uint32(ledgerCount-1)
	// a bounded request for fewer ledgers than we have caps what we actually serve
	if ledgerRange.Bounded() && ledgerRange.To() < latestLedgerSeq {
		latestLedgerSeq = ledgerRange.To()
	}

	generatedLedgers := newLedgerReader(r.config.LedgersFilePaths, r.config.MaxLedgersPerFile)
	defer generatedLedgers.Close()

	mergedLedgersFile, err := os.CreateTemp("", "merged-ledgers")
	if err != nil {
		return fmt.Errorf("could not create merged ledgers file: %w", err)
	}
	log.WithField("path", mergedLedgersFile.Name()).
		Info("creating temporary merged ledgers file")

	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(mergedLedgersFile.Name())
		}
	}()
	writer, err := zstd.NewWriter(mergedLedgersFile)
	if err != nil {
		return fmt.Errorf("could not create zstd writer for merged ledgers file: %w", err)
	}

	var firstLedger xdr.LedgerCloseMeta
	var validatedNetworkLedgers bool
	// prevHash chains each served ledger to the one before it: merging rewrites
	// the header hash.
	var prevHash *xdr.Hash
	lastValidatedFile := -1
	for cur := ledgerRange.From(); cur <= latestLedgerSeq && (!ledgerRange.Bounded() || cur <= ledgerRange.To()); cur++ {
		var generatedLedger xdr.LedgerCloseMeta
		if err = generatedLedgers.ReadOne(&generatedLedger); err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("could not get generated ledger: %w", err)
		}
		if lastValidatedFile != generatedLedgers.idx && generatedLedger.CountTransactions() > 0 {
			// Validate that the generated ledgers carry the expected network passphrase. We do
			// this once per file (ledgers within a file share a passphrase), which also rejects
			// accidentally combining bundles generated for different networks.
			if err = validateNetworkPassphrase(r.config.NetworkPassphrase, generatedLedger); err != nil {
				return err
			}
			lastValidatedFile = generatedLedgers.idx
		}

		ledgerDiff := int64(cur) - int64(generatedLedger.LedgerSequence())
		setLedgerSeq := func(cur uint32) uint32 {
			newLedgerSeq := int64(cur) + ledgerDiff
			if newLedgerSeq > math.MaxUint32 {
				panic(fmt.Sprintf(
					"value %v overflows when applying ledger diff %v",
					cur, ledgerDiff,
				))
			}
			minLedger := ledgerRange.From()
			if newLedgerSeq <= int64(minLedger) {
				// All ledger entry fixtures are attached to the very first ledger in the range.
				// Any new or updated ledger entry will occur in a later ledger sequence.
				// So, the smallest possible ledger sequence associated with any ledger entry we merge is
				// ledgerRange.From()
				return minLedger
			}
			return uint32(newLedgerSeq)
		}

		var raw []byte
		if r.config.LedgerBackend != nil {
			if cur == ledgerRange.From() {
				err = r.optimizedPrepareRange(ctx, ledgerRange, ledgerCount)
				if err != nil {
					return fmt.Errorf("could not prepare range using real ledger backend: %w", err)
				}
			}
			var ledger xdr.LedgerCloseMeta
			ledger, err = r.config.LedgerBackend.GetLedger(ctx, cur)
			if err != nil {
				return fmt.Errorf("could not get ledger %v from real ledger backend: %w", cur, err)
			}
			if !validatedNetworkLedgers && ledger.CountTransactions() > 0 {
				if err = validateNetworkPassphrase(r.config.NetworkPassphrase, ledger); err != nil {
					return err
				}
				validatedNetworkLedgers = true
			}
			var hash xdr.Hash
			raw, hash, err = mergeGenerated(ledger, generatedLedger, setLedgerSeq, prevHash)
			if err != nil {
				return fmt.Errorf("could not merge ledgers: %w", err)
			}
			prevHash = &hash
		} else {
			var generatedRaw []byte
			generatedRaw, err = generatedLedger.MarshalBinary()
			if err != nil {
				return fmt.Errorf("could not marshal generated ledger: %w", err)
			}
			var hash xdr.Hash
			raw, hash, err = MergeLedgerBytes([][]byte{generatedRaw}, MergeOptions{
				LedgerSeq:          cur,
				PreviousLedgerHash: prevHash,
				RemapLedgerSeq:     func(_ int, seq uint32) uint32 { return setLedgerSeq(seq) },
			})
			if err != nil {
				return fmt.Errorf("could not update ledger seq: %w", err)
			}
			prevHash = &hash
		}

		if cur == ledgerRange.From() {
			if err = firstLedger.UnmarshalBinary(raw); err != nil {
				return fmt.Errorf("could not decode first ledger: %w", err)
			}
		} else if err = writeFramed(writer, raw); err != nil {
			return fmt.Errorf("could not write ledger to stream: %w", err)
		}
	}
	if err = generatedLedgers.Close(); err != nil {
		return fmt.Errorf("could not close generated ledgers xdr stream: %w", err)
	}
	if err = writer.Close(); err != nil {
		return fmt.Errorf("could not close zstd writer: %w", err)
	}
	if err = mergedLedgersFile.Sync(); err != nil {
		return fmt.Errorf("could not sync merged ledgers file: %w", err)
	}

	if _, err = mergedLedgersFile.Seek(0, 0); err != nil {
		return fmt.Errorf("could not seek to beginning of merged ledgers file: %w", err)
	}
	mergedLedgersStream, err := xdr.NewZstdStream(mergedLedgersFile)
	if err != nil {
		return fmt.Errorf("could not open zstd read stream for merged ledgers file: %w", err)
	}
	cleanup = false

	r.mergedLedgersFilePath = mergedLedgersFile.Name()
	r.mergedLedgersStream = mergedLedgersStream
	// from this point, ledgers will be available at a rate of once
	// every r.ledgerCloseDuration time has elapsed
	r.startTime = time.Now()
	r.startLedgerSeq = ledgerRange.From()
	r.nextLedgerSeq = r.startLedgerSeq + 1
	r.latestLedgerSeq = latestLedgerSeq
	r.cachedLedger = firstLedger
	r.preparedRange = ledgerRange
	log.WithField("start", r.startLedgerSeq).
		WithField("end", latestLedgerSeq).
		Info("ingesting ledgers from loadtest ledger backend")
	return nil
}

func (r *LedgerBackend) optimizedPrepareRange(ctx context.Context, ledgerRange ledgerbackend.Range, ledgerCount int) error {
	// we have ledgerCount synthetic ledgers so there is no use in preparing a larger ledger range
	maxBoundedRange := ledgerbackend.BoundedRange(ledgerRange.From(), ledgerRange.From()+uint32(ledgerCount-1))
	if ledgerRange.Bounded() && ledgerRange.Contains(maxBoundedRange) {
		// The requested ledger range contains more ledgers than what we have.
		// In that case, clamp down the ledger range to only contain the total amount
		// of synthetic ledgers we have.
		return r.config.LedgerBackend.PrepareRange(ctx, maxBoundedRange)
	} else if !ledgerRange.Bounded() && r.isCaptiveCore {
		// it is faster to run stellar-core catchup than stellar-core run
		// because the run command has to sync to the latest ledger in consensus
		err := r.config.LedgerBackend.PrepareRange(ctx, maxBoundedRange)
		if err == nil {
			return nil
		}
		// if maxBoundedRange overlaps with the latest ledgers in the network which
		// are ahead of the most recent checkpoint ledger we must use the
		// stellar-core run command
		if !errors.Is(err, ledgerbackend.ErrCannotCatchupAheadLatestCheckpoint) {
			return err
		}
	}
	return r.config.LedgerBackend.PrepareRange(ctx, ledgerRange)
}

func validateNetworkPassphrase(networkPassphrase string, ledger xdr.LedgerCloseMeta) error {
	// If the network passphrase which is passed into ingest.NewLedgerChangeReaderFromLedgerCloseMeta()
	// is invalid, the reader will encounter an error at some point while streaming changes.
	reader, err := ingest.NewLedgerChangeReaderFromLedgerCloseMeta(networkPassphrase, ledger)
	if err != nil {
		return err
	}
	for {
		if _, err = reader.Read(); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (r *LedgerBackend) IsPrepared(ctx context.Context, ledgerRange ledgerbackend.Range) (bool, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.isPrepared(ledgerRange), nil
}

func (r *LedgerBackend) isPrepared(ledgerRange ledgerbackend.Range) bool {
	if r.nextLedgerSeq == 0 {
		return false
	}

	if r.preparedRange.Bounded() != ledgerRange.Bounded() {
		return false
	}

	if ledgerRange.From() < r.cachedLedger.LedgerSequence() {
		return false
	}

	return ledgerRange.From() >= r.cachedLedger.LedgerSequence() && ledgerRange.To() <= r.preparedRange.To()
}

func (r *LedgerBackend) GetLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error) {
	r.lock.RLock()
	closeLedgerBackend := false
	defer func() {
		r.lock.RUnlock()
		if closeLedgerBackend {
			r.Close()
		}
	}()

	if r.nextLedgerSeq == 0 {
		return xdr.LedgerCloseMeta{}, fmt.Errorf("PrepareRange() must be called before GetLedger()")
	}
	if sequence < r.cachedLedger.LedgerSequence() {
		return xdr.LedgerCloseMeta{}, fmt.Errorf(
			"sequence number %v is behind the ledger stream sequence %d",
			sequence,
			r.cachedLedger.LedgerSequence(),
		)
	}
	if r.done {
		return xdr.LedgerCloseMeta{}, ErrLoadTestDone
	}
	if sequence > r.latestLedgerSeq {
		closeLedgerBackend = true
		return xdr.LedgerCloseMeta{}, ErrLoadTestDone
	}
	for ; r.nextLedgerSeq <= sequence && ctx.Err() == nil; r.nextLedgerSeq++ {
		var ledger xdr.LedgerCloseMeta
		if err := r.mergedLedgersStream.ReadOne(&ledger); err == io.EOF {
			return ledger, fmt.Errorf(
				"sequence number %v is greater than the latest ledger available",
				sequence,
			)
		} else if err != nil {
			return ledger, fmt.Errorf("could read ledger from merged ledgers stream: %w", err)
		}
		if ledger.LedgerSequence() != r.nextLedgerSeq {
			return ledger, fmt.Errorf(
				"unexpected ledger sequence (expected=%d actual=%d)",
				r.nextLedgerSeq,
				ledger.LedgerSequence(),
			)
		}
		r.cachedLedger = ledger
	}
	i := int(sequence - r.startLedgerSeq)
	// the i'th ledger will only be available after (i+1) * r.ledgerCloseDuration time has elapsed
	closeTime := r.startTime.Add(time.Duration(i+1) * r.config.LedgerCloseDuration)

	// Sleep until closeTime or context is cancelled
	if sleepDuration := time.Until(closeTime); sleepDuration > 0 {
		select {
		case <-time.After(sleepDuration):
		case <-ctx.Done():
			return xdr.LedgerCloseMeta{}, ctx.Err()
		}
	}

	return r.cachedLedger, nil
}

func (r *LedgerBackend) Close() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.done = true
	if r.config.LedgerBackend != nil {
		if err := r.config.LedgerBackend.Close(); err != nil {
			return fmt.Errorf("could not close real ledger backend: %w", err)
		}
	}
	if r.mergedLedgersStream != nil {
		// closing the stream will also close the ledgers file
		if err := r.mergedLedgersStream.Close(); err != nil {
			return fmt.Errorf("could not close merged ledgers xdr stream: %w", err)
		}
		r.mergedLedgersStream = nil
	}
	if r.mergedLedgersFilePath != "" {
		if err := os.Remove(r.mergedLedgersFilePath); err != nil {
			return fmt.Errorf("could not remove merged ledgers file: %w", err)
		}
		r.mergedLedgersFilePath = ""
	}
	return nil
}

func validLedger(ledger xdr.LedgerCloseMeta) error {
	switch ledger.V {
	case 2:
		if _, ok := ledger.MustV2().TxSet.GetV1TxSet(); !ok {
			return fmt.Errorf("ledger txset %v is not supported", ledger.MustV2().TxSet.V)
		}
	default:
		return fmt.Errorf("ledger version %v is not supported", ledger.V)
	}
	return nil
}

// MergeLedgers merges src into dst: dst keeps its header and gains src's
// transactions, upgrades and evicted keys, merged phase by phase (see
// MergeLedgerBytes). getLedgerSeq is used to determine the ledger sequence
// value for all ledger entries contained in src during the merge.
func MergeLedgers(dst *xdr.LedgerCloseMeta, src xdr.LedgerCloseMeta, getLedgerSeq func(cur uint32) uint32) error {
	raw, _, err := mergeGenerated(*dst, src, getLedgerSeq, nil)
	if err != nil {
		return err
	}
	var merged xdr.LedgerCloseMeta
	if err := merged.UnmarshalBinary(raw); err != nil {
		return err
	}
	*dst = merged
	return nil
}

// mergeGenerated merges generated into real (see MergeLedgers) and returns the
// merged ledger's XDR and header hash. prevHash, when non-nil, becomes the
// merged ledger's previousLedgerHash.
func mergeGenerated(
	real, generated xdr.LedgerCloseMeta, getLedgerSeq func(uint32) uint32, prevHash *xdr.Hash,
) ([]byte, xdr.Hash, error) {
	if err := validLedger(real); err != nil {
		return nil, xdr.Hash{}, err
	}
	if err := validLedger(generated); err != nil {
		return nil, xdr.Hash{}, err
	}
	realRaw, err := real.MarshalBinary()
	if err != nil {
		return nil, xdr.Hash{}, err
	}
	generatedRaw, err := generated.MarshalBinary()
	if err != nil {
		return nil, xdr.Hash{}, err
	}
	// The real ledger is input 0, so the merge keeps its header.
	return MergeLedgerBytes([][]byte{realRaw, generatedRaw}, MergeOptions{
		PreviousLedgerHash: prevHash,
		RemapLedgerSeq: func(input int, seq uint32) uint32 {
			if input == 1 {
				return getLedgerSeq(seq)
			}
			return seq
		},
	})
}

// writeFramed writes raw as one XDR record-marked frame, the framing
// xdr.MarshalFramed writes and xdr.Stream reads.
func writeFramed(w io.Writer, raw []byte) error {
	const maxFrame, lastFragment = 0x7fffffff, 0x80000000
	if uint64(len(raw)) > maxFrame {
		return fmt.Errorf("overlong write: %d bytes", len(raw))
	}
	mark := binary.BigEndian.AppendUint32(nil, uint32(len(raw))|lastFragment) //nolint:gosec // bounded above
	if _, err := w.Write(mark); err != nil {
		return err
	}
	_, err := w.Write(raw)
	return err
}
