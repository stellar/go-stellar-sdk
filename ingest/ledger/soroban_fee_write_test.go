package ledger

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestSorobanFeeWrite1KbLedgerCloseMetaV2(t *testing.T) {
	meta := xdr.LedgerCloseMeta{V: 2, V2: &xdr.LedgerCloseMetaV2{
		Ext: xdr.LedgerCloseMetaExt{V: 1, V1: &xdr.LedgerCloseMetaExtV1{SorobanFeeWrite1Kb: 42}},
	}}

	fee, ok := SorobanFeeWrite1Kb(meta)

	assert.True(t, ok)
	assert.Equal(t, int64(42), fee)
}
