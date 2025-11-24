// Package horizon contains the type definitions for all of horizon's
// response resources.
package horizon

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/stellar/go-stellar-sdk/protocols/horizon/base"
	proto "github.com/stellar/go-stellar-sdk/protocols/stellarcore"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/support/render/hal"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// KeyTypeNames maps from strkey version bytes into json string values to use in
// horizon responses.
var KeyTypeNames = map[strkey.VersionByte]string{
	strkey.VersionByteAccountID:     "ed25519_public_key",
	strkey.VersionByteSeed:          "ed25519_secret_seed",
	strkey.VersionByteMuxedAccount:  "muxed_account",
	strkey.VersionByteHashTx:        "preauth_tx",
	strkey.VersionByteHashX:         "sha256_hash",
	strkey.VersionByteSignedPayload: "ed25519_signed_payload",
}

var coreStatusToHTTPStatus = map[string]int{
	proto.TXStatusPending:       http.StatusCreated,
	proto.TXStatusDuplicate:     http.StatusConflict,
	proto.TXStatusTryAgainLater: http.StatusServiceUnavailable,
	proto.TXStatusError:         http.StatusBadRequest,
}

// Account is the summary of an account
type Account struct {
	Links struct {
		Self         hal.Link `json:"self"`
		Transactions hal.Link `json:"transactions"`
		Operations   hal.Link `json:"operations"`
		Payments     hal.Link `json:"payments"`
		Effects      hal.Link `json:"effects"`
		Offers       hal.Link `json:"offers"`
		Trades       hal.Link `json:"trades"`
		Data         hal.Link `json:"data"`
	} `json:"_links"`

	ID                   string            `json:"id"`
	AccountID            string            `json:"account_id"`
	Sequence             int64             `json:"sequence,string"`
	SequenceLedger       uint32            `json:"sequence_ledger,omitempty"`
	SequenceTime         string            `json:"sequence_time,omitempty"`
	SubentryCount        int32             `json:"subentry_count"`
	InflationDestination string            `json:"inflation_destination,omitempty"`
	HomeDomain           string            `json:"home_domain,omitempty"`
	LastModifiedLedger   uint32            `json:"last_modified_ledger"`
	LastModifiedTime     *time.Time        `json:"last_modified_time"`
	Thresholds           AccountThresholds `json:"thresholds"`
	Flags                AccountFlags      `json:"flags"`
	Balances             []Balance         `json:"balances"`
	Signers              []Signer          `json:"signers"`
	Data                 map[string]string `json:"data"`
	NumSponsoring        uint32            `json:"num_sponsoring"`
	NumSponsored         uint32            `json:"num_sponsored"`
	Sponsor              string            `json:"sponsor,omitempty"`
	PT                   string            `json:"paging_token"`
}

// PagingToken implementation for hal.Pageable
func (res Account) PagingToken() string {
	return res.PT
}

// GetAccountID returns the Stellar account ID. This is to satisfy the
// Account interface of txnbuild.
func (a Account) GetAccountID() string {
	return a.AccountID
}

// GetNativeBalance returns the native balance of the account
func (a Account) GetNativeBalance() (string, error) {
	for _, balance := range a.Balances {
		if balance.Asset.Type == "native" {
			return balance.Balance, nil
		}
	}

	return "0", errors.New("account does not have a native balance")
}

// GetCreditBalance returns the balance for given code and issuer
func (a Account) GetCreditBalance(code string, issuer string) string {
	for _, balance := range a.Balances {
		if balance.Asset.Code == code && balance.Asset.Issuer == issuer {
			return balance.Balance
		}
	}

	return "0"
}

// GetSequenceNumber returns the sequence number of the account,
// and returns it as a 64-bit integer.
// TODO: since Account.Sequence was changed to int64, error is no longer needed.
func (a Account) GetSequenceNumber() (int64, error) {
	return a.Sequence, nil
}

// IncrementSequenceNumber increments the internal record of the account's sequence
// number by 1. This is typically used after a transaction build so that the next
// transaction to be built will be valid.
func (a *Account) IncrementSequenceNumber() (int64, error) {
	if a.Sequence == math.MaxInt64 {
		return 0, fmt.Errorf("sequence cannot be increased, it already reached MaxInt64 (%d)", int64(math.MaxInt64))
	}
	a.Sequence++
	return a.Sequence, nil
}

