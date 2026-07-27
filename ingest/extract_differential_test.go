package ingest

import (
	"fmt"
	"iter"
	"math/rand"
	"os"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/randxdr"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Differential harness for the view-extractor rewrite: every change to the
// ExtractLedgerEvents walk chain must produce output IDENTICAL to the frozen
// oracle below — same errors-or-success on every input (including every
// truncation prefix), same hashes, and raw event slices that alias the exact
// same bytes of the input buffer (base pointer + length), with the
// empty-not-nil contract preserved.
//
// The oracle is a self-contained copy of the extraction chain frozen at commit
// 2bfffb15 (pre-rewrite HEAD): dispatch, TxProcessing walk, hash extraction,
// and the version-dispatched meta walk. It depends only on the generated view
// primitives, whose semantics are the stable substrate the rewrite composes
// differently. Do not "clean it up" to track the live code — divergence from
// live is the entire point.
//
// Ported mechanically to the walk API when the hook-era surface
// (Fields structs, At/Iter, Must*/TryVoid) was deleted: every navigation step
// maps 1:1 onto its replacement (arm entry + per-field accessors, All(),
// Raw()), preserving the
// oracle's read order — including the V4 arm reading Events before Operations,
// which is NOT wire order. Outputs, aliasing, and error-vs-success behavior
// are unchanged; the assertions, corpus, and structure below are untouched.

// ---------------------------------------------------------------------------
// Frozen oracle (copied from HEAD @ 2bfffb15; renamed and mechanically ported
// to the walk API, otherwise verbatim).
// ---------------------------------------------------------------------------

type oracleTxParts struct {
	Result            xdr.TransactionResultPairView
	TxApplyProcessing xdr.TransactionMetaView
}

// oracleParts resolves a TxProcessing element's Result and TxApplyProcessing
// through the tier-1 accessors (the same per-access locate semantics the
// pre-rewrite chain used).
func oracleParts(result func() (xdr.TransactionResultPairView, error), meta func() (xdr.TransactionMetaView, error)) (oracleTxParts, error) {
	res, err := result()
	if err != nil {
		return oracleTxParts{}, err
	}
	m, err := meta()
	if err != nil {
		return oracleTxParts{}, err
	}
	return oracleTxParts{Result: res, TxApplyProcessing: m}, nil
}

func oracleTxProcessingMeta(elems iter.Seq2[xdr.TransactionResultMetaView, error]) iter.Seq2[oracleTxParts, error] {
	return func(yield func(oracleTxParts, error) bool) {
		k := 0
		for elem, err := range elems {
			if err != nil {
				yield(oracleTxParts{}, fmt.Errorf("oracle: TxProcessing element %d: %w", k, err))
				return
			}
			parts, perr := oracleParts(elem.Result, elem.TxApplyProcessing)
			if perr != nil {
				yield(oracleTxParts{}, fmt.Errorf("oracle: TxProcessing element %d: %w", k, perr))
				return
			}
			if !yield(parts, nil) {
				return
			}
			k++
		}
	}
}

func oracleTxProcessingMetaV1(elems iter.Seq2[xdr.TransactionResultMetaV1View, error]) iter.Seq2[oracleTxParts, error] {
	return func(yield func(oracleTxParts, error) bool) {
		k := 0
		for elem, err := range elems {
			if err != nil {
				yield(oracleTxParts{}, fmt.Errorf("oracle: TxProcessing element %d: %w", k, err))
				return
			}
			parts, perr := oracleParts(elem.Result, elem.TxApplyProcessing)
			if perr != nil {
				yield(oracleTxParts{}, fmt.Errorf("oracle: TxProcessing element %d: %w", k, perr))
				return
			}
			if !yield(parts, nil) {
				return
			}
			k++
		}
	}
}

func oracleDispatchTP(lcm xdr.LedgerCloseMetaView) (iter.Seq2[oracleTxParts, error], error) {
	disc, err := lcm.V()
	if err != nil {
		return nil, fmt.Errorf("oracle: LCM.V: %w", err)
	}
	switch disc {
	case 0:
		v0, err := lcm.ArmV0()
		if err != nil {
			return nil, fmt.Errorf("oracle: LCM V0: %w", err)
		}
		raw, err := v0.TxProcessing()
		if err != nil {
			return nil, fmt.Errorf("oracle: V0 TxProcessing: %w", err)
		}
		return oracleTxProcessingMeta(raw.All()), nil
	case 1:
		v1, err := lcm.ArmV1()
		if err != nil {
			return nil, fmt.Errorf("oracle: LCM V1: %w", err)
		}
		raw, err := v1.TxProcessing()
		if err != nil {
			return nil, fmt.Errorf("oracle: V1 TxProcessing: %w", err)
		}
		return oracleTxProcessingMeta(raw.All()), nil
	case 2:
		v2, err := lcm.ArmV2()
		if err != nil {
			return nil, fmt.Errorf("oracle: LCM V2: %w", err)
		}
		raw, err := v2.TxProcessing()
		if err != nil {
			return nil, fmt.Errorf("oracle: V2 TxProcessing: %w", err)
		}
		return oracleTxProcessingMetaV1(raw.All()), nil
	default:
		return nil, fmt.Errorf("oracle: unknown LCM V=%d", disc)
	}
}

func oracleTxHashes(parts oracleTxParts) (h, inner xdr.Hash, feeBump bool, err error) {
	fail := func(err error) (xdr.Hash, xdr.Hash, bool, error) {
		return xdr.Hash{}, xdr.Hash{}, false, fmt.Errorf("oracle: tx hashes: %w", err)
	}
	hv, err := parts.Result.TransactionHash()
	if err != nil {
		return fail(err)
	}
	if h, err = hv.Value(); err != nil {
		return fail(err)
	}
	rv, err := parts.Result.Result()
	if err != nil {
		return fail(err)
	}
	res, err := rv.Result()
	if err != nil {
		return fail(err)
	}
	code, err := res.Code()
	if err != nil {
		return fail(err)
	}
	switch code {
	case xdr.TransactionResultCodeTxFeeBumpInnerSuccess,
		xdr.TransactionResultCodeTxFeeBumpInnerFailed:
		pair, err := res.ArmInnerResultPair()
		if err != nil {
			return fail(err)
		}
		ihv, err := pair.TransactionHash()
		if err != nil {
			return fail(err)
		}
		if inner, err = ihv.Value(); err != nil {
			return fail(err)
		}
		feeBump = true
	}
	return h, inner, feeBump, nil
}

// oracleMetaEvents is the oracle's analog of both txMetaEvents and the diag
// return of metaEventRaws, so one oracle walk serves both differential arms.
type oracleMetaEvents struct {
	v                 int32
	transactionEvents [][]byte
	operationEvents   [][][]byte
	diag              [][]byte
}

func oracleMetaEventRaws(mv xdr.TransactionMetaView, wantEvents, wantDiag bool) (oracleMetaEvents, error) {
	v, err := mv.V()
	if err != nil {
		return oracleMetaEvents{}, fmt.Errorf("oracle: meta.V: %w", err)
	}
	out := oracleMetaEvents{v: v, transactionEvents: [][]byte{}, operationEvents: [][][]byte{}, diag: [][]byte{}}
	switch v {
	case 0, 1, 2:
	case 3:
		err = func() error {
			v3, err := mv.ArmV3()
			if err != nil {
				return err
			}
			smOpt, err := v3.SorobanMeta()
			if err != nil {
				return err
			}
			sm, present, err := smOpt.Unwrap()
			if err != nil {
				return err
			}
			if !present {
				return nil
			}
			if wantEvents {
				evs, err := sm.Events()
				if err != nil {
					return err
				}
				raws, err := oracleCollectRaws(evs.All())
				if err != nil {
					return err
				}
				out.operationEvents = [][][]byte{raws}
			}
			if wantDiag {
				des, err := sm.DiagnosticEvents()
				if err != nil {
					return err
				}
				if out.diag, err = oracleCollectRaws(des.All()); err != nil {
					return err
				}
			}
			return nil
		}()
	case 4:
		err = func() error {
			v4, err := mv.ArmV4()
			if err != nil {
				return err
			}
			if wantEvents {
				evs, err := v4.Events()
				if err != nil {
					return err
				}
				if out.transactionEvents, err = oracleCollectRaws(evs.All()); err != nil {
					return err
				}
				ops, err := v4.Operations()
				if err != nil {
					return err
				}
				count, err := ops.Len()
				if err != nil {
					return err
				}
				opEventRaws := make([][][]byte, 0, count)
				for op, err := range ops.All() {
					if err != nil {
						return err
					}
					opEvs, err := op.Events()
					if err != nil {
						return err
					}
					raws, err := oracleCollectRaws(opEvs.All())
					if err != nil {
						return err
					}
					opEventRaws = append(opEventRaws, raws)
				}
				out.operationEvents = opEventRaws
			}
			if wantDiag {
				des, err := v4.DiagnosticEvents()
				if err != nil {
					return err
				}
				if out.diag, err = oracleCollectRaws(des.All()); err != nil {
					return err
				}
			}
			return nil
		}()
	default:
		return oracleMetaEvents{v: v}, fmt.Errorf("oracle: unsupported TransactionMeta V=%d", v)
	}
	if err != nil {
		return oracleMetaEvents{v: v}, err
	}
	return out, nil
}

func oracleCollectRaws[V interface{ Raw() ([]byte, error) }](it iter.Seq2[V, error]) ([][]byte, error) {
	out := [][]byte{}
	for ev, err := range it {
		if err != nil {
			return nil, err
		}
		raw, rerr := ev.Raw()
		if rerr != nil {
			return nil, rerr
		}
		out = append(out, raw)
	}
	return out, nil
}

func oracleExtractLedgerEvents(lcm xdr.LedgerCloseMetaView) ([]LedgerTransactionEvents, error) {
	tp, err := oracleDispatchTP(lcm)
	if err != nil {
		return nil, err
	}
	var out []LedgerTransactionEvents
	for parts, iterErr := range tp {
		if iterErr != nil {
			return nil, fmt.Errorf("oracle: TxProcessing iter: %w", iterErr)
		}
		h, inner, feeBump, herr := oracleTxHashes(parts)
		if herr != nil {
			return nil, herr
		}
		mev, terr := oracleMetaEventRaws(parts.TxApplyProcessing, true, false)
		if terr != nil {
			return nil, terr
		}
		out = append(out, LedgerTransactionEvents{
			Hash:              [32]byte(h),
			InnerHash:         [32]byte(inner),
			FeeBump:           feeBump,
			TransactionEvents: mev.transactionEvents,
			OperationEvents:   mev.operationEvents,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Differential assertions.
// ---------------------------------------------------------------------------

type eventsExtractor func(xdr.LedgerCloseMetaView) ([]LedgerTransactionEvents, error)

// streamCollect adapts the streaming extractor to the harness's slice shape
// (the collect loop every slice-wanting caller writes).
func streamCollect(lcm xdr.LedgerCloseMetaView) ([]LedgerTransactionEvents, error) {
	var out []LedgerTransactionEvents
	err := StreamLedgerEvents(lcm,
		func(n int) error {
			if n > 0 {
				out = make([]LedgerTransactionEvents, 0, n)
			}
			return nil
		},
		func(_ int, ev LedgerTransactionEvents) error {
			out = append(out, ev)
			return nil
		})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// diffRawSpine requires a and b to be the same [][]byte: nil-ness (the
// empty-not-nil contract), length, and every element aliasing the identical
// bytes — same base pointer, same length. Byte equality follows from pointer
// equality; comparing pointers is the stronger claim (zero-copy preserved).
func diffRawSpine(a, b [][]byte) error {
	if (a == nil) != (b == nil) {
		return fmt.Errorf("nil-ness diverges: a==nil %v, b==nil %v", a == nil, b == nil)
	}
	if len(a) != len(b) {
		return fmt.Errorf("len diverges: %d vs %d", len(a), len(b))
	}
	for j := range a {
		if (a[j] == nil) != (b[j] == nil) {
			return fmt.Errorf("[%d] nil-ness diverges", j)
		}
		if len(a[j]) != len(b[j]) {
			return fmt.Errorf("[%d] len diverges: %d vs %d", j, len(a[j]), len(b[j]))
		}
		if len(a[j]) != 0 && unsafe.SliceData(a[j]) != unsafe.SliceData(b[j]) {
			return fmt.Errorf("[%d] base pointer diverges (not the same buffer bytes)", j)
		}
	}
	return nil
}

// diffCompareExtractors runs both extractors on raw and requires full
// agreement: error parity (both fail or both succeed; error text is free to
// differ), and on success identical Hash/InnerHash/FeeBump plus
// pointer-identical raw event slices with spine nil-ness preserved.
func diffCompareExtractors(raw []byte, a, b eventsExtractor) error {
	av, aerr := a(xdr.ParseLedgerCloseMetaView(raw))
	bv, berr := b(xdr.ParseLedgerCloseMetaView(raw))
	if (aerr != nil) != (berr != nil) {
		return fmt.Errorf("error parity diverges: a=%v, b=%v", aerr, berr)
	}
	if aerr != nil {
		return nil
	}
	if (av == nil) != (bv == nil) {
		return fmt.Errorf("top-level nil-ness diverges: a==nil %v, b==nil %v", av == nil, bv == nil)
	}
	if len(av) != len(bv) {
		return fmt.Errorf("tx count diverges: %d vs %d", len(av), len(bv))
	}
	for i := range av {
		if av[i].Hash != bv[i].Hash {
			return fmt.Errorf("tx %d: Hash diverges", i)
		}
		if av[i].InnerHash != bv[i].InnerHash {
			return fmt.Errorf("tx %d: InnerHash diverges", i)
		}
		if av[i].FeeBump != bv[i].FeeBump {
			return fmt.Errorf("tx %d: FeeBump diverges", i)
		}
		if err := diffRawSpine(av[i].TransactionEvents, bv[i].TransactionEvents); err != nil {
			return fmt.Errorf("tx %d: TransactionEvents: %w", i, err)
		}
		if (av[i].OperationEvents == nil) != (bv[i].OperationEvents == nil) {
			return fmt.Errorf("tx %d: OperationEvents nil-ness diverges", i)
		}
		if len(av[i].OperationEvents) != len(bv[i].OperationEvents) {
			return fmt.Errorf("tx %d: OperationEvents op count diverges: %d vs %d",
				i, len(av[i].OperationEvents), len(bv[i].OperationEvents))
		}
		for op := range av[i].OperationEvents {
			if err := diffRawSpine(av[i].OperationEvents[op], bv[i].OperationEvents[op]); err != nil {
				return fmt.Errorf("tx %d: OperationEvents[%d]: %w", i, op, err)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Fixture corpus.
// ---------------------------------------------------------------------------

type diffFixture struct {
	name string
	raw  []byte
	// wellFormed fixtures must extract without error — guards against corpus
	// rot silently degrading every differential into error-vs-error.
	wellFormed bool
	// sweep includes the fixture in the truncation sweep (the real-ledger
	// fixture is excluded: its O(n²) sweep dwarfs the value the crafted and
	// randxdr shapes already provide; the fuzzer truncates it instead).
	sweep bool
}

func diffLedgerEntryChanges(t testing.TB, n int) xdr.LedgerEntryChanges {
	t.Helper()
	out := make(xdr.LedgerEntryChanges, n)
	for i := range out {
		aid := xdr.MustAddress(keypair.MustRandom().Address())
		out[i] = xdr.LedgerEntryChange{
			Type:    xdr.LedgerEntryChangeTypeLedgerEntryRemoved,
			Removed: &xdr.LedgerKey{Type: xdr.LedgerEntryTypeAccount, Account: &xdr.LedgerKeyAccount{AccountId: aid}},
		}
	}
	return out
}

// diffMetaV4Full exercises every V4 field the walk must size past or drain:
// non-empty TxChangesBefore/After, per-op Changes + Events (including an op
// with zero events), a present SorobanMeta (sized past, between TxChangesAfter
// and Events), top-level events at all three stages, and diagnostics.
func diffMetaV4Full(t testing.TB) xdr.TransactionMeta {
	t.Helper()
	rv := xdr.ScVal{Type: xdr.ScValTypeScvVoid}
	return xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
		TxChangesBefore: diffLedgerEntryChanges(t, 1),
		Operations: []xdr.OperationMetaV2{
			{Changes: diffLedgerEntryChanges(t, 2), Events: []xdr.ContractEvent{vContractEvent("op0a"), vContractEvent("op0b")}},
			{Events: []xdr.ContractEvent{}},
			{Events: []xdr.ContractEvent{vContractEvent("op2")}},
		},
		TxChangesAfter: diffLedgerEntryChanges(t, 1),
		SorobanMeta:    &xdr.SorobanTransactionMetaV2{ReturnValue: &rv},
		Events: []xdr.TransactionEvent{
			{Stage: xdr.TransactionEventStageTransactionEventStageBeforeAllTxs, Event: vContractEvent("before")},
			{Stage: xdr.TransactionEventStageTransactionEventStageAfterTx, Event: vContractEvent("aftertx")},
			{Stage: xdr.TransactionEventStageTransactionEventStageAfterAllTxs, Event: vContractEvent("afterall")},
		},
		DiagnosticEvents: []xdr.DiagnosticEvent{vDiagEvent("d1"), vDiagEvent("d2")},
	}}
}

// diffMetaMatrix is one tx per meta version V0–V4 — combined with each LCM
// version it covers the full LCM×meta matrix, V3 explicitly (both SorobanMeta
// arms), not fuzz-only.
func diffMetaMatrix(t testing.TB) []xdr.TransactionMeta {
	t.Helper()
	return []xdr.TransactionMeta{
		{V: 0, Operations: &[]xdr.OperationMeta{{Changes: diffLedgerEntryChanges(t, 1)}}},
		{V: 1, V1: &xdr.TransactionMetaV1{TxChanges: diffLedgerEntryChanges(t, 1), Operations: []xdr.OperationMeta{{}}}},
		{V: 2, V2: &xdr.TransactionMetaV2{
			TxChangesBefore: diffLedgerEntryChanges(t, 1),
			Operations:      []xdr.OperationMeta{{Changes: diffLedgerEntryChanges(t, 1)}},
			TxChangesAfter:  diffLedgerEntryChanges(t, 2),
		}},
		{V: 3, V3: &xdr.TransactionMetaV3{ // SorobanMeta absent
			TxChangesBefore: diffLedgerEntryChanges(t, 1),
			Operations:      []xdr.OperationMeta{{}},
			TxChangesAfter:  diffLedgerEntryChanges(t, 1),
		}},
		vMetaV3SorobanWithDiag(
			[]xdr.ContractEvent{vContractEvent("m3a"), vContractEvent("m3b")},
			[]xdr.DiagnosticEvent{vDiagEvent("m3d")},
		),
		diffMetaV4Full(t),
	}
}

// diffLCM builds an LCM of the given version whose txs carry the given metas,
// then stamps non-empty fee-processing changes on every TxProcessing entry —
// FeeProcessing on all versions, PostTxApplyFeeProcessing on V2's
// TransactionResultMetaV1 (the trailing field a short element advance would
// silently swallow).
func diffLCM(t testing.TB, version int32, metas []xdr.TransactionMeta) []byte {
	t.Helper()
	txs := make([]txWithHash, 0, len(metas))
	for _, meta := range metas {
		tx := sorobanTx(t, "diff")
		tx.meta = meta
		txs = append(txs, tx)
	}
	lcm := buildLCM(t, version, 9000+uint32(version), 1_700_100_000, txs, false)
	fee := diffLedgerEntryChanges(t, 1)
	postFee := diffLedgerEntryChanges(t, 2)
	switch version {
	case 0:
		for i := range lcm.V0.TxProcessing {
			lcm.V0.TxProcessing[i].FeeProcessing = fee
		}
	case 1:
		for i := range lcm.V1.TxProcessing {
			lcm.V1.TxProcessing[i].FeeProcessing = fee
		}
	case 2:
		for i := range lcm.V2.TxProcessing {
			lcm.V2.TxProcessing[i].FeeProcessing = fee
			lcm.V2.TxProcessing[i].PostTxApplyFeeProcessing = postFee
		}
	}
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	return raw
}

func diffMarshalLCM(t testing.TB, lcm xdr.LedgerCloseMeta) []byte {
	t.Helper()
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	return raw
}

// loadRealLedgerIfPresent returns the captured pubnet ledger, or nil when the
// testdata is absent (the corpus then simply lacks the production-size entry;
// the external-package equivalence tests fail loudly on the same path).
func loadRealLedgerIfPresent() []byte {
	raw, err := os.ReadFile("../xdr/testdata/ledger_58752000.bin")
	if err != nil {
		return nil
	}
	return raw
}

func differentialCorpus(t testing.TB) []diffFixture {
	t.Helper()
	var out []diffFixture
	crafted := func(name string, raw []byte) {
		out = append(out, diffFixture{name: name, raw: raw, wellFormed: true, sweep: true})
	}

	// LCM V0/V1/V2 × meta V0–V4, with fee processing populated per version.
	for _, v := range []int32{0, 1, 2} {
		crafted(fmt.Sprintf("lcmV%d/meta-matrix", v), diffLCM(t, v, diffMetaMatrix(t)))
		crafted(fmt.Sprintf("lcmV%d/empty-ledger", v), diffMarshalLCM(t, buildLCM(t, v, 9100+uint32(v), 1_700_100_100, nil, false)))
	}

	// Fee-bump txs (outer hash + inner hash from the result) over V3 and V4 metas.
	crafted("fee-bump-v3-v4", diffMarshalLCM(t, buildLCM(t, 2, 9200, 1_700_100_200, []txWithHash{
		feeBumpTx(t, vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("fb3")})),
		feeBumpTx(t, diffMetaV4Full(t)),
	}, false)))

	// Zero-op tx: V4 meta with no operations, no events, no diagnostics.
	crafted("v4-zero-op", diffLCM(t, 2, []xdr.TransactionMeta{
		{V: 4, V4: &xdr.TransactionMetaV4{}},
	}))

	// Every op-events group empty (empty-not-nil at the innermost spine).
	crafted("v4-empty-op-events", diffLCM(t, 2, []xdr.TransactionMeta{
		vMetaV4OpEvents([][]xdr.ContractEvent{{}, {}, {}}),
	}))

	// Multi-op V4 with per-op events, plus a staged-tx-events-only tx and a
	// diagnostics-heavy tx in the same ledger.
	crafted("v4-multi-op-staged-diag", diffLCM(t, 2, []xdr.TransactionMeta{
		vMetaV4OpEvents([][]xdr.ContractEvent{
			{vContractEvent("a1"), vContractEvent("a2")},
			{vContractEvent("b1")},
			{},
			{vContractEvent("d1"), vContractEvent("d2"), vContractEvent("d3")},
		}),
		txMetaWithStagedEvents([]xdr.TransactionEvent{
			{Stage: xdr.TransactionEventStageTransactionEventStageBeforeAllTxs, Event: vContractEvent("s0")},
			{Stage: xdr.TransactionEventStageTransactionEventStageAfterTx, Event: vContractEvent("s1")},
			{Stage: xdr.TransactionEventStageTransactionEventStageAfterAllTxs, Event: vContractEvent("s2")},
		}),
		{V: 4, V4: &xdr.TransactionMetaV4{DiagnosticEvents: []xdr.DiagnosticEvent{vDiagEvent("dg")}}},
	}))

	// V3 SorobanMeta arms: absent, present-with-empty-events, present-with-
	// events-and-diagnostics.
	crafted("v3-soroban-arms", diffLCM(t, 1, []xdr.TransactionMeta{
		{V: 3, V3: &xdr.TransactionMetaV3{}},
		vMetaV3Soroban([]xdr.ContractEvent{}),
		vMetaV3SorobanWithDiag(
			[]xdr.ContractEvent{vContractEvent("v3e")},
			[]xdr.DiagnosticEvent{vDiagEvent("v3d1"), vDiagEvent("v3d2")},
		),
	}))

	// Stage-0 family (a): V3 meta with ZERO operations but SorobanMeta PRESENT
	// — the one-group-despite-no-ops shape (a draft visitor collector crashed
	// on it: it derived the group spine from the op count). Explicit empty
	// Operations plus non-empty changes on both sides so the soroban unwrap
	// sits between real structure; covered with events, and with an empty
	// events array (group must still exist, empty).
	crafted("v3-zero-ops-soroban-present", diffLCM(t, 1, []xdr.TransactionMeta{
		{V: 3, V3: &xdr.TransactionMetaV3{
			TxChangesBefore: diffLedgerEntryChanges(t, 1),
			Operations:      []xdr.OperationMeta{},
			TxChangesAfter:  diffLedgerEntryChanges(t, 1),
			SorobanMeta: &xdr.SorobanTransactionMeta{
				Events:      []xdr.ContractEvent{vContractEvent("z1"), vContractEvent("z2")},
				ReturnValue: xdr.ScVal{Type: xdr.ScValTypeScvVoid},
			},
		}},
		{V: 3, V3: &xdr.TransactionMetaV3{
			Operations: []xdr.OperationMeta{},
			SorobanMeta: &xdr.SorobanTransactionMeta{
				Events:      []xdr.ContractEvent{},
				ReturnValue: xdr.ScVal{Type: xdr.ScValTypeScvVoid},
			},
		}},
	}))

	// Stage-0 family (b): V4 with zero-event ops INTERLEAVED among
	// event-bearing ops — leading, middle, and trailing empty groups (a
	// trailing one is what a short group loop silently drops).
	crafted("v4-zero-event-ops-interleaved", diffLCM(t, 2, []xdr.TransactionMeta{
		vMetaV4OpEvents([][]xdr.ContractEvent{
			{},
			{vContractEvent("i1")},
			{},
			{vContractEvent("i2a"), vContractEvent("i2b")},
			{},
		}),
		vMetaV4OpEvents([][]xdr.ContractEvent{
			{vContractEvent("j1")},
			{},
		}),
	}))

	// Stage-0 family (c): V0/V1/V2 meta spine shapes across every LCM
	// version: eventless metas in all their zero shapes (nil-vs-empty
	// operations, zero changes) mixed with a V4 whose TRAILING op has zero
	// events — the output spines must come out empty-not-nil, with the V4
	// group count preserved, identically to the oracle.
	spineMetas := func() []xdr.TransactionMeta {
		return []xdr.TransactionMeta{
			{V: 0, Operations: &[]xdr.OperationMeta{}},
			{V: 1, V1: &xdr.TransactionMetaV1{Operations: []xdr.OperationMeta{}}},
			{V: 2, V2: &xdr.TransactionMetaV2{
				Operations:     []xdr.OperationMeta{{}},
				TxChangesAfter: xdr.LedgerEntryChanges{},
			}},
			vMetaV4OpEvents([][]xdr.ContractEvent{{vContractEvent("t0")}, {}}),
		}
	}
	for _, v := range []int32{0, 1, 2} {
		crafted(fmt.Sprintf("lcmV%d/spine-shapes", v), diffLCM(t, v, spineMetas()))
	}

	// randxdr shapes: schema-valid, generator-collapsed recursion, fixed seed.
	gen := randxdr.NewGenerator()
	for i := 0; i < 6; i++ {
		shape := &gxdr.LedgerCloseMeta{}
		gen.Next(shape, randxdr.LedgerCloseMetaPresets)
		var lcm xdr.LedgerCloseMeta
		if err := gxdr.Convert(shape, &lcm); err != nil {
			continue
		}
		raw, err := lcm.MarshalBinary()
		if err != nil {
			continue
		}
		out = append(out, diffFixture{name: fmt.Sprintf("randxdr-%d", i), raw: raw, wellFormed: true, sweep: true})
	}

	// A real pubnet ledger at production size, when the testdata is present.
	if raw := loadRealLedgerIfPresent(); raw != nil {
		out = append(out, diffFixture{name: "real-ledger", raw: raw, wellFormed: true, sweep: false})
	}
	return out
}

// ---------------------------------------------------------------------------
// Harness tests. At Stage 0 the live extractor and the oracle are the same
// code (self-check); from Stage 1a on, these gate the rewrite.
// ---------------------------------------------------------------------------

func TestExtractLedgerEvents_DifferentialCorpus(t *testing.T) {
	for _, fx := range differentialCorpus(t) {
		t.Run(fx.name, func(t *testing.T) {
			if fx.wellFormed {
				_, err := streamCollect(xdr.ParseLedgerCloseMetaView(fx.raw))
				require.NoError(t, err, "well-formed fixture must extract cleanly")
			}
			require.NoError(t, diffCompareExtractors(fx.raw, oracleExtractLedgerEvents, streamCollect))
		})
	}
}

// TestDifferentialCorpus_CoverageInventory pins that the corpus actually
// contains the shapes the harness claims to cover — a fixture edit that
// silently drops (say) the only fee-bump would otherwise leave that axis
// untested with every differential still green.
func TestDifferentialCorpus_CoverageInventory(t *testing.T) {
	var feeBumps, txEvents, opEvents, emptyOpGroups, txCount int
	for _, fx := range differentialCorpus(t) {
		evs, err := oracleExtractLedgerEvents(xdr.ParseLedgerCloseMetaView(fx.raw))
		require.NoError(t, err, fx.name)
		for _, ev := range evs {
			txCount++
			if ev.FeeBump {
				feeBumps++
			}
			txEvents += len(ev.TransactionEvents)
			for _, op := range ev.OperationEvents {
				opEvents += len(op)
				if len(op) == 0 {
					emptyOpGroups++
				}
			}
		}
	}
	require.NotZero(t, txCount, "corpus has transactions")
	require.NotZero(t, feeBumps, "corpus has fee-bump txs")
	require.NotZero(t, txEvents, "corpus has V4 top-level transaction events")
	require.NotZero(t, opEvents, "corpus has per-op contract events")
	require.NotZero(t, emptyOpGroups, "corpus has ops with empty events")
}

// TestExtractLedgerEvents_TruncationSweep runs the differential on byte
// prefixes of every sweep fixture: both extractors must fail together or
// succeed with identical output at every prefix. This is the gate that
// catches a short element advance (e.g. a missed trailing
// PostTxApplyFeeProcessing) that a full-buffer comparison cannot see.
func TestExtractLedgerEvents_TruncationSweep(t *testing.T) {
	for _, fx := range differentialCorpus(t) {
		if !fx.sweep {
			continue
		}
		t.Run(fx.name, func(t *testing.T) {
			// Every byte prefix; large fixtures beyond 8KiB stride 4-aligned
			// (XDR item boundaries) to bound the O(n²) sweep.
			step := 1
			if len(fx.raw) > 8192 {
				step = ((len(fx.raw) / 8192) + 1) * 4
			}
			for n := 0; n <= len(fx.raw); n += step {
				if err := diffCompareExtractors(fx.raw[:n:n], oracleExtractLedgerEvents, streamCollect); err != nil {
					t.Fatalf("prefix %d of %d: %v", n, len(fx.raw), err)
				}
			}
			if len(fx.raw)%step != 0 {
				require.NoError(t, diffCompareExtractors(fx.raw, oracleExtractLedgerEvents, streamCollect))
			}
		})
	}
}

// TestMetaEventRaws_DifferentialCorpus compares the shared version-dispatched
// meta walk (both arms: contract events AND diagnostics) against the oracle,
// per transaction. ExtractLedgerEvents never reads the diag arm, so this is
// what keeps the read path's diagnostics gated when the walk is rewritten.
func TestMetaEventRaws_DifferentialCorpus(t *testing.T) {
	for _, fx := range differentialCorpus(t) {
		t.Run(fx.name, func(t *testing.T) {
			tp, err := oracleDispatchTP(xdr.ParseLedgerCloseMetaView(fx.raw))
			require.NoError(t, err)
			i := 0
			for parts, iterErr := range tp {
				require.NoError(t, iterErr)
				want, werr := oracleMetaEventRaws(parts.TxApplyProcessing, true, true)
				v, tev, diag, gerr := metaEventRaws(parts.TxApplyProcessing)
				require.Equal(t, werr != nil, gerr != nil, "tx %d: error parity: oracle=%v live=%v", i, werr, gerr)
				if werr != nil {
					continue
				}
				require.Equal(t, want.v, v, "tx %d: meta version", i)
				require.NoError(t, diffRawSpine(want.transactionEvents, tev.TransactionEvents), "tx %d: TransactionEvents", i)
				require.Equal(t, len(want.operationEvents), len(tev.OperationEvents), "tx %d: op group count", i)
				require.Equal(t, want.operationEvents == nil, tev.OperationEvents == nil, "tx %d: op spine nil-ness", i)
				for op := range want.operationEvents {
					require.NoError(t, diffRawSpine(want.operationEvents[op], tev.OperationEvents[op]), "tx %d: OperationEvents[%d]", i, op)
				}
				require.NoError(t, diffRawSpine(want.diag, diag), "tx %d: DiagnosticEvents", i)
				i++
			}
		})
	}
}

// FuzzExtractLedgerEventsDifferential fuzzes the differential itself: on any
// input, live and oracle must fail together or agree exactly. Seeded with the
// crafted corpus, randxdr shapes, deterministic byte-flip mutants of both, and
// the degenerate empty/zero prefixes.
func FuzzExtractLedgerEventsDifferential(f *testing.F) {
	corpus := differentialCorpus(f)
	rng := rand.New(rand.NewSource(7))
	for _, fx := range corpus {
		// Oversized seeds (the real ledger) starve the mutator — a 1.2MB
		// input burns whole fuzz cycles for structure the small crafted
		// seeds already express; the corpus tests still cover it in full.
		if len(fx.raw) == 0 || len(fx.raw) > 1<<16 {
			continue
		}
		f.Add(fx.raw)
		for m := 0; m < 2; m++ {
			mut := append([]byte(nil), fx.raw...)
			for flips := 1 + rng.Intn(4); flips > 0; flips-- {
				mut[rng.Intn(len(mut))] ^= byte(1 + rng.Intn(255))
			}
			f.Add(mut)
		}
	}
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if err := diffCompareExtractors(data, oracleExtractLedgerEvents, streamCollect); err != nil {
			t.Fatal(err)
		}
	})
}
