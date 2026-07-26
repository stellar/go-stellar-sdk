package xdr

import (
	"fmt"
	"math/rand"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"
)

// Property tests for the walk-based traversal machinery: sizeResume must be
// contractually identical to size() — same success-or-error on every input
// (including truncated and bit-flipped ones), and identical extents on
// success — in every walk state: cold (fresh walk), detached (nil walk), and
// warmed (records left behind by public-API consumption of the interior).
// Inconsistent walk records must be ignored, never trusted: a hostile or
// stale record yields a correct fallback walk, never a panic, a wrong
// extent, or an out-of-extent slice.

// metaCorpus builds raw TransactionMeta buffers from random LedgerCloseMeta
// shapes (each tx's meta re-marshaled standalone), plus deterministic
// byte-flip mutants. Truncation prefixes are swept by the callers per raw.
func metaCorpus(t *testing.T) [][]byte {
	t.Helper()
	gen := randxdr.NewGenerator()
	rng := rand.New(rand.NewSource(11))
	var out [][]byte
	add := func(m TransactionMeta) {
		raw, err := m.MarshalBinary()
		require.NoError(t, err)
		out = append(out, raw)
		// Byte-flip mutants: error-parity coverage on malformed interiors.
		for k := 0; k < 2; k++ {
			mut := append([]byte(nil), raw...)
			for flips := 1 + rng.Intn(3); flips > 0; flips-- {
				mut[rng.Intn(len(mut))] ^= byte(1 + rng.Intn(255))
			}
			out = append(out, mut)
		}
	}
	for i := 0; i < 40; i++ {
		shape := &gxdr.LedgerCloseMeta{}
		gen.Next(shape, randxdr.LedgerCloseMetaPresets)
		var lcm LedgerCloseMeta
		if err := gxdr.Convert(shape, &lcm); err != nil {
			continue
		}
		switch lcm.V {
		case 0:
			for _, tp := range lcm.V0.TxProcessing {
				add(tp.TxApplyProcessing)
			}
		case 1:
			for _, tp := range lcm.V1.TxProcessing {
				add(tp.TxApplyProcessing)
			}
		case 2:
			for _, tp := range lcm.V2.TxProcessing {
				add(tp.TxApplyProcessing)
			}
		}
	}
	require.NotEmpty(t, out, "corpus must contain transaction metas")
	return out
}

// prefixStride returns the truncation-sweep stride for a raw of length n:
// every 4-aligned cut for small inputs, coarser for large ones.
func prefixStride(n int) int {
	if n <= 512 {
		return 4
	}
	return ((n / 512) + 1) * 4
}

// consumeMeta walks a TransactionMetaView's interior through the public
// surface only, ignoring errors — its purpose is to leave whatever walk
// records real consumption leaves. It descends into the V3/V4 arms' bundles
// and drains their event/operation arrays element by element.
func consumeMeta(mv TransactionMetaView) {
	drainChanges := func(c LedgerEntryChangesView) {
		for elem, err := range c.All() {
			if err != nil {
				return
			}
			_, _ = elem.Raw()
		}
	}
	v, err := mv.V()
	if err != nil {
		return
	}
	switch v {
	case 3:
		v3, err := mv.ArmV3()
		if err != nil {
			return
		}
		f, _ := v3.Fields()
		if c, err := f.TxChangesBefore(); err == nil {
			drainChanges(c)
		}
		if ops, err := f.Operations(); err == nil {
			for op, err := range ops.All() {
				if err != nil {
					break
				}
				_, _ = op.Raw()
			}
		}
		if c, err := f.TxChangesAfter(); err == nil {
			drainChanges(c)
		}
		if sm, err := f.SorobanMeta(); err == nil {
			_, _ = sm.Raw()
		}
	case 4:
		v4, err := mv.ArmV4()
		if err != nil {
			return
		}
		f, _ := v4.Fields()
		if c, err := f.TxChangesBefore(); err == nil {
			drainChanges(c)
		}
		if ops, err := f.Operations(); err == nil {
			for op, err := range ops.All() {
				if err != nil {
					break
				}
				of, _ := op.Fields()
				if c, err := of.Changes(); err == nil {
					drainChanges(c)
				}
				if ev, err := of.Events(); err == nil {
					for e, err := range ev.All() {
						if err != nil {
							break
						}
						_, _ = e.Raw()
					}
				}
			}
		}
		if c, err := f.TxChangesAfter(); err == nil {
			drainChanges(c)
		}
		if ev, err := f.Events(); err == nil {
			for e, err := range ev.All() {
				if err != nil {
					break
				}
				_, _ = e.Raw()
			}
		}
		if de, err := f.DiagnosticEvents(); err == nil {
			for e, err := range de.All() {
				if err != nil {
					break
				}
				_, _ = e.Raw()
			}
		}
	default:
		if v >= 0 && v <= 2 {
			// Shallow consumption for the legacy arms: Raw() sizes the whole node.
			_, _ = mv.Raw()
		}
	}
}