// MustGetData returns decoded value for a given key. If the key does
// not exist, empty slice will be returned. If there is an error
// decoding a value, it will panic.
func (a *Account) MustGetData(key string) []byte {
	bytes, err := a.GetData(key)
	if err != nil {
		panic(err)
	}
	return bytes
}

// GetData returns decoded value for a given key. If the key does
// not exist, empty slice will be returned.
func (a *Account) GetData(key string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(a.Data[key])
}

// SignerSummary returns a map of signer's keys to weights.
func (a *Account) SignerSummary() map[string]int32 {
	m := map[string]int32{}
	for _, s := range a.Signers {
		m[s.Key] = s.Weight
	}
	return m
}

// AccountFlags represents the state of an account's flags
type AccountFlags struct {
	AuthRequired        bool `json:"auth_required"`
	AuthRevocable       bool `json:"auth_revocable"`
	AuthImmutable       bool `json:"auth_immutable"`
	AuthClawbackEnabled bool `json:"auth_clawback_enabled"`
}

// AccountThresholds represents an accounts "thresholds", the numerical values
// needed to satisfy the authorization of a given operation.
type AccountThresholds struct {
	LowThreshold  byte `json:"low_threshold"`
	MedThreshold  byte `json:"med_threshold"`
	HighThreshold byte `json:"high_threshold"`
}

// Asset represents a single asset
type Asset base.Asset

// AssetStat represents the statistics for a single Asset
type AssetStat struct {
	Links struct {
		Toml hal.Link `json:"toml"`
	} `json:"_links"`

	base.Asset
	PT                      string            `json:"paging_token"`
	ContractID              string            `json:"contract_id,omitempty"`
	NumClaimableBalances    int32             `json:"num_claimable_balances"`
	NumLiquidityPools       int32             `json:"num_liquidity_pools"`
	NumContracts            int32             `json:"num_contracts"`
	Accounts                AssetStatAccounts `json:"accounts"`
	ClaimableBalancesAmount string            `json:"claimable_balances_amount"`
	LiquidityPoolsAmount    string            `json:"liquidity_pools_amount"`
	ContractsAmount         string            `json:"contracts_amount"`
	Balances                AssetStatBalances `json:"balances"`
	Flags                   AccountFlags      `json:"flags"`
}

// PagingToken implementation for hal.Pageable
func (res AssetStat) PagingToken() string {
	return res.PT
}

// AssetStatBalances represents the summarized balances for a single Asset
type AssetStatBalances struct {
	Authorized                      string `json:"authorized"`
	AuthorizedToMaintainLiabilities string `json:"authorized_to_maintain_liabilities"`
	Unauthorized                    string `json:"unauthorized"`
}

// AssetStatAccounts represents the summarized acount numbers for a single Asset
type AssetStatAccounts struct {
	Authorized                      int32 `json:"authorized"`
	AuthorizedToMaintainLiabilities int32 `json:"authorized_to_maintain_liabilities"`
	Unauthorized                    int32 `json:"unauthorized"`
}

// Balance represents an account's holdings for either a single currency type or
// shares in a liquidity pool.
type Balance struct {
	Balance                           string `json:"balance"`
	LiquidityPoolId                   string `json:"liquidity_pool_id,omitempty"`
	Limit                             string `json:"limit,omitempty"`
	BuyingLiabilities                 string `json:"buying_liabilities,omitempty"`
	SellingLiabilities                string `json:"selling_liabilities,omitempty"`
	Sponsor                           string `json:"sponsor,omitempty"`
	LastModifiedLedger                uint32 `json:"last_modified_ledger,omitempty"`
	IsAuthorized                      *bool  `json:"is_authorized,omitempty"`
	IsAuthorizedToMaintainLiabilities *bool  `json:"is_authorized_to_maintain_liabilities,omitempty"`
	IsClawbackEnabled                 *bool  `json:"is_clawback_enabled,omitempty"`
	base.Asset
}

