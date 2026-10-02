package ingest

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// LedgerTxParts is one transaction's handles from ExtractLedgerTxParts. Result
// and Meta alias the LedgerCloseMetaView buffer; callers copy what they retain.
//
// For a fee-bump transaction (FeeBump true), Hash is the outer transaction's
// and InnerHash the inner's. FeeBump comes from the result code, so a fee-bump
// whose operations never ran (see FeesFromTxParts) reports FeeBump false and a
// zero InnerHash.
type LedgerTxParts struct {
	Hash      [32]byte
	InnerHash [32]byte
	FeeBump   bool

	// Result is the transaction's TransactionResultPair.
	Result xdr.TransactionResultPairView
	// Meta is the transaction's apply-processing TransactionMeta. The products
	// read it at offsets the walk recorded, so it may be replaced by a copy of
	// its bytes but not by anything else.
	Meta xdr.TransactionMetaView

	// ElemStart and ElemEnd delimit the transaction's TxProcessing element (a
	// TransactionResultMeta, or a TransactionResultMetaV1 on an LCM V2 ledger)
	// in the LedgerCloseMetaView passed to ExtractLedgerTxParts.
	ElemStart int
	ElemEnd   int

	// rec[recStart:recEnd] is the walk's entry for Meta; nil rec for parts
	// built by hand.
	rec              *offsetRecord
	recStart, recEnd uint32
}

// ExtractLedgerTxParts walks the ledger's TxProcessing once and returns one
// LedgerTxParts per transaction, in apply order, recording where the products
// read their fields so they do not walk again:
//
//	txParts, err := ingest.ExtractLedgerTxParts(lcmView)
//	txEvents, err := ingest.EventsFromTxParts(txParts) // events indexer
//	fees, err := ingest.FeesFromTxParts(txParts)       // fee stats
//	hash := txParts[i].Hash                            // tx-hash index
//
// The TxSet (envelopes) is never read.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractLedgerTxParts(lcmView xdr.LedgerCloseMetaView) ([]LedgerTxParts, error) {
	d, err := dispatchLCMView(lcmView)
	if err != nil {
		return nil, err
	}
	if d.txs.count == 0 {
		return nil, nil
	}
	out, rec := presized(d.txs.count)
	return d.txs.walk(out, rec, func(int, xdr.TransactionResultPairView) (bool, bool) {
		return true, false
	})
}

// offsetsPerTx presizes the offset record: a median transaction on recent
// pubnet ledgers needs 12 to 14 entries.
const offsetsPerTx = 14

// TxEvents is one transaction's contract events, index-aligned with the
// LedgerTxParts slice it was derived from. The events alias the
// LedgerCloseMetaView buffer; callers copy what they retain.
//
//   - TransactionEvents holds the V4 top-level transaction events.
//   - OperationEvents holds each operation's contract events. For V3 there is
//     one operation group when SorobanMeta is present and none when it is
//     absent; for V4, one group per operation. LedgerTransactionView's
//     ContractEvents differs on the absent case: one empty group, matching
//     GetTransactionEvents.
//
// V0, V1 and V2 metas carry no contract events, so both fields are empty.
type TxEvents struct {
	TransactionEvents []xdr.TransactionEventView
	OperationEvents   [][]xdr.ContractEventView
}

