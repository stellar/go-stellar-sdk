package xdr

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStellarValueExtLedgerCloseValueSignature(t *testing.T) {
	sig := LedgerCloseValueSignature{
		NodeId:    NodeId{Type: PublicKeyTypePublicKeyTypeEd25519, Ed25519: &Uint256{1, 2, 3}},
		Signature: Signature{4, 5, 6},
	}

	for name, ext := range map[string]StellarValueExt{
		"SIGNED": {
			V:                StellarValueTypeStellarValueSigned,
			LcValueSignature: &sig,
		},
		"EMPTY_TX_SET": {
			V:             StellarValueTypeStellarValueEmptyTxSet,
			ProposedValue: &StellarValueProposedValue{LcValueSignature: sig},
		},
		"SIGNED_MS": {
			V: StellarValueTypeStellarValueSignedMs,
			SignedMsValue: &StellarValueSignedMsValue{
				CloseTimeMs:      TimePointMs(1594584547000),
				LcValueSignature: sig,
			},
		},
		"EMPTY_TX_SET_MS": {
			V: StellarValueTypeStellarValueEmptyTxSetMs,
			ProposedMsValue: &StellarValueProposedMsValue{
				CloseTimeMs:      TimePointMs(1594584547000),
				LcValueSignature: sig,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := ext.LedgerCloseValueSignature()
			assert.True(t, ok)
			assert.Equal(t, sig, got)
		})
	}

	t.Run("BASIC", func(t *testing.T) {
		_, ok := StellarValueExt{V: StellarValueTypeStellarValueBasic}.LedgerCloseValueSignature()
		assert.False(t, ok)
	})
}

func TestStellarValueCloseTimeMs(t *testing.T) {
	sig := LedgerCloseValueSignature{
		NodeId:    NodeId{Type: PublicKeyTypePublicKeyTypeEd25519, Ed25519: &Uint256{1, 2, 3}},
		Signature: Signature{4, 5, 6},
	}

	// Deliberately not a multiple of 1000, so a test that passes proves the
	// millisecond arm was read rather than closeTime scaled up.
	const closeTimeSeconds = TimePoint(1594584547)
	const closeTimeMs = TimePointMs(1594584547123)

	t.Run("whole second arms scale closeTime", func(t *testing.T) {
		for name, ext := range map[string]StellarValueExt{
			"BASIC":  {V: StellarValueTypeStellarValueBasic},
			"SIGNED": {V: StellarValueTypeStellarValueSigned, LcValueSignature: &sig},
			"EMPTY_TX_SET": {
				V:             StellarValueTypeStellarValueEmptyTxSet,
				ProposedValue: &StellarValueProposedValue{LcValueSignature: sig},
			},
		} {
			t.Run(name, func(t *testing.T) {
				got, err := StellarValue{CloseTime: closeTimeSeconds, Ext: ext}.CloseTimeMs()
				assert.NoError(t, err)
				assert.Equal(t, int64(1594584547000), got)
			})
		}
	})

	t.Run("millisecond arms carry sub-second precision", func(t *testing.T) {
		for name, ext := range map[string]StellarValueExt{
			"SIGNED_MS": {
				V: StellarValueTypeStellarValueSignedMs,
				SignedMsValue: &StellarValueSignedMsValue{
					CloseTimeMs:      closeTimeMs,
					LcValueSignature: sig,
				},
			},
			"EMPTY_TX_SET_MS": {
				V: StellarValueTypeStellarValueEmptyTxSetMs,
				ProposedMsValue: &StellarValueProposedMsValue{
					CloseTimeMs:      closeTimeMs,
					LcValueSignature: sig,
				},
			},
		} {
			t.Run(name, func(t *testing.T) {
				got, err := StellarValue{CloseTime: closeTimeSeconds, Ext: ext}.CloseTimeMs()
				assert.NoError(t, err)
				assert.Equal(t, int64(1594584547123), got)
				assert.NotZero(t, got%1000, "a millisecond arm must not return a scaled whole second")
			})
		}
	})

	// The reason this function returns an error at all. Falling back to
	// closeTime*1000 here would write a plausible wrong value that no consumer
	// could distinguish from a real pre-Protocol-30 close time.
	t.Run("unrecognised arm is an error, not a fallback", func(t *testing.T) {
		_, err := StellarValue{
			CloseTime: closeTimeSeconds,
			Ext:       StellarValueExt{V: StellarValueType(99)},
		}.CloseTimeMs()
		assert.ErrorContains(t, err, "unknown StellarValue.ext arm: 99")
	})

	t.Run("millisecond arm with no payload is an error", func(t *testing.T) {
		for name, ext := range map[string]StellarValueExt{
			"SIGNED_MS":       {V: StellarValueTypeStellarValueSignedMs},
			"EMPTY_TX_SET_MS": {V: StellarValueTypeStellarValueEmptyTxSetMs},
		} {
			t.Run(name, func(t *testing.T) {
				_, err := StellarValue{Ext: ext}.CloseTimeMs()
				assert.Error(t, err)
			})
		}
	})

	t.Run("out of range close times are errors", func(t *testing.T) {
		_, err := StellarValue{
			CloseTime: TimePoint(math.MaxUint64),
			Ext:       StellarValueExt{V: StellarValueTypeStellarValueBasic},
		}.CloseTimeMs()
		assert.ErrorContains(t, err, "close time out of range")

		_, err = StellarValue{
			Ext: StellarValueExt{
				V:             StellarValueTypeStellarValueSignedMs,
				SignedMsValue: &StellarValueSignedMsValue{CloseTimeMs: TimePointMs(math.MaxUint64)},
			},
		}.CloseTimeMs()
		assert.ErrorContains(t, err, "close time in ms out of range")
	})
}