// Ledger represents a single closed ledger
type Ledger struct {
	Links struct {
		Self         hal.Link `json:"self"`
		Transactions hal.Link `json:"transactions"`
		Operations   hal.Link `json:"operations"`
		Payments     hal.Link `json:"payments"`
		Effects      hal.Link `json:"effects"`
	} `json:"_links"`
	ID                         string    `json:"id"`
	PT                         string    `json:"paging_token"`
	Hash                       string    `json:"hash"`
	PrevHash                   string    `json:"prev_hash,omitempty"`
	Sequence                   int32     `json:"sequence"`
	SuccessfulTransactionCount int32     `json:"successful_transaction_count"`
	FailedTransactionCount     *int32    `json:"failed_transaction_count"`
	OperationCount             int32     `json:"operation_count"`
	TxSetOperationCount        *int32    `json:"tx_set_operation_count"`
	ClosedAt                   time.Time `json:"closed_at"`
	TotalCoins                 string    `json:"total_coins"`
	FeePool                    string    `json:"fee_pool"`
	BaseFee                    int32     `json:"base_fee_in_stroops"`
	BaseReserve                int32     `json:"base_reserve_in_stroops"`
	MaxTxSetSize               int32     `json:"max_tx_set_size"`
	ProtocolVersion            int32     `json:"protocol_version"`
	HeaderXDR                  string    `json:"header_xdr"`
}

func (l Ledger) PagingToken() string {
	return l.PT
}

// Offer is the display form of an offer to trade currency.
type Offer struct {
	Links struct {
		Self       hal.Link `json:"self"`
		OfferMaker hal.Link `json:"offer_maker"`
	} `json:"_links"`

	ID                 int64      `json:"id,string"`
	PT                 string     `json:"paging_token"`
	Seller             string     `json:"seller"`
	Selling            Asset      `json:"selling"`
	Buying             Asset      `json:"buying"`
	Amount             string     `json:"amount"`
	PriceR             Price      `json:"price_r"`
	Price              string     `json:"price"`
	LastModifiedLedger int32      `json:"last_modified_ledger"`
	LastModifiedTime   *time.Time `json:"last_modified_time"`
	Sponsor            string     `json:"sponsor,omitempty"`
}

func (o Offer) PagingToken() string {
	return o.PT
}

// OrderBookSummary represents a snapshot summary of a given order book
type OrderBookSummary struct {
	Bids    []PriceLevel `json:"bids"`
	Asks    []PriceLevel `json:"asks"`
	Selling Asset        `json:"base"`
	Buying  Asset        `json:"counter"`
}

// Path represents a single payment path.
type Path struct {
	SourceAssetType        string  `json:"source_asset_type"`
	SourceAssetCode        string  `json:"source_asset_code,omitempty"`
	SourceAssetIssuer      string  `json:"source_asset_issuer,omitempty"`
	SourceAmount           string  `json:"source_amount"`
	DestinationAssetType   string  `json:"destination_asset_type"`
	DestinationAssetCode   string  `json:"destination_asset_code,omitempty"`
	DestinationAssetIssuer string  `json:"destination_asset_issuer,omitempty"`
	DestinationAmount      string  `json:"destination_amount"`
	Path                   []Asset `json:"path"`
}

// stub implementation to satisfy pageable interface
func (p Path) PagingToken() string {
	return ""
}

// Price represents a price for an offer
type Price base.Price

// PriceLevel represents an aggregation of offers that share a given price
type PriceLevel struct {
	PriceR Price  `json:"price_r"`
	Price  string `json:"price"`
	Amount string `json:"amount"`
}