// checkSizeResumeEquiv asserts sizeResume ≡ size() on one buffer in every
// walk state: cold walk, nil walk, and a walk warmed by public consumption.
func checkSizeResumeEquiv(t *testing.T, raw []byte, ctx string) {
	t.Helper()
	wantSz, wantErr := TransactionMetaView{view{d: raw}}.size(0)

	states := map[string]TransactionMetaView{
		"cold":     ParseTransactionMetaView(raw),
		"detached": {view{d: raw}},
	}
	warmed := ParseTransactionMetaView(raw)
	consumeMeta(warmed)
	states["warmed"] = warmed

	for name, mv := range states {
		gotSz, gotErr := mv.sizeResume(0)
		require.Equal(t, wantErr != nil, gotErr != nil,
			"%s: sizeResume(%s) error parity: size=%v resume=%v", ctx, name, wantErr, gotErr)
		if wantErr == nil {
			require.Equal(t, wantSz, gotSz, "%s: sizeResume(%s) extent", ctx, name)
		}
		// Raw must agree byte-for-byte with the blind engine in every state.
		gotRaw, gotRawErr := mv.Raw()
		require.Equal(t, wantErr != nil, gotRawErr != nil, "%s: Raw(%s) error parity", ctx, name)
		if wantErr == nil {
			require.Equal(t, wantSz, len(gotRaw), "%s: Raw(%s) length", ctx, name)
			require.True(t, unsafe.SliceData(gotRaw) == unsafe.SliceData(raw),
				"%s: Raw(%s) must alias the input", ctx, name)
		}
	}
}

// v3FieldRaws resolves every TransactionMetaV3 bundle field and returns the
// per-field (raw bytes, error) outcomes, reading fields in the given order.
func v3FieldRaws(v3 TransactionMetaV3View, reverse bool) (raws [][]byte, errs []error) {
	f, _ := v3.Fields()
	get := []func() ([]byte, error){
		func() ([]byte, error) {
			x, err := f.TxChangesBefore()
			if err != nil {
				return nil, err
			}
			return x.Raw()
		},
		func() ([]byte, error) {
			x, err := f.Operations()
			if err != nil {
				return nil, err
			}
			return x.Raw()
		},
		func() ([]byte, error) {
			x, err := f.TxChangesAfter()
			if err != nil {
				return nil, err
			}
			return x.Raw()
		},
		func() ([]byte, error) {
			x, err := f.SorobanMeta()
			if err != nil {
				return nil, err
			}
			return x.Raw()
		},
	}
	raws = make([][]byte, len(get))
	errs = make([]error, len(get))
	idxs := []int{0, 1, 2, 3}
	if reverse {
		idxs = []int{3, 2, 1, 0}
	}
	for _, i := range idxs {
		raws[i], errs[i] = get[i]()
	}
	return raws, errs
}

