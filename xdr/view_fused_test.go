package xdr

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"
)

// Property tests for the fused traversal primitives (FieldsFused / SizeFused /
// DrainFused): with zero hooks or size-delegating hooks they must be
// contractually identical to Fields() / size() — same success-or-error on every
// input (including truncated and bit-flipped ones), and identical results on
// success. Lying hooks must yield a bounded *ViewError, never a panic or an
// out-of-extent slice.

// fusedMetaCorpus builds raw TransactionMeta buffers from random
// LedgerCloseMeta shapes (each tx's meta re-marshaled standalone), plus
// deterministic byte-flip mutants. Truncation prefixes are swept by the
// callers per raw.
func fusedMetaCorpus(t *testing.T) [][]byte {
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

// requireSameBundle asserts two Fields bundles are identical: every field
// (all named []byte views) aliasing the same base pointer with the same length.
func requireSameBundle(t *testing.T, a, b any, ctx string) {
	t.Helper()
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	require.Equal(t, va.Type(), vb.Type(), ctx)
	for i := 0; i < va.NumField(); i++ {
		name := va.Type().Field(i).Name
		fa, fb := va.Field(i).Bytes(), vb.Field(i).Bytes()
		require.Equal(t, len(fa), len(fb), "%s: field %s length", ctx, name)
		if len(fa) != 0 {
			require.True(t, unsafe.SliceData(fa) == unsafe.SliceData(fb),
				"%s: field %s base pointer", ctx, name)
		}
	}
}

// delegatingMetaArmHooks returns TransactionMetaArmHooks where every arm hook
// delegates to the arm's own size() — SizeFused with these must be ≡ size().
func delegatingMetaArmHooks() TransactionMetaArmHooks {
	return TransactionMetaArmHooks{
		Operations: func(v TransactionMetaOperationsView, d int) (int, error) { return v.size(d) },
		V1:         func(v TransactionMetaV1View, d int) (int, error) { return v.size(d) },
		V2:         func(v TransactionMetaV2View, d int) (int, error) { return v.size(d) },
		V3:         func(v TransactionMetaV3View, d int) (int, error) { return v.size(d) },
		V4:         func(v TransactionMetaV4View, d int) (int, error) { return v.size(d) },
	}
}

// delegatingV4FieldsHooks returns TransactionMetaV4FieldsHooks where every
// variable-size field hook delegates to that field's size().
func delegatingV4FieldsHooks() TransactionMetaV4FieldsHooks {
	return TransactionMetaV4FieldsHooks{
		TxChangesBefore:  func(v LedgerEntryChangesView, d int) (int, error) { return v.size(d) },
		Operations:       func(v TransactionMetaV4OperationsView, d int) (int, error) { return v.size(d) },
		TxChangesAfter:   func(v LedgerEntryChangesView, d int) (int, error) { return v.size(d) },
		SorobanMeta:      func(v TransactionMetaV4SorobanMetaOptView, d int) (int, error) { return v.size(d) },
		Events:           func(v TransactionMetaV4EventsView, d int) (int, error) { return v.size(d) },
		DiagnosticEvents: func(v TransactionMetaV4DiagnosticEventsView, d int) (int, error) { return v.size(d) },
	}
}

// delegatingV3FieldsHooks is delegatingV4FieldsHooks for TransactionMetaV3.
func delegatingV3FieldsHooks() TransactionMetaV3FieldsHooks {
	return TransactionMetaV3FieldsHooks{
		TxChangesBefore: func(v LedgerEntryChangesView, d int) (int, error) { return v.size(d) },
		Operations:      func(v TransactionMetaV3OperationsView, d int) (int, error) { return v.size(d) },
		TxChangesAfter:  func(v LedgerEntryChangesView, d int) (int, error) { return v.size(d) },
		SorobanMeta:     func(v TransactionMetaV3SorobanMetaOptView, d int) (int, error) { return v.size(d) },
	}
}

// checkSizeFusedEquiv asserts SizeFused ≡ size() on one buffer, for both zero
// hooks and size-delegating hooks.
func checkSizeFusedEquiv(t *testing.T, raw []byte, ctx string) {
	t.Helper()
	mv := TransactionMetaView(raw)
	wantSz, wantErr := mv.size(0)
	for name, hooks := range map[string]TransactionMetaArmHooks{
		"zero":       {},
		"delegating": delegatingMetaArmHooks(),
	} {
		gotSz, gotErr := mv.SizeFused(hooks, 0)
		require.Equal(t, wantErr != nil, gotErr != nil,
			"%s: SizeFused(%s) error parity: size=%v fused=%v", ctx, name, wantErr, gotErr)
		if wantErr == nil {
			require.Equal(t, wantSz, gotSz, "%s: SizeFused(%s) size", ctx, name)
		}
	}
}

// checkFieldsFusedEquiv asserts FieldsFused ≡ Fields() on the meta's V3/V4 arm
// (when selected), for both zero hooks and size-delegating hooks.
func checkFieldsFusedEquiv(t *testing.T, raw []byte, ctx string) {
	t.Helper()
	mv := TransactionMetaView(raw)
	v, err := mv.V()
	if err != nil {
		return
	}
	switch v {
	case 3:
		v3, err := mv.V3()
		if err != nil {
			return
		}
		want, wantErr := v3.Fields()
		for name, hooks := range map[string]TransactionMetaV3FieldsHooks{
			"zero":       {},
			"delegating": delegatingV3FieldsHooks(),
		} {
			got, gotErr := v3.FieldsFused(hooks, 0)
			require.Equal(t, wantErr != nil, gotErr != nil,
				"%s: V3 FieldsFused(%s) error parity: fields=%v fused=%v", ctx, name, wantErr, gotErr)
			if wantErr == nil {
				requireSameBundle(t, want, got, fmt.Sprintf("%s: V3 FieldsFused(%s)", ctx, name))
			}
		}
	case 4:
		v4, err := mv.V4()
		if err != nil {
			return
		}
		want, wantErr := v4.Fields()
		for name, hooks := range map[string]TransactionMetaV4FieldsHooks{
			"zero":       {},
			"delegating": delegatingV4FieldsHooks(),
		} {
			got, gotErr := v4.FieldsFused(hooks, 0)
			require.Equal(t, wantErr != nil, gotErr != nil,
				"%s: V4 FieldsFused(%s) error parity: fields=%v fused=%v", ctx, name, wantErr, gotErr)
			if wantErr == nil {
				requireSameBundle(t, want, got, fmt.Sprintf("%s: V4 FieldsFused(%s)", ctx, name))
			}
		}
	}
}

// checkDrainFusedEquiv asserts DrainFused with a size-delegating callback ≡
// size() on the V4 Operations array (when present), and that the drained
// element extents equal AllRaw's.
func checkDrainFusedEquiv(t *testing.T, raw []byte, ctx string) {
	t.Helper()
	mv := TransactionMetaView(raw)
	if v, err := mv.V(); err != nil || v != 4 {
		return
	}
	v4, err := mv.V4()
	if err != nil {
		return
	}
	f, err := v4.Fields()
	if err != nil {
		return
	}
	ops := f.Operations
	wantSz, wantErr := ops.size(0)
	var extents [][]byte
	gotSz, gotErr := ops.DrainFused(func(i int, elem OperationMetaV2View, d int) (int, error) {
		require.Equal(t, len(extents), i, "%s: DrainFused index order", ctx)
		sz, serr := elem.size(d)
		if serr != nil {
			return 0, serr
		}
		extents = append(extents, []byte(elem)[:sz])
		return sz, nil
	}, 0)
	require.Equal(t, wantErr != nil, gotErr != nil,
		"%s: DrainFused error parity: size=%v fused=%v", ctx, wantErr, gotErr)
	if wantErr != nil {
		return
	}
	require.Equal(t, wantSz, gotSz, "%s: DrainFused total", ctx)
	raws, err := ops.AllRaw()
	require.NoError(t, err, ctx)
	require.Equal(t, len(raws), len(extents), "%s: DrainFused element count", ctx)
	for i := range raws {
		require.Equal(t, len(raws[i]), len(extents[i]), "%s: element %d extent", ctx, i)
		if len(raws[i]) != 0 {
			require.True(t, unsafe.SliceData(raws[i]) == unsafe.SliceData(extents[i]),
				"%s: element %d base pointer", ctx, i)
		}
	}
}

func TestViewFused_PropertyEquivalence(t *testing.T) {
	for idx, raw := range fusedMetaCorpus(t) {
		ctx := fmt.Sprintf("meta[%d]", idx)
		checkSizeFusedEquiv(t, raw, ctx)
		checkFieldsFusedEquiv(t, raw, ctx)
		checkDrainFusedEquiv(t, raw, ctx)
		// Truncation sweep: the same three equivalences on every prefix.
		for n := 0; n < len(raw); n += prefixStride(len(raw)) {
			pctx := fmt.Sprintf("%s/prefix[%d]", ctx, n)
			p := raw[:n:n]
			checkSizeFusedEquiv(t, p, pctx)
			checkFieldsFusedEquiv(t, p, pctx)
			checkDrainFusedEquiv(t, p, pctx)
		}
	}
}

// TestViewFused_FixedElemDrain covers DrainFused's fixed-size-element branch
// via LedgerHeader's Hash skipList[4]: elements arrive trimmed to their exact
// extent and the total equals the array's constant wire size.
func TestViewFused_FixedElemDrain(t *testing.T) {
	var hdr LedgerHeader
	for i := range hdr.SkipList {
		hdr.SkipList[i] = Hash{byte(i + 1)}
	}
	raw, err := hdr.MarshalBinary()
	require.NoError(t, err)
	sl, err := LedgerHeaderView(raw).SkipList()
	require.NoError(t, err)

	var n int
	total, err := sl.DrainFused(func(i int, elem HashView, d int) (int, error) {
		require.Len(t, []byte(elem), 32, "element %d must be trimmed", i)
		require.Equal(t, byte(i+1), elem[0])
		n++
		return len(elem), nil
	}, 0)
	require.NoError(t, err)
	require.Equal(t, 4, n)
	require.Equal(t, 128, total)

	// A lying advance is caught by the bounds check on the LAST element (the
	// earlier ones stay in bounds on the fat view) — never unsafety.
	_, err = sl.DrainFused(func(i int, elem HashView, d int) (int, error) {
		return len(raw), nil
	}, 0)
	var verr *ViewError
	require.ErrorAs(t, err, &verr)
}

// TestViewFused_LyingHooks pins the bounded-failure contract: hooks returning
// negative or beyond-buffer advances yield *ViewError, never a panic.
func TestViewFused_LyingHooks(t *testing.T) {
	rv := ScVal{Type: ScValTypeScvVoid}
	meta := TransactionMeta{V: 4, V4: &TransactionMetaV4{
		Operations:  []OperationMetaV2{{}},
		SorobanMeta: &SorobanTransactionMetaV2{ReturnValue: &rv},
	}}
	raw, err := meta.MarshalBinary()
	require.NoError(t, err)
	mv := TransactionMetaView(raw)
	v4 := mv.MustV4()

	for name, adv := range map[string]int{"negative": -8, "huge": 1 << 30} {
		t.Run(name, func(t *testing.T) {
			var verr *ViewError

			_, err := v4.FieldsFused(TransactionMetaV4FieldsHooks{
				Operations: func(TransactionMetaV4OperationsView, int) (int, error) { return adv, nil },
			}, 0)
			require.ErrorAs(t, err, &verr, "FieldsFused")

			_, err = mv.SizeFused(TransactionMetaArmHooks{
				V4: func(TransactionMetaV4View, int) (int, error) { return adv, nil },
			}, 0)
			require.ErrorAs(t, err, &verr, "SizeFused")

			f, ferr := v4.Fields()
			require.NoError(t, ferr)
			ops := f.Operations
			_, err = ops.DrainFused(func(int, OperationMetaV2View, int) (int, error) { return adv, nil }, 0)
			require.ErrorAs(t, err, &verr, "DrainFused")
		})
	}
}

// TestViewFused_Unhandled pins SizeFused's unknown-discriminant contract:
// Unhandled's error is returned verbatim; a nil or nil-returning Unhandled
// falls back to the generic unknown-discriminant *ViewError — unknown arms
// always fail.
func TestViewFused_Unhandled(t *testing.T) {
	raw := []byte{0, 0, 0, 99, 1, 2, 3, 4}
	mv := TransactionMetaView(raw)

	sentinel := errors.New("unsupported meta version")
	_, err := mv.SizeFused(TransactionMetaArmHooks{Unhandled: func(disc int32) error {
		require.Equal(t, int32(99), disc)
		return sentinel
	}}, 0)
	require.ErrorIs(t, err, sentinel)

	var verr *ViewError
	_, err = mv.SizeFused(TransactionMetaArmHooks{}, 0)
	require.ErrorAs(t, err, &verr)
	require.Equal(t, ViewErrUnknownDiscriminant, verr.Kind)

	_, err = mv.SizeFused(TransactionMetaArmHooks{Unhandled: func(int32) error { return nil }}, 0)
	require.ErrorAs(t, err, &verr)
	require.Equal(t, ViewErrUnknownDiscriminant, verr.Kind)
}
