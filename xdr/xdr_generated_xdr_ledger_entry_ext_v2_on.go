//go:build xdr_ledger_entry_ext_v2

//lint:file-ignore S1005 The issue should be fixed in xdrgen. Unfortunately, there's no way to ignore a single file in staticcheck.

// DO NOT EDIT or your changes may be overwritten
package xdr

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"

	"github.com/stellar/go-xdr/xdr3"
)

// LedgerEntryData is an XDR NestedUnion defines as:
//
//	union switch (LedgerEntryType type)
//	     {
//	     case ACCOUNT:
//	         AccountEntry account;
//	     case TRUSTLINE:
//	         TrustLineEntry trustLine;
//	     case OFFER:
//	         OfferEntry offer;
//	     case DATA:
//	         DataEntry data;
//	     case CLAIMABLE_BALANCE:
//	         ClaimableBalanceEntry claimableBalance;
//	     case LIQUIDITY_POOL:
//	         LiquidityPoolEntry liquidityPool;
//	     case CONTRACT_DATA:
//	         ContractDataEntry contractData;
//	     case CONTRACT_CODE:
//	         ContractCodeEntry contractCode;
//	     case CONFIG_SETTING:
//	         ConfigSettingEntry configSetting;
//	     case TTL:
//	         TTLEntry ttl;
//	     }
type LedgerEntryData struct {
	Type             LedgerEntryType
	Account          *AccountEntry
	TrustLine        *TrustLineEntry
	Offer            *OfferEntry
	Data             *DataEntry
	ClaimableBalance *ClaimableBalanceEntry
	LiquidityPool    *LiquidityPoolEntry
	ContractData     *ContractDataEntry
	ContractCode     *ContractCodeEntry
	ConfigSetting    *ConfigSettingEntry
	Ttl              *TtlEntry
}

// SwitchFieldName returns the field name in which this union's
// discriminant is stored
func (u LedgerEntryData) SwitchFieldName() string {
	return "Type"
}

// ArmForSwitch returns which field name should be used for storing
// the value for an instance of LedgerEntryData
func (u LedgerEntryData) ArmForSwitch(sw int32) (string, bool) {
	switch LedgerEntryType(sw) {
	case LedgerEntryTypeAccount:
		return "Account", true
	case LedgerEntryTypeTrustline:
		return "TrustLine", true
	case LedgerEntryTypeOffer:
		return "Offer", true
	case LedgerEntryTypeData:
		return "Data", true
	case LedgerEntryTypeClaimableBalance:
		return "ClaimableBalance", true
	case LedgerEntryTypeLiquidityPool:
		return "LiquidityPool", true
	case LedgerEntryTypeContractData:
		return "ContractData", true
	case LedgerEntryTypeContractCode:
		return "ContractCode", true
	case LedgerEntryTypeConfigSetting:
		return "ConfigSetting", true
	case LedgerEntryTypeTtl:
		return "Ttl", true
	}
	return "-", false
}

// NewLedgerEntryData creates a new  LedgerEntryData.
func NewLedgerEntryData(aType LedgerEntryType, value interface{}) (result LedgerEntryData, err error) {
	result.Type = aType
	switch LedgerEntryType(aType) {
	case LedgerEntryTypeAccount:
		tv, ok := value.(AccountEntry)
		if !ok {
			err = errors.New("invalid value, must be AccountEntry")
			return
		}
		result.Account = &tv
	case LedgerEntryTypeTrustline:
		tv, ok := value.(TrustLineEntry)
		if !ok {
			err = errors.New("invalid value, must be TrustLineEntry")
			return
		}
		result.TrustLine = &tv
	case LedgerEntryTypeOffer:
		tv, ok := value.(OfferEntry)
		if !ok {
			err = errors.New("invalid value, must be OfferEntry")
			return
		}
		result.Offer = &tv
	case LedgerEntryTypeData:
		tv, ok := value.(DataEntry)
		if !ok {
			err = errors.New("invalid value, must be DataEntry")
			return
		}
		result.Data = &tv
	case LedgerEntryTypeClaimableBalance:
		tv, ok := value.(ClaimableBalanceEntry)
		if !ok {
			err = errors.New("invalid value, must be ClaimableBalanceEntry")
			return
		}
		result.ClaimableBalance = &tv
	case LedgerEntryTypeLiquidityPool:
		tv, ok := value.(LiquidityPoolEntry)
		if !ok {
			err = errors.New("invalid value, must be LiquidityPoolEntry")
			return
		}
		result.LiquidityPool = &tv
	case LedgerEntryTypeContractData:
		tv, ok := value.(ContractDataEntry)
		if !ok {
			err = errors.New("invalid value, must be ContractDataEntry")
			return
		}
		result.ContractData = &tv
	case LedgerEntryTypeContractCode:
		tv, ok := value.(ContractCodeEntry)
		if !ok {
			err = errors.New("invalid value, must be ContractCodeEntry")
			return
		}
		result.ContractCode = &tv
	case LedgerEntryTypeConfigSetting:
		tv, ok := value.(ConfigSettingEntry)
		if !ok {
			err = errors.New("invalid value, must be ConfigSettingEntry")
			return
		}
		result.ConfigSetting = &tv
	case LedgerEntryTypeTtl:
		tv, ok := value.(TtlEntry)
		if !ok {
			err = errors.New("invalid value, must be TtlEntry")
			return
		}
		result.Ttl = &tv
	}
	return
}