// checkBundleEquiv asserts the lazy bundle resolves identical field extents
// regardless of access order and walk state, on the meta's V3 arm (when
// selected): in-order with walk ≡ reverse-order with walk ≡ detached.
func checkBundleEquiv(t *testing.T, raw []byte, ctx string) {
	t.Helper()
	mv := ParseTransactionMetaView(raw)
	if v, err := mv.V(); err != nil || v != 3 {
		return
	}
	v3, err := mv.ArmV3()
	if err != nil {
		return
	}
	wantRaws, wantErrs := v3FieldRaws(TransactionMetaV3View{view{d: v3.d}}, false)

	for name, alt := range map[string]func() ([][]byte, []error){
		"walk in-order":      func() ([][]byte, []error) { return v3FieldRaws(v3, false) },
		"walk reverse-order": func() ([][]byte, []error) { return v3FieldRaws(v3, true) },
		"warmed in-order": func() ([][]byte, []error) {
			consumeMeta(mv)
			return v3FieldRaws(v3, false)
		},
	} {
		gotRaws, gotErrs := alt()
		for i := range wantRaws {
			require.Equal(t, wantErrs[i] != nil, gotErrs[i] != nil,
				"%s: %s field %d error parity: want=%v got=%v", ctx, name, i, wantErrs[i], gotErrs[i])
			if wantErrs[i] != nil {
				continue
			}
			require.Equal(t, len(wantRaws[i]), len(gotRaws[i]), "%s: %s field %d extent", ctx, name, i)
			if len(wantRaws[i]) != 0 {
				require.True(t, unsafe.SliceData(wantRaws[i]) == unsafe.SliceData(gotRaws[i]),
					"%s: %s field %d base pointer", ctx, name, i)
			}
		}
	}
}

// checkAllEquiv asserts All() iteration over the V4 Operations array agrees
// with the blind engine: full error-free consumption iff size() succeeds, the
// concatenated element extents equal the array's wire size, and every
// element's Raw() aliases the parent buffer (zero-copy).
func checkAllEquiv(t *testing.T, raw []byte, ctx string) {
	t.Helper()
	mv := ParseTransactionMetaView(raw)
	if v, err := mv.V(); err != nil || v != 4 {
		return
	}
	v4, err := mv.ArmV4()
	if err != nil {
		return
	}
	f, err := v4.Fields()
	if err != nil {
		return
	}
	ops, err := f.Operations()
	if err != nil {
		return
	}
	wantSz, wantErr := TransactionMetaV4OperationsView{view{d: ops.d}}.size(0)

	var total int
	var n uint32
	var iterErr error
	for elem, err := range ops.All() {
		if err != nil {
			iterErr = err
			break
		}
		r, rerr := elem.Raw()
		if rerr != nil {
			iterErr = rerr
			break
		}
		require.True(t, unsafe.SliceData(r) == unsafe.SliceData(elem.d),
			"%s: element %d raw must alias its view", ctx, n)
		total += len(r)
		n++
	}
	require.Equal(t, wantErr != nil, iterErr != nil,
		"%s: All error parity: size=%v iter=%v", ctx, wantErr, iterErr)
	if wantErr != nil {
		return
	}
	require.Equal(t, wantSz, total+4, "%s: sum of element extents + count word", ctx)
	count, err := ops.Len()
	require.NoError(t, err, ctx)
	require.Equal(t, count, n, "%s: element count", ctx)
	// After full consumption the array's own record makes Raw O(1); it must
	// still be the identical trimmed slice.
	r, err := ops.Raw()
	require.NoError(t, err, ctx)
	require.Equal(t, wantSz, len(r), "%s: Raw after All", ctx)
}

func TestViewWalk_PropertyEquivalence(t *testing.T) {
	for idx, raw := range metaCorpus(t) {
		ctx := fmt.Sprintf("meta[%d]", idx)
		checkSizeResumeEquiv(t, raw, ctx)
		checkBundleEquiv(t, raw, ctx)
		checkAllEquiv(t, raw, ctx)
		// Truncation sweep: the same equivalences on every prefix.
		for n := 0; n < len(raw); n += prefixStride(len(raw)) {
			pctx := fmt.Sprintf("%s/prefix[%d]", ctx, n)
			p := raw[:n:n]
			checkSizeResumeEquiv(t, p, pctx)
			checkBundleEquiv(t, p, pctx)
			checkAllEquiv(t, p, pctx)
		}
	}
}

