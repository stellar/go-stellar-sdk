package txnbuild

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestRevokeSponsorship(t *testing.T) {
	accountAddress := "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z"
	accountAddress2 := "GBUKBCG5VLRKAVYAIREJRUJHOKLIADZJOICRW43WVJCLES52BDOTCQZU"
	claimableBalanceId, err := xdr.MarshalHex(xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &xdr.Hash{0xca, 0xfe, 0xba, 0xbe, 0xde, 0xad, 0xbe, 0xef},
	})
	assert.NoError(t, err)
	for _, testcase := range []struct {
		name string
		op   RevokeSponsorship
	}{
		{
			name: "Account",
			op: RevokeSponsorship{
				SponsorshipType: RevokeSponsorshipTypeAccount,
				Account:         &accountAddress,
			},
		},
		{
			name: "Account with source",
			op: RevokeSponsorship{
				SponsorshipType: RevokeSponsorshipTypeAccount,
				Account:         &accountAddress,
				SourceAccount:   accountAddress2,
			},
		},
		{
			name: "TrustLine",
			op: RevokeSponsorship{
				SponsorshipType: RevokeSponsorshipTypeTrustLine,
				TrustLine: &TrustLineID{
					Account: accountAddress,
					Asset: CreditAsset{
						Code:   "USD",
						Issuer: newKeypair0().Address(),
					}.MustToTrustLineAsset(),
				},
			},
		},
		{
			name: "TrustLine pool share",
			op: RevokeSponsorship{
				SponsorshipType: RevokeSponsorshipTypeTrustLine,
				TrustLine: &TrustLineID{
					Account: accountAddress,
					Asset:   LiquidityPoolShareTrustLineAsset{LiquidityPoolID: LiquidityPoolId{0xca, 0xfe}},
				},
			},
		},
		{
			name: "Offer",
			op: RevokeSponsorship{
				SponsorshipType: RevokeSponsorshipTypeOffer,
				Offer: &OfferID{
					SellerAccountAddress: accountAddress,
					OfferID:              0xdeadbeef,
				},
			},
		},
		{
			name: "Data",
			op: RevokeSponsorship{
				SponsorshipType: RevokeSponsorshipTypeData,
				Data: &DataID{
					Account:  accountAddress,
					DataName: "foobar",
				},
			},
		},
		{
			name: "Data",
			op: RevokeSponsorship{
				SponsorshipType:  RevokeSponsorshipTypeClaimableBalance,
				ClaimableBalance: &claimableBalanceId,
			},
		},
		{
			name: "Signer",
			op: RevokeSponsorship{
				SponsorshipType: RevokeSponsorshipTypeSigner,
				Signer: &SignerID{
					AccountID:     accountAddress,
					SignerAddress: "XBU2RRGLXH3E5CQHTD3ODLDF2BWDCYUSSBLLZ5GNW7JXHDIYKXZWGTOG",
				},
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
			var op2 RevokeSponsorship
			assert.NoError(t, op2.FromXDR(xdrOp2))
			assert.Equal(t, op, op2)
			testOperationsMarshalingRoundtrip(t, []Operation{&testcase.op}, false)
		})
	}

	// without muxed accounts
	revokeOp := RevokeSponsorship{
		SourceAccount:   "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
		SponsorshipType: RevokeSponsorshipTypeAccount,
		Account:         &accountAddress,
	}
	testOperationsMarshalingRoundtrip(t, []Operation{&revokeOp}, false)

	// with muxed accounts
	revokeOp = RevokeSponsorship{
		SourceAccount:   "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK",
		SponsorshipType: RevokeSponsorshipTypeAccount,
		Account:         &accountAddress,
	}
	testOperationsMarshalingRoundtrip(t, []Operation{&revokeOp}, true)
}

// TestRevokeSponsorshipPoolShareTrustLine checks that a pool-share trust line
// key is encoded as ASSET_TYPE_POOL_SHARE with its pool ID, as in testnet
// transaction 25e65885d13542f523597f9f46844cd80eb1fd9b87fc6b1a3c8811487b625053.
func TestRevokeSponsorshipPoolShareTrustLine(t *testing.T) {
	var id LiquidityPoolId
	_, err := hex.Decode(id[:], []byte("74b85a9c1802cc0b3882858ab907b4678bc05209768088b8bc713f41442ff090"))
	require.NoError(t, err)

	op := RevokeSponsorship{
		SponsorshipType: RevokeSponsorshipTypeTrustLine,
		TrustLine: &TrustLineID{
			Account: "GA67CC7ZJIA57MYVBEWIJJTGPQMWMQM7CZCLKGFUL7QC4T7S47AEBQL3",
			Asset:   LiquidityPoolShareTrustLineAsset{LiquidityPoolID: id},
		},
	}
	require.NoError(t, op.Validate())
	xdrOp, err := op.BuildXDR()
	require.NoError(t, err)
	asset := xdrOp.Body.MustRevokeSponsorshipOp().MustLedgerKey().MustTrustLine().Asset
	assert.Equal(t, xdr.AssetTypeAssetTypePoolShare, asset.Type)
	require.NotNil(t, asset.LiquidityPoolId)
	assert.Equal(t, xdr.PoolId(id), *asset.LiquidityPoolId)
}

// TestRevokeSponsorshipFromXDRReuse checks that FromXDR into a RevokeSponsorship of another type sets only the field for the decoded type.
func TestRevokeSponsorshipFromXDRReuse(t *testing.T) {
	accountAddress := newKeypair0().Address()
	source := RevokeSponsorship{
		SponsorshipType: RevokeSponsorshipTypeData,
		Data:            &DataID{Account: accountAddress, DataName: "foobar"},
	}
	op, err := source.BuildXDR()
	require.NoError(t, err)

	reused := RevokeSponsorship{
		SourceAccount:   newKeypair1().Address(),
		SponsorshipType: RevokeSponsorshipTypeAccount,
		Account:         &accountAddress,
	}
	require.NoError(t, reused.FromXDR(op))
	assert.Equal(t, source, reused)
}

// TestRevokeSponsorshipFromXDRError checks that FromXDR leaves the receiver unchanged when it returns an error.
func TestRevokeSponsorshipFromXDRError(t *testing.T) {
	ttlBody, err := xdr.NewOperationBody(xdr.OperationTypeRevokeSponsorship, xdr.RevokeSponsorshipOp{
		Type:      xdr.RevokeSponsorshipTypeRevokeSponsorshipLedgerEntry,
		LedgerKey: &xdr.LedgerKey{Type: xdr.LedgerEntryTypeTtl, Ttl: &xdr.LedgerKeyTtl{}},
	})
	require.NoError(t, err)

	for _, tc := range []struct {
		name string
		op   xdr.Operation
	}{
		{"wrong operation type", xdr.Operation{Body: xdr.OperationBody{Type: xdr.OperationTypeInflation}}},
		{"unsupported ledger key", xdr.Operation{Body: ttlBody}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accountAddress := newKeypair0().Address()
			r := RevokeSponsorship{
				SourceAccount:   newKeypair1().Address(),
				SponsorshipType: RevokeSponsorshipTypeAccount,
				Account:         &accountAddress,
			}
			before := r
			require.Error(t, r.FromXDR(tc.op))
			assert.Equal(t, before, r)
		})
	}
}