// MustAccount retrieves the Account value from the union,
// panicing if the value is not set.
func (u LedgerEntryData) MustAccount() AccountEntry {
	val, ok := u.GetAccount()

	if !ok {
		panic("arm Account is not set")
	}

	return val
}

// GetAccount retrieves the Account value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryData) GetAccount() (result AccountEntry, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Account" {
		result = *u.Account
		ok = true
	}

	return
}

// MustTrustLine retrieves the TrustLine value from the union,
// panicing if the value is not set.
func (u LedgerEntryData) MustTrustLine() TrustLineEntry {
	val, ok := u.GetTrustLine()

	if !ok {
		panic("arm TrustLine is not set")
	}

	return val
}

// GetTrustLine retrieves the TrustLine value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryData) GetTrustLine() (result TrustLineEntry, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "TrustLine" {
		result = *u.TrustLine
		ok = true
	}

	return
}

// MustOffer retrieves the Offer value from the union,
// panicing if the value is not set.
func (u LedgerEntryData) MustOffer() OfferEntry {
	val, ok := u.GetOffer()

	if !ok {
		panic("arm Offer is not set")
	}

	return val
}

// GetOffer retrieves the Offer value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryData) GetOffer() (result OfferEntry, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Offer" {
		result = *u.Offer
		ok = true
	}

	return
}

// MustData retrieves the Data value from the union,
// panicing if the value is not set.
func (u LedgerEntryData) MustData() DataEntry {
	val, ok := u.GetData()

	if !ok {
		panic("arm Data is not set")
	}

	return val
}

// GetData retrieves the Data value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryData) GetData() (result DataEntry, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Data" {
		result = *u.Data
		ok = true
	}

	return
}

// MustClaimableBalance retrieves the ClaimableBalance value from the union,
// panicing if the value is not set.
func (u LedgerEntryData) MustClaimableBalance() ClaimableBalanceEntry {
	val, ok := u.GetClaimableBalance()

	if !ok {
		panic("arm ClaimableBalance is not set")
	}

	return val
}

// GetClaimableBalance retrieves the ClaimableBalance value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryData) GetClaimableBalance() (result ClaimableBalanceEntry, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ClaimableBalance" {
		result = *u.ClaimableBalance
		ok = true
	}

	return
}

// MustLiquidityPool retrieves the LiquidityPool value from the union,
// panicing if the value is not set.
func (u LedgerEntryData) MustLiquidityPool() LiquidityPoolEntry {
	val, ok := u.GetLiquidityPool()

	if !ok {
		panic("arm LiquidityPool is not set")
	}

	return val
}

// GetLiquidityPool retrieves the LiquidityPool value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryData) GetLiquidityPool() (result LiquidityPoolEntry, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "LiquidityPool" {
		result = *u.LiquidityPool
		ok = true
	}

	return
}

// MustContractData retrieves the ContractData value from the union,
// panicing if the value is not set.
func (u LedgerEntryData) MustContractData() ContractDataEntry {
	val, ok := u.GetContractData()

	if !ok {
		panic("arm ContractData is not set")
	}

	return val
}