// TestViewWalk_FixedElemAll covers All()'s fixed-size-element branch via
// LedgerHeader's Hash skipList[4]: elements arrive at their exact 32-byte
// stride offsets and the array's constant wire size is 128.
func TestViewWalk_FixedElemAll(t *testing.T) {
	var hdr LedgerHeader
	for i := range hdr.SkipList {
		hdr.SkipList[i] = Hash{byte(i + 1)}
	}
	raw, err := hdr.MarshalBinary()
	require.NoError(t, err)
	f, err := ParseLedgerHeaderView(raw).Fields()
	require.NoError(t, err)
	sl, err := f.SkipList()
	require.NoError(t, err)

	n, err := sl.Len()
	require.NoError(t, err)
	require.Equal(t, uint32(4), n)

	var k int
	for elem, err := range sl.All() {
		require.NoError(t, err)
		r, err := elem.Raw()
		require.NoError(t, err)
		require.Len(t, r, 32, "element %d must trim to its exact extent", k)
		require.Equal(t, byte(k+1), r[0])
		k++
	}
	require.Equal(t, 4, k)

	total, err := sl.Raw()
	require.NoError(t, err)
	require.Len(t, total, 128)

	// Truncated fixed-element arrays yield an in-band error, never a panic.
	trunc := ParseLedgerHeaderView(raw[:len(raw)-8])
	tf, err := trunc.Fields()
	require.NoError(t, err)
	tsl, err := tf.SkipList()
	require.NoError(t, err)
	sawErr := false
	for _, err := range tsl.All() {
		if err != nil {
			sawErr = true
			var vErr *ViewError
			require.ErrorAs(t, err, &vErr)
		}
	}
	require.True(t, sawErr, "truncated skip list must surface an in-band error")
}

// TestViewWalk_InconsistentRecordsIgnored pins the record trust boundary from
// the traversal side (the analog of the old lying-hooks test: the only way
// left to lie to the sizing engine is a bad walk record, and validation must
// reject it): records with a foreign type identity, a mismatched start, or an
// end outside the enclosing buffer must not change any result — sizing falls
// back to the plain walk. A poisoned record never causes a panic or a wrong
// extent.
func TestViewWalk_InconsistentRecordsIgnored(t *testing.T) {
	rv := ScVal{Type: ScValTypeScvVoid}
	meta := TransactionMeta{V: 4, V4: &TransactionMetaV4{
		Operations:  []OperationMetaV2{{}, {}},
		SorobanMeta: &SorobanTransactionMetaV2{ReturnValue: &rv},
	}}
	raw, err := meta.MarshalBinary()
	require.NoError(t, err)

	wantSz, wantErr := TransactionMetaView{view{d: raw}}.size(0)
	require.NoError(t, wantErr)

	poisons := []struct {
		name       string
		tid        int32
		start, end int64
	}{
		{"foreign tid at node start", tidTransactionMetaView + 1, 0, int64(len(raw))},
		{"own tid, foreign start", tidTransactionMetaView, 4, int64(len(raw))},
		{"own tid, end beyond buffer", tidTransactionMetaView, 0, int64(len(raw)) + 64},
		{"own tid, end before start", tidTransactionMetaView, 0, -8},
		{"garbage span", 9999, -3, 1 << 40},
	}
	for _, p := range poisons {
		t.Run(p.name, func(t *testing.T) {
			mv := ParseTransactionMetaView(raw)
			mv.w.record(p.tid, p.start, p.end)
			gotSz, gotErr := mv.sizeResume(0)
			require.NoError(t, gotErr)
			require.Equal(t, wantSz, gotSz)

			mv = ParseTransactionMetaView(raw)
			mv.w.record(p.tid, p.start, p.end)
			r, err := mv.Raw()
			require.NoError(t, err)
			require.Len(t, r, wantSz)
		})
	}
}

