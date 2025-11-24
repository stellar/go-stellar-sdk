package txnbuild

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/stellar/go-stellar-sdk/xdr"
)

type RevokeSponsorshipType int

const (
	RevokeSponsorshipTypeAccount RevokeSponsorshipType = iota + 1
	RevokeSponsorshipTypeTrustLine
	RevokeSponsorshipTypeOffer
	RevokeSponsorshipTypeData
	RevokeSponsorshipTypeClaimableBalance
	RevokeSponsorshipTypeSigner
)

// RevokeSponsorship is a union type representing a RevokeSponsorship Operation.
// SponsorshipType stablishes which sponsorship is being revoked.
// The other fields should be ignored.
type RevokeSponsorship struct {
	SourceAccount   string
	SponsorshipType RevokeSponsorshipType
	// Account ID (strkey)
	Account   *string
	TrustLine *TrustLineID
	Offer     *OfferID
	Data      *DataID
	// Claimable Balance ID
	ClaimableBalance *string
	Signer           *SignerID
}

type TrustLineID struct {
	Account string
	Asset   TrustLineAsset
}

type OfferID struct {
	SellerAccountAddress string
	OfferID              int64
}

type DataID struct {
	Account  string
	DataName string
}

type SignerID struct {
	AccountID     string
	SignerAddress string
}

func (r *RevokeSponsorship) BuildXDR() (xdr.Operation, error) {
	xdrOp := xdr.RevokeSponsorshipOp{}
	switch r.SponsorshipType {
	case RevokeSponsorshipTypeAccount:
		var key xdr.LedgerKeyAccount
		if r.Account == nil {
			return xdr.Operation{}, errors.New("Account can't be nil")
		}
		if err := key.AccountId.SetAddress(*r.Account); err != nil {
			return xdr.Operation{}, errors.Wrap(err, "incorrect Account address")
		}
		xdrOp.Type = xdr.RevokeSponsorshipTypeRevokeSponsorshipLedgerEntry
		xdrOp.LedgerKey = &xdr.LedgerKey{
			Type:    xdr.LedgerEntryTypeAccount,
			Account: &key,
		}
	case RevokeSponsorshipTypeTrustLine:
		var key xdr.LedgerKeyTrustLine
		if r.TrustLine == nil {
			return xdr.Operation{}, errors.New("TrustLine can't be nil")
		}
		if err := key.AccountId.SetAddress(r.TrustLine.Account); err != nil {
			return xdr.Operation{}, errors.Wrap(err, "incorrect Account address")
		}
		asset, err := r.TrustLine.Asset.ToXDR()
		if err != nil {
			return xdr.Operation{}, errors.Wrap(err, "incorrect TrustLine asset")
		}
		key.Asset = asset
		xdrOp.Type = xdr.RevokeSponsorshipTypeRevokeSponsorshipLedgerEntry
		xdrOp.LedgerKey = &xdr.LedgerKey{
			Type:      xdr.LedgerEntryTypeTrustline,
			TrustLine: &key,
		}
	case RevokeSponsorshipTypeOffer:
		var key xdr.LedgerKeyOffer
		if r.Offer == nil {
			return xdr.Operation{}, errors.New("Offer can't be nil")
		}
		if err := key.SellerId.SetAddress(r.Offer.SellerAccountAddress); err != nil {
			return xdr.Operation{}, errors.Wrap(err, "incorrect Seller account address")
		}
		key.OfferId = xdr.Int64(r.Offer.OfferID)
		xdrOp.Type = xdr.RevokeSponsorshipTypeRevokeSponsorshipLedgerEntry
		xdrOp.LedgerKey = &xdr.LedgerKey{
			Type:  xdr.LedgerEntryTypeOffer,
			Offer: &key,
		}
	case RevokeSponsorshipTypeData:
		var key xdr.LedgerKeyData
		if r.Data == nil {
			return xdr.Operation{}, errors.New("Data can't be nil")
		}
		if err := key.AccountId.SetAddress(r.Data.Account); err != nil {
			return xdr.Operation{}, errors.Wrap(err, "incorrect Account address")
		}
		key.DataName = xdr.String64(r.Data.DataName)
		xdrOp.Type = xdr.RevokeSponsorshipTypeRevokeSponsorshipLedgerEntry
		xdrOp.LedgerKey = &xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeData,
			Data: &key,
		}
	case RevokeSponsorshipTypeClaimableBalance:
		var key xdr.LedgerKeyClaimableBalance

		if r.ClaimableBalance == nil {
			return xdr.Operation{}, errors.New("ClaimableBalance can't be nil")
		}
		if err := xdr.SafeUnmarshalHex(*r.ClaimableBalance, &key.BalanceId); err != nil {
			return xdr.Operation{}, errors.Wrap(err, "cannot parse ClaimableBalance")
		}
		xdrOp.Type = xdr.RevokeSponsorshipTypeRevokeSponsorshipLedgerEntry
		xdrOp.LedgerKey = &xdr.LedgerKey{
			Type:             xdr.LedgerEntryTypeClaimableBalance,
			ClaimableBalance: &key,
		}
	case RevokeSponsorshipTypeSigner:
		var signer xdr.RevokeSponsorshipOpSigner
		if r.Signer == nil {
			return xdr.Operation{}, errors.New("Signer can't be nil")
		}
		if err := signer.AccountId.SetAddress(r.Signer.AccountID); err != nil {
			return xdr.Operation{}, errors.New("incorrect Account address")
		}
		if err := signer.SignerKey.SetAddress(r.Signer.SignerAddress); err != nil {
			return xdr.Operation{}, errors.New("incorrect Signer account address")
		}
		xdrOp.Type = xdr.RevokeSponsorshipTypeRevokeSponsorshipSigner
		xdrOp.Signer = &signer
	default:
		return xdr.Operation{}, fmt.Errorf("unknown SponsorshipType: %d", r.SponsorshipType)
	}
	opType := xdr.OperationTypeRevokeSponsorship
	body, err := xdr.NewOperationBody(opType, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR OperationBody")
	}
	op := xdr.Operation{Body: body}
	SetOpSourceAccount(&op, r.SourceAccount)

	return op, nil
}