// Root is the initial map of links into the api.
type Root struct {
	Links struct {
		Account             hal.Link  `json:"account"`
		Accounts            *hal.Link `json:"accounts,omitempty"`
		AccountTransactions hal.Link  `json:"account_transactions"`
		ClaimableBalances   *hal.Link `json:"claimable_balances"`
		Assets              hal.Link  `json:"assets"`
		Effects             hal.Link  `json:"effects"`
		FeeStats            hal.Link  `json:"fee_stats"`
		Friendbot           *hal.Link `json:"friendbot,omitempty"`
		Ledger              hal.Link  `json:"ledger"`
		Ledgers             hal.Link  `json:"ledgers"`
		LiquidityPools      *hal.Link `json:"liquidity_pools"`
		Offer               *hal.Link `json:"offer,omitempty"`
		Offers              *hal.Link `json:"offers,omitempty"`
		Operation           hal.Link  `json:"operation"`
		Operations          hal.Link  `json:"operations"`
		OrderBook           hal.Link  `json:"order_book"`
		Payments            hal.Link  `json:"payments"`
		Self                hal.Link  `json:"self"`
		StrictReceivePaths  *hal.Link `json:"strict_receive_paths"`
		StrictSendPaths     *hal.Link `json:"strict_send_paths"`
		TradeAggregations   hal.Link  `json:"trade_aggregations"`
		Trades              hal.Link  `json:"trades"`
		Transaction         hal.Link  `json:"transaction"`
		Transactions        hal.Link  `json:"transactions"`
	} `json:"_links"`

	HorizonVersion               string    `json:"horizon_version"`
	StellarCoreVersion           string    `json:"core_version"`
	IngestSequence               uint32    `json:"ingest_latest_ledger"`
	HorizonSequence              int32     `json:"history_latest_ledger"`
	HorizonLatestClosedAt        time.Time `json:"history_latest_ledger_closed_at"`
	HistoryElderSequence         int32     `json:"history_elder_ledger"`
	CoreSequence                 int32     `json:"core_latest_ledger"`
	NetworkPassphrase            string    `json:"network_passphrase"`
	CurrentProtocolVersion       int32     `json:"current_protocol_version"`
	SupportedProtocolVersion     uint32    `json:"supported_protocol_version"`
	CoreSupportedProtocolVersion int32     `json:"core_supported_protocol_version"`
}

// Signer represents one of an account's signers.
type Signer struct {
	Weight  int32  `json:"weight"`
	Key     string `json:"key"`
	Type    string `json:"type"`
	Sponsor string `json:"sponsor,omitempty"`
}

// TradePrice represents a price for a trade
type TradePrice struct {
	N int64 `json:"n,string"`
	D int64 `json:"d,string"`
}

// String returns a string representation of the trade price
func (p TradePrice) String() string {
	return big.NewRat(p.N, p.D).FloatString(7)
}

// UnmarshalJSON implements a custom unmarshaler for TradePrice
// which can handle a numerator and denominator fields which can be a string or int
func (p *TradePrice) UnmarshalJSON(data []byte) error {
	v := struct {
		N json.Number `json:"n"`
		D json.Number `json:"d"`
	}{}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}

	if v.N != "" {
		p.N, err = v.N.Int64()
		if err != nil {
			return err
		}
	}
	if v.D != "" {
		p.D, err = v.D.Int64()
		if err != nil {
			return err
		}
	}
	return nil
}

// Trade represents a horizon digested trade
type Trade struct {
	Links struct {
		Self      hal.Link `json:"self"`
		Base      hal.Link `json:"base"`
		Counter   hal.Link `json:"counter"`
		Operation hal.Link `json:"operation"`
	} `json:"_links"`

	ID                     string     `json:"id"`
	PT                     string     `json:"paging_token"`
	LedgerCloseTime        time.Time  `json:"ledger_close_time"`
	OfferID                string     `json:"offer_id,omitempty"`
	TradeType              string     `json:"trade_type"`
	LiquidityPoolFeeBP     uint32     `json:"liquidity_pool_fee_bp,omitempty"`
	BaseLiquidityPoolID    string     `json:"base_liquidity_pool_id,omitempty"`
	BaseOfferID            string     `json:"base_offer_id,omitempty"`
	BaseAccount            string     `json:"base_account,omitempty"`
	BaseAmount             string     `json:"base_amount"`
	BaseAssetType          string     `json:"base_asset_type"`
	BaseAssetCode          string     `json:"base_asset_code,omitempty"`
	BaseAssetIssuer        string     `json:"base_asset_issuer,omitempty"`
	CounterLiquidityPoolID string     `json:"counter_liquidity_pool_id,omitempty"`
	CounterOfferID         string     `json:"counter_offer_id,omitempty"`
	CounterAccount         string     `json:"counter_account,omitempty"`
	CounterAmount          string     `json:"counter_amount"`
	CounterAssetType       string     `json:"counter_asset_type"`
	CounterAssetCode       string     `json:"counter_asset_code,omitempty"`
	CounterAssetIssuer     string     `json:"counter_asset_issuer,omitempty"`
	BaseIsSeller           bool       `json:"base_is_seller"`
	Price                  TradePrice `json:"price,omitempty"`
}