// TestViewWalk_AdvanceAfterLeadingFields pins that the O(1) in-order advance
// survives struct elements with fields BEFORE the bulk one — the TxProcessing
// extract shape: TransactionResultMeta{Result, FeeProcessing,
// TxApplyProcessing}. The caller reads Result's hash, then consumes the meta
// (bulk field, wire order last). The iterator advance re-sizes the leading
// fields, and their completion spans must NOT clobber the meta's record
// (walk.record drops left-lying spans); the advance must consume the record —
// observable because it succeeds even after the meta's interior is corrupted,
// where any blind re-walk fails.
func TestViewWalk_AdvanceAfterLeadingFields(t *testing.T) {
	rv := ScVal{Type: ScValTypeScvVoid}
	meta := func() TransactionMeta {
		return TransactionMeta{V: 3, V3: &TransactionMetaV3{
			Operations:  []OperationMeta{{}, {}},
			SorobanMeta: &SorobanTransactionMeta{Events: []ContractEvent{}, ReturnValue: rv},
		}}
	}
	tr := TransactionResult{Result: TransactionResultResult{Code: TransactionResultCodeTxInternalError}}
	lcm := LedgerCloseMeta{
		V: 1,
		V1: &LedgerCloseMetaV1{
			TxSet: GeneralizedTransactionSet{V: 1, V1TxSet: &TransactionSetV1{}},
			TxProcessing: []TransactionResultMeta{
				{Result: TransactionResultPair{Result: tr}, TxApplyProcessing: meta()},
				{Result: TransactionResultPair{Result: tr}, TxApplyProcessing: meta()},
			},
		},
	}
	data, err := lcm.MarshalBinary()
	require.NoError(t, err)

	v1, err := ParseLedgerCloseMetaView(data).ArmV1()
	require.NoError(t, err)
	f, err := v1.Fields()
	require.NoError(t, err)
	tp, err := f.TxProcessing()
	require.NoError(t, err)

	k := 0
	for elem, err := range tp.All() {
		require.NoError(t, err, "element %d", k)
		ef, _ := elem.Fields()
		res, err := ef.Result()
		require.NoError(t, err)
		rf, _ := res.Fields()
		hv, err := rf.TransactionHash()
		require.NoError(t, err)
		_, err = hv.Value()
		require.NoError(t, err)

		m, err := ef.TxApplyProcessing()
		require.NoError(t, err)
		_, err = m.Raw() // consume the meta fully; records its span
		require.NoError(t, err)

		if k == 0 {
			// Corrupt the meta's union discriminant: any re-walk must fail.
			data[m.off] = 0xff
		}
		k++
	}
	require.Equal(t, 2, k,
		"advance after in-order consumption must consume the meta record (corruption invisible)")
}

