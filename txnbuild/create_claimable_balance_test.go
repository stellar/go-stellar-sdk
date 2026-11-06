package txnbuild

import (
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateClaimableBalanceRoundTrip(t *testing.T) {
	and := AndPredicate(BeforeAbsoluteTimePredicate(100), BeforeRelativeTimePredicate(10))
	createNativeBalance := &CreateClaimableBalance{
		Amount: "1234.0000000",
		Asset:  NativeAsset{},
		Destinations: []Claimant{
			NewClaimant(newKeypair1().Address(), &UnconditionalPredicate),
			NewClaimant(newKeypair1().Address(), &and),
		},
	}

	not := NotPredicate(UnconditionalPredicate)
	or := OrPredicate(BeforeAbsoluteTimePredicate(100), BeforeRelativeTimePredicate(10))
	createAssetBalance := &CreateClaimableBalance{
		Amount: "99.0000000",
		Asset: CreditAsset{
			Code:   "COP",
			Issuer: "GB56OJGSA6VHEUFZDX6AL2YDVG2TS5JDZYQJHDYHBDH7PCD5NIQKLSDO",
		},
		Destinations: []Claimant{
			NewClaimant(newKeypair1().Address(), &not),
			NewClaimant(newKeypair1().Address(), &or),
		},
	}

	testOperationsMarshalingRoundtrip(t, []Operation{createNativeBalance, createAssetBalance}, false)

	createNativeBalanceWithMuxedAcounts := &CreateClaimableBalance{
		SourceAccount: "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK",
		Amount:        "1234.0000000",
		Asset:         NativeAsset{},
		Destinations: []Claimant{
			NewClaimant(newKeypair1().Address(), &UnconditionalPredicate),
			NewClaimant(newKeypair1().Address(), &and),
		},
	}

	testOperationsMarshalingRoundtrip(t, []Operation{createNativeBalanceWithMuxedAcounts}, true)
}

// TestCreateClaimableBalanceFromXDRReuse checks that FromXDR into a populated CreateClaimableBalance keeps no claimants from the earlier value.
func TestCreateClaimableBalanceFromXDRReuse(t *testing.T) {
	source := CreateClaimableBalance{
		Amount:       "1.0000000",
		Asset:        NativeAsset{},
		Destinations: []Claimant{NewClaimant(newKeypair1().Address(), &UnconditionalPredicate)},
	}
	op, err := source.BuildXDR()
	require.NoError(t, err)

	var want CreateClaimableBalance
	require.NoError(t, want.FromXDR(op))

	reused := CreateClaimableBalance{
		SourceAccount: newKeypair0().Address(),
		Amount:        "2.0000000",
		Asset:         CreditAsset{Code: "USD", Issuer: newKeypair0().Address()},
		Destinations:  []Claimant{NewClaimant(newKeypair2().Address(), &UnconditionalPredicate)},
	}
	require.NoError(t, reused.FromXDR(op))
	assert.Equal(t, want, reused)

	rebuilt, err := reused.BuildXDR()
	require.NoError(t, err)
	assert.Equal(t, op, rebuilt)
}

func TestClaimableBalanceID(t *testing.T) {
	A := "SCZANGBA5YHTNYVVV4C3U252E2B6P6F5T3U6MM63WBSBZATAQI3EBTQ4"
	B := "GA2C5RFPE6GCKMY3US5PAB6UZLKIGSPIUKSLRB6Q723BM2OARMDUYEJ5"

	aKeys := keypair.MustParseFull(A)
	aAccount := SimpleAccount{AccountID: aKeys.Address(), Sequence: 123}

	soon := time.Now().Add(time.Second * 60)
	bCanClaim := BeforeRelativeTimePredicate(60)
	aCanReclaim := NotPredicate(BeforeAbsoluteTimePredicate(soon.Unix()))

	claimants := []Claimant{
		NewClaimant(B, &bCanClaim),
		NewClaimant(aKeys.Address(), &aCanReclaim),
	}

	claimableBalanceEntry := CreateClaimableBalance{
		Destinations:  claimants,
		Asset:         NativeAsset{},
		Amount:        "420",
		SourceAccount: "GB56OJGSA6VHEUFZDX6AL2YDVG2TS5JDZYQJHDYHBDH7PCD5NIQKLSDO",
	}

	// Build and sign the transaction
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &aAccount,
			IncrementSequenceNum: true,
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			Operations:           []Operation{&claimableBalanceEntry},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, aKeys)
	assert.NoError(t, err)

	balanceId, err := tx.ClaimableBalanceID(0)
	assert.NoError(t, err)
	assert.Equal(t, "0000000095001252ab3b4d16adbfa5364ce526dfcda03cb2258b827edbb2e0450087be51", balanceId)
}