// PagingToken implementation for hal.Pageable
func (res Trade) PagingToken() string {
	return res.PT
}

// TradeEffect represents a trade effect resource.
type TradeEffect struct {
	Links struct {
		Self      hal.Link `json:"self"`
		Seller    hal.Link `json:"seller"`
		Buyer     hal.Link `json:"buyer"`
		Operation hal.Link `json:"operation"`
	} `json:"_links"`

	ID                string    `json:"id"`
	PT                string    `json:"paging_token"`
	OfferID           string    `json:"offer_id"`
	Seller            string    `json:"seller"`
	SoldAmount        string    `json:"sold_amount"`
	SoldAssetType     string    `json:"sold_asset_type"`
	SoldAssetCode     string    `json:"sold_asset_code,omitempty"`
	SoldAssetIssuer   string    `json:"sold_asset_issuer,omitempty"`
	Buyer             string    `json:"buyer"`
	BoughtAmount      string    `json:"bought_amount"`
	BoughtAssetType   string    `json:"bought_asset_type"`
	BoughtAssetCode   string    `json:"bought_asset_code,omitempty"`
	BoughtAssetIssuer string    `json:"bought_asset_issuer,omitempty"`
	LedgerCloseTime   time.Time `json:"created_at"`
}

// TradeAggregation represents trade data aggregation over a period of time
type TradeAggregation struct {
	Timestamp     int64      `json:"timestamp,string"`
	TradeCount    int64      `json:"trade_count,string"`
	BaseVolume    string     `json:"base_volume"`
	CounterVolume string     `json:"counter_volume"`
	Average       string     `json:"avg"`
	High          string     `json:"high"`
	HighR         TradePrice `json:"high_r"`
	Low           string     `json:"low"`
	LowR          TradePrice `json:"low_r"`
	Open          string     `json:"open"`
	OpenR         TradePrice `json:"open_r"`
	Close         string     `json:"close"`
	CloseR        TradePrice `json:"close_r"`
}

// PagingToken implementation for hal.Pageable. Not actually used
func (res TradeAggregation) PagingToken() string {
	return strconv.FormatInt(res.Timestamp, 10)
}

// Transaction represents a single, successful transaction
type Transaction struct {
	Links struct {
		Self       hal.Link `json:"self"`
		Account    hal.Link `json:"account"`
		Ledger     hal.Link `json:"ledger"`
		Operations hal.Link `json:"operations"`
		Effects    hal.Link `json:"effects"`
		Precedes   hal.Link `json:"precedes"`
		Succeeds   hal.Link `json:"succeeds"`
		// Temporarily include Transaction as a link so that Transaction
		// can be fully compatible with TransactionSuccess
		// When TransactionSuccess is removed from the SDKs we can remove this HAL link
		Transaction hal.Link `json:"transaction"`
	} `json:"_links"`
	ID                 string                    `json:"id"`
	PT                 string                    `json:"paging_token"`
	Successful         bool                      `json:"successful"`
	Hash               string                    `json:"hash"`
	Ledger             int32                     `json:"ledger"`
	LedgerCloseTime    time.Time                 `json:"created_at"`
	Account            string                    `json:"source_account"`
	AccountMuxed       string                    `json:"account_muxed,omitempty"`
	AccountMuxedID     uint64                    `json:"account_muxed_id,omitempty,string"`
	AccountSequence    int64                     `json:"source_account_sequence,string"`
	FeeAccount         string                    `json:"fee_account"`
	FeeAccountMuxed    string                    `json:"fee_account_muxed,omitempty"`
	FeeAccountMuxedID  uint64                    `json:"fee_account_muxed_id,omitempty,string"`
	FeeCharged         int64                     `json:"fee_charged,string"`
	MaxFee             int64                     `json:"max_fee,string"`
	OperationCount     int32                     `json:"operation_count"`
	EnvelopeXdr        string                    `json:"envelope_xdr"`
	ResultXdr          string                    `json:"result_xdr"`
	ResultMetaXdr      string                    `json:"result_meta_xdr,omitempty"`
	FeeMetaXdr         string                    `json:"fee_meta_xdr"`
	MemoType           string                    `json:"memo_type"`
	MemoBytes          string                    `json:"memo_bytes,omitempty"`
	Memo               string                    `json:"memo,omitempty"`
	Signatures         []string                  `json:"signatures"`
	Preconditions      *TransactionPreconditions `json:"preconditions,omitempty"`
	FeeBumpTransaction *FeeBumpTransaction       `json:"fee_bump_transaction,omitempty"`
	InnerTransaction   *InnerTransaction         `json:"inner_transaction,omitempty"`
}