// TestViewWalk_FrontierMiddleField pins the frontier stack on the V4
// Operations shape: a struct's bulk MIDDLE field is consumed, then FURTHER
// fields are consumed after it (so the single leaf record has moved past the
// middle field), and the enclosing element then advances. Without the
// frontier the advance would blind re-walk the middle field (only the
// most-recently-consumed subtree has a leaf record); with it, the element's
// and arm's bundle-resolve frontier entries restart each field loop at the
// resolved boundary and the middle field is never re-walked — observable
// because the advance succeeds even after the middle field's interior is
// corrupted, where any blind re-walk fails.
func TestViewWalk_FrontierMiddleField(t *testing.T) {
	ev := func(sym string) ContractEvent {
		s := ScSymbol(sym)
		return ContractEvent{
			Type: ContractEventTypeContract,
			Body: ContractEventBody{V: 0, V0: &ContractEventV0{
				Topics: []ScVal{{Type: ScValTypeScvSymbol, Sym: &s}},
				Data:   ScVal{Type: ScValTypeScvVoid},
			}},
		}
	}
	metaV4 := func() TransactionMeta {
		return TransactionMeta{V: 4, V4: &TransactionMetaV4{
			Operations: []OperationMetaV2{
				{Events: []ContractEvent{ev("op0")}},
				{Events: []ContractEvent{ev("op1a"), ev("op1b")}},
			},
			Events: []TransactionEvent{
				{Stage: TransactionEventStageTransactionEventStageAfterTx, Event: ev("top")},
			},
			DiagnosticEvents: []DiagnosticEvent{{InSuccessfulContractCall: true, Event: ev("diag")}},
		}}
	}
	tr := TransactionResult{Result: TransactionResultResult{Code: TransactionResultCodeTxInternalError}}
	lcm := LedgerCloseMeta{
		V: 2,
		V2: &LedgerCloseMetaV2{
			TxSet: GeneralizedTransactionSet{V: 1, V1TxSet: &TransactionSetV1{}},
			TxProcessing: []TransactionResultMetaV1{
				{Result: TransactionResultPair{Result: tr}, TxApplyProcessing: metaV4()},
				{Result: TransactionResultPair{Result: tr}, TxApplyProcessing: metaV4()},
			},
		},
	}
	data, err := lcm.MarshalBinary()
	require.NoError(t, err)

	v2, err := ParseLedgerCloseMetaView(data).ArmV2()
	require.NoError(t, err)
	f, err := v2.Fields()
	require.NoError(t, err)
	tp, err := f.TxProcessing()
	require.NoError(t, err)

	k := 0
	for elem, err := range tp.All() {
		require.NoError(t, err, "element %d", k)
		ef, _ := elem.Fields()
		m, err := ef.TxApplyProcessing()
		require.NoError(t, err)
		v4, err := m.ArmV4()
		require.NoError(t, err)
		mf, _ := v4.Fields()

		// Consume the MIDDLE bulk field (Operations, with per-op descent),
		// remembering the deepest resumable point: the LAST event of the LAST
		// op. Every fallback re-walk variant — full blind size, or resumption
		// from the ops-array / op-struct / op-events frontiers — re-sizes at
		// least that event; only the arm-frontier path that skips Operations
		// entirely never touches it.
		ops, err := mf.Operations()
		require.NoError(t, err)
		var lastEvOff int64
		for op, err := range ops.All() {
			require.NoError(t, err)
			of, _ := op.Fields()
			evs, err := of.Events()
			require.NoError(t, err)
			for e, err := range evs.All() {
				require.NoError(t, err)
				_, err = e.Raw()
				require.NoError(t, err)
				lastEvOff = e.off
			}
		}
		// ...then consume a LATER field (Events), moving the leaf record past
		// Operations; resolving it consumes the ops record into the arm bundle.
		evs, err := mf.Events()
		require.NoError(t, err)
		for e, err := range evs.All() {
			require.NoError(t, err)
			_, err = e.Raw()
			require.NoError(t, err)
		}

		if k == 0 {
			// Corrupt the deepest event's interior (its optional-ContractId
			// presence flag, which sizing validates): any re-walk into
			// Operations must now fail — proven on a detached blind size.
			require.NotZero(t, lastEvOff)
			data[lastEvOff+4] = 0xff
			_, blindErr := TransactionMetaV4OperationsView{view{d: ops.d}}.size(0)
			require.Error(t, blindErr, "corruption must break any blind re-walk of Operations")
		}
		k++
	}
	require.Equal(t, 2, k,
		"advance must resume from bundle frontiers, never re-walking the consumed middle field")
}

// TestViewWalk_FrontierBrokenLoop pins the broken-loop arm of the frontier:
// an All() iteration abandoned mid-array leaves the array resumable at the
// last yielded element, so a later resolution that must size the array (the
// bundle resolving a following field) skips the elements the loop already
// advanced past instead of re-walking them.
func TestViewWalk_FrontierBrokenLoop(t *testing.T) {
	meta := TransactionMeta{V: 3, V3: &TransactionMetaV3{
		Operations: []OperationMeta{
			{Changes: LedgerEntryChanges{}},
			{Changes: LedgerEntryChanges{}},
			{Changes: LedgerEntryChanges{}},
		},
		TxChangesAfter: LedgerEntryChanges{},
	}}
	raw, err := meta.MarshalBinary()
	require.NoError(t, err)

	// Expected layout, measured on a pristine detached walk.
	pf, err := TransactionMetaV3View{view{d: append([]byte(nil), raw...)[4:]}}.Fields()
	require.NoError(t, err)
	pOps, err := pf.Operations()
	require.NoError(t, err)
	pOpsRaw, err := pOps.Raw()
	require.NoError(t, err)

	data := append([]byte(nil), raw...)
	v3, err := ParseTransactionMetaView(data).ArmV3()
	require.NoError(t, err)
	f, err := v3.Fields()
	require.NoError(t, err)
	ops, err := f.Operations()
	require.NoError(t, err)

	// Break the loop at element 2: element 0 is consumed (leaving records the
	// advance path gates on, as any real consumer's reads would), element 1 is
	// advanced past untouched — so no leaf record covers it, only the array's
	// frontier knows it was measured — and element 2 is yielded but untouched.
	seen := 0
	var elem1Off int64
	for elem, err := range ops.All() {
		require.NoError(t, err)
		if seen == 2 {
			break
		}
		if seen == 0 {
			_, err = elem.Raw()
			require.NoError(t, err)
		}
		if seen == 1 {
			elem1Off = elem.off
		}
		seen++
	}
	require.Equal(t, 2, seen)
	require.NotZero(t, elem1Off)

	// Corrupt element 1's interior (its changes count). A blind size over the
	// array must now fail — and so would a leaf-record-only resume, which
	// skips element 0 but still re-sizes element 1...
	data[elem1Off] = 0xff
	_, blindErr := TransactionMetaV3OperationsView{view{d: data[int(ops.off):]}}.size(0)
	require.Error(t, blindErr, "corruption must break the blind walk")

	// ...but resolving the NEXT field sizes the array from the broken loop's
	// frontier (element 2), never re-walking elements 0..1.
	after, err := f.TxChangesAfter()
	require.NoError(t, err, "resolution must resume the broken loop's frontier")
	require.Equal(t, ops.off+int64(len(pOpsRaw)), after.off,
		"frontier resume must place the next field exactly")
}

