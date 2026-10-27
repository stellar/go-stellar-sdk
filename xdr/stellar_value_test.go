package xdr

import (
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
