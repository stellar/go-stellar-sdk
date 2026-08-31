package ingest

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// LedgerTxParts is one transaction's handles from the single TxProcessing
// walk (see ExtractLedgerTxParts). The hashes are copied out of the buffer;
// Result and Meta are lazy zero-copy views that ALIAS the source
// LedgerCloseMetaView buffer — callers copy what they retain.
//
// Everything about the transaction pairs by Hash. For a fee-bump transaction
// (FeeBump true), Hash is the OUTER transaction's and InnerHash carries the
// inner transaction's; InnerHash is meaningless otherwise.
//
// FeeBump comes from the RESULT CODE — only a fee-bump result carries the
// inner hash. So a fee-bump whose operations never ran (its result is a
// plain error code; FeesFromTxParts explains how that can happen) reports
// FeeBump=false and a zero InnerHash. A hash index built from this walk
// will not find such a transaction by its inner hash — the inner hash
// simply is not in TxProcessing.
type LedgerTxParts struct {
	Hash      [32]byte
	InnerHash [32]byte
	FeeBump   bool

	// Result is the transaction's TransactionResultPair, located and trimmed
	// during the walk. Read fields zero-copy via the generated accessors,
	// e.g. Result.MustResult().MustFeeCharged().MustValue().
	Result xdr.TransactionResultPairView
	// Meta is the transaction's apply-processing TransactionMeta, located and
	// trimmed during the walk.
	Meta xdr.TransactionMetaView
}

// ExtractLedgerTxParts walks the ledger's TxProcessing once and returns one
// LedgerTxParts per transaction, in apply order. It is the ONLY function in
// this package that walks TxProcessing; every per-ledger product is a plain
// function of its output, so a consumer composes exactly the products it
// needs from exactly one walk:
//
//	txParts, err := ingest.ExtractLedgerTxParts(lcmView)
//	txEvents, err := ingest.EventsFromTxParts(txParts) // events indexer
//	fees, err := ingest.FeesFromTxParts(txParts)       // fee stats
//	hash := txParts[i].Hash                            // tx-hash index
//
// The TxSet (envelopes) is never read — everything a product needs comes
// from each transaction's result and meta. The returned Result/Meta views
// alias the lcmView buffer.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractLedgerTxParts(lcmView xdr.LedgerCloseMetaView) ([]LedgerTxParts, error) {
	d, err := dispatchLCMView(lcmView)
	if err != nil {
		return nil, err
	}
	var out []LedgerTxParts
	for parts, iterErr := range d.TxProcessing() {
		if iterErr != nil {
			return nil, fmt.Errorf("ingest: TxProcessing iter: %w", iterErr)
		}
		hash, innerHash, feeBump, herr := txProcessingHashes(parts)
		if herr != nil {
			return nil, herr
		}
		out = append(out, LedgerTxParts{
			Hash:      [32]byte(hash),
			InnerHash: [32]byte(innerHash),
			FeeBump:   feeBump,
			Result:    parts.Result,
			Meta:      parts.TxApplyProcessing,
		})
	}
	return out, nil
}

// TxEvents is one transaction's contract events in the flat raw-bytes shape
// an events indexer consumes, index-aligned with the LedgerTxParts slice it
// was derived from (the transaction hash lives on the parts element, not
// here). Every byte slice ALIASES the source LedgerCloseMetaView buffer
// (zero-copy); callers copy what they retain.
//
//   - TransactionEvents holds the V4 top-level transaction events, each a raw
//     xdr.TransactionEvent. Read Stage / the inner event zero-copy by wrapping
//     an element: xdr.TransactionEventView(raw).Stage() / .Event().
//   - OperationEvents holds, per operation, the raw xdr.ContractEvent bytes.
//     For V3 there is a single operation group when SorobanMeta is PRESENT (the
//     soroban tx has one op) and NO group at all when it is absent; for V4
//     there is one group per operation. The absent-SorobanMeta case carries no
//     events either way, so an indexer sees the same (empty) event set; the
//     transaction read path does report the empty group there, because it
//     mirrors LedgerTransaction.GetTransactionEvents' arity — see
//     LedgerTransactionView.ContractEvents.
//
// V0/V1/V2 meta carry no contract events, so both fields are empty.
type TxEvents struct {
	TransactionEvents [][]byte   // raw xdr.TransactionEvent (V4 top-level)
	OperationEvents   [][][]byte // raw xdr.ContractEvent, per operation
}

