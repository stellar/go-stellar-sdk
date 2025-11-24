package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/amount"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ManageBuyOffer represents the Stellar manage buy offer operation. See
// https://developers.stellar.org/docs/start/list-of-operations/
type ManageBuyOffer struct {
	Selling       Asset
	Buying        Asset
	Amount        string
	Price         xdr.Price
	OfferID       int64
	SourceAccount string
}

// BuildXDR for ManageBuyOffer returns a fully configured XDR Operation.
func (mo *ManageBuyOffer) BuildXDR() (xdr.Operation, error) {
	xdrSelling, err := mo.Selling.ToXDR()
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to set XDR 'Selling' field")
	}

	xdrBuying, err := mo.Buying.ToXDR()
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to set XDR 'Buying' field")
	}

	xdrAmount, err := amount.Parse(mo.Amount)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to parse 'Amount'")
	}

	opType := xdr.OperationTypeManageBuyOffer
	xdrOp := xdr.ManageBuyOfferOp{
		Selling:   xdrSelling,
		Buying:    xdrBuying,
		BuyAmount: xdrAmount,
		Price:     mo.Price,
		OfferId:   xdr.Int64(mo.OfferID),
	}
	body, err := xdr.NewOperationBody(opType, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR OperationBody")
	}

	op := xdr.Operation{Body: body}
	SetOpSourceAccount(&op, mo.SourceAccount)
	return op, nil
}

// FromXDR for ManageBuyOffer initialises the txnbuild struct from the corresponding xdr Operation.
func (mo *ManageBuyOffer) FromXDR(xdrOp xdr.Operation) error {
	result, ok := xdrOp.Body.GetManageBuyOfferOp()
	if !ok {
		return errors.New("error parsing manage_buy_offer operation from xdr")
	}

	mo.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	mo.OfferID = int64(result.OfferId)
	mo.Amount = amount.String(result.BuyAmount)
	mo.Price = result.Price
	buyingAsset, err := assetFromXDR(result.Buying)
	if err != nil {
		return errors.Wrap(err, "error parsing buying_asset in manage_buy_offer operation")
	}
	mo.Buying = buyingAsset

	sellingAsset, err := assetFromXDR(result.Selling)
	if err != nil {
		return errors.Wrap(err, "error parsing selling_asset in manage_buy_offer operation")
	}
	mo.Selling = sellingAsset
	return nil
}

// Validate for ManageBuyOffer validates the required struct fields. It returns an error if any
// of the fields are invalid. Otherwise, it returns nil.
func (mo *ManageBuyOffer) Validate() error {
	return validateOffer(mo.Buying, mo.Selling, mo.Amount, mo.Price, mo.OfferID)
}

// GetSourceAccount returns the source account of the operation, or the empty string if not
// set.
func (mo *ManageBuyOffer) GetSourceAccount() string {
	return mo.SourceAccount
}