// TestViewWalk_FrontierPoisonIgnored pins the frontier trust boundary, the
// analog of TestViewWalk_InconsistentRecordsIgnored: entries with a foreign
// tid, a mismatched start, no progress, or an offset outside the node's
// buffer must not change any result — sizing falls back to the plain walk.
func TestViewWalk_FrontierPoisonIgnored(t *testing.T) {
	rv := ScVal{Type: ScValTypeScvVoid}
	meta := TransactionMeta{V: 4, V4: &TransactionMetaV4{
		Operations:  []OperationMetaV2{{}, {}},
		SorobanMeta: &SorobanTransactionMetaV2{ReturnValue: &rv},
	}}
	raw, err := meta.MarshalBinary()
	require.NoError(t, err)

	wantSz, wantErr := TransactionMetaView{view{d: raw}}.size(0)
	require.NoError(t, wantErr)

	// Poison entries target the V4 arm struct (the node whose sizeResume
	// consults the frontier); the arm starts at offset 4, past the union
	// discriminant. Only validation-failing entries are poisonable — an entry
	// passing (tid, start, progress, bounds) validation is trusted, exactly
	// like a leaf record's recEnd.
	armStart := int64(4)
	armLen := int64(len(raw)) - 4
	poisons := []struct {
		name  string
		tid   int32
		start int64
		idx   int32
		off   int64
	}{
		{"foreign tid at arm start", tidTransactionMetaV4View + 1, armStart, 2, 8},
		{"own tid, foreign start", tidTransactionMetaV4View, armStart + 4, 2, 8},
		{"own tid, zero progress", tidTransactionMetaV4View, armStart, 0, 8},
		{"own tid, negative idx", tidTransactionMetaV4View, armStart, -3, 8},
		{"own tid, idx past field count", tidTransactionMetaV4View, armStart, 99, 8},
		{"own tid, off beyond buffer", tidTransactionMetaV4View, armStart, 2, armLen + 64},
		{"own tid, negative off", tidTransactionMetaV4View, armStart, 2, -8},
	}
	for _, p := range poisons {
		t.Run(p.name, func(t *testing.T) {
			mv := ParseTransactionMetaView(raw)
			mv.w.fr[0] = frontierSlot{tid: p.tid, start: p.start, idx: p.idx, off: p.off}
			// Force the arm down its resume path so the poisoned frontier is
			// actually consulted (a record ahead of the arm gates it in).
			mv.w.record(tidTransactionMetaView+1, int64(len(raw))-4, int64(len(raw))-4)
			gotSz, gotErr := mv.sizeResume(0)
			require.NoError(t, gotErr)
			require.Equal(t, wantSz, gotSz)
		})
	}
}

