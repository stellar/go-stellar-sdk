package xdr

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLedgerCloseMetaViewLedgerCloseTimeMs checks the zero-copy path against
// the parsed one on the same bytes. The two read the union separately, so a
// test that only exercised one would not catch them drifting apart.
func TestLedgerCloseMetaViewLedgerCloseTimeMs(t *testing.T) {
	sig := LedgerCloseValueSignature{
		NodeId:    NodeId{Type: PublicKeyTypePublicKeyTypeEd25519, Ed25519: &Uint256{1, 2, 3}},
		Signature: Signature{4, 5, 6},
	}

	// Deliberately not a multiple of 1000, so a passing test proves the
	// millisecond arm was read rather than closeTime scaled up.
	const msCloseTime = TimePointMs(1594584547123)

	for name, tc := range map[string]struct {
		ext  StellarValueExt
		want int64
	}{
		"BASIC": {
			ext:  StellarValueExt{V: StellarValueTypeStellarValueBasic},
			want: 1594584547000,
		},
		"SIGNED": {
			ext:  StellarValueExt{V: StellarValueTypeStellarValueSigned, LcValueSignature: &sig},
			want: 1594584547000,
		},
		"EMPTY_TX_SET": {
			ext: StellarValueExt{
				V:             StellarValueTypeStellarValueEmptyTxSet,
				ProposedValue: &StellarValueProposedValue{LcValueSignature: sig},
			},
			want: 1594584547000,
		},
		"SIGNED_MS": {
			ext: StellarValueExt{
				V: StellarValueTypeStellarValueSignedMs,
				SignedMsValue: &StellarValueSignedMsValue{
					CloseTimeMs:      msCloseTime,
					LcValueSignature: sig,
				},
			},
			want: 1594584547123,
		},
		"EMPTY_TX_SET_MS": {
			ext: StellarValueExt{
				V: StellarValueTypeStellarValueEmptyTxSetMs,
				ProposedMsValue: &StellarValueProposedMsValue{
					CloseTimeMs:      msCloseTime,
					LcValueSignature: sig,
				},
			},
			want: 1594584547123,
		},
	} {
		t.Run(name, func(t *testing.T) {
			scpValue := StellarValue{CloseTime: TimePoint(1594584547), Ext: tc.ext}
			lcm := LedgerCloseMeta{
				V: 0,
				V0: &LedgerCloseMetaV0{
					LedgerHeader: LedgerHeaderHistoryEntry{
						Header: LedgerHeader{LedgerSeq: 12345, ScpValue: scpValue},
					},
				},
			}

			data, err := lcm.MarshalBinary()
			require.NoError(t, err)

			got, err := LedgerCloseMetaView(data).LedgerCloseTimeMs()
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)

			parsed, err := scpValue.CloseTimeMs()
			assert.NoError(t, err)
			assert.Equal(t, parsed, got, "view path disagrees with the parsed path")

			// Whole-second close time is unchanged on every arm.
			seconds, err := LedgerCloseMetaView(data).LedgerCloseTime()
			assert.NoError(t, err)
			assert.Equal(t, int64(1594584547), seconds)
		})
	}

	// An unknown discriminant cannot be marshalled, so the bytes are built by
	// overwriting the ext discriminant of a valid BASIC ledger in place. This
	// is what a ledger from a future protocol looks like to today's code.
	t.Run("unrecognised arm is an error", func(t *testing.T) {
		lcm := LedgerCloseMeta{
			V: 0,
			V0: &LedgerCloseMetaV0{
				LedgerHeader: LedgerHeaderHistoryEntry{
					Header: LedgerHeader{
						LedgerSeq: 12345,
						ScpValue: StellarValue{
							CloseTime: TimePoint(1594584547),
							Ext:       StellarValueExt{V: StellarValueTypeStellarValueBasic},
						},
					},
				},
			},
		}

		data, err := lcm.MarshalBinary()
		require.NoError(t, err)

		// The ext discriminant is the last 4 bytes of the StellarValue, which
		// is the last field of the header that this ledger encodes before the
		// empty tx set and upgrades.
		basic, err := LedgerCloseMetaView(data).LedgerCloseTimeMs()
		require.NoError(t, err)
		require.Equal(t, int64(1594584547000), basic)

		scpValueBytes, err := lcm.V0.LedgerHeader.Header.ScpValue.MarshalBinary()
		require.NoError(t, err)
		offset := bytes.Index(data, scpValueBytes)
		require.NotEqual(t, -1, offset, "could not locate the encoded ScpValue")

		// ext is the last field of StellarValue and BASIC is void, so the
		// final 4 bytes of the encoded value are the ext discriminant, big
		// endian. Overwriting its low byte is what a ledger from a future
		// protocol looks like to this code.
		corrupted := append([]byte(nil), data...)
		corrupted[offset+len(scpValueBytes)-1] = 99

		_, err = LedgerCloseMetaView(corrupted).LedgerCloseTimeMs()
		assert.Error(t, err)

		var viewErr *ViewError
		if assert.ErrorAs(t, err, &viewErr) {
			assert.Equal(t, ViewErrUnknownDiscriminant, viewErr.Kind)
		}
	})
}
