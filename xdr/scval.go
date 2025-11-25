package xdr

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/stellar/go-stellar-sdk/strkey"
)

func (address ScAddress) String() (string, error) {
	var result string
	var err error

	switch address.Type {
	case ScAddressTypeScAddressTypeAccount:
		pubkey := address.MustAccountId().Ed25519
		result, err = strkey.Encode(strkey.VersionByteAccountID, pubkey[:])
	case ScAddressTypeScAddressTypeContract:
		contractID := address.MustContractId()
		result, err = strkey.Encode(strkey.VersionByteContract, contractID[:])
	case ScAddressTypeScAddressTypeMuxedAccount:
		payload := address.MustMuxedAccount()
		muxed := MuxedAccount{
			Type: CryptoKeyTypeKeyTypeMuxedEd25519,
			Med25519: &MuxedAccountMed25519{
				Id:      payload.Id,
				Ed25519: payload.Ed25519,
			},
		}
		result, err = muxed.GetAddress()
	case ScAddressTypeScAddressTypeLiquidityPool:
		poolID := address.MustLiquidityPoolId()
		result, err = strkey.Encode(strkey.VersionByteLiquidityPool, poolID[:])
	case ScAddressTypeScAddressTypeClaimableBalance:
		cbId := address.MustClaimableBalanceId()
		result, err = cbId.EncodeToStrkey()
	default:
		return "", fmt.Errorf("unfamiliar address type: %v", address.Type)
	}

	if err != nil {
		return "", err
	}

	return result, nil
}

func (s ContractExecutable) Equals(o ContractExecutable) bool {
	if s.Type != o.Type {
		return false
	}
	switch s.Type {
	case ContractExecutableTypeContractExecutableStellarAsset:
		return true
	case ContractExecutableTypeContractExecutableWasm:
		return s.MustWasmHash().Equals(o.MustWasmHash())
	default:
		panic("unknown ScContractExecutable type: " + s.Type.String())
	}
}

func (s ScError) Equals(o ScError) bool {
	if s.Type != o.Type {
		return false
	}
	switch s.Type {
	case ScErrorTypeSceContract:
		return *s.ContractCode == *o.ContractCode
	case ScErrorTypeSceWasmVm, ScErrorTypeSceContext, ScErrorTypeSceStorage, ScErrorTypeSceObject,
		ScErrorTypeSceCrypto, ScErrorTypeSceEvents, ScErrorTypeSceBudget, ScErrorTypeSceValue, ScErrorTypeSceAuth:
		return *s.Code == *o.Code
	default:
		panic("unknown ScError type: " + s.Type.String())
	}
}

func (s ScVal) Equals(o ScVal) bool {
	if s.Type != o.Type {
		return false
	}

	switch s.Type {
	case ScValTypeScvBool:
		return s.MustB() == o.MustB()
	case ScValTypeScvVoid:
		return true
	case ScValTypeScvError:
		return s.MustError().Equals(o.MustError())
	case ScValTypeScvU32:
		return s.MustU32() == o.MustU32()
	case ScValTypeScvI32:
		return s.MustI32() == o.MustI32()
	case ScValTypeScvU64:
		return s.MustU64() == o.MustU64()
	case ScValTypeScvI64:
		return s.MustI64() == o.MustI64()
	case ScValTypeScvTimepoint:
		return s.MustTimepoint() == o.MustTimepoint()
	case ScValTypeScvDuration:
		return s.MustDuration() == o.MustDuration()
	case ScValTypeScvU128:
		return s.MustU128() == o.MustU128()
	case ScValTypeScvI128:
		return s.MustI128() == o.MustI128()
	case ScValTypeScvU256:
		return s.MustU256() == o.MustU256()
	case ScValTypeScvI256:
		return s.MustI256() == o.MustI256()
	case ScValTypeScvBytes:
		return s.MustBytes().Equals(o.MustBytes())
	case ScValTypeScvString:
		return s.MustStr() == o.MustStr()
	case ScValTypeScvSymbol:
		return s.MustSym() == o.MustSym()
	case ScValTypeScvVec:
		return s.MustVec().Equals(o.MustVec())
	case ScValTypeScvMap:
		return s.MustMap().Equals(o.MustMap())
	case ScValTypeScvAddress:
		return s.MustAddress().Equals(o.MustAddress())
	case ScValTypeScvContractInstance:
		return s.MustInstance().Executable.Equals(o.MustInstance().Executable) && s.MustInstance().Storage.Equals(o.MustInstance().Storage)
	case ScValTypeScvLedgerKeyContractInstance:
		return true
	case ScValTypeScvLedgerKeyNonce:
		return s.MustNonceKey().Equals(o.MustNonceKey())

	default:
		panic("unknown ScVal type: " + s.Type.String())
	}
}

func (s ScBytes) Equals(o ScBytes) bool {
	return bytes.Equal([]byte(s), []byte(o))
}

func (s ScAddress) Equals(o ScAddress) bool {
	if s.Type != o.Type {
		return false
	}

	switch s.Type {
	case ScAddressTypeScAddressTypeAccount:
		sAccountID := s.MustAccountId()
		return sAccountID.Equals(o.MustAccountId())
	case ScAddressTypeScAddressTypeContract:
		return s.MustContractId() == o.MustContractId()
	case ScAddressTypeScAddressTypeClaimableBalance:
		return s.MustClaimableBalanceId().MustV0() == o.MustClaimableBalanceId().MustV0()
	case ScAddressTypeScAddressTypeLiquidityPool:
		return s.MustLiquidityPoolId() == o.MustLiquidityPoolId()
	case ScAddressTypeScAddressTypeMuxedAccount:
		return s.MustMuxedAccount().Id == o.MustMuxedAccount().Id &&
			s.MustMuxedAccount().Ed25519.Equals(o.MustMuxedAccount().Ed25519)
	default:
		panic("unknown ScAddress type: " + s.Type.String())
	}
}

