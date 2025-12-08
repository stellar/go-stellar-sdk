package operations

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/protocols/horizon/base"
	"github.com/stellar/go-stellar-sdk/support/render/hal"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TypeNames maps from operation type to the string used to represent that type
// in horizon's JSON responses
var TypeNames = map[xdr.OperationType]string{
	xdr.OperationTypeCreateAccount:                 "create_account",
	xdr.OperationTypePayment:                       "payment",
	xdr.OperationTypePathPaymentStrictReceive:      "path_payment_strict_receive",
	xdr.OperationTypeManageSellOffer:               "manage_sell_offer",
	xdr.OperationTypeCreatePassiveSellOffer:        "create_passive_sell_offer",
	xdr.OperationTypeSetOptions:                    "set_options",
	xdr.OperationTypeChangeTrust:                   "change_trust",
	xdr.OperationTypeAllowTrust:                    "allow_trust",
	xdr.OperationTypeAccountMerge:                  "account_merge",
	xdr.OperationTypeInflation:                     "inflation",
	xdr.OperationTypeManageData:                    "manage_data",
	xdr.OperationTypeBumpSequence:                  "bump_sequence",
	xdr.OperationTypeManageBuyOffer:                "manage_buy_offer",
	xdr.OperationTypePathPaymentStrictSend:         "path_payment_strict_send",
	xdr.OperationTypeCreateClaimableBalance:        "create_claimable_balance",
	xdr.OperationTypeClaimClaimableBalance:         "claim_claimable_balance",
	xdr.OperationTypeBeginSponsoringFutureReserves: "begin_sponsoring_future_reserves",
	xdr.OperationTypeEndSponsoringFutureReserves:   "end_sponsoring_future_reserves",
	xdr.OperationTypeRevokeSponsorship:             "revoke_sponsorship",
	xdr.OperationTypeClawback:                      "clawback",
	xdr.OperationTypeClawbackClaimableBalance:      "clawback_claimable_balance",
	xdr.OperationTypeSetTrustLineFlags:             "set_trust_line_flags",
	xdr.OperationTypeLiquidityPoolDeposit:          "liquidity_pool_deposit",
	xdr.OperationTypeLiquidityPoolWithdraw:         "liquidity_pool_withdraw",
	xdr.OperationTypeInvokeHostFunction:            "invoke_host_function",
	xdr.OperationTypeExtendFootprintTtl:            "extend_footprint_ttl",
	xdr.OperationTypeRestoreFootprint:              "restore_footprint",
}

// Base represents the common attributes of an operation resource
type Base struct {
	Links struct {
		Self        hal.Link `json:"self"`
		Transaction hal.Link `json:"transaction"`
		Effects     hal.Link `json:"effects"`
		Succeeds    hal.Link `json:"succeeds"`
		Precedes    hal.Link `json:"precedes"`
	} `json:"_links"`

	ID string `json:"id"`
	PT string `json:"paging_token"`
	// TransactionSuccessful defines if this operation is part of
	// successful transaction.
	TransactionSuccessful bool      `json:"transaction_successful"`
	SourceAccount         string    `json:"source_account"`
	SourceAccountMuxed    string    `json:"source_account_muxed,omitempty"`
	SourceAccountMuxedID  uint64    `json:"source_account_muxed_id,omitempty,string"`
	Type                  string    `json:"type"`
	TypeI                 int32     `json:"type_i"`
	LedgerCloseTime       time.Time `json:"created_at"`
	// TransactionHash is the hash of the transaction which created the operation
	// Note that the Transaction field below is not always present in the Operation response.
	// If the Transaction field is present TransactionHash is redundant since the same information
	// is present in Transaction. But, if the Transaction field is nil then TransactionHash is useful.
	// Transaction is non nil when the "join=transactions" parameter is present in the operations request
	TransactionHash string               `json:"transaction_hash"`
	Transaction     *horizon.Transaction `json:"transaction,omitempty"`
	Sponsor         string               `json:"sponsor,omitempty"`
}

// PagingToken implements hal.Pageable
func (base Base) PagingToken() string {
	return base.PT
}

// BumpSequence is the json resource representing a single operation whose type is
// BumpSequence.
type BumpSequence struct {
	Base
	BumpTo string `json:"bump_to"`
}