// TestViewWalk_UnknownDiscriminant pins unknown-arm behavior: sizing (blind
// and resumed), validation, and arm entry all fail with the discriminant
// error kinds; the decoded int discriminant itself stays observable so
// callers can implement forward-compatible handling.
func TestViewWalk_UnknownDiscriminant(t *testing.T) {
	raw := []byte{0, 0, 0, 99, 1, 2, 3, 4}
	mv := ParseTransactionMetaView(raw)

	v, err := mv.V()
	require.NoError(t, err)
	require.Equal(t, int32(99), v)

	_, err = mv.sizeResume(0)
	requireViewErrKind(t, err, ViewErrUnknownDiscriminant)
	_, err = TransactionMetaView{view{d: raw}}.size(0)
	requireViewErrKind(t, err, ViewErrUnknownDiscriminant)
	err = mv.ValidateFull()
	requireViewErrKind(t, err, ViewErrUnknownDiscriminant)
	_, err = mv.Raw()
	requireViewErrKind(t, err, ViewErrUnknownDiscriminant)
	_, err = mv.ArmOperations()
	requireViewErrKind(t, err, ViewErrWrongDiscriminant)
}

// TestViewWalk_ResumptionConsumesRecords pins that the record fast paths
// actually fire (the property tests above only prove they never corrupt
// results): after a subtree is fully consumed through the public API, the
// walk record must make the enclosing resolution O(1) — observable because
// it succeeds even after the consumed subtree's interior bytes are corrupted,
// where any blind re-walk fails.
func TestViewWalk_ResumptionConsumesRecords(t *testing.T) {
	meta := TransactionMeta{V: 3, V3: &TransactionMetaV3{
		Operations: []OperationMeta{
			{Changes: LedgerEntryChanges{}},
			{Changes: LedgerEntryChanges{}},
		},
	}}
	raw, err := meta.MarshalBinary()
	require.NoError(t, err)

	// Bundle resolution: resolve the Operations field, drain it (recording the
	// array's span), then corrupt the array's interior. Resolving the NEXT
	// field must consume the record instead of re-walking the corrupted bytes.
	t.Run("bundle field resolution", func(t *testing.T) {
		data := append([]byte(nil), raw...)
		mv := ParseTransactionMetaView(data)
		v3, err := mv.ArmV3()
		require.NoError(t, err)
		f, err := v3.Fields()
		require.NoError(t, err)
		ops, err := f.Operations()
		require.NoError(t, err)
		opsRaw, err := ops.Raw()
		require.NoError(t, err)
		for _, err := range ops.All() {
			require.NoError(t, err)
		}

		// Corrupt the first operation's changes count (inside the ops array).
		opsStart := int(ops.off)
		data[opsStart+4] = 0xff

		// Blind sizing over the corrupted interior must now fail...
		_, blindErr := TransactionMetaV3View{view{d: v3.d}}.size(0)
		require.Error(t, blindErr, "corruption must break the blind walk")
		// ...but the bundle resolves the next field from the record, O(1).
		after, err := f.TxChangesAfter()
		require.NoError(t, err, "resolution past a consumed subtree must use its record")
		require.Equal(t, int64(opsStart+len(opsRaw)), after.off, "record must place the next field exactly")
	})

	// Iterator advance: fully consume element 0's extent inside the loop
	// (recording its span), then corrupt its interior; the advance to element
	// 1 must consume the record instead of re-sizing corrupted bytes.
	t.Run("iterator advance", func(t *testing.T) {
		data := append([]byte(nil), raw...)
		mv := ParseTransactionMetaView(data)
		v3, err := mv.ArmV3()
		require.NoError(t, err)
		f, err := v3.Fields()
		require.NoError(t, err)
		ops, err := f.Operations()
		require.NoError(t, err)

		var starts []int64
		k := 0
		for elem, err := range ops.All() {
			require.NoError(t, err, "element %d", k)
			starts = append(starts, elem.off)
			if k == 0 {
				r, err := elem.Raw() // records element 0's exact span
				require.NoError(t, err)
				data[int(elem.off)] = 0xff // corrupt its interior (changes count)
				_ = r
			}
			k++
		}
		require.Equal(t, 2, k, "both elements must be reached")
		require.Greater(t, starts[1], starts[0])

		// Sanity: with a fresh walk the corrupted element 0 breaks iteration.
		freshV3, err := ParseTransactionMetaView(data).ArmV3()
		require.NoError(t, err)
		freshF, err := freshV3.Fields()
		require.NoError(t, err)
		freshOps, err := freshF.Operations()
		require.NoError(t, err)
		sawErr := false
		for _, err := range freshOps.All() {
			if err != nil {
				sawErr = true
			}
		}
		require.True(t, sawErr, "fresh walk over corrupted bytes must error")
	})
}
