package ingest

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ExtractTxHashes returns every transaction hash of the ledger in apply
// (TxProcessing) order, read straight from each TransactionResultPair without
// decoding anything else — the cheapest possible per-ledger hash listing
// (e.g. for building a tx-hash → ledger index).
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractTxHashes(lcm xdr.LedgerCloseMetaView) ([]xdr.Hash, error) {
	d, err := dispatchLCMView(lcm)
	if err != nil {
		return nil, err
	}
	var out []xdr.Hash
	for parts, iterErr := range d.TxProcessing() {
		if iterErr != nil {
			return nil, fmt.Errorf("ingest: TxProcessing iter: %w", iterErr)
		}
		h, herr := txProcessingHash(parts)
		if herr != nil {
			return nil, herr
		}
		out = append(out, h)
	}
	return out, nil
}

// LedgerTransactionEvents is one transaction's contract events plus its hash,
// in the flat raw-bytes shape an events indexer consumes. Every byte slice
// ALIASES the source LedgerCloseMetaView buffer (zero-copy); callers copy what
// they retain.
//
//   - TransactionEvents holds the V4 top-level transaction events, each a raw
//     xdr.TransactionEvent. Read Stage / the inner event zero-copy by wrapping
//     an element: xdr.TransactionEventView(raw).Stage() / .Event().
//   - OperationEvents holds, per operation, the raw xdr.ContractEvent bytes.
//     For V3 SorobanMeta there is a single operation group (the soroban tx has
//     one op); for V4 there is one group per operation.
//
// V0/V1/V2 meta carry no contract events, so both event fields are empty.
type LedgerTransactionEvents struct {
	Hash              [32]byte
	InnerHash         [32]byte   // the inner transaction's hash; meaningful iff FeeBump
	FeeBump           bool       // the transaction is a fee-bump
	TransactionEvents [][]byte   // raw xdr.TransactionEvent (V4 top-level)
	OperationEvents   [][][]byte // raw xdr.ContractEvent, per operation
}

// ExtractLedgerEvents returns the contract events of every transaction in the
// ledger, in apply order, each paired with its transaction hash — hash and
// events come from ONE TxProcessing walk (sizing each element to advance the
// iterator is the dominant cost, so a separate hash pass would nearly double
// it). For a fee-bump transaction, InnerHash carries the inner transaction's
// hash.
//
// It does NOT gate V3 SorobanMeta events on whether the transaction is soroban
// — an events-index consumer relies on the trusted-input invariant
// (SorobanMeta present ⟺ soroban tx); the transaction read path
// (LedgerTransactionViewByHash / LedgerTransactionViewRange) applies that gate
// where the paired envelope is in hand, matching the parsed
// GetTransactionEvents. Diagnostic events are not included — they are a
// read-path concern, available per transaction via
// LedgerTransactionView.DiagnosticEvents.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractLedgerEvents(lcm xdr.LedgerCloseMetaView) ([]LedgerTransactionEvents, error) {
	d, err := dispatchLCMView(lcm)
	if err != nil {
		return nil, err
	}
	var out []LedgerTransactionEvents
	for parts, iterErr := range d.TxProcessing() {
		if iterErr != nil {
			return nil, fmt.Errorf("ingest: TxProcessing iter: %w", iterErr)
		}
		h, inner, feeBump, herr := txProcessingHashes(parts)
		if herr != nil {
			return nil, herr
		}
		tev, terr := transactionEventsFromMeta(parts.TxApplyProcessing)
		if terr != nil {
			return nil, terr
		}
		out = append(out, LedgerTransactionEvents{
			Hash:              [32]byte(h),
			InnerHash:         [32]byte(inner),
			FeeBump:           feeBump,
			TransactionEvents: tev.TransactionEvents,
			OperationEvents:   tev.OperationEvents,
		})
	}
	return out, nil
}