type TransactionPreconditions struct {
	TimeBounds   *TransactionPreconditionsTimebounds   `json:"timebounds,omitempty"`
	LedgerBounds *TransactionPreconditionsLedgerbounds `json:"ledgerbounds,omitempty"`

	MinAccountSequence          string `json:"min_account_sequence,omitempty"`
	MinAccountSequenceAge       string `json:"min_account_sequence_age,omitempty"`
	MinAccountSequenceLedgerGap uint32 `json:"min_account_sequence_ledger_gap,omitempty"`

	ExtraSigners []string `json:"extra_signers,omitempty"`
}

type TransactionPreconditionsTimebounds struct {
	MinTime string `json:"min_time,omitempty"`
	MaxTime string `json:"max_time,omitempty"`
}

type TransactionPreconditionsLedgerbounds struct {
	MinLedger uint32 `json:"min_ledger"`
	MaxLedger uint32 `json:"max_ledger,omitempty"`
}

// FeeBumpTransaction contains information about a fee bump transaction
type FeeBumpTransaction struct {
	Hash       string   `json:"hash"`
	Signatures []string `json:"signatures"`
}

// InnerTransaction contains information about the inner transaction contained
// within a fee bump transaction
type InnerTransaction struct {
	Hash       string   `json:"hash"`
	Signatures []string `json:"signatures"`
	MaxFee     int64    `json:"max_fee,string"`
}

// AsyncTransactionSubmissionResponse represents the response returned by Horizon
// when using the transaction-async endpoint.
type AsyncTransactionSubmissionResponse struct {
	// ErrorResultXDR is present only if Status is equal to proto.TXStatusError.
	// ErrorResultXDR is a TransactionResult xdr string which contains details on why
	// the transaction could not be accepted by stellar-core.
	ErrorResultXDR string `json:"error_result_xdr,omitempty"`
	// TxStatus represents the status of the transaction submission returned by stellar-core.
	// It can be one of: proto.TXStatusPending, proto.TXStatusDuplicate,
	// proto.TXStatusTryAgainLater, or proto.TXStatusError.
	TxStatus string `json:"tx_status"`
	// Hash is a hash of the transaction which can be used to look up whether
	// the transaction was included in the ledger.
	Hash string `json:"hash"`
}

func (response AsyncTransactionSubmissionResponse) GetStatus() int {
	return coreStatusToHTTPStatus[response.TxStatus]
}

// MarshalJSON implements a custom marshaler for Transaction.
// The memo field should be omitted if and only if the
// memo_type is "none".
func (t Transaction) MarshalJSON() ([]byte, error) {
	type Alias Transaction
	v := &struct {
		Memo      *string `json:"memo,omitempty"`
		MemoBytes *string `json:"memo_bytes,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(&t),
	}
	if t.MemoType != "none" {
		v.Memo = &t.Memo
	}

	if t.MemoType == "text" {
		v.MemoBytes = &t.MemoBytes
	}

	return json.Marshal(v)
}

// UnmarshalJSON implements a custom unmarshaler for Transaction
// which can handle a max_fee field which can be a string or int
func (t *Transaction) UnmarshalJSON(data []byte) error {
	type Alias Transaction // we define Alias to avoid infinite recursion when calling UnmarshalJSON()
	v := &struct {
		FeeCharged json.Number `json:"fee_charged"`
		MaxFee     json.Number `json:"max_fee"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}

	if v.FeeCharged != "" {
		t.FeeCharged, err = v.FeeCharged.Int64()
		if err != nil {
			return err
		}
	}
	if v.MaxFee != "" {
		t.MaxFee, err = v.MaxFee.Int64()
		if err != nil {
			return err
		}
	}
	return nil
}