// CreateAccount is the json resource representing a single operation whose type
// is CreateAccount.
type CreateAccount struct {
	Base
	StartingBalance string `json:"starting_balance"`
	Funder          string `json:"funder"`
	FunderMuxed     string `json:"funder_muxed,omitempty"`
	FunderMuxedID   uint64 `json:"funder_muxed_id,omitempty,string"`
	Account         string `json:"account"`
}

// Payment is the json resource representing a single operation whose type is
// Payment.
type Payment struct {
	Base
	base.Asset
	From        string `json:"from"`
	FromMuxed   string `json:"from_muxed,omitempty"`
	FromMuxedID uint64 `json:"from_muxed_id,omitempty,string"`
	To          string `json:"to"`
	ToMuxed     string `json:"to_muxed,omitempty"`
	ToMuxedID   uint64 `json:"to_muxed_id,omitempty,string"`
	Amount      string `json:"amount"`
}

// PathPayment is the json resource representing a single operation whose type
// is PathPayment.
type PathPayment struct {
	Payment
	Path              []base.Asset `json:"path"`
	SourceAmount      string       `json:"source_amount"`
	SourceMax         string       `json:"source_max"`
	SourceAssetType   string       `json:"source_asset_type"`
	SourceAssetCode   string       `json:"source_asset_code,omitempty"`
	SourceAssetIssuer string       `json:"source_asset_issuer,omitempty"`
}

// PathPaymentStrictSend is the json resource representing a single operation whose type
// is PathPaymentStrictSend.
type PathPaymentStrictSend struct {
	Payment
	Path              []base.Asset `json:"path"`
	SourceAmount      string       `json:"source_amount"`
	DestinationMin    string       `json:"destination_min"`
	SourceAssetType   string       `json:"source_asset_type"`
	SourceAssetCode   string       `json:"source_asset_code,omitempty"`
	SourceAssetIssuer string       `json:"source_asset_issuer,omitempty"`
}