func (r *RevokeSponsorship) FromXDR(xdrOp xdr.Operation) error {
	r.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	op, ok := xdrOp.Body.GetRevokeSponsorshipOp()
	if !ok {
		return errors.New("error parsing revoke_sponsorship operation from xdr")
	}
	switch op.Type {
	case xdr.RevokeSponsorshipTypeRevokeSponsorshipLedgerEntry:
		lkey := op.LedgerKey
		switch lkey.Type {
		case xdr.LedgerEntryTypeAccount:
			var sponsorshipId string
			sponsorshipId = lkey.Account.AccountId.Address()
			r.SponsorshipType = RevokeSponsorshipTypeAccount
			r.Account = &sponsorshipId
		case xdr.LedgerEntryTypeTrustline:
			var sponsorshipId TrustLineID
			sponsorshipId.Account = lkey.TrustLine.AccountId.Address()
			asset, err := assetFromTrustLineAssetXDR(lkey.TrustLine.Asset)
			if err != nil {
				return errors.Wrap(err, "error parsing Trustline Asset")
			}
			sponsorshipId.Asset = asset
			r.SponsorshipType = RevokeSponsorshipTypeTrustLine
			r.TrustLine = &sponsorshipId
		case xdr.LedgerEntryTypeOffer:
			var sponsorshipId OfferID
			sponsorshipId.SellerAccountAddress = lkey.Offer.SellerId.Address()
			sponsorshipId.OfferID = int64(lkey.Offer.OfferId)
			r.SponsorshipType = RevokeSponsorshipTypeOffer
			r.Offer = &sponsorshipId
		case xdr.LedgerEntryTypeData:
			var sponsorshipId DataID
			sponsorshipId.Account = lkey.Data.AccountId.Address()
			sponsorshipId.DataName = string(lkey.Data.DataName)
			r.SponsorshipType = RevokeSponsorshipTypeData
			r.Data = &sponsorshipId
		case xdr.LedgerEntryTypeClaimableBalance:
			if lkey.ClaimableBalance.BalanceId.Type != 0 {
				return fmt.Errorf(
					"unexpected ClaimableBalance Id Type: %d",
					lkey.ClaimableBalance.BalanceId.Type,
				)
			}
			claimableBalanceId, err := xdr.MarshalHex(&lkey.ClaimableBalance.BalanceId)
			if err != nil {
				return errors.Wrap(err, "cannot generate Claimable Balance Id")
			}
			r.SponsorshipType = RevokeSponsorshipTypeClaimableBalance
			r.ClaimableBalance = &claimableBalanceId
		default:
			return fmt.Errorf("unexpected LedgerEntryType: %d", lkey.Type)
		}
	case xdr.RevokeSponsorshipTypeRevokeSponsorshipSigner:
		var sponsorshipId SignerID
		sponsorshipId.AccountID = op.Signer.AccountId.Address()
		sponsorshipId.SignerAddress = op.Signer.SignerKey.Address()
		r.SponsorshipType = RevokeSponsorshipTypeSigner
		r.Signer = &sponsorshipId
	default:
		return fmt.Errorf("unexpected RevokeSponsorshipType: %d", op.Type)
	}
	return nil
}

