package txnbuild

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/price"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/stretchr/testify/assert"
)

func TestManageSellOfferValidateSellingAsset(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761092))

	selling := CreditAsset{"", kp0.Address()}
	buying := NativeAsset{}
	sellAmount := "100"
	createOffer, err := CreateOfferOp(selling, buying, sellAmount, price.MustParse("0.01"))
	check(err)

	_, err = NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: false,
			Operations:           []Operation{&createOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	if assert.Error(t, err) {
		expected := "validation failed for *txnbuild.ManageSellOffer operation: Field: Selling, Error: asset code length must be between 1 and 12 characters"
		assert.Contains(t, err.Error(), expected)
	}
}

func TestManageSellOfferValidateBuyingAsset(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761092))

	selling := NativeAsset{}
	buying := CreditAsset{"", kp0.Address()}
	sellAmount := "100"
	createOffer, err := CreateOfferOp(selling, buying, sellAmount, price.MustParse("0.01"))
	check(err)

	_, err = NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: false,
			Operations:           []Operation{&createOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	if assert.Error(t, err) {
		expected := "validation failed for *txnbuild.ManageSellOffer operation: Field: Buying, Error: asset code length must be between 1 and 12 characters"
		assert.Contains(t, err.Error(), expected)
	}
}

func TestManageSellOfferValidateAmount(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761092))

	selling := NativeAsset{}
	buying := CreditAsset{"ABCD", kp0.Address()}
	sellAmount := "-1"
	createOffer, err := CreateOfferOp(selling, buying, sellAmount, price.MustParse("0.01"))
	check(err)

	_, err = NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: false,
			Operations:           []Operation{&createOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	if assert.Error(t, err) {
		expected := "validation failed for *txnbuild.ManageSellOffer operation: Field: Amount, Error: amount can not be negative"
		assert.Contains(t, err.Error(), expected)
	}
}

func TestManageSellOfferValidatePrice(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761092))

	selling := NativeAsset{}
	buying := CreditAsset{"ABCD", kp0.Address()}
	sellAmount := "0"
	createOffer, err := CreateOfferOp(selling, buying, sellAmount, xdr.Price{-1, 100})
	check(err)

	_, err = NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: false,
			Operations:           []Operation{&createOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	if assert.Error(t, err) {
		expected := "validation failed for *txnbuild.ManageSellOffer operation: Field: Price, Error: price cannot be negative: -1/100"
		assert.Contains(t, err.Error(), expected)
	}
}

func TestManageSellOfferValidateOfferID(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761092))

	mso := ManageSellOffer{
		Selling: CreditAsset{"ABCD", kp0.Address()},
		Buying:  NativeAsset{},
		Amount:  "0",
		Price:   price.MustParse("0.01"),
		OfferID: -1,
	}

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: false,
			Operations:           []Operation{&mso},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	if assert.Error(t, err) {
		expected := "validation failed for *txnbuild.ManageSellOffer operation: Field: OfferID, Error: amount can not be negative"
		assert.Contains(t, err.Error(), expected)
	}
}

func TestManageSellOfferPrice(t *testing.T) {
	kp0 := newKeypair0()

	mso := ManageSellOffer{
		Selling: CreditAsset{"ABCD", kp0.Address()},
		Buying:  NativeAsset{},
		Amount:  "1",
		Price:   price.MustParse("0.000000001"),
		OfferID: 1,
	}

	xdrOp, err := mso.BuildXDR()
	assert.NoError(t, err)
	expectedPrice := xdr.Price{N: 1, D: 1000000000}
	assert.Equal(t, expectedPrice, xdrOp.Body.ManageSellOfferOp.Price)

	parsed := ManageSellOffer{}
	assert.NoError(t, parsed.FromXDR(xdrOp))
	assert.Equal(t, mso.Price, parsed.Price)
}

func TestManageSellOfferRoundtrip(t *testing.T) {
	manageSellOffer := ManageSellOffer{
		SourceAccount: "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
		Selling:       CreditAsset{"USD", "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H"},
		Buying:        NativeAsset{},
		Amount:        "100.0000000",
		Price:         price.MustParse("0.01"),
		OfferID:       0,
	}
	testOperationsMarshalingRoundtrip(t, []Operation{&manageSellOffer}, false)

	// with muxed accounts
	manageSellOffer = ManageSellOffer{
		SourceAccount: "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK",
		Selling:       CreditAsset{"USD", "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H"},
		Buying:        NativeAsset{},
		Amount:        "100.0000000",
		Price:         price.MustParse("0.01"),
		OfferID:       0,
	}
	testOperationsMarshalingRoundtrip(t, []Operation{&manageSellOffer}, true)
}
