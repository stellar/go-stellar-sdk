package txnbuild

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stretchr/testify/assert"
)

func TestChangeTrustMaxLimit(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	changeTrust := ChangeTrust{
		Line: CreditAsset{"ABCD", kp0.Address()}.MustToChangeTrustAsset(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&changeTrust},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABgAAAAFBQkNEAAAAAODcbeFyXKxmUWK1L6znNbKKIkPkHRJNbLktcKPqLnLFf/////////8AAAAAAAAAAeoucsUAAABAXp/gGvNtaqn2/gEh4QoNO+LpT3AmyLFDb81INsfdkf70USiBheUc7bzxgZJLVpFy2qw3ucqpQQPi986XFbPsAQ=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestChangeTrustValidateInvalidAsset(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	changeTrust := ChangeTrust{
		Line: NativeAsset{}.MustToChangeTrustAsset(),
	}

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&changeTrust},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	if assert.Error(t, err) {
		expected := "validation failed for *txnbuild.ChangeTrust operation: Field: Line, Error: native (XLM) asset type is not allowed"
		assert.Contains(t, err.Error(), expected)
	}
}

func TestChangeTrustValidateInvalidLimit(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	changeTrust := ChangeTrust{
		Line:  CreditAsset{"ABCD", kp0.Address()}.MustToChangeTrustAsset(),
		Limit: "-1",
	}

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&changeTrust},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	if assert.Error(t, err) {
		expected := "validation failed for *txnbuild.ChangeTrust operation: Field: Limit, Error: amount can not be negative"
		assert.Contains(t, err.Error(), expected)
	}
}

func TestChangeTrustRoundtrip(t *testing.T) {
	changeTrust := ChangeTrust{
		SourceAccount: "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
		Line:          CreditAsset{"ABCD", "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H"}.MustToChangeTrustAsset(),
		Limit:         "1.0000000",
	}
	testOperationsMarshalingRoundtrip(t, []Operation{&changeTrust}, false)

	// with muxed accounts
	changeTrust = ChangeTrust{
		SourceAccount: "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK",
		Line:          CreditAsset{"ABCD", "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H"}.MustToChangeTrustAsset(),
		Limit:         "1.0000000",
	}
	testOperationsMarshalingRoundtrip(t, []Operation{&changeTrust}, true)
}