// GetContractData retrieves the ContractData value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryData) GetContractData() (result ContractDataEntry, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ContractData" {
		result = *u.ContractData
		ok = true
	}

	return
}

// MustContractCode retrieves the ContractCode value from the union,
// panicing if the value is not set.
func (u LedgerEntryData) MustContractCode() ContractCodeEntry {
	val, ok := u.GetContractCode()

	if !ok {
		panic("arm ContractCode is not set")
	}

	return val
}

// GetContractCode retrieves the ContractCode value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryData) GetContractCode() (result ContractCodeEntry, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ContractCode" {
		result = *u.ContractCode
		ok = true
	}

	return
}

// MustConfigSetting retrieves the ConfigSetting value from the union,
// panicing if the value is not set.
func (u LedgerEntryData) MustConfigSetting() ConfigSettingEntry {
	val, ok := u.GetConfigSetting()

	if !ok {
		panic("arm ConfigSetting is not set")
	}

	return val
}

// GetConfigSetting retrieves the ConfigSetting value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryData) GetConfigSetting() (result ConfigSettingEntry, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ConfigSetting" {
		result = *u.ConfigSetting
		ok = true
	}

	return
}

// MustTtl retrieves the Ttl value from the union,
// panicing if the value is not set.
func (u LedgerEntryData) MustTtl() TtlEntry {
	val, ok := u.GetTtl()

	if !ok {
		panic("arm Ttl is not set")
	}

	return val
}

// GetTtl retrieves the Ttl value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryData) GetTtl() (result TtlEntry, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Ttl" {
		result = *u.Ttl
		ok = true
	}

	return
}

// EncodeTo encodes this value using the Encoder.
func (u LedgerEntryData) EncodeTo(e *xdr.Encoder) error {
	var err error
	if err = u.Type.EncodeTo(e); err != nil {
		return err
	}
	switch LedgerEntryType(u.Type) {
	case LedgerEntryTypeAccount:
		if err = (*u.Account).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case LedgerEntryTypeTrustline:
		if err = (*u.TrustLine).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case LedgerEntryTypeOffer:
		if err = (*u.Offer).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case LedgerEntryTypeData:
		if err = (*u.Data).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case LedgerEntryTypeClaimableBalance:
		if err = (*u.ClaimableBalance).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case LedgerEntryTypeLiquidityPool:
		if err = (*u.LiquidityPool).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case LedgerEntryTypeContractData:
		if err = (*u.ContractData).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case LedgerEntryTypeContractCode:
		if err = (*u.ContractCode).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case LedgerEntryTypeConfigSetting:
		if err = (*u.ConfigSetting).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case LedgerEntryTypeTtl:
		if err = (*u.Ttl).EncodeTo(e); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("Type (LedgerEntryType) switch value '%d' is not valid for union LedgerEntryData", u.Type)
}

var _ decoderFrom = (*LedgerEntryData)(nil)

// DecodeFrom decodes this value using the Decoder.
func (u *LedgerEntryData) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding LedgerEntryData: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	var err error
	var n, nTmp int
	nTmp, err = u.Type.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding LedgerEntryType: %w", err)
	}
	switch LedgerEntryType(u.Type) {
	case LedgerEntryTypeAccount:
		u.Account = new(AccountEntry)
		nTmp, err = (*u.Account).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding AccountEntry: %w", err)
		}
		return n, nil
	case LedgerEntryTypeTrustline:
		u.TrustLine = new(TrustLineEntry)
		nTmp, err = (*u.TrustLine).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding TrustLineEntry: %w", err)
		}
		return n, nil
	case LedgerEntryTypeOffer:
		u.Offer = new(OfferEntry)
		nTmp, err = (*u.Offer).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding OfferEntry: %w", err)
		}
		return n, nil
	case LedgerEntryTypeData:
		u.Data = new(DataEntry)
		nTmp, err = (*u.Data).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding DataEntry: %w", err)
		}
		return n, nil
	case LedgerEntryTypeClaimableBalance:
		u.ClaimableBalance = new(ClaimableBalanceEntry)
		nTmp, err = (*u.ClaimableBalance).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ClaimableBalanceEntry: %w", err)
		}
		return n, nil
	case LedgerEntryTypeLiquidityPool:
		u.LiquidityPool = new(LiquidityPoolEntry)
		nTmp, err = (*u.LiquidityPool).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding LiquidityPoolEntry: %w", err)
		}
		return n, nil
	case LedgerEntryTypeContractData:
		u.ContractData = new(ContractDataEntry)
		nTmp, err = (*u.ContractData).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ContractDataEntry: %w", err)
		}
		return n, nil
	case LedgerEntryTypeContractCode:
		u.ContractCode = new(ContractCodeEntry)
		nTmp, err = (*u.ContractCode).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ContractCodeEntry: %w", err)
		}
		return n, nil
	case LedgerEntryTypeConfigSetting:
		u.ConfigSetting = new(ConfigSettingEntry)
		nTmp, err = (*u.ConfigSetting).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ConfigSettingEntry: %w", err)
		}
		return n, nil
	case LedgerEntryTypeTtl:
		u.Ttl = new(TtlEntry)
		nTmp, err = (*u.Ttl).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding TtlEntry: %w", err)
		}
		return n, nil
	}
	return n, fmt.Errorf("union LedgerEntryData has invalid Type (LedgerEntryType) switch value '%d'", u.Type)
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s LedgerEntryData) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *LedgerEntryData) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*LedgerEntryData)(nil)
	_ encoding.BinaryUnmarshaler = (*LedgerEntryData)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s LedgerEntryData) xdrType() {}