// IsBool returns true if the given ScVal is a boolean
func (s ScVal) IsBool() bool {
	return s.Type == ScValTypeScvBool
}

func (s *ScVec) Equals(o *ScVec) bool {
	if s == nil && o == nil {
		return true
	}
	if s == nil || o == nil {
		return false
	}
	if len(*s) != len(*o) {
		return false
	}
	for i := range *s {
		if !(*s)[i].Equals((*o)[i]) {
			return false
		}
	}
	return true
}

func (s *ScMap) Equals(o *ScMap) bool {
	if s == nil && o == nil {
		return true
	}
	if s == nil || o == nil {
		return false
	}
	if len(*s) != len(*o) {
		return false
	}
	for i, entry := range *s {
		if !entry.Equals((*o)[i]) {
			return false
		}
	}
	return true
}

func (s ScMapEntry) Equals(o ScMapEntry) bool {
	return s.Key.Equals(o.Key) && s.Val.Equals(o.Val)
}

func (s ScNonceKey) Equals(o ScNonceKey) bool {
	return s.Nonce == o.Nonce
}

func bigIntFromParts(hi Int64, lowerParts ...Uint64) *big.Int {
	result := new(big.Int).SetInt64(int64(hi))
	secondary := new(big.Int)
	for _, part := range lowerParts {
		result.Lsh(result, 64)
		result.Or(result, secondary.SetUint64(uint64(part)))
	}
	return result
}

func bigUIntFromParts(hi Uint64, lowerParts ...Uint64) *big.Int {
	result := new(big.Int).SetUint64(uint64(hi))
	secondary := new(big.Int)
	for _, part := range lowerParts {
		result.Lsh(result, 64)
		result.Or(result, secondary.SetUint64(uint64(part)))
	}
	return result
}

func (s ScVal) String() string {
	switch s.Type {
	case ScValTypeScvBool:
		return fmt.Sprintf("%t", *s.B)
	case ScValTypeScvVoid:
		return "(void)"
	case ScValTypeScvError:
		switch s.Error.Type {
		case ScErrorTypeSceContract:
			return fmt.Sprintf("%s(%d)", s.Error.Type, *s.Error.ContractCode)
		case ScErrorTypeSceWasmVm, ScErrorTypeSceContext, ScErrorTypeSceStorage, ScErrorTypeSceObject,
			ScErrorTypeSceCrypto, ScErrorTypeSceEvents, ScErrorTypeSceBudget, ScErrorTypeSceValue, ScErrorTypeSceAuth:
			return fmt.Sprintf("%s(%s)", s.Error.Type, *s.Error.Code)
		}
	case ScValTypeScvU32:
		return fmt.Sprintf("%d", *s.U32)
	case ScValTypeScvI32:
		return fmt.Sprintf("%d", *s.I32)
	case ScValTypeScvU64:
		return fmt.Sprintf("%d", *s.U64)
	case ScValTypeScvI64:
		return fmt.Sprintf("%d", *s.I64)
	case ScValTypeScvTimepoint:
		return time.Unix(int64(*s.Timepoint), 0).String()
	case ScValTypeScvDuration:
		return fmt.Sprintf("%d", *s.Duration)
	case ScValTypeScvU128:
		return bigUIntFromParts(s.U128.Hi, s.U128.Lo).String()
	case ScValTypeScvI128:
		return bigIntFromParts(s.I128.Hi, s.I128.Lo).String()
	case ScValTypeScvU256:
		return bigUIntFromParts(s.U256.HiHi, s.U256.HiLo, s.U256.LoHi, s.U256.LoLo).String()
	case ScValTypeScvI256:
		return bigIntFromParts(s.I256.HiHi, s.I256.HiLo, s.I256.LoHi, s.I256.LoLo).String()
	case ScValTypeScvBytes:
		return hex.EncodeToString(*s.Bytes)
	case ScValTypeScvString:
		return string(*s.Str)
	case ScValTypeScvSymbol:
		return string(*s.Sym)
	case ScValTypeScvVec:
		if *s.Vec == nil {
			return "nil"
		}
		return fmt.Sprintf("%s", **s.Vec)
	case ScValTypeScvMap:
		if *s.Map == nil {
			return "nil"
		}
		return fmt.Sprintf("%v", **s.Map)
	case ScValTypeScvAddress:
		str, err := s.Address.String()
		if err != nil {
			return err.Error()
		}
		return str
	case ScValTypeScvContractInstance:
		result := ""
		switch s.Instance.Executable.Type {
		case ContractExecutableTypeContractExecutableStellarAsset:
			result = "(StellarAssetContract)"
		case ContractExecutableTypeContractExecutableWasm:
			result = hex.EncodeToString(s.Instance.Executable.WasmHash[:])
		}
		if s.Instance.Storage != nil && len(*s.Instance.Storage) > 0 {
			result += fmt.Sprintf(": %v", *s.Instance.Storage)
		}
		return result
	case ScValTypeScvLedgerKeyContractInstance:
		return "(LedgerKeyContractInstance)"
	case ScValTypeScvLedgerKeyNonce:
		return fmt.Sprintf("%X", *s.NonceKey)
	}

	return "unknown"
}