// ManageData represents a ManageData operation as it is serialized into json
// for the horizon API.
type ManageData struct {
	Base
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Offer is an embedded resource used in offer type operations.
type Offer struct {
	Base
	Amount             string     `json:"amount"`
	Price              string     `json:"price"`
	PriceR             base.Price `json:"price_r"`
	BuyingAssetType    string     `json:"buying_asset_type"`
	BuyingAssetCode    string     `json:"buying_asset_code,omitempty"`
	BuyingAssetIssuer  string     `json:"buying_asset_issuer,omitempty"`
	SellingAssetType   string     `json:"selling_asset_type"`
	SellingAssetCode   string     `json:"selling_asset_code,omitempty"`
	SellingAssetIssuer string     `json:"selling_asset_issuer,omitempty"`
}

// CreatePassiveSellOffer is the json resource representing a single operation whose
// type is CreatePassiveSellOffer.
type CreatePassiveSellOffer struct {
	Offer
}

// ManageSellOffer is the json resource representing a single operation whose type
// is ManageSellOffer.
type ManageSellOffer struct {
	Offer
	OfferID int64 `json:"offer_id,string"`
}

// ManageBuyOffer is the json resource representing a single operation whose type
// is ManageBuyOffer.
type ManageBuyOffer struct {
	Offer
	OfferID int64 `json:"offer_id,string"`
}

// SetOptions is the json resource representing a single operation whose type is
// SetOptions.
type SetOptions struct {
	Base
	HomeDomain    string `json:"home_domain,omitempty"`
	InflationDest string `json:"inflation_dest,omitempty"`

	MasterKeyWeight *int   `json:"master_key_weight,omitempty"`
	SignerKey       string `json:"signer_key,omitempty"`
	SignerWeight    *int   `json:"signer_weight,omitempty"`

	SetFlags    []int    `json:"set_flags,omitempty"`
	SetFlagsS   []string `json:"set_flags_s,omitempty"`
	ClearFlags  []int    `json:"clear_flags,omitempty"`
	ClearFlagsS []string `json:"clear_flags_s,omitempty"`

	LowThreshold  *int `json:"low_threshold,omitempty"`
	MedThreshold  *int `json:"med_threshold,omitempty"`
	HighThreshold *int `json:"high_threshold,omitempty"`
}

// ChangeTrust is the json resource representing a single operation whose type
// is ChangeTrust.
type ChangeTrust struct {
	Base
	base.LiquidityPoolOrAsset
	Limit          string `json:"limit"`
	Trustee        string `json:"trustee,omitempty"`
	Trustor        string `json:"trustor"`
	TrustorMuxed   string `json:"trustor_muxed,omitempty"`
	TrustorMuxedID uint64 `json:"trustor_muxed_id,omitempty,string"`
}

// Deprecated: use SetTrustLineFlags instead.
// AllowTrust is the json resource representing a single operation whose type is
// AllowTrust.
type AllowTrust struct {
	Base
	base.Asset
	Trustee                        string `json:"trustee"`
	TrusteeMuxed                   string `json:"trustee_muxed,omitempty"`
	TrusteeMuxedID                 uint64 `json:"trustee_muxed_id,omitempty,string"`
	Trustor                        string `json:"trustor"`
	Authorize                      bool   `json:"authorize"`
	AuthorizeToMaintainLiabilities bool   `json:"authorize_to_maintain_liabilities"`
}

// AccountMerge is the json resource representing a single operation whose type
// is AccountMerge.
type AccountMerge struct {
	Base
	Account        string `json:"account"`
	AccountMuxed   string `json:"account_muxed,omitempty"`
	AccountMuxedID uint64 `json:"account_muxed_id,omitempty,string"`
	Into           string `json:"into"`
	IntoMuxed      string `json:"into_muxed,omitempty"`
	IntoMuxedID    uint64 `json:"into_muxed_id,omitempty,string"`
}

// Inflation is the json resource representing a single operation whose type is
// Inflation.
type Inflation struct {
	Base
}

// CreateClaimableBalance is the json resource representing a single operation whose type is
// CreateClaimableBalance.
type CreateClaimableBalance struct {
	Base
	Asset     string             `json:"asset"`
	Amount    string             `json:"amount"`
	Claimants []horizon.Claimant `json:"claimants"`
}

// ClaimClaimableBalance is the json resource representing a single operation whose type is
// ClaimClaimableBalance.
type ClaimClaimableBalance struct {
	Base
	BalanceID       string `json:"balance_id"`
	Claimant        string `json:"claimant"`
	ClaimantMuxed   string `json:"claimant_muxed,omitempty"`
	ClaimantMuxedID uint64 `json:"claimant_muxed_id,omitempty,string"`
}

// BeginSponsoringFutureReserves is the json resource representing a single operation whose type is
// BeginSponsoringFutureReserves.
type BeginSponsoringFutureReserves struct {
	Base
	SponsoredID string `json:"sponsored_id"`
}

// EndSponsoringFutureReserves is the json resource representing a single operation whose type is
// EndSponsoringFutureReserves.
type EndSponsoringFutureReserves struct {
	Base
	BeginSponsor        string `json:"begin_sponsor,omitempty"`
	BeginSponsorMuxed   string `json:"begin_sponsor_muxed,omitempty"`
	BeginSponsorMuxedID uint64 `json:"begin_sponsor_muxed_id,omitempty,string"`
}

// RevokeSponsorship is the json resource representing a single operation whose type is
// RevokeSponsorship.
type RevokeSponsorship struct {
	Base
	AccountID                *string `json:"account_id,omitempty"`
	ClaimableBalanceID       *string `json:"claimable_balance_id,omitempty"`
	DataAccountID            *string `json:"data_account_id,omitempty"`
	DataName                 *string `json:"data_name,omitempty"`
	OfferID                  *int64  `json:"offer_id,omitempty,string"`
	TrustlineAccountID       *string `json:"trustline_account_id,omitempty"`
	TrustlineLiquidityPoolID *string `json:"trustline_liquidity_pool_id,omitempty"`
	TrustlineAsset           *string `json:"trustline_asset,omitempty"`
	SignerAccountID          *string `json:"signer_account_id,omitempty"`
	SignerKey                *string `json:"signer_key,omitempty"`
}

// Clawback is the json resource representing a single operation whose type is
// Clawback.
type Clawback struct {
	Base
	base.Asset
	From        string `json:"from"`
	FromMuxed   string `json:"from_muxed,omitempty"`
	FromMuxedID uint64 `json:"from_muxed_id,omitempty,string"`
	Amount      string `json:"amount"`
}

// ClawbackClaimableBalance is the json resource representing a single operation whose type is
// ClawbackClaimableBalance.
type ClawbackClaimableBalance struct {
	Base
	BalanceID string `json:"balance_id"`
}

// SetTrustLineFlags is the json resource representing a single operation whose type is
// SetTrustLineFlags.
type SetTrustLineFlags struct {
	Base
	base.Asset
	Trustor     string   `json:"trustor"`
	SetFlags    []int    `json:"set_flags,omitempty"`
	SetFlagsS   []string `json:"set_flags_s,omitempty"`
	ClearFlags  []int    `json:"clear_flags,omitempty"`
	ClearFlagsS []string `json:"clear_flags_s,omitempty"`
}

// LiquidityPoolDeposit is the json resource representing a single operation whose type is
// LiquidityPoolDeposit.
type LiquidityPoolDeposit struct {
	Base
	LiquidityPoolID   string             `json:"liquidity_pool_id"`
	ReservesMax       []base.AssetAmount `json:"reserves_max"`
	MinPrice          string             `json:"min_price"`
	MinPriceR         base.Price         `json:"min_price_r"`
	MaxPrice          string             `json:"max_price"`
	MaxPriceR         base.Price         `json:"max_price_r"`
	ReservesDeposited []base.AssetAmount `json:"reserves_deposited"`
	SharesReceived    string             `json:"shares_received"`
}

// LiquidityPoolWithdraw is the json resource representing a single operation whose type is
// LiquidityPoolWithdraw.
type LiquidityPoolWithdraw struct {
	Base
	LiquidityPoolID  string             `json:"liquidity_pool_id"`
	ReservesMin      []base.AssetAmount `json:"reserves_min"`
	Shares           string             `json:"shares"`
	ReservesReceived []base.AssetAmount `json:"reserves_received"`
}

// InvokeHostFunction is the json resource representing a single InvokeHostFunctionOp.
// The model for InvokeHostFunction assimilates InvokeHostFunctionOp, but is simplified.
// HostFunction         - contract function invocation to be performed.
// AssetBalanceChanges  - array of asset balance changed records related to contract invocations in this host invocation.
//
//	The asset balance change record is captured at ingestion time from the asset contract
//	events present in tx meta. Only asset contract events that have a reference to classic account in
//	either the 'from' or 'to' participants will be included here as an asset balance change.
//	Any pure contract-to-contract events with no reference to classic accounts are not included,
//	as there is no explicit model in horizon for contract addresses yet.
type InvokeHostFunction struct {
	Base
	Function            string                       `json:"function"`
	Parameters          []HostFunctionParameter      `json:"parameters"`
	Address             string                       `json:"address"`
	Salt                string                       `json:"salt"`
	AssetBalanceChanges []AssetContractBalanceChange `json:"asset_balance_changes"`
}

// InvokeHostFunction parameter model, intentionally simplified, Value
// just contains a base64 encoded string of the ScVal xdr serialization.
type HostFunctionParameter struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

// ExtendFootprintTtl is the json resource representing a single ExtendFootprintTtlOp.
// The model for ExtendFootprintTtl assimilates ExtendFootprintTtlOp, but is simplified.
type ExtendFootprintTtl struct {
	Base
	ExtendTo uint32 `json:"extend_to"`
}

// RestoreFootprint is the json resource representing a single RestoreFootprint.
type RestoreFootprint struct {
	Base
}

// Type   - refers to the source SAC Event
//
//	it can only be one of 'transfer', 'mint', 'clawback' or 'burn'
//
// From   - this is classic account that asset balance was changed,
//
//	or absent if not applicable for function
//
// To     - this is the classic account that asset balance was changed,
//
//	         or absent if not applicable for function
//
//		for asset contract event type, it can be absent such as 'burn'
//
// Amount - expressed as a signed decimal to 7 digits precision.
// Asset  - the classic asset expressed as issuer and code.
// DestinationMuxedId - If the event's destination was a muxed address, then include the uint64 memoId in the response as a string
type AssetContractBalanceChange struct {
	base.Asset
	Type               string `json:"type"`
	From               string `json:"from,omitempty"`
	To                 string `json:"to,omitempty"`
	Amount             string `json:"amount"`
	DestinationMuxedId string `json:"destination_muxed_id,omitempty"`
}

// Operation interface contains methods implemented by the operation types
type Operation interface {
	GetBase() Base
	PagingToken() string
	GetType() string
	GetID() string
	GetTransactionHash() string
	IsTransactionSuccessful() bool
}

// GetBase returns the base object of operation
func (base Base) GetBase() Base {
	return base
}

// GetType returns the type of operation
func (base Base) GetType() string {
	return base.Type
}

// GetTypeI returns the ID of type of operation
func (base Base) GetTypeI() int32 {
	return base.TypeI
}

func (base Base) GetID() string {
	return base.ID
}

func (base Base) GetTransactionHash() string {
	return base.TransactionHash
}

func (base Base) IsTransactionSuccessful() bool {
	return base.TransactionSuccessful
}

// OperationsPage is the json resource representing a page of operations.
// OperationsPage.Record can contain various operation types.
type OperationsPage struct {
	Links    hal.Links `json:"_links"`
	Embedded struct {
		Records []Operation
	} `json:"_embedded"`
}

func (ops *OperationsPage) UnmarshalJSON(data []byte) error {
	var opsPage struct {
		Links    hal.Links `json:"_links"`
		Embedded struct {
			Records []interface{}
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(data, &opsPage); err != nil {
		return err
	}

	for _, j := range opsPage.Embedded.Records {
		var b Base
		dataString, err := json.Marshal(j)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(dataString, &b); err != nil {
			return err
		}

		op, err := UnmarshalOperation(b.TypeI, dataString)
		if err != nil {
			return err
		}

		ops.Embedded.Records = append(ops.Embedded.Records, op)
	}

	ops.Links = opsPage.Links
	return nil
}

// UnmarshalOperation decodes responses to the correct operation struct
func UnmarshalOperation(operationTypeID int32, dataString []byte) (ops Operation, err error) {
	switch xdr.OperationType(operationTypeID) {
	case xdr.OperationTypeCreateAccount:
		var op CreateAccount
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypePathPaymentStrictReceive:
		var op PathPayment
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypePayment:
		var op Payment
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeManageSellOffer:
		var op ManageSellOffer
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeCreatePassiveSellOffer:
		var op CreatePassiveSellOffer
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeSetOptions:
		var op SetOptions
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeChangeTrust:
		var op ChangeTrust
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeAllowTrust:
		var op AllowTrust
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeAccountMerge:
		var op AccountMerge
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeInflation:
		var op Inflation
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeManageData:
		var op ManageData
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeBumpSequence:
		var op BumpSequence
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeManageBuyOffer:
		var op ManageBuyOffer
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypePathPaymentStrictSend:
		var op PathPaymentStrictSend
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeCreateClaimableBalance:
		var op CreateClaimableBalance
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeClaimClaimableBalance:
		var op ClaimClaimableBalance
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeBeginSponsoringFutureReserves:
		var op BeginSponsoringFutureReserves
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeEndSponsoringFutureReserves:
		var op EndSponsoringFutureReserves
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeRevokeSponsorship:
		var op RevokeSponsorship
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeClawback:
		var op Clawback
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeClawbackClaimableBalance:
		var op ClawbackClaimableBalance
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeSetTrustLineFlags:
		var op SetTrustLineFlags
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeLiquidityPoolDeposit:
		var op LiquidityPoolDeposit
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeLiquidityPoolWithdraw:
		var op LiquidityPoolWithdraw
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeInvokeHostFunction:
		var op InvokeHostFunction
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeExtendFootprintTtl:
		var op ExtendFootprintTtl
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	case xdr.OperationTypeRestoreFootprint:
		var op RestoreFootprint
		if err = json.Unmarshal(dataString, &op); err != nil {
			return
		}
		ops = op
	default:
		err = errors.New("Invalid operation format, unable to unmarshal json response")
	}

	return
}