func (r *RevokeSponsorship) Validate() error {
	switch r.SponsorshipType {
	case RevokeSponsorshipTypeAccount:
		if r.Account == nil {
			return errors.New("Account can't be nil")
		}
		return validateStellarPublicKey(*r.Account)
	case RevokeSponsorshipTypeTrustLine:
		if r.TrustLine == nil {
			return errors.New("Trustline can't be nil")
		}
		if err := validateStellarPublicKey(r.TrustLine.Account); err != nil {
			return errors.Wrap(err, "invalid Account address")
		}
		if err := validateStellarAsset(r.TrustLine.Asset); err != nil {
			return errors.Wrap(err, "invalid TrustLine asset")
		}
	case RevokeSponsorshipTypeOffer:
		if r.Offer == nil {
			return errors.New("Offer can't be nil")
		}
		if err := validateStellarPublicKey(r.Offer.SellerAccountAddress); err != nil {
			return errors.Wrap(err, "invalid Seller account address")
		}
		return validateStellarPublicKey(r.Offer.SellerAccountAddress)
	case RevokeSponsorshipTypeData:
		if r.Data == nil {
			return errors.New("Data can't be nil")
		}
		if err := validateStellarPublicKey(r.Data.Account); err != nil {
			return errors.Wrap(err, "invalid Account address")
		}
	case RevokeSponsorshipTypeClaimableBalance:
		if r.ClaimableBalance == nil {
			return errors.New("ClaimableBalance can't be nil")
		}
		var unused xdr.ClaimableBalanceId
		if err := xdr.SafeUnmarshalHex(*r.ClaimableBalance, &unused); err != nil {
			return errors.Wrap(err, "cannot parse ClaimableBalance")
		}
	case RevokeSponsorshipTypeSigner:
		if r.Signer == nil {
			return errors.New("Signer can't be nil")
		}
		if err := validateStellarPublicKey(r.Signer.AccountID); err != nil {
			return errors.New("invalid Account address")
		}
		if err := validateStellarSignerKey(r.Signer.SignerAddress); err != nil {
			return errors.New("invalid Signer account address")
		}
	default:
		return fmt.Errorf("unknown SponsorshipType: %d", r.SponsorshipType)
	}
	return nil
}

func (r *RevokeSponsorship) GetSourceAccount() string {
	return r.SourceAccount
}
