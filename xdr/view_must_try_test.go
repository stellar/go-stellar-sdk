package xdr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Gated tests for the tier-1 Must*/Try surface: Must panics with the
// *ViewError sentinel; Try/Try0/Try2 recover ONLY that sentinel and re-panic
// everything else untouched. The goroutine caveat (a Must panic in a spawned
// goroutine is not recoverable by the parent's Try) is documented on
// mustView; it cannot be pinned as a test without crashing the process.

func TestTry_RecoversViewErrorSentinel(t *testing.T) {
	// Malformed buffer: Must accessor panics, Try converts to the error.
	v := NewTransactionResultPairView(nil)
	h, err := Try(func() Hash { return v.MustTransactionHash().MustValue() })
	require.Zero(t, h)
	var vErr *ViewError
	require.ErrorAs(t, err, &vErr)
	require.Equal(t, ViewErrShortBuffer, vErr.Kind)

	// Well-formed: values flow through.
	pair := TransactionResultPair{TransactionHash: Hash{7},
		Result: TransactionResult{Result: TransactionResultResult{Code: TransactionResultCodeTxInternalError}}}
	raw, merr := pair.MarshalBinary()
	require.NoError(t, merr)
	h, err = Try(func() Hash { return NewTransactionResultPairView(raw).MustTransactionHash().MustValue() })
	require.NoError(t, err)
	require.Equal(t, Hash{7}, h)
}

func TestTry0_Void(t *testing.T) {
	err := Try0(func() { NewTransactionMetaView(nil).MustV() })
	var vErr *ViewError
	require.ErrorAs(t, err, &vErr)
	require.NoError(t, Try0(func() {}))
}

func TestTry2_Values(t *testing.T) {
	meta := TransactionMeta{V: 3, V3: &TransactionMetaV3{
		SorobanMeta: &SorobanTransactionMeta{ReturnValue: ScVal{Type: ScValTypeScvVoid}},
	}}
	raw, err := meta.MarshalBinary()
	require.NoError(t, err)
	sm, present, err := Try2(func() (SorobanTransactionMetaView, bool) {
		return NewTransactionMetaView(raw).MustArmV3().MustSorobanMeta().MustUnwrap()
	})
	require.NoError(t, err)
	require.True(t, present)
	require.NotNil(t, sm.d)

	_, _, err = Try2(func() (SorobanTransactionMetaView, bool) {
		return NewTransactionMetaView(raw[:5]).MustArmV3().MustSorobanMeta().MustUnwrap()
	})
	var vErr *ViewError
	require.ErrorAs(t, err, &vErr)
}

func TestTry_RepanicsForeignPanics(t *testing.T) {
	// Any non-*ViewError panic value must pass through untouched.
	for _, payload := range []any{"boom", 42, errNotViewError{}} {
		func() {
			defer func() {
				r := recover()
				require.NotNil(t, r, "foreign panic must not be swallowed")
				require.Equal(t, payload, r)
			}()
			_ = Try0(func() { panic(payload) })
			t.Fatal("unreachable: Try0 must re-panic")
		}()
	}
}

// errNotViewError is an error type that is NOT the sentinel: Try must
// re-panic it rather than converting it (only *ViewError is blessed).
type errNotViewError struct{}

func (errNotViewError) Error() string { return "not a view error" }

func TestMustScan_PanicsSentinelInBand(t *testing.T) {
	meta := TransactionMeta{V: 3, V3: &TransactionMetaV3{
		Operations: []OperationMeta{{}, {}},
	}}
	raw, err := meta.MarshalBinary()
	require.NoError(t, err)

	// Well-formed: MustScan steps every element.
	ops, err := NewTransactionMetaView(raw).MustArmV3().Operations()
	require.NoError(t, err)
	n := 0
	for m := ops.MustScan(); m.Next(); {
		n++
	}
	require.Equal(t, 2, n)

	// Truncated mid-array (through the second element): the in-band error
	// becomes a Must panic, recovered by Try0.
	tops, err := NewTransactionMetaView(raw[:22]).MustArmV3().Operations()
	require.NoError(t, err)
	err = Try0(func() {
		for m := tops.MustScan(); m.Next(); {
			_ = n
		}
	})
	var vErr *ViewError
	require.ErrorAs(t, err, &vErr)
}