var _ xdrType = (*LedgerEntryData)(nil)

// LedgerEntry is an XDR Struct defines as:
//
//	struct LedgerEntry
//	 {
//	     uint32 lastModifiedLedgerSeq; // ledger the LedgerEntry was last changed
//
//	     union switch (LedgerEntryType type)
//	     {
//	     case ACCOUNT:
//	         AccountEntry account;
//	     case TRUSTLINE:
//	         TrustLineEntry trustLine;
//	     case OFFER:
//	         OfferEntry offer;
//	     case DATA:
//	         DataEntry data;
//	     case CLAIMABLE_BALANCE:
//	         ClaimableBalanceEntry claimableBalance;
//	     case LIQUIDITY_POOL:
//	         LiquidityPoolEntry liquidityPool;
//	     case CONTRACT_DATA:
//	         ContractDataEntry contractData;
//	     case CONTRACT_CODE:
//	         ContractCodeEntry contractCode;
//	     case CONFIG_SETTING:
//	         ConfigSettingEntry configSetting;
//	     case TTL:
//	         TTLEntry ttl;
//	     }
//	     data;
//
//	     // reserved for future use
//	     union switch (int v)
//	     {
//	     case 0:
//	         void;
//	     case 1:
//	         LedgerEntryExtensionV1 v1;
//
//	     case 2:
//	         ExtensionPoint v2;
//
//	     }
//	     ext;
//	 };
type LedgerEntry struct {
	LastModifiedLedgerSeq Uint32
	Data                  LedgerEntryData
	Ext                   LedgerEntryExt
}

// EncodeTo encodes this value using the Encoder.
func (s *LedgerEntry) EncodeTo(e *xdr.Encoder) error {
	var err error
	if err = s.LastModifiedLedgerSeq.EncodeTo(e); err != nil {
		return err
	}
	if err = s.Data.EncodeTo(e); err != nil {
		return err
	}
	if err = s.Ext.EncodeTo(e); err != nil {
		return err
	}
	return nil
}

var _ decoderFrom = (*LedgerEntry)(nil)

// DecodeFrom decodes this value using the Decoder.
func (s *LedgerEntry) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding LedgerEntry: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	var err error
	var n, nTmp int
	nTmp, err = s.LastModifiedLedgerSeq.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding Uint32: %w", err)
	}
	nTmp, err = s.Data.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding LedgerEntryData: %w", err)
	}
	nTmp, err = s.Ext.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding LedgerEntryExt: %w", err)
	}
	return n, nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s LedgerEntry) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *LedgerEntry) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*LedgerEntry)(nil)
	_ encoding.BinaryUnmarshaler = (*LedgerEntry)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s LedgerEntry) xdrType() {}

var _ xdrType = (*LedgerEntry)(nil)