// EventsFromTxParts returns the contract events of every transaction, one
// TxEvents per LedgerTxParts element, index-aligned with txParts. It only
// reads the already-located Meta views — no TxProcessing walk of its own.
//
// It does NOT gate V3 SorobanMeta events on whether the transaction is
// soroban — an events-index consumer relies on the trusted-input invariant
// (SorobanMeta present ⟺ soroban tx); the transaction read path
// (LedgerTransactionViewByHash / LedgerTransactionViewRange) applies that
// gate where the paired envelope is in hand, matching the parsed
// GetTransactionEvents. Diagnostic events are not included — they are a
// read-path concern, available per transaction via
// LedgerTransactionView.DiagnosticEvents.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func EventsFromTxParts(txParts []LedgerTxParts) ([]TxEvents, error) {
	out := make([]TxEvents, 0, len(txParts))
	for i := range txParts {
		txEvents, err := transactionEventsFromMeta(txParts[i].Meta)
		if err != nil {
			return nil, err
		}
		out = append(out, txEvents)
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
// the classification rules). It only reads the already-located Result and
// Meta views — no TxProcessing walk of its own, no TxSet read, and no
// network passphrase: whether a transaction is soroban and how many
// operations it has are both answered from TxProcessing (SorobanMeta
// presence and the per-operation result count).
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
	for i := range txParts {
		fee, bucket, err := classifyTxFee(txParts[i])
		if err != nil {
			return LedgerFees{}, err
		}
		switch bucket {
		case feeBucketClassic:
			out.ClassicFeesPerOp = append(out.ClassicFeesPerOp, fee)
		case feeBucketSoroban:
			out.SorobanInclusionFees = append(out.SorobanInclusionFees, fee)
		case feeBucketNone:
		}
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

// classifyTxFee runs the per-transaction classification (see LedgerFees) for
// one walk element, reading only its Result and Meta views.
func classifyTxFee(txParts LedgerTxParts) (fee uint64, bucket feeBucket, err error) {
	var rawFeeCharged int64
	if terr := xdr.TryVoid(func() {
		rawFeeCharged = txParts.Result.MustResult().MustFeeCharged().MustValue()
	}); terr != nil {
		return 0, feeBucketNone, fmt.Errorf("ingest: tx %x: fee charged: %w", txParts.Hash, terr)
	}
	if rawFeeCharged < 0 {
		return 0, feeBucketNone, fmt.Errorf("ingest: tx %x: fee charged cannot be negative", txParts.Hash)
	}
	feeCharged := uint64(rawFeeCharged)

	nonRefundable, refundable, sorobanMetaPresent, hasExt, feesErr := sorobanFeesFromMeta(txParts.Meta)
	if feesErr != nil {
		return 0, feeBucketNone, feesErr
	}
	if sorobanMetaPresent {
		if !hasExt {
			return 0, feeBucketNone, nil
		}
		// Each component is validated separately BEFORE the addition — a
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

	opCount, opErr := txOperationCount(txParts.Result)
	if opErr != nil {
		return 0, feeBucketNone, opErr
	}
	if opCount == 0 {
		// No per-operation results: an empty list should not happen (core
		// rejects op-less transactions), and a result code with no list at
		// all means the operations never ran — core malfunctioned
		// (txINTERNAL_ERROR) or an earlier transaction in the same ledger
		// invalidated this one (see FeesFromTxParts for the example). Either
		// way the fee says nothing about fee bidding; skip it.
		return 0, feeBucketNone, nil
	}
	//nolint:gosec // opCount > 0 was checked above
	return feeCharged / uint64(opCount), feeBucketClassic, nil
}

// txOperationCount reads a transaction's operation count off its result: the
// number of per-operation results — the outer result's for txSUCCESS/txFAILED,
// the INNER result's for a fee-bump. A result code with no per-operation list
// (txINTERNAL_ERROR, or a transaction invalidated before its operations ran)
// counts as zero.
func txOperationCount(resultPairView xdr.TransactionResultPairView) (int, error) {
	var opCount int
	err := xdr.TryVoid(func() {
		resultView := resultPairView.MustResult().MustResult()
		switch resultView.MustCode() {
		case xdr.TransactionResultCodeTxSuccess, xdr.TransactionResultCodeTxFailed:
			opCount = resultView.MustResults().MustCount()
		case xdr.TransactionResultCodeTxFeeBumpInnerSuccess, xdr.TransactionResultCodeTxFeeBumpInnerFailed:
			innerResultView := resultView.MustInnerResultPair().MustResult().MustResult()
			switch innerResultView.MustCode() {
			case xdr.TransactionResultCodeTxSuccess, xdr.TransactionResultCodeTxFailed:
				opCount = innerResultView.MustResults().MustCount()
			default:
			}
		default:
		}
	})
	if err != nil {
		return 0, fmt.Errorf("ingest: tx operation count: %w", err)
	}
	return opCount, nil
}

// sorobanFeesFromMeta reads a transaction meta's soroban fee facts:
// sorobanMetaPresent reports whether the meta carries SorobanMeta at all
// (TransactionMeta V3/V4 — the fee classification's soroban gate), and
// hasExt whether that SorobanMeta carries the SorobanTransactionMetaExtV1
// with the charged resource fees. nonRefundable/refundable are meaningful
// only when hasExt is true. Meta versions 0/1/2 report neither; bytes with a
// version the generated views don't know fail the TxProcessing walk before
// classification.
func sorobanFeesFromMeta(metaView xdr.TransactionMetaView) (nonRefundable, refundable int64, sorobanMetaPresent, hasExt bool, err error) {
	v, err := metaView.V()
	if err != nil {
		return 0, 0, false, false, fmt.Errorf("ingest: meta.V: %w", err)
	}
	err = xdr.TryVoid(func() {
		var extView xdr.SorobanTransactionMetaExtView
		switch v {
		case 3: //nolint:mnd // TransactionMeta version discriminant
			sorobanMetaView, present := metaView.MustV3().MustSorobanMeta().MustUnwrap()
			if !present {
				return
			}
			sorobanMetaPresent = true
			extView = sorobanMetaView.MustExt()
		case 4: //nolint:mnd // TransactionMeta version discriminant
			sorobanMetaView, present := metaView.MustV4().MustSorobanMeta().MustUnwrap()
			if !present {
				return
			}
			sorobanMetaPresent = true
			extView = sorobanMetaView.MustExt()
		default:
			return
		}
		if extView.MustV() != 1 {
			return
		}
		extV1View := extView.MustV1()
		nonRefundable = extV1View.MustTotalNonRefundableResourceFeeCharged().MustValue()
		refundable = extV1View.MustTotalRefundableResourceFeeCharged().MustValue()
		hasExt = true
	})
	if err != nil {
		return 0, 0, false, false, fmt.Errorf("ingest: soroban meta fees: %w", err)
	}
	return nonRefundable, refundable, sorobanMetaPresent, hasExt, nil
}
