package ingest

import (
	"context"
	"encoding/hex"
	"io"

	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var badMetaVersionErr = errors.New(
	"TransactionMeta.V=2 is required in protocol version older than version 10. " +
		"Please process ledgers again using the latest stellar-core version.",
)

// LedgerTransactionReader reads transactions for a given ledger sequence from a
// backend. Use NewTransactionReader to create a new instance.
type LedgerTransactionReader struct {
	lcm             xdr.LedgerCloseMeta                  // read-only
	envelopesByHash map[xdr.Hash]xdr.TransactionEnvelope // set once
	phases          []txPhase                            // set once

	readIdx int // tracks iteration & seeking
}

// txPhase describes one transaction set phase in TxProcessing order. end is
// the index one past the last transaction of the phase. parallel is true for
// a phase stellar-core applied with the parallel apply semantics of CAP-63,
// which is encoded as TransactionPhase V1.
type txPhase struct {
	end      int
	parallel bool
}

// phaseOf returns, for the transaction at processing index i, the processing
// index at which its phase starts and whether that phase is parallel.
// LedgerCloseMeta V0 has no phases and reports a single sequential phase.
func (reader *LedgerTransactionReader) phaseOf(i int) (start int, parallel bool) {
	for _, ph := range reader.phases {
		if i < ph.end {
			return start, ph.parallel
		}
		start = ph.end
	}
	return start, false
}

// setPhases records the phase boundaries of the transaction set. Phases are
// applied in index order and TxProcessing is in apply order, so the
// transactions of phase k occupy one contiguous index range.
func (reader *LedgerTransactionReader) setPhases() error {
	var phases []xdr.TransactionPhase
	if v1, ok := reader.lcm.GetV1(); ok {
		phases = v1.TxSet.V1TxSet.Phases
	} else if v2, ok := reader.lcm.GetV2(); ok {
		phases = v2.TxSet.V1TxSet.Phases
	} else if reader.lcm.V == 0 {
		// No phases: every transaction is in one sequential phase.
		return nil
	} else {
		return errors.Errorf("unsupported LedgerCloseMeta.V: %d", reader.lcm.V)
	}
	end := 0
	for _, phase := range phases {
		switch phase.V {
		case 0:
			for _, component := range *phase.V0Components {
				end += len(component.TxsMaybeDiscountedFee.Txs)
			}
		case 1:
			for _, stage := range phase.ParallelTxsComponent.ExecutionStages {
				for _, cluster := range stage {
					end += len(cluster)
				}
			}
		default:
			return errors.Errorf("unsupported TransactionPhase.V: %d", phase.V)
		}
		reader.phases = append(reader.phases, txPhase{end: end, parallel: phase.V == 1})
	}
	if end != reader.lcm.CountTransactions() {
		return errors.Errorf("transaction set holds %d transactions but TxProcessing holds %d", end, reader.lcm.CountTransactions())
	}
	return nil
}

// NewLedgerTransactionReader creates a new TransactionReader instance. Note
// that TransactionReader is not thread safe and should not be shared by
// multiple goroutines.
func NewLedgerTransactionReader(
	ctx context.Context,
	backend ledgerbackend.LedgerBackend,
	networkPassphrase string,
	sequence uint32,
) (*LedgerTransactionReader, error) {
	ledgerCloseMeta, err := backend.GetLedger(ctx, sequence)
	if err != nil {
		return nil, errors.Wrap(err, "error getting ledger from the backend")
	}

	return NewLedgerTransactionReaderFromLedgerCloseMeta(networkPassphrase, ledgerCloseMeta)
}

// NewLedgerTransactionReaderFromLedgerCloseMeta creates a new TransactionReader
// instance from xdr.LedgerCloseMeta. Note that TransactionReader is not thread
// safe and should not be shared by multiple goroutines.
func NewLedgerTransactionReaderFromLedgerCloseMeta(
	networkPassphrase string,
	ledgerCloseMeta xdr.LedgerCloseMeta,
) (*LedgerTransactionReader, error) {
	reader := &LedgerTransactionReader{
		lcm:             ledgerCloseMeta,
		envelopesByHash: make(map[xdr.Hash]xdr.TransactionEnvelope, ledgerCloseMeta.CountTransactions()),
		readIdx:         0,
	}

	if err := reader.storeTransactions(networkPassphrase); err != nil {
		return nil, errors.Wrap(err, "error extracting transactions from ledger close meta")
	}
	if err := reader.setPhases(); err != nil {
		return nil, errors.Wrap(err, "error reading transaction set phases from ledger close meta")
	}
	return reader, nil
}

// GetSequence returns the sequence number of the ledger data stored by this object.
func (reader *LedgerTransactionReader) GetSequence() uint32 {
	return reader.lcm.LedgerSequence()
}

// GetHeader returns the XDR Header data associated with the stored ledger.
func (reader *LedgerTransactionReader) GetHeader() xdr.LedgerHeaderHistoryEntry {
	return reader.lcm.LedgerHeaderHistoryEntry()
}

// Read returns the next transaction in the ledger, ordered by tx number, each time
// it is called. When there are no more transactions to return, an EOF error is returned.
func (reader *LedgerTransactionReader) Read() (LedgerTransaction, error) {
	if reader.readIdx >= reader.lcm.CountTransactions() {
		return LedgerTransaction{}, io.EOF
	}
	i := reader.readIdx
	reader.readIdx++ // next read will advance even on error

	hash := reader.lcm.TransactionHash(i)
	envelope, ok := reader.envelopesByHash[hash]
	if !ok {
		hexHash := hex.EncodeToString(hash[:])
		return LedgerTransaction{}, errors.Errorf("unknown tx hash in LedgerCloseMeta: %v", hexHash)
	}

	var postTxApplyFeeChanges xdr.LedgerEntryChanges
	if lcmV2, ok := reader.lcm.GetV2(); ok {
		postTxApplyFeeChanges = lcmV2.TxProcessing[i].PostTxApplyFeeProcessing
	}

	return LedgerTransaction{
		Index:                 uint32(i + 1), // Transactions start at '1'
		Envelope:              envelope,
		Result:                reader.lcm.TransactionResultPair(i),
		UnsafeMeta:            reader.lcm.TxApplyProcessing(i),
		FeeChanges:            reader.lcm.FeeProcessing(i),
		PostTxApplyFeeChanges: postTxApplyFeeChanges,
		LedgerVersion:         uint32(reader.lcm.LedgerHeaderHistoryEntry().Header.LedgerVersion),
		Ledger:                reader.lcm,
		Hash:                  hash,
	}, nil
}

// Rewind resets the reader back to the first transaction in the ledger
func (reader *LedgerTransactionReader) Rewind() {
	reader.Seek(0)
}

// Seek sets the reader back to a specific transaction in the ledger
func (reader *LedgerTransactionReader) Seek(index int) error {
	if index >= reader.lcm.CountTransactions() || index < 0 {
		return io.EOF
	}

	reader.readIdx = index
	return nil
}

// storeHashes creates a mapping between hashes and envelopes in order to
// correctly provide a per-transaction view on-the-fly when Read() is called.
func (reader *LedgerTransactionReader) storeTransactions(networkPassphrase string) error {
	// See https://github.com/stellar/go-stellar-sdk/pull/2720: envelopes in the meta (which
	// just come straight from the agreed-upon transaction set) are not in the
	// same order as the actual list of metas (which are sorted by hash), so we
	// need to hash the envelopes *first* to properly associate them with their
	// metas.
	for i, tx := range reader.lcm.TransactionEnvelopes() {
		hash, err := network.HashTransactionInEnvelope(tx, networkPassphrase)
		if err != nil {
			return errors.Wrapf(err, "could not hash transaction %d in TxSet", i)
		}
		reader.envelopesByHash[xdr.Hash(hash)] = tx

		// We check the version only if FeeProcessing is non-empty, because some
		// backends (like HistoryArchiveBackend) do not return meta.
		//
		// Note that the ordering differences are irrelevant here because all we
		// care about is checking every meta for this condition.
		if reader.lcm.ProtocolVersion() < 10 && reader.lcm.TxApplyProcessing(i).V < 2 &&
			len(reader.lcm.FeeProcessing(i)) > 0 {
			return badMetaVersionErr
		}
	}

	return nil
}

// Close should be called when reading is finished. This is especially
// helpful when there are still some transactions available so reader can stop
// streaming them.
func (reader *LedgerTransactionReader) Close() error {
	reader.envelopesByHash = nil
	reader.phases = nil
	return nil
}