// LedgerEntryExt is an XDR NestedUnion defines as:
//
//	union switch (int v)
//	     {
//	     case 0:
//	         void;
//	     case 1:
//	         LedgerEntryExtensionV1 v1;
//
//	     case 2:
//	         ExtensionPoint v2;
//
//	     }
type LedgerEntryExt struct {
	V  int32
	V1 *LedgerEntryExtensionV1
	V2 *ExtensionPoint
}

// SwitchFieldName returns the field name in which this union's
// discriminant is stored
func (u LedgerEntryExt) SwitchFieldName() string {
	return "V"
}

// ArmForSwitch returns which field name should be used for storing
// the value for an instance of LedgerEntryExt
func (u LedgerEntryExt) ArmForSwitch(sw int32) (string, bool) {
	switch int32(sw) {
	case 0:
		return "", true
	case 1:
		return "V1", true
	case 2:
		return "V2", true
	}
	return "-", false
}

// NewLedgerEntryExt creates a new  LedgerEntryExt.
func NewLedgerEntryExt(v int32, value interface{}) (result LedgerEntryExt, err error) {
	result.V = v
	switch int32(v) {
	case 0:
		// void
	case 1:
		tv, ok := value.(LedgerEntryExtensionV1)
		if !ok {
			err = errors.New("invalid value, must be LedgerEntryExtensionV1")
			return
		}
		result.V1 = &tv
	case 2:
		tv, ok := value.(ExtensionPoint)
		if !ok {
			err = errors.New("invalid value, must be ExtensionPoint")
			return
		}
		result.V2 = &tv
	}
	return
}

// MustV1 retrieves the V1 value from the union,
// panicing if the value is not set.
func (u LedgerEntryExt) MustV1() LedgerEntryExtensionV1 {
	val, ok := u.GetV1()

	if !ok {
		panic("arm V1 is not set")
	}

	return val
}

// GetV1 retrieves the V1 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryExt) GetV1() (result LedgerEntryExtensionV1, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.V))

	if armName == "V1" {
		result = *u.V1
		ok = true
	}

	return
}

// MustV2 retrieves the V2 value from the union,
// panicing if the value is not set.
func (u LedgerEntryExt) MustV2() ExtensionPoint {
	val, ok := u.GetV2()

	if !ok {
		panic("arm V2 is not set")
	}

	return val
}

// GetV2 retrieves the V2 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u LedgerEntryExt) GetV2() (result ExtensionPoint, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.V))

	if armName == "V2" {
		result = *u.V2
		ok = true
	}

	return
}

// EncodeTo encodes this value using the Encoder.
func (u LedgerEntryExt) EncodeTo(e *xdr.Encoder) error {
	var err error
	if _, err = e.EncodeInt(int32(u.V)); err != nil {
		return err
	}
	switch int32(u.V) {
	case 0:
		// Void
		return nil
	case 1:
		if err = (*u.V1).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case 2:
		if err = (*u.V2).EncodeTo(e); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("V (int32) switch value '%d' is not valid for union LedgerEntryExt", u.V)
}

var _ decoderFrom = (*LedgerEntryExt)(nil)

// DecodeFrom decodes this value using the Decoder.
func (u *LedgerEntryExt) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding LedgerEntryExt: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	var err error
	var n, nTmp int
	u.V, nTmp, err = d.DecodeInt()
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding Int: %w", err)
	}
	switch int32(u.V) {
	case 0:
		// Void
		return n, nil
	case 1:
		u.V1 = new(LedgerEntryExtensionV1)
		nTmp, err = (*u.V1).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding LedgerEntryExtensionV1: %w", err)
		}
		return n, nil
	case 2:
		u.V2 = new(ExtensionPoint)
		nTmp, err = (*u.V2).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ExtensionPoint: %w", err)
		}
		return n, nil
	}
	return n, fmt.Errorf("union LedgerEntryExt has invalid V (int32) switch value '%d'", u.V)
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s LedgerEntryExt) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *LedgerEntryExt) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*LedgerEntryExt)(nil)
	_ encoding.BinaryUnmarshaler = (*LedgerEntryExt)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s LedgerEntryExt) xdrType() {}

var _ xdrType = (*LedgerEntryExt)(nil)