// LedgerFees is one ledger's fee observations, split the way fee-stats
// consumers bucket them (stellar-rpc's getFeeStats windows). The values are
// plain integers copied out of the view — nothing in the result aliases the
// source buffer.
//
// The classification replicates stellar-rpc v1's FeeWindows.IngestFees
// exactly:
//
//   - A transaction with exactly one operation of a Soroban type
//     (invokeHostFunction, extendFootprintTtl, restoreFootprint) whose meta
//     carries SorobanTransactionMetaExtV1 (TransactionMeta V3 or V4 with
//     SorobanMeta present and SorobanMeta.Ext.V == 1) contributes
//     FeeCharged − (TotalNonRefundableResourceFeeCharged +
//     TotalRefundableResourceFeeCharged) to SorobanInclusionFees.
//   - Such a single-Soroban-op transaction WITHOUT that meta extension
//     (SorobanMeta absent, Ext.V != 1, or any other meta version) contributes
//     nothing at all — it is skipped, not counted as classic. The gate is the
//     operation type alone; the envelope's SorobanTransactionData is not
//     consulted.
//   - Every other transaction with at least one operation contributes
//     FeeCharged / numOperations (integer division) to ClassicFeesPerOp.
//   - A transaction with zero operations is skipped.
//
// For a fee-bump transaction the operations are the INNER transaction's and
// FeeCharged is the OUTER result's. Both buckets are in apply order. FeeCharged
// is counted whether or not the transaction succeeded.
type LedgerFees struct {
	// ClassicFeesPerOp holds, for every non-Soroban transaction,
	// FeeCharged / numOperations (integer division).
	ClassicFeesPerOp []uint64
	// SorobanInclusionFees holds, for every Soroban transaction with
	// SorobanTransactionMetaExtV1, FeeCharged minus the total (refundable +
	// non-refundable) resource fee charged.
	SorobanInclusionFees []uint64
	LedgerSequence       uint32
	LedgerCloseTime      int64
}

