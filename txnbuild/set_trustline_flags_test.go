package txnbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestSetTrustLineFlags(t *testing.T) {
	asset := CreditAsset{"ABCD", "GAEJJMDDCRYF752PKIJICUVL7MROJBNXDV2ZB455T7BAFHU2LCLSE2LW"}
	source := "GBUKBCG5VLRKAVYAIREJRUJHOKLIADZJOICRW43WVJCLES52BDOTCQZU"
	trustor := "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z"
	for _, testcase := range []struct {
		name string
		op   SetTrustLineFlags
	}{
		{
			name: "Both set and clear",
			op: SetTrustLineFlags{
				Trustor:       trustor,
				Asset:         asset,
				SetFlags:      []TrustLineFlag{TrustLineClawbackEnabled},
				ClearFlags:    []TrustLineFlag{TrustLineAuthorized, TrustLineAuthorizedToMaintainLiabilities},
				SourceAccount: source,
			},
		},
		{
			name: "Both set and clear 2",
			op: SetTrustLineFlags{
				Trustor:       trustor,
				Asset:         asset,
				SetFlags:      []TrustLineFlag{TrustLineAuthorized, TrustLineAuthorizedToMaintainLiabilities},
				ClearFlags:    []TrustLineFlag{TrustLineClawbackEnabled},
				SourceAccount: source,
			},
		},
		{
			name: "Only set",
			op: SetTrustLineFlags{
				Trustor:       trustor,
				Asset:         asset,
				SetFlags:      []TrustLineFlag{TrustLineClawbackEnabled},
				ClearFlags:    nil,
				SourceAccount: source,
			},
		},
		{
			name: "Only clear",
			op: SetTrustLineFlags{
				Trustor:       trustor,
				Asset:         asset,
				SetFlags:      nil,
				ClearFlags:    []TrustLineFlag{TrustLineClawbackEnabled},
				SourceAccount: source,
			},
		},
		{
			name: "No set nor clear",
			op: SetTrustLineFlags{
				Trustor:       trustor,
				Asset:         asset,
				SetFlags:      nil,
				ClearFlags:    nil,
				SourceAccount: source,
			},
		},
		{
			name: "No source",
			op: SetTrustLineFlags{
				Trustor:    trustor,
				Asset:      asset,
				SetFlags:   []TrustLineFlag{TrustLineClawbackEnabled},
				ClearFlags: []TrustLineFlag{TrustLineAuthorized, TrustLineAuthorizedToMaintainLiabilities},
			},
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			op := testcase.op
			assert.NoError(t, op.Validate())
			xdrOp, err := op.BuildXDR()
			assert.NoError(t, err)
			xdrBin, err := xdrOp.MarshalBinary()
			assert.NoError(t, err)
			var xdrOp2 xdr.Operation
			assert.NoError(t, xdr.SafeUnmarshal(xdrBin, &xdrOp2))
			var op2 SetTrustLineFlags
			assert.NoError(t, op2.FromXDR(xdrOp2))
			assert.Equal(t, op, op2)
			testOperationsMarshalingRoundtrip(t, []Operation{&testcase.op}, false)
		})
	}

	// with muxed accounts
	setTrustLineFlags := SetTrustLineFlags{
		Trustor:       trustor,
		Asset:         asset,
		SetFlags:      []TrustLineFlag{TrustLineClawbackEnabled},
		ClearFlags:    []TrustLineFlag{TrustLineAuthorized, TrustLineAuthorizedToMaintainLiabilities},
		SourceAccount: "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK",
	}
	testOperationsMarshalingRoundtrip(t, []Operation{&setTrustLineFlags}, true)
}