// PagingToken implementation for hal.Pageable
func (t Transaction) PagingToken() string {
	return t.PT
}

// TransactionResultCodes represent a summary of result codes returned from
// a single xdr TransactionResult
type TransactionResultCodes struct {
	TransactionCode      string   `json:"transaction"`
	InnerTransactionCode string   `json:"inner_transaction,omitempty"`
	OperationCodes       []string `json:"operations,omitempty"`
}

// KeyTypeFromAddress converts the version byte of the provided strkey encoded
// value (for example an account id or a signer key) and returns the appropriate
// horizon-specific type name.
func KeyTypeFromAddress(address string) (string, error) {
	vb, err := strkey.Version(address)
	if err != nil {
		return "", errors.Wrap(err, "invalid address")
	}

	result, ok := KeyTypeNames[vb]
	if !ok {
		result = "unknown"
	}

	return result, nil
}

// MustKeyTypeFromAddress is the panicking variant of KeyTypeFromAddress.
func MustKeyTypeFromAddress(address string) string {
	ret, err := KeyTypeFromAddress(address)
	if err != nil {
		panic(err)
	}

	return ret
}

// AccountData represents a single data object stored on by an account
type AccountData struct {
	Value   string `json:"value"`
	Sponsor string `json:"sponsor,omitempty"`
}

// AccountsPage returns a list of account records
type AccountsPage struct {
	Links    hal.Links `json:"_links"`
	Embedded struct {
		Records []Account `json:"records"`
	} `json:"_embedded"`
}

// TradeAggregationsPage returns a list of aggregated trade records, aggregated by resolution
type TradeAggregationsPage struct {
	Links    hal.Links `json:"_links"`
	Embedded struct {
		Records []TradeAggregation `json:"records"`
	} `json:"_embedded"`
}

// TradesPage returns a list of trade records
type TradesPage struct {
	Links    hal.Links `json:"_links"`
	Embedded struct {
		Records []Trade `json:"records"`
	} `json:"_embedded"`
}

// OffersPage returns a list of offers
type OffersPage struct {
	Links    hal.Links `json:"_links"`
	Embedded struct {
		Records []Offer `json:"records"`
	} `json:"_embedded"`
}

// AssetsPage contains page of assets returned by Horizon.
type AssetsPage struct {
	Links    hal.Links `json:"_links"`
	Embedded struct {
		Records []AssetStat
	} `json:"_embedded"`
}

// LedgersPage contains page of ledger information returned by Horizon
type LedgersPage struct {
	Links    hal.Links `json:"_links"`
	Embedded struct {
		Records []Ledger
	} `json:"_embedded"`
}

type FeeDistribution struct {
	Max  int64 `json:"max,string"`
	Min  int64 `json:"min,string"`
	Mode int64 `json:"mode,string"`
	P10  int64 `json:"p10,string"`
	P20  int64 `json:"p20,string"`
	P30  int64 `json:"p30,string"`
	P40  int64 `json:"p40,string"`
	P50  int64 `json:"p50,string"`
	P60  int64 `json:"p60,string"`
	P70  int64 `json:"p70,string"`
	P80  int64 `json:"p80,string"`
	P90  int64 `json:"p90,string"`
	P95  int64 `json:"p95,string"`
	P99  int64 `json:"p99,string"`
}

// FeeStats represents a response of fees from horizon
// To do: implement fee suggestions if agreement is reached in https://github.com/stellar/go-stellar-sdk/issues/926
type FeeStats struct {
	LastLedger          uint32  `json:"last_ledger,string"`
	LastLedgerBaseFee   int64   `json:"last_ledger_base_fee,string"`
	LedgerCapacityUsage float64 `json:"ledger_capacity_usage,string"`

	FeeCharged FeeDistribution `json:"fee_charged"`
	MaxFee     FeeDistribution `json:"max_fee"`
}

// TransactionsPage contains records of transaction information returned by Horizon
type TransactionsPage struct {
	Links    hal.Links `json:"_links"`
	Embedded struct {
		Records []Transaction
	} `json:"_embedded"`
}

