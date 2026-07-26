package xdr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestView_LedgerCloseMeta_RoundTrip(t *testing.T) {
	lcm := LedgerCloseMeta{
		V: int32(0),
		V0: &LedgerCloseMetaV0{
			LedgerHeader: LedgerHeaderHistoryEntry{
				Header: LedgerHeader{
					LedgerSeq: 12345,
				},
			},
		},
	}

	data, err := lcm.MarshalBinary()
	require.NoError(t, err)

	view := ParseLedgerCloseMetaView(data)

	raw, err := view.Raw()
	require.NoError(t, err)
	require.Equal(t, len(data), len(raw))

	verVal, err := view.V()
	require.NoError(t, err)
	require.Equal(t, int32(0), verVal)

	v0, err := view.ArmV0()
	require.NoError(t, err)

	f, err := v0.Fields()
	require.NoError(t, err)

	hdr, err := f.LedgerHeader()
	require.NoError(t, err)

	hf, err := hdr.Fields()
	require.NoError(t, err)

	header, err := hf.Header()
	require.NoError(t, err)

	lf, err := header.Fields()
	require.NoError(t, err)

	seq, err := lf.LedgerSeq()
	require.NoError(t, err)
	seqVal, err := seq.Value()
	require.NoError(t, err)
	require.Equal(t, uint32(12345), seqVal)

	// Raw() again after interior consumption must still return the exact input.
	raw2, err := view.Raw()
	require.NoError(t, err)
	require.Equal(t, data, raw2)

	require.NoError(t, view.ValidateFull())

	// The wrong arm must be rejected by its discriminant check.
	_, err = view.ArmV1()
	requireViewErrKind(t, err, ViewErrWrongDiscriminant)
}

func requireViewErrKind(t *testing.T, err error, kind ViewErrorKind) {
	t.Helper()
	var vErr *ViewError
	require.ErrorAs(t, err, &vErr)
	require.Equal(t, kind, vErr.Kind)
}
