package effects

import (
	"encoding/json"
	"time"

	"github.com/stellar/go-stellar-sdk/protocols/horizon/base"
	"github.com/stellar/go-stellar-sdk/support/render/hal"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Peter 30-04-2019: this is copied from the history package "github.com/stellar/go-stellar-sdk/services/horizon/internal/db2/history"
// Could not import this because internal package imports must share the same path prefix as the importer.
// Maybe this should be housed here and imported into internal packages?

// EffectType is the numeric type for an effect
type EffectType int

const (
	// account effects

	// EffectAccountCreated effects occur when a new account is created
	EffectAccountCreated EffectType = 0 // from create_account

	// EffectAccountRemoved effects occur when one account is merged into another
	EffectAccountRemoved EffectType = 1 // from merge_account

	// EffectAccountCredited effects occur when an account receives some currency
	EffectAccountCredited EffectType = 2 // from create_account, payment, path_payment, merge_account

	// EffectAccountDebited effects occur when an account sends some currency
	EffectAccountDebited EffectType = 3 // from create_account, payment, path_payment, create_account

	// EffectAccountThresholdsUpdated effects occur when an account changes its
	// multisig thresholds.
	EffectAccountThresholdsUpdated EffectType = 4 // from set_options

	// EffectAccountHomeDomainUpdated effects occur when an account changes its
	// home domain.
	EffectAccountHomeDomainUpdated EffectType = 5 // from set_options

	// EffectAccountFlagsUpdated effects occur when an account changes its
	// account flags, either clearing or setting.
	EffectAccountFlagsUpdated EffectType = 6 // from set_options

	// unused
	// EffectAccountInflationDestinationUpdated effects occur when an account changes its
	// inflation destination.
	EffectAccountInflationDestinationUpdated EffectType = 7 // from set_options

	// signer effects

	// EffectSignerCreated occurs when an account gains a signer
	EffectSignerCreated EffectType = 10 // from set_options

	// EffectSignerRemoved occurs when an account loses a signer
	EffectSignerRemoved EffectType = 11 // from set_options

	// EffectSignerUpdated occurs when an account changes the weight of one of its
	// signers.
	EffectSignerUpdated EffectType = 12 // from set_options

	// trustline effects

	// EffectTrustlineCreated occurs when an account trusts an anchor
	EffectTrustlineCreated EffectType = 20 // from change_trust

	// EffectTrustlineRemoved occurs when an account removes struct by setting the
	// limit of a trustline to 0
	EffectTrustlineRemoved EffectType = 21 // from change_trust

	// EffectTrustlineUpdated occurs when an account changes a trustline's limit
	EffectTrustlineUpdated EffectType = 22 // from change_trust, allow_trust

	// Deprecated: use EffectTrustlineFlagsUpdated instead
	// EffectTrustlineAuthorized occurs when an anchor has AUTH_REQUIRED flag set
	// to true and it authorizes another account's trustline
	EffectTrustlineAuthorized EffectType = 23 // from allow_trust

	// Deprecated: use EffectTrustlineFlagsUpdated instead
	// EffectTrustlineDeauthorized occurs when an anchor revokes access to a asset
	// it issues.
	EffectTrustlineDeauthorized EffectType = 24 // from allow_trust

	// Deprecated: use EffectTrustlineFlagsUpdated instead
	// EffectTrustlineAuthorizedToMaintainLiabilities occurs when an anchor has AUTH_REQUIRED flag set
	// to true and it authorizes another account's trustline to maintain liabilities
	EffectTrustlineAuthorizedToMaintainLiabilities EffectType = 25 // from allow_trust

	// EffectTrustlineFlagsUpdated effects occur when a TrustLine changes its
	// flags, either clearing or setting.
	EffectTrustlineFlagsUpdated EffectType = 26 // from set_trust_line flags

	// trading effects

	// unused
	// EffectOfferCreated occurs when an account offers to trade an asset
	// EffectOfferCreated EffectType = 30 // from manage_offer, creat_passive_offer
	// EffectOfferRemoved occurs when an account removes an offer
	// EffectOfferRemoved EffectType = 31 // from manage_offer, creat_passive_offer, path_payment
	// EffectOfferUpdated occurs when an offer is updated by the offering account.
	// EffectOfferUpdated EffectType = 32 // from manage_offer, creat_passive_offer, path_payment

	// EffectTrade occurs when a trade is initiated because of a path payment or
	// offer operation.
	EffectTrade EffectType = 33 // from manage_offer, creat_passive_offer, path_payment

	// data effects

	// EffectDataCreated occurs when an account gets a new data field
	EffectDataCreated EffectType = 40 // from manage_data

	// EffectDataRemoved occurs when an account removes a data field
	EffectDataRemoved EffectType = 41 // from manage_data

	// EffectDataUpdated occurs when an account changes a data field's value
	EffectDataUpdated EffectType = 42 // from manage_data

	// EffectSequenceBumped occurs when an account bumps their sequence number
	EffectSequenceBumped EffectType = 43 // from bump_sequence

	// claimable balance effects

	// EffectClaimableBalanceCreated occurs when a claimable balance is created
	EffectClaimableBalanceCreated EffectType = 50 // from create_claimable_balance

	// EffectClaimableBalanceClaimantCreated occurs when a claimable balance claimant is created
	EffectClaimableBalanceClaimantCreated EffectType = 51 // from create_claimable_balance

	// EffectClaimableBalanceClaimed occurs when a claimable balance is claimed
	EffectClaimableBalanceClaimed EffectType = 52 // from claim_claimable_balance

	// sponsorship effects

	// EffectAccountSponsorshipCreated occurs when an account ledger entry is sponsored
	EffectAccountSponsorshipCreated EffectType = 60 // from create_account

	// EffectAccountSponsorshipUpdated occurs when the sponsoring of an account ledger entry is updated
	EffectAccountSponsorshipUpdated EffectType = 61 // from revoke_sponsorship

	// EffectAccountSponsorshipRemoved occurs when the sponsorship of an account ledger entry is removed
	EffectAccountSponsorshipRemoved EffectType = 62 // from revoke_sponsorship

	// EffectTrustlineSponsorshipCreated occurs when a trustline ledger entry is sponsored
	EffectTrustlineSponsorshipCreated EffectType = 63 // from change_trust

	// EffectTrustlineSponsorshipUpdated occurs when the sponsoring of a trustline ledger entry is updated
	EffectTrustlineSponsorshipUpdated EffectType = 64 // from revoke_sponsorship

	// EffectTrustlineSponsorshipRemoved occurs when the sponsorship of a trustline ledger entry is removed
	EffectTrustlineSponsorshipRemoved EffectType = 65 // from revoke_sponsorship

	// EffectDataSponsorshipCreated occurs when a trustline ledger entry is sponsored
	EffectDataSponsorshipCreated EffectType = 66 // from manage_data

	// EffectDataSponsorshipUpdated occurs when the sponsoring of a trustline ledger entry is updated
	EffectDataSponsorshipUpdated EffectType = 67 // from revoke_sponsorship

	// EffectDataSponsorshipRemoved occurs when the sponsorship of a trustline ledger entry is removed
	EffectDataSponsorshipRemoved EffectType = 68 // from revoke_sponsorship

	// EffectClaimableBalanceSponsorshipCreated occurs when a claimable balance ledger entry is sponsored
	EffectClaimableBalanceSponsorshipCreated EffectType = 69 // from create_claimable_balance

	// EffectClaimableBalanceSponsorshipUpdated occurs when the sponsoring of a claimable balance ledger entry
	// is updated
	EffectClaimableBalanceSponsorshipUpdated EffectType = 70 // from revoke_sponsorship

	// EffectClaimableBalanceSponsorshipRemoved occurs when the sponsorship of a claimable balance ledger entry
	// is removed
	EffectClaimableBalanceSponsorshipRemoved EffectType = 71 // from claim_claimable_balance

	// EffectSignerSponsorshipCreated occurs when the sponsorship of a signer is created
	EffectSignerSponsorshipCreated EffectType = 72 // from set_options

	// EffectSignerSponsorshipUpdated occurs when the sponsorship of a signer is updated
	EffectSignerSponsorshipUpdated EffectType = 73 // from revoke_sponsorship

	// EffectSignerSponsorshipRemoved occurs when the sponsorship of a signer is removed
	EffectSignerSponsorshipRemoved EffectType = 74 // from revoke_sponsorship

	// EffectClaimableBalanceClawedBack occurs when a claimable balance is clawed back
	EffectClaimableBalanceClawedBack EffectType = 80 // from clawback_claimable_balance

	// EffectLiquidityPoolDeposited occurs when a liquidity pool incurs a deposit
	EffectLiquidityPoolDeposited EffectType = 90 // from liquidity_pool_deposit

	// EffectLiquidityPoolWithdrew occurs when a liquidity pool incurs a withdrawal
	EffectLiquidityPoolWithdrew EffectType = 91 // from liquidity_pool_withdraw

	// EffectLiquidityPoolTrade occurs when a trade happens in a liquidity pool
	EffectLiquidityPoolTrade EffectType = 92

	// EffectLiquidityPoolCreated occurs when a liquidity pool is created
	EffectLiquidityPoolCreated EffectType = 93 // from change_trust

	// EffectLiquidityPoolRemoved occurs when a liquidity pool is removed
	EffectLiquidityPoolRemoved EffectType = 94 // from change_trust

	// EffectLiquidityPoolRevoked occurs when a liquidity pool is revoked
	EffectLiquidityPoolRevoked EffectType = 95 // from change_trust_line_flags and allow_trust

	// EffectContractCredited effects occur when a contract receives some
	// currency from SAC events involving transfers, mints, and burns.
	// https://github.com/stellar/rs-soroban-env/blob/5695440da452837555d8f7f259cc33341fdf07b0/soroban-env-host/src/native_contract/token/contract.rs#L51-L63
	EffectContractCredited EffectType = 96

	// EffectContractDebited effects occur when a contract sends some currency
	// from SAC events involving transfers, mints, and burns.
	// https://github.com/stellar/rs-soroban-env/blob/5695440da452837555d8f7f259cc33341fdf07b0/soroban-env-host/src/native_contract/token/contract.rs#L51-L63
	EffectContractDebited EffectType = 97
)

// Peter 30-04-2019: this is copied from the resourcadapter package
// "github.com/stellar/go-stellar-sdk/services/horizon/internal/resourceadapter"
// Could not import this because internal package imports must share the same path prefix as the importer.

// EffectTypeNames stores a map of effect type ID and names
var EffectTypeNames = map[EffectType]string{
	EffectAccountCreated:                           "account_created",
	EffectAccountRemoved:                           "account_removed",
	EffectAccountCredited:                          "account_credited",
	EffectAccountDebited:                           "account_debited",
	EffectAccountThresholdsUpdated:                 "account_thresholds_updated",
	EffectAccountHomeDomainUpdated:                 "account_home_domain_updated",
	EffectAccountFlagsUpdated:                      "account_flags_updated",
	EffectAccountInflationDestinationUpdated:       "account_inflation_destination_updated",
	EffectSignerCreated:                            "signer_created",
	EffectSignerRemoved:                            "signer_removed",
	EffectSignerUpdated:                            "signer_updated",
	EffectTrustlineCreated:                         "trustline_created",
	EffectTrustlineRemoved:                         "trustline_removed",
	EffectTrustlineUpdated:                         "trustline_updated",
	EffectTrustlineAuthorized:                      "trustline_authorized",
	EffectTrustlineAuthorizedToMaintainLiabilities: "trustline_authorized_to_maintain_liabilities",
	EffectTrustlineDeauthorized:                    "trustline_deauthorized",
	EffectTrustlineFlagsUpdated:                    "trustline_flags_updated",
	// unused
	// EffectOfferCreated:                             "offer_created",
	// EffectOfferRemoved:                             "offer_removed",
	// EffectOfferUpdated:                             "offer_updated",
	EffectTrade:                              "trade",
	EffectDataCreated:                        "data_created",
	EffectDataRemoved:                        "data_removed",
	EffectDataUpdated:                        "data_updated",
	EffectSequenceBumped:                     "sequence_bumped",
	EffectClaimableBalanceCreated:            "claimable_balance_created",
	EffectClaimableBalanceClaimed:            "claimable_balance_claimed",
	EffectClaimableBalanceClaimantCreated:    "claimable_balance_claimant_created",
	EffectAccountSponsorshipCreated:          "account_sponsorship_created",
	EffectAccountSponsorshipUpdated:          "account_sponsorship_updated",
	EffectAccountSponsorshipRemoved:          "account_sponsorship_removed",
	EffectTrustlineSponsorshipCreated:        "trustline_sponsorship_created",
	EffectTrustlineSponsorshipUpdated:        "trustline_sponsorship_updated",
	EffectTrustlineSponsorshipRemoved:        "trustline_sponsorship_removed",
	EffectDataSponsorshipCreated:             "data_sponsorship_created",
	EffectDataSponsorshipUpdated:             "data_sponsorship_updated",
	EffectDataSponsorshipRemoved:             "data_sponsorship_removed",
	EffectClaimableBalanceSponsorshipCreated: "claimable_balance_sponsorship_created",
	EffectClaimableBalanceSponsorshipUpdated: "claimable_balance_sponsorship_updated",
	EffectClaimableBalanceSponsorshipRemoved: "claimable_balance_sponsorship_removed",
	EffectSignerSponsorshipCreated:           "signer_sponsorship_created",
	EffectSignerSponsorshipUpdated:           "signer_sponsorship_updated",
	EffectSignerSponsorshipRemoved:           "signer_sponsorship_removed",
	EffectClaimableBalanceClawedBack:         "claimable_balance_clawed_back",
	EffectLiquidityPoolDeposited:             "liquidity_pool_deposited",
	EffectLiquidityPoolWithdrew:              "liquidity_pool_withdrew",
	EffectLiquidityPoolTrade:                 "liquidity_pool_trade",
	EffectLiquidityPoolCreated:               "liquidity_pool_created",
	EffectLiquidityPoolRemoved:               "liquidity_pool_removed",
	EffectLiquidityPoolRevoked:               "liquidity_pool_revoked",
	EffectContractCredited:                   "contract_credited",
	EffectContractDebited:                    "contract_debited",
}

// Base provides the common structure for any effect resource effect.
type Base struct {
	Links struct {
		Operation hal.Link `json:"operation"`
		Succeeds  hal.Link `json:"succeeds"`
		Precedes  hal.Link `json:"precedes"`
	} `json:"_links"`

	ID              string    `json:"id"`
	PT              string    `json:"paging_token"`
	Account         string    `json:"account"`
	AccountMuxed    string    `json:"account_muxed,omitempty"`
	AccountMuxedID  uint64    `json:"account_muxed_id,omitempty,string"`
	Type            string    `json:"type"`
	TypeI           int32     `json:"type_i"`
	LedgerCloseTime time.Time `json:"created_at"`
}

// PagingToken implements `hal.Pageable` and Effect
func (b Base) PagingToken() string {
	return b.PT
}

type AccountCreated struct {
	Base
	StartingBalance string `json:"starting_balance"`
}

type AccountCredited struct {
	Base
	base.Asset
	Amount string `json:"amount"`
}

type AccountDebited struct {
	Base
	base.Asset
	Amount string `json:"amount"`
}

type ContractCredited struct {
	Base
	base.Asset
	Contract string `json:"contract"`
	Amount   string `json:"amount"`
}

type ContractDebited struct {
	Base
	base.Asset
	Contract string `json:"contract"`
	Amount   string `json:"amount"`
}

type AccountThresholdsUpdated struct {
	Base
	LowThreshold  int32 `json:"low_threshold"`
	MedThreshold  int32 `json:"med_threshold"`
	HighThreshold int32 `json:"high_threshold"`
}

type AccountHomeDomainUpdated struct {
	Base
	HomeDomain string `json:"home_domain"`
}

type AccountFlagsUpdated struct {
	Base
	AuthRequired  *bool `json:"auth_required_flag,omitempty"`
	AuthRevokable *bool `json:"auth_revokable_flag,omitempty"`
}

type DataCreated struct {
	Base
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DataUpdated struct {
	Base
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DataRemoved struct {
	Base
	Name string `json:"name"`
}

type SequenceBumped struct {
	Base
	NewSeq int64 `json:"new_seq,string"`
}

type SignerCreated struct {
	Base
	Weight    int32  `json:"weight"`
	PublicKey string `json:"public_key"`
	Key       string `json:"key"`
}

type SignerRemoved struct {
	Base
	Weight    int32  `json:"weight"`
	PublicKey string `json:"public_key"`
	Key       string `json:"key"`
}

type SignerUpdated struct {
	Base
	Weight    int32  `json:"weight"`
	PublicKey string `json:"public_key"`
	Key       string `json:"key"`
}

type TrustlineCreated struct {
	Base
	base.LiquidityPoolOrAsset
	Limit string `json:"limit"`
}

type TrustlineRemoved struct {
	Base
	base.LiquidityPoolOrAsset
	Limit string `json:"limit"`
}

type TrustlineUpdated struct {
	Base
	base.LiquidityPoolOrAsset
	Limit string `json:"limit"`
}

// Deprecated: use TrustlineFlagsUpdated instead
type TrustlineAuthorized struct {
	Base
	Trustor   string `json:"trustor"`
	AssetType string `json:"asset_type"`
	AssetCode string `json:"asset_code,omitempty"`
}

// Deprecated: use TrustlineFlagsUpdated instead
type TrustlineAuthorizedToMaintainLiabilities struct {
	Base
	Trustor   string `json:"trustor"`
	AssetType string `json:"asset_type"`
	AssetCode string `json:"asset_code,omitempty"`
}

// Deprecated: use TrustlineFlagsUpdated instead
type TrustlineDeauthorized struct {
	Base
	Trustor   string `json:"trustor"`
	AssetType string `json:"asset_type"`
	AssetCode string `json:"asset_code,omitempty"`
}

type Trade struct {
	Base
	Seller            string `json:"seller"`
	SellerMuxed       string `json:"seller_muxed,omitempty"`
	SellerMuxedID     uint64 `json:"seller_muxed_id,omitempty,string"`
	OfferID           int64  `json:"offer_id,string"`
	SoldAmount        string `json:"sold_amount"`
	SoldAssetType     string `json:"sold_asset_type"`
	SoldAssetCode     string `json:"sold_asset_code,omitempty"`
	SoldAssetIssuer   string `json:"sold_asset_issuer,omitempty"`
	BoughtAmount      string `json:"bought_amount"`
	BoughtAssetType   string `json:"bought_asset_type"`
	BoughtAssetCode   string `json:"bought_asset_code,omitempty"`
	BoughtAssetIssuer string `json:"bought_asset_issuer,omitempty"`
}

type ClaimableBalanceCreated struct {
	Base
	Asset     string `json:"asset"`
	BalanceID string `json:"balance_id"`
	Amount    string `json:"amount"`
}

type ClaimableBalanceClaimed struct {
	Base
	Asset     string `json:"asset"`
	BalanceID string `json:"balance_id"`
	Amount    string `json:"amount"`
}

type ClaimableBalanceClaimantCreated struct {
	Base
	Asset     string             `json:"asset"`
	BalanceID string             `json:"balance_id"`
	Amount    string             `json:"amount"`
	Predicate xdr.ClaimPredicate `json:"predicate"`
}

type AccountSponsorshipCreated struct {
	Base
	Sponsor string `json:"sponsor"`
}

type AccountSponsorshipUpdated struct {
	Base
	FormerSponsor string `json:"former_sponsor"`
	NewSponsor    string `json:"new_sponsor"`
}

type AccountSponsorshipRemoved struct {
	Base
	FormerSponsor string `json:"former_sponsor"`
}

type TrustlineSponsorshipCreated struct {
	Base
	Type            string `json:"asset_type"`
	Asset           string `json:"asset,omitempty"`
	LiquidityPoolID string `json:"liquidity_pool_id,omitempty"`
	Sponsor         string `json:"sponsor"`
}

type TrustlineSponsorshipUpdated struct {
	Base
	Type            string `json:"asset_type"`
	Asset           string `json:"asset,omitempty"`
	LiquidityPoolID string `json:"liquidity_pool_id,omitempty"`
	FormerSponsor   string `json:"former_sponsor"`
	NewSponsor      string `json:"new_sponsor"`
}

type TrustlineSponsorshipRemoved struct {
	Base
	Type            string `json:"asset_type"`
	Asset           string `json:"asset,omitempty"`
	LiquidityPoolID string `json:"liquidity_pool_id,omitempty"`
	FormerSponsor   string `json:"former_sponsor"`
}

type DataSponsorshipCreated struct {
	Base
	DataName string `json:"data_name"`
	Sponsor  string `json:"sponsor"`
}

type DataSponsorshipUpdated struct {
	Base
	DataName      string `json:"data_name"`
	FormerSponsor string `json:"former_sponsor"`
	NewSponsor    string `json:"new_sponsor"`
}

type DataSponsorshipRemoved struct {
	Base
	DataName      string `json:"data_name"`
	FormerSponsor string `json:"former_sponsor"`
}

type ClaimableBalanceSponsorshipCreated struct {
	Base
	BalanceID string `json:"balance_id"`
	Sponsor   string `json:"sponsor"`
}

type ClaimableBalanceSponsorshipUpdated struct {
	Base
	BalanceID     string `json:"balance_id"`
	FormerSponsor string `json:"former_sponsor"`
	NewSponsor    string `json:"new_sponsor"`
}

type ClaimableBalanceSponsorshipRemoved struct {
	Base
	BalanceID     string `json:"balance_id"`
	FormerSponsor string `json:"former_sponsor"`
}

type SignerSponsorshipCreated struct {
	Base
	Signer  string `json:"signer"`
	Sponsor string `json:"sponsor"`
}

type SignerSponsorshipUpdated struct {
	Base
	Signer        string `json:"signer"`
	FormerSponsor string `json:"former_sponsor"`
	NewSponsor    string `json:"new_sponsor"`
}

type SignerSponsorshipRemoved struct {
	Base
	Signer        string `json:"signer"`
	FormerSponsor string `json:"former_sponsor"`
}

type ClaimableBalanceClawedBack struct {
	Base
	BalanceID string `json:"balance_id"`
}

type TrustlineFlagsUpdated struct {
	Base
	base.Asset
	Trustor                         string `json:"trustor"`
	Authorized                      *bool  `json:"authorized_flag,omitempty"`
	AuthorizedToMaintainLiabilities *bool  `json:"authorized_to_maintain_liabilites_flag,omitempty"`
	ClawbackEnabled                 *bool  `json:"clawback_enabled_flag,omitempty"`
}

type LiquidityPool struct {
	ID              string             `json:"id"`
	FeeBP           uint32             `json:"fee_bp"`
	Type            string             `json:"type"`
	TotalTrustlines uint64             `json:"total_trustlines,string"`
	TotalShares     string             `json:"total_shares"`
	Reserves        []base.AssetAmount `json:"reserves"`
}

type LiquidityPoolDeposited struct {
	Base
	LiquidityPool     LiquidityPool      `json:"liquidity_pool"`
	ReservesDeposited []base.AssetAmount `json:"reserves_deposited"`
	SharesReceived    string             `json:"shares_received"`
}

type LiquidityPoolWithdrew struct {
	Base
	LiquidityPool    LiquidityPool      `json:"liquidity_pool"`
	ReservesReceived []base.AssetAmount `json:"reserves_received"`
	SharesRedeemed   string             `json:"shares_redeemed"`
}

type LiquidityPoolTrade struct {
	Base
	LiquidityPool LiquidityPool    `json:"liquidity_pool"`
	Sold          base.AssetAmount `json:"sold"`
	Bought        base.AssetAmount `json:"bought"`
}

type LiquidityPoolCreated struct {
	Base
	LiquidityPool LiquidityPool `json:"liquidity_pool"`
}

type LiquidityPoolRemoved struct {
	Base
	LiquidityPoolID string `json:"liquidity_pool_id"`
}

type LiquidityPoolClaimableAssetAmount struct {
	Asset              string `json:"asset"`
	Amount             string `json:"amount"`
	ClaimableBalanceID string `json:"claimable_balance_id"`
}

type LiquidityPoolRevoked struct {
	Base
	LiquidityPool   LiquidityPool                       `json:"liquidity_pool"`
	ReservesRevoked []LiquidityPoolClaimableAssetAmount `json:"reserves_revoked"`
	SharesRevoked   string                              `json:"shares_revoked"`
}

// Effect contains methods that are implemented by all effect types.
type Effect interface {
	PagingToken() string
	GetType() string
	GetID() string
	GetAccount() string
}

// GetType implements Effect
func (b Base) GetType() string {
	return b.Type
}

// GetID implements Effect
func (b Base) GetID() string {
	return b.ID
}

// GetAccount implements Effect
func (b Base) GetAccount() string {
	return b.Account
}

// EffectsPage contains page of effects returned by Horizon.
type EffectsPage struct {
	Links    hal.Links `json:"_links"`
	Embedded struct {
		Records []Effect
	} `json:"_embedded"`
}

// UnmarshalJSON is the custom unmarshal method for EffectsPage
func (effects *EffectsPage) UnmarshalJSON(data []byte) error {
	var effectsPage struct {
		Links    hal.Links `json:"_links"`
		Embedded struct {
			Records []interface{}
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(data, &effectsPage); err != nil {
		return err
	}

	for _, j := range effectsPage.Embedded.Records {
		var b Base
		dataString, err := json.Marshal(j)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(dataString, &b); err != nil {
			return err
		}

		ef, err := UnmarshalEffect(b.Type, dataString)
		if err != nil {
			return err
		}

		effects.Embedded.Records = append(effects.Embedded.Records, ef)
	}

	effects.Links = effectsPage.Links
	return nil
}

// UnmarshalEffect decodes responses to the correct effect struct
func UnmarshalEffect(effectType string, dataString []byte) (effects Effect, err error) {
	switch effectType {
	case EffectTypeNames[EffectAccountCreated]:
		var effect AccountCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectAccountCredited]:
		var effect AccountCredited
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectAccountDebited]:
		var effect AccountDebited
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectAccountThresholdsUpdated]:
		var effect AccountThresholdsUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectAccountHomeDomainUpdated]:
		var effect AccountHomeDomainUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectAccountFlagsUpdated]:
		var effect AccountFlagsUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectSequenceBumped]:
		var effect SequenceBumped
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectSignerCreated]:
		var effect SignerCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectSignerRemoved]:
		var effect SignerRemoved
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectSignerUpdated]:
		var effect SignerUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectTrustlineAuthorized]:
		var effect TrustlineAuthorized
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectTrustlineAuthorizedToMaintainLiabilities]:
		var effect TrustlineAuthorizedToMaintainLiabilities
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectTrustlineCreated]:
		var effect TrustlineCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectTrustlineDeauthorized]:
		var effect TrustlineDeauthorized
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectTrustlineFlagsUpdated]:
		var effect TrustlineFlagsUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectTrustlineRemoved]:
		var effect TrustlineRemoved
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectTrustlineUpdated]:
		var effect TrustlineUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectTrade]:
		var effect Trade
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectDataCreated]:
		var effect DataCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectDataUpdated]:
		var effect DataUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectDataRemoved]:
		var effect DataRemoved
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectClaimableBalanceCreated]:
		var effect ClaimableBalanceCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectClaimableBalanceClaimed]:
		var effect ClaimableBalanceClaimed
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectClaimableBalanceClaimantCreated]:
		var effect ClaimableBalanceClaimantCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectAccountSponsorshipCreated]:
		var effect AccountSponsorshipCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectAccountSponsorshipUpdated]:
		var effect AccountSponsorshipUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectAccountSponsorshipRemoved]:
		var effect AccountSponsorshipRemoved
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectTrustlineSponsorshipCreated]:
		var effect TrustlineSponsorshipCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectTrustlineSponsorshipUpdated]:
		var effect TrustlineSponsorshipUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectTrustlineSponsorshipRemoved]:
		var effect TrustlineSponsorshipRemoved
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectDataSponsorshipCreated]:
		var effect DataSponsorshipCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectDataSponsorshipUpdated]:
		var effect DataSponsorshipUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectDataSponsorshipRemoved]:
		var effect DataSponsorshipRemoved
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectClaimableBalanceSponsorshipCreated]:
		var effect ClaimableBalanceSponsorshipCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectClaimableBalanceSponsorshipUpdated]:
		var effect ClaimableBalanceSponsorshipUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectClaimableBalanceSponsorshipRemoved]:
		var effect ClaimableBalanceSponsorshipRemoved
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectSignerSponsorshipCreated]:
		var effect SignerSponsorshipCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectSignerSponsorshipUpdated]:
		var effect SignerSponsorshipUpdated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectSignerSponsorshipRemoved]:
		var effect SignerSponsorshipRemoved
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectClaimableBalanceClawedBack]:
		var effect ClaimableBalanceClawedBack
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectLiquidityPoolDeposited]:
		var effect LiquidityPoolDeposited
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectLiquidityPoolWithdrew]:
		var effect LiquidityPoolWithdrew
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectLiquidityPoolTrade]:
		var effect LiquidityPoolTrade
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectLiquidityPoolCreated]:
		var effect LiquidityPoolCreated
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectLiquidityPoolRemoved]:
		var effect LiquidityPoolRemoved
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectLiquidityPoolRevoked]:
		var effect LiquidityPoolRevoked
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectContractCredited]:
		var effect ContractCredited
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	case EffectTypeNames[EffectContractDebited]:
		var effect ContractDebited
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	default:
		var effect Base
		if err = json.Unmarshal(dataString, &effect); err != nil {
			return
		}
		effects = effect
	}
	return
}

// interface implementations
var _ base.Rehydratable = &SignerCreated{}
var _ base.Rehydratable = &SignerRemoved{}
var _ base.Rehydratable = &SignerUpdated{}