// PathsPage contains records of payment paths found by horizon
type PathsPage struct {
	Links    hal.Links `json:"_links"`
	Embedded struct {
		Records []Path
	} `json:"_embedded"`
}

// ClaimableBalanceFlags represents the state of a claimable balance's flags
type ClaimableBalanceFlags struct {
	ClawbackEnabled bool `json:"clawback_enabled"`
}

// ClaimableBalance represents a claimable balance
type ClaimableBalance struct {
	Links struct {
		Self         hal.Link `json:"self"`
		Transactions hal.Link `json:"transactions"`
		Operations   hal.Link `json:"operations"`
	} `json:"_links"`

	BalanceID          string                `json:"id"`
	Asset              string                `json:"asset"`
	Amount             string                `json:"amount"`
	Sponsor            string                `json:"sponsor,omitempty"`
	LastModifiedLedger uint32                `json:"last_modified_ledger"`
	LastModifiedTime   *time.Time            `json:"last_modified_time"`
	Claimants          []Claimant            `json:"claimants"`
	Flags              ClaimableBalanceFlags `json:"flags"`
	PT                 string                `json:"paging_token"`
}

type ClaimableBalances struct {
	Links struct {
		Self hal.Link `json:"self"`
	} `json:"_links"`

	Embedded struct {
		Records []ClaimableBalance `json:"records"`
	} `json:"_embedded"`
}

// PagingToken implementation for hal.Pageable
func (res ClaimableBalance) PagingToken() string {
	return res.PT
}

// Claimant represents a claimable balance claimant
type Claimant struct {
	Destination string             `json:"destination"`
	Predicate   xdr.ClaimPredicate `json:"predicate"`
}

// LiquidityPool represents a liquidity pool
type LiquidityPool struct {
	Links struct {
		Self         hal.Link `json:"self"`
		Transactions hal.Link `json:"transactions"`
		Operations   hal.Link `json:"operations"`
	} `json:"_links"`

	ID                 string                 `json:"id"`
	PT                 string                 `json:"paging_token"`
	FeeBP              uint32                 `json:"fee_bp"`
	Type               string                 `json:"type"`
	TotalTrustlines    uint64                 `json:"total_trustlines,string"`
	TotalShares        string                 `json:"total_shares"`
	Reserves           []LiquidityPoolReserve `json:"reserves"`
	LastModifiedLedger uint32                 `json:"last_modified_ledger"`
	LastModifiedTime   *time.Time             `json:"last_modified_time"`
}

// PagingToken implementation for hal.Pageable
func (res LiquidityPool) PagingToken() string {
	return res.PT
}

// LiquidityPoolsPage returns a list of liquidity pool records
type LiquidityPoolsPage struct {
	Links    hal.Links `json:"_links"`
	Embedded struct {
		Records []LiquidityPool `json:"records"`
	} `json:"_embedded"`
}

// LiquidityPoolReserve represents a liquidity pool asset reserve
type LiquidityPoolReserve struct {
	Asset  string `json:"asset"`
	Amount string `json:"amount"`
}

type AssetFilterConfig struct {
	Whitelist    []string `json:"whitelist"`
	Enabled      *bool    `json:"enabled"`
	LastModified int64    `json:"last_modified,omitempty"`
}

type AccountFilterConfig struct {
	Whitelist    []string `json:"whitelist"`
	Enabled      *bool    `json:"enabled"`
	LastModified int64    `json:"last_modified,omitempty"`
}

func (f *AccountFilterConfig) UnmarshalJSON(data []byte) error {
	type accountFilterConfig AccountFilterConfig
	var config = accountFilterConfig{}

	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if config.Whitelist == nil {
		return errors.New("missing required whitelist")
	}

	if config.Enabled == nil {
		return errors.New("missing required enabled")
	}

	*f = AccountFilterConfig(config)
	return nil
}

func (f *AssetFilterConfig) UnmarshalJSON(data []byte) error {
	type assetFilterConfig AssetFilterConfig
	var config = assetFilterConfig{}

	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if config.Whitelist == nil {
		return errors.New("missing required whitelist")
	}

	if config.Enabled == nil {
		return errors.New("missing required enabled")
	}

	*f = AssetFilterConfig(config)
	return nil
}