// ExtractFees returns the ledger's fee observations (see LedgerFees for the
// classification rules). One TxProcessing walk collects each transaction's
// FeeCharged and meta view; the TxSet envelopes are then paired back by hash —
// the TxSet is in agreed-set order, not apply order, and hashing envelopes
// needs the network passphrase — to read each transaction's operation count
// and, for single-op transactions, the operation type. Those are the only
// three inputs the classification needs, so nothing else is decoded.
//
// A negative FeeCharged or a negative total resource fee is an error, and so
// is a pre-protocol-10 ledger whose meta is older than V2 while FeeProcessing
// is populated (the parsed reader's badMetaVersionErr guard). When the charged
// resource fee exceeds FeeCharged, the Soroban inclusion fee wraps around
// (uint64 subtraction) — preserving stellar-rpc v1's IngestFees behavior,
// which this function replicates bug-for-bug. Matching v1's reader, the WHOLE
// TxSet is hashed even after every transaction is paired, so a malformed
// envelope anywhere in it is an error, and the passphrase is validated only
// when there is at least one envelope to hash.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractFees(lcm xdr.LedgerCloseMetaView, passphrase string) (LedgerFees, error) {
	d, err := dispatchLCMView(lcm)
	if err != nil {
		return LedgerFees{}, err
	}
	ledgerSeq, ledgerCloseTime, err := d.Header()
	if err != nil {
		return LedgerFees{}, err
	}
	out := LedgerFees{LedgerSequence: ledgerSeq, LedgerCloseTime: ledgerCloseTime}

	txs, err := collectFeeTxParts(d)
	if err != nil {
		return LedgerFees{}, err
	}

	want := make([][32]byte, len(txs))
	for k := range txs {
		want[k] = txs[k].hash
	}
	byHash, err := feeShapesByHash(d, passphrase, want)
	if err != nil {
		return LedgerFees{}, err
	}

	for _, tx := range txs {
		shape, ok := byHash[tx.hash]
		if !ok {
			return LedgerFees{}, errMissingEnvelope(tx.hash)
		}
		fee, bucket, cerr := classifyFeeTx(tx, shape)
		if cerr != nil {
			return LedgerFees{}, cerr
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

// feeTxParts is the per-tx projection of one TxProcessing element that the fee
// classification needs: hash (for envelope pairing), the outer result's
// FeeCharged, and the meta view — an unread alias into the buffer, opened
// later only for the transactions whose shape turns out to be Soroban.
type feeTxParts struct {
	hash       [32]byte
	feeCharged int64
	meta       xdr.TransactionMetaView
}

// collectFeeTxParts is ExtractFees' single TxProcessing walk. On ledgers older
// than protocol 10 it also runs the parsed reader's meta-version guard, so
// both paths reject the same outdated-stellar-core ledgers.
func collectFeeTxParts(d lcmViewDispatch) ([]feeTxParts, error) {
	protocol, err := d.lcm.ProtocolVersion()
	if err != nil {
		return nil, fmt.Errorf("ingest: protocol version: %w", err)
	}
	var txs []feeTxParts
	for parts, iterErr := range d.TxProcessing() {
		if iterErr != nil {
			return nil, fmt.Errorf("ingest: TxProcessing iter: %w", iterErr)
		}
		if protocol < guardMinProtocol {
			if guardErr := checkMetaVersionGuard(parts); guardErr != nil {
				return nil, guardErr
			}
		}
		h, herr := txProcessingHash(parts)
		if herr != nil {
			return nil, herr
		}
		var fee int64
		if terr := xdr.TryVoid(func() {
			fee = parts.Result.MustResult().MustFeeCharged().MustValue()
		}); terr != nil {
			return nil, fmt.Errorf("ingest: tx fee charged: %w", terr)
		}
		txs = append(txs, feeTxParts{hash: [32]byte(h), feeCharged: fee, meta: parts.TxApplyProcessing})
	}
	return txs, nil
}

// From protocol 10 on, stellar-core always emits TransactionMeta V2 or newer;
// on older ledgers a pre-V2 meta next to populated fee processing means the
// meta came from an outdated stellar-core (see badMetaVersionErr and the
// matching check in the parsed reader's storeTransactions).
const (
	guardMinProtocol    = 10
	guardMinMetaVersion = 2
)

// checkMetaVersionGuard replicates the parsed reader's pre-protocol-10 check:
// meta older than V2 combined with non-empty FeeProcessing rejects the ledger
// with badMetaVersionErr. Called only for ledgers below guardMinProtocol.
func checkMetaVersionGuard(parts txResultParts) error {
	metaV, err := parts.TxApplyProcessing.V()
	if err != nil {
		return fmt.Errorf("ingest: meta.V: %w", err)
	}
	if metaV >= guardMinMetaVersion {
		return nil
	}
	feeChanges, err := parts.FeeProcessing.Count()
	if err != nil {
		return fmt.Errorf("ingest: FeeProcessing count: %w", err)
	}
	if feeChanges > 0 {
		return badMetaVersionErr
	}
	return nil
}

// feeBucket is a classifyFeeTx outcome: which LedgerFees bucket the
// transaction's fee lands in, if any.
type feeBucket int

const (
	feeBucketNone feeBucket = iota // contributes nothing (0 ops, or soroban shape without Ext.V1 meta)
	feeBucketClassic
	feeBucketSoroban
)

// classifyFeeTx runs v1's per-transaction classification (see LedgerFees) for
// one collected tx and its paired envelope's shape.
func classifyFeeTx(tx feeTxParts, shape feeTxShape) (fee uint64, bucket feeBucket, err error) {
	if tx.feeCharged < 0 {
		return 0, feeBucketNone, fmt.Errorf("ingest: tx %x: fee charged cannot be negative", tx.hash)
	}
	feeCharged := uint64(tx.feeCharged)
	if shape.opCount == 0 {
		// Should not happen (core rejects op-less transactions); skipped,
		// matching v1.
		return 0, feeBucketNone, nil
	}
	if shape.opCount == 1 && isSorobanFeeOp(shape.soleOpType) {
		nonRefundable, refundable, hasExt, feesErr := sorobanFeesFromMeta(tx.meta)
		if feesErr != nil {
			return 0, feeBucketNone, feesErr
		}
		if !hasExt {
			return 0, feeBucketNone, nil
		}
		// int64 addition first, exactly like v1: two huge fees wrap negative
		// and hit the error below rather than summing wide.
		resourceFee := nonRefundable + refundable
		if resourceFee < 0 {
			return 0, feeBucketNone, fmt.Errorf("ingest: tx %x: resource fee charged cannot be negative", tx.hash)
		}
		// uint64 subtraction, exactly like v1: a resource fee above FeeCharged
		// wraps around.
		return feeCharged - uint64(resourceFee), feeBucketSoroban, nil
	}
	//nolint:gosec // opCount > 0 was checked above
	return feeCharged / uint64(shape.opCount), feeBucketClassic, nil
}

// isSorobanFeeOp reports whether a single-operation transaction of this
// operation type is fee-classified as Soroban — the operation types v1's
// IngestFees switches on.
func isSorobanFeeOp(t xdr.OperationType) bool {
	switch t {
	case xdr.OperationTypeInvokeHostFunction,
		xdr.OperationTypeExtendFootprintTtl,
		xdr.OperationTypeRestoreFootprint:
		return true
	default:
		return false
	}
}

// feeTxShape is what the fee classification needs from a paired envelope: the
// operation count and — meaningful only when the count is exactly one — that
// operation's type. Operations follow xdr.TransactionEnvelope.Operations()
// semantics: for a fee-bump envelope they are the INNER transaction's.
type feeTxShape struct {
	opCount    int
	soleOpType xdr.OperationType
}

// feeShapesByHash enumerates the TxSet, hashes every envelope, and resolves
// the fee shape of those whose transaction hash appears in want. Unlike
// envelopesForHashes it never stops early: v1's parsed reader hashes the
// ENTIRE TxSet at construction time, so a malformed envelope after the last
// wanted hash — or an invalid passphrase on any non-empty TxSet — must be an
// error here exactly when it is one there. The hasher is built when the first
// envelope is seen, so an empty TxSet never validates the passphrase, which
// is also how v1 behaves.
func feeShapesByHash(d lcmViewDispatch, passphrase string, want [][32]byte) (map[[32]byte]feeTxShape, error) {
	need := make(map[[32]byte]struct{}, len(want))
	for _, h := range want {
		need[h] = struct{}{}
	}
	byHash := make(map[[32]byte]feeTxShape, len(need))
	var hasher *network.TransactionViewHasher
	for env, err := range d.Envelopes() {
		if err != nil {
			return nil, err
		}
		if hasher == nil {
			hasher, err = network.NewTransactionViewHasher(passphrase)
			if err != nil {
				return nil, err
			}
		}
		h, err := hasher.Hash(env)
		if err != nil {
			return nil, err
		}
		if _, ok := need[h]; !ok {
			continue
		}
		shape, err := envelopeFeeShape(env)
		if err != nil {
			return nil, err
		}
		byHash[h] = shape
		delete(need, h)
	}
	return byHash, nil
}

// envelopeFeeShape reads one envelope's feeTxShape in a single walk: the
// envelope-type discriminant picks the arm, then the arm's operations array
// yields the count and (for a single op) the type.
func envelopeFeeShape(env xdr.TransactionEnvelopeView) (feeTxShape, error) {
	var s feeTxShape
	var typ xdr.EnvelopeType
	unknownType := false
	err := xdr.TryVoid(func() {
		typ = env.MustType()
		switch typ {
		case xdr.EnvelopeTypeEnvelopeTypeTxV0:
			s.opCount, s.soleOpType = opsShape(env.MustV0().MustTx().MustOperations())
		case xdr.EnvelopeTypeEnvelopeTypeTx:
			s.opCount, s.soleOpType = opsShape(env.MustV1().MustTx().MustOperations())
		case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
			s.opCount, s.soleOpType = opsShape(env.MustFeeBump().MustTx().MustInnerTx().MustV1().MustTx().MustOperations())
		default:
			unknownType = true
		}
	})
	if err != nil {
		return feeTxShape{}, fmt.Errorf("ingest: envelope operations: %w", err)
	}
	if unknownType {
		return feeTxShape{}, fmt.Errorf("ingest: unknown TransactionEnvelope type %d", typ)
	}
	return s, nil
}

// opsShape is the operations-array read shared by envelopeFeeShape's three
// arms (the V0 array is a distinct view type from the V1/fee-bump one, hence
// the generic). It uses the generated Must accessors, which panic with
// *xdr.ViewError on malformed input; envelopeFeeShape's TryVoid recovers that
// panic into an ordinary error.
func opsShape[A interface {
	MustCount() int
	MustAt(int) xdr.OperationView
}](ops A) (int, xdr.OperationType) {
	n := ops.MustCount()
	if n != 1 {
		return n, 0
	}
	return 1, ops.MustAt(0).MustBody().MustType()
}

// sorobanFeesFromMeta reads the charged resource fees from a transaction
// meta's SorobanTransactionMetaExtV1. hasExt is false when the meta carries no
// such extension — SorobanMeta absent, Ext.V != 1, or a meta version other
// than 3/4. The default arm mirrors v1's IngestFees, which skips such
// transactions rather than erroring; today it is reachable only for versions
// 0/1/2, since bytes with a version the generated views don't know fail the
// TxProcessing walk before classification.
func sorobanFeesFromMeta(mv xdr.TransactionMetaView) (nonRefundable, refundable int64, hasExt bool, err error) {
	v, err := mv.V()
	if err != nil {
		return 0, 0, false, fmt.Errorf("ingest: meta.V: %w", err)
	}
	err = xdr.TryVoid(func() {
		var ext xdr.SorobanTransactionMetaExtView
		switch v {
		case 3: //nolint:mnd // TransactionMeta version discriminant
			sm, present := mv.MustV3().MustSorobanMeta().MustUnwrap()
			if !present {
				return
			}
			ext = sm.MustExt()
		case 4: //nolint:mnd // TransactionMeta version discriminant
			sm, present := mv.MustV4().MustSorobanMeta().MustUnwrap()
			if !present {
				return
			}
			ext = sm.MustExt()
		default:
			return
		}
		if ext.MustV() != 1 {
			return
		}
		v1 := ext.MustV1()
		nonRefundable = v1.MustTotalNonRefundableResourceFeeCharged().MustValue()
		refundable = v1.MustTotalRefundableResourceFeeCharged().MustValue()
		hasExt = true
	})
	if err != nil {
		return 0, 0, false, fmt.Errorf("ingest: soroban meta fees: %w", err)
	}
	return nonRefundable, refundable, hasExt, nil
}
