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
		pubkey := address.MustAccountId().MustEd25519()
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
		return s.MustContractCode() == o.MustContractCode()
	case ScErrorTypeSceWasmVm, ScErrorTypeSceContext, ScErrorTypeSceStorage, ScErrorTypeSceObject,
		ScErrorTypeSceCrypto, ScErrorTypeSceEvents, ScErrorTypeSceBudget, ScErrorTypeSceValue, ScErrorTypeSceAuth:
		return s.MustCode() == o.MustCode()
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
		return fmt.Sprintf("%t", s.MustB())
	case ScValTypeScvVoid:
		return "(void)"
	case ScValTypeScvError:
		err := s.MustError()
		switch err.Type {
		case ScErrorTypeSceContract:
			return fmt.Sprintf("%s(%d)", err.Type, err.MustContractCode())
		case ScErrorTypeSceWasmVm, ScErrorTypeSceContext, ScErrorTypeSceStorage, ScErrorTypeSceObject,
			ScErrorTypeSceCrypto, ScErrorTypeSceEvents, ScErrorTypeSceBudget, ScErrorTypeSceValue, ScErrorTypeSceAuth:
			return fmt.Sprintf("%s(%s)", err.Type, err.MustCode())
		}
	case ScValTypeScvU32:
		return fmt.Sprintf("%d", s.MustU32())
	case ScValTypeScvI32:
		return fmt.Sprintf("%d", s.MustI32())
	case ScValTypeScvU64:
		return fmt.Sprintf("%d", s.MustU64())
	case ScValTypeScvI64:
		return fmt.Sprintf("%d", s.MustI64())
	case ScValTypeScvTimepoint:
		return time.Unix(int64(s.MustTimepoint()), 0).String()
	case ScValTypeScvDuration:
		return fmt.Sprintf("%d", s.MustDuration())
	case ScValTypeScvU128:
		u128 := s.MustU128()
		return bigUIntFromParts(u128.Hi, u128.Lo).String()
	case ScValTypeScvI128:
		i128 := s.MustI128()
		return bigIntFromParts(i128.Hi, i128.Lo).String()
	case ScValTypeScvU256:
		u256 := s.MustU256()
		return bigUIntFromParts(u256.HiHi, u256.HiLo, u256.LoHi, u256.LoLo).String()
	case ScValTypeScvI256:
		i256 := s.MustI256()
		return bigIntFromParts(i256.HiHi, i256.HiLo, i256.LoHi, i256.LoLo).String()
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
			wasmHash := s.Instance.Executable.MustWasmHash()
			result = hex.EncodeToString(wasmHash[:])
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