// EventsFromTxParts returns the contract events of every transaction,
// index-aligned with txParts.
//
// V3 SorobanMeta events are returned whether or not the transaction is a
// Soroban one; the read path (LedgerTransactionViewByHash and
// LedgerTransactionViewRange) gates them on the paired envelope, as
// GetTransactionEvents does. Diagnostic events are left to the read path.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func EventsFromTxParts(txParts []LedgerTxParts) ([]TxEvents, error) {
	slabs := newEventSlabs(txParts)
	out := make([]TxEvents, len(txParts))
	var scratch offsetRecord
	for i := range txParts {
		entry, err := txParts[i].entry(&scratch)
		if err != nil {
			return nil, err
		}
		s := &slabs
		if txParts[i].rec == nil { // not counted in the slabs
			s = nil
		}
		if out[i], err = metaEvents(txParts[i].Meta, entry, s); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// LedgerFees is one ledger's fee observations, split the way fee-stats
// consumers bucket them (stellar-rpc's getFeeStats windows). The values are
// plain integers copied out of the views — nothing in the result aliases the
// source buffer. Both buckets are in apply order; the ledger sequence and
// close time are the caller's to track.
//
// The per-transaction classification (see FeesFromTxParts):
//
//   - A transaction whose meta carries SorobanMeta (TransactionMeta V3 or V4)
//     with SorobanMeta.Ext.V == 1 contributes
//     FeeCharged − (TotalNonRefundableResourceFeeCharged +
//     TotalRefundableResourceFeeCharged) to SorobanInclusionFees.
//   - A transaction whose meta carries SorobanMeta WITHOUT that extension
//     contributes nothing — skipped, not counted as classic. (Since protocol
//     21 the extension is always present, so tip ledgers never take this arm;
//     protocol-20 ledgers — real soroban traffic from before the extension
//     existed — do land here and are skipped, matching v1.)
//   - Every other transaction contributes FeeCharged / opCount (integer
//     division) to ClassicFeesPerOp, where opCount is the number of
//     per-operation results in the transaction's result — the outer result's
//     for txSUCCESS/txFAILED, the INNER result's for a fee-bump.
//   - A transaction with no per-operation result list is skipped. That
//     covers txINTERNAL_ERROR (core malfunction) and transactions
//     invalidated by an earlier transaction in the same ledger before their
//     operations ran — possible when an account signs an operation against
//     itself (account merge, signer removal, sequence bump) inside another
//     account's transaction. FeesFromTxParts has a worked example.
//
// For a fee-bump transaction FeeCharged is the OUTER result's. FeeCharged is
// counted whether or not the transaction succeeded.
type LedgerFees struct {
	// ClassicFeesPerOp holds, for every non-Soroban transaction,
	// FeeCharged / opCount (integer division).
	ClassicFeesPerOp []uint64
	// SorobanInclusionFees holds, for every Soroban transaction with
	// SorobanTransactionMetaExtV1, FeeCharged minus the total (refundable +
	// non-refundable) resource fee charged.
	SorobanInclusionFees []uint64
}

// FeesFromTxParts returns the ledger's fee observations (see LedgerFees for
// the classification rules). It reads only TxProcessing, so no TxSet and no
// network passphrase: whether a transaction is Soroban and how many
// operations it has both come from TxProcessing (SorobanMeta presence and the
// per-operation result count).
//
// Classification errors are loud: a negative FeeCharged, a negative resource
// fee component (or an int64-overflowing sum), or a charged resource fee
// exceeding FeeCharged is an error — fee stats ingest trusted tip ledgers, so
// a shape like that means the input is corrupt, not that a bucket should
// quietly absorb it.
//
// The output matches stellar-rpc v1's FeeWindows.IngestFees on organic
// current-protocol traffic, but the definition differs: v1 classifies from
// the ENVELOPE (a single operation of a soroban type; opCount = envelope
// operations), this classifies from TxProcessing alone as described above.
// The behavioral deltas are confined to classic transactions whose
// operations never ran, so their results carry no per-operation list — v1
// counts them from the envelope, this skips them. Only two things put such
// a transaction in a ledger:
//
//   - a core malfunction (txINTERNAL_ERROR), or
//   - an account invalidating its own pending transaction. Example: Alice's
//     payment is in the ledger, and so is Bob's transaction carrying an
//     Alice-SIGNED operation that merges Alice's account away (or removes
//     her signer, or bumps her sequence). Bob's applies first, so Alice's
//     payment lands with just txNO_ACCOUNT (or txBAD_AUTH / txBAD_SEQ), its
//     fee charged and no operation results. Every such operation needs
//     Alice's own signature — self-inflicted and absent from organic
//     traffic, but a healthy network will happily include it.
//
// Pre-protocol-20 tx sets also allowed several transactions per account, so
// old ledgers contain such never-ran failures organically; fee stats only
// ever ingests tip ledgers. Separately, v1 lets a resource fee above
// FeeCharged wrap around uint64 where this errors. (Soroban is untouched by
// all of this: core writes the fee ext before the validation step that can
// kill a transaction, so even a never-ran soroban transaction carries its
// charged fees and both definitions count it identically.)
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func FeesFromTxParts(txParts []LedgerTxParts) (LedgerFees, error) {
	var out LedgerFees
	var scratch offsetRecord
	var i int
	err, viewErr := xdr.Try(func() error {
		for i = range txParts {
			entry, err := txParts[i].entry(&scratch)
			if err != nil {
				return err
			}
			fee, bucket, err := classifyTxFee(&txParts[i], entry)
			if err != nil {
				return err
			}
			switch bucket {
			case feeBucketClassic:
				out.ClassicFeesPerOp = append(out.ClassicFeesPerOp, fee)
			case feeBucketSoroban:
				out.SorobanInclusionFees = append(out.SorobanInclusionFees, fee)
			case feeBucketNone:
			}
		}
		return nil
	})
	if viewErr != nil {
		return LedgerFees{}, fmt.Errorf("ingest: tx %x: fees: %w", txParts[i].Hash, viewErr)
	}
	if err != nil {
		return LedgerFees{}, err
	}
	return out, nil
}

// feeBucket is a classifyTxFee outcome: which LedgerFees bucket the
// transaction's fee lands in, if any.
type feeBucket int

const (
	feeBucketNone feeBucket = iota // contributes nothing (no per-op results, or SorobanMeta without the Ext.V1 fees)
	feeBucketClassic
	feeBucketSoroban
)

// classifyTxFee runs the per-transaction classification (see LedgerFees).
// Malformed views panic with *xdr.ViewError.
func classifyTxFee(txParts *LedgerTxParts, entry []uint32) (fee uint64, bucket feeBucket, err error) {
	rawFeeCharged := txParts.Result.MustResult().MustFeeCharged().MustValue()
	if rawFeeCharged < 0 {
		return 0, feeBucketNone, fmt.Errorf("ingest: tx %x: fee charged cannot be negative", txParts.Hash)
	}
	feeCharged := uint64(rawFeeCharged)

	if entry[entrySoroban] != 0 {
		ext := sorobanMetaExt(txParts.Meta, entry)
		if ext.MustV() != 1 {
			return 0, feeBucketNone, nil
		}
		extV1 := ext.MustV1()
		nonRefundable := extV1.MustTotalNonRefundableResourceFeeCharged().MustValue()
		refundable := extV1.MustTotalRefundableResourceFeeCharged().MustValue()
		// Each component is validated separately before the addition: a
		// sum-only check can be wrapped through (two huge negative components
		// sum back to non-negative). With both components non-negative, a
		// negative sum can only mean the addition overflowed int64.
		if nonRefundable < 0 || refundable < 0 {
			return 0, feeBucketNone, fmt.Errorf("ingest: tx %x: resource fee charged cannot be negative", txParts.Hash)
		}
		resourceFee := nonRefundable + refundable
		if resourceFee < 0 {
			return 0, feeBucketNone, fmt.Errorf("ingest: tx %x: resource fee charged overflows int64", txParts.Hash)
		}
		if uint64(resourceFee) > feeCharged {
			return 0, feeBucketNone, fmt.Errorf(
				"ingest: tx %x: resource fee charged %d exceeds fee charged %d", txParts.Hash, resourceFee, feeCharged)
		}
		return feeCharged - uint64(resourceFee), feeBucketSoroban, nil
	}

	opCount := txOperationCount(txParts.Result)
	if opCount == 0 {
		// No per-operation results: an empty list should not happen (core
		// rejects op-less transactions), and a result code with no list at
		// all means the operations never ran: core malfunctioned
		// (txINTERNAL_ERROR) or an earlier transaction in the same ledger
		// invalidated this one (see FeesFromTxParts for the example). Either
		// way the fee says nothing about fee bidding; skip it.
		return 0, feeBucketNone, nil
	}
	//nolint:gosec // opCount > 0 was checked above
	return feeCharged / uint64(opCount), feeBucketClassic, nil
}

// txOperationCount reads a transaction's operation count off its result: the
// number of per-operation results, the outer result's for txSUCCESS/txFAILED
// and the inner result's for a fee-bump. A result code with no per-operation
// list (txINTERNAL_ERROR, or a transaction invalidated before its operations
// ran) counts as zero.
func txOperationCount(resultPairView xdr.TransactionResultPairView) int {
	resultView := resultPairView.MustResult().MustResult()
	switch resultView.MustCode() {
	case xdr.TransactionResultCodeTxSuccess, xdr.TransactionResultCodeTxFailed:
		return resultView.MustResults().MustCount()
	case xdr.TransactionResultCodeTxFeeBumpInnerSuccess, xdr.TransactionResultCodeTxFeeBumpInnerFailed:
		innerResultView := resultView.MustInnerResultPair().MustResult().MustResult()
		switch innerResultView.MustCode() {
		case xdr.TransactionResultCodeTxSuccess, xdr.TransactionResultCodeTxFailed:
			return innerResultView.MustResults().MustCount()
		default:
		}
	default:
	}
	return 0
}

// sorobanMetaExt returns the extension of the SorobanMeta the entry points at.
func sorobanMetaExt(meta xdr.TransactionMetaView, entry []uint32) xdr.SorobanTransactionMetaExtView {
	at := entry[entrySoroban]
	if entry[entryVersion] == 3 { //nolint:mnd // TransactionMeta version
		return xdr.SorobanTransactionMetaView(meta[at:]).MustExt()
	}
	return xdr.SorobanTransactionMetaV2View(meta[at:]).MustExt()
}
