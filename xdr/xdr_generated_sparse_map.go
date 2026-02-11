//go:build sparse_map

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

// ScValType is an XDR Enum defines as:
//
//	enum SCValType
//	 {
//	     SCV_BOOL = 0,
//	     SCV_VOID = 1,
//	     SCV_ERROR = 2,
//
//	     // 32 bits is the smallest type in WASM or XDR; no need for u8/u16.
//	     SCV_U32 = 3,
//	     SCV_I32 = 4,
//
//	     // 64 bits is naturally supported by both WASM and XDR also.
//	     SCV_U64 = 5,
//	     SCV_I64 = 6,
//
//	     // Time-related u64 subtypes with their own functions and formatting.
//	     SCV_TIMEPOINT = 7,
//	     SCV_DURATION = 8,
//
//	     // 128 bits is naturally supported by Rust and we use it for Soroban
//	     // fixed-point arithmetic prices / balances / similar "quantities". These
//	     // are represented in XDR as a pair of 2 u64s.
//	     SCV_U128 = 9,
//	     SCV_I128 = 10,
//
//	     // 256 bits is the size of sha256 output, ed25519 keys, and the EVM machine
//	     // word, so for interop use we include this even though it requires a small
//	     // amount of Rust guest and/or host library code.
//	     SCV_U256 = 11,
//	     SCV_I256 = 12,
//
//	     // Bytes come in 3 flavors, 2 of which have meaningfully different
//	     // formatting and validity-checking / domain-restriction.
//	     SCV_BYTES = 13,
//	     SCV_STRING = 14,
//	     SCV_SYMBOL = 15,
//
//	     // Vecs and maps are just polymorphic containers of other ScVals.
//	     SCV_VEC = 16,
//	     SCV_MAP = 17,
//
//	     // Address is the universal identifier for contracts and classic
//	     // accounts.
//	     SCV_ADDRESS = 18,
//
//	     // The following are the internal SCVal variants that are not
//	     // exposed to the contracts.
//	     SCV_CONTRACT_INSTANCE = 19,
//
//	     // SCV_LEDGER_KEY_CONTRACT_INSTANCE and SCV_LEDGER_KEY_NONCE are unique
//	     // symbolic SCVals used as the key for ledger entries for a contract's
//	     // instance and an address' nonce, respectively.
//	     SCV_LEDGER_KEY_CONTRACT_INSTANCE = 20,
//
//	     SCV_LEDGER_KEY_NONCE = 21,
//	     SCV_SPARSE_MAP = 22
//
//	     SCV_LEDGER_KEY_NONCE = 21
//
//	 };
type ScValType int32

const (
	ScValTypeScvBool                      ScValType = 0
	ScValTypeScvVoid                      ScValType = 1
	ScValTypeScvError                     ScValType = 2
	ScValTypeScvU32                       ScValType = 3
	ScValTypeScvI32                       ScValType = 4
	ScValTypeScvU64                       ScValType = 5
	ScValTypeScvI64                       ScValType = 6
	ScValTypeScvTimepoint                 ScValType = 7
	ScValTypeScvDuration                  ScValType = 8
	ScValTypeScvU128                      ScValType = 9
	ScValTypeScvI128                      ScValType = 10
	ScValTypeScvU256                      ScValType = 11
	ScValTypeScvI256                      ScValType = 12
	ScValTypeScvBytes                     ScValType = 13
	ScValTypeScvString                    ScValType = 14
	ScValTypeScvSymbol                    ScValType = 15
	ScValTypeScvVec                       ScValType = 16
	ScValTypeScvMap                       ScValType = 17
	ScValTypeScvAddress                   ScValType = 18
	ScValTypeScvContractInstance          ScValType = 19
	ScValTypeScvLedgerKeyContractInstance ScValType = 20
	ScValTypeScvLedgerKeyNonce            ScValType = 21
	ScValTypeScvSparseMap                 ScValType = 22
)

var scValTypeMap = map[int32]string{
	0:  "ScValTypeScvBool",
	1:  "ScValTypeScvVoid",
	2:  "ScValTypeScvError",
	3:  "ScValTypeScvU32",
	4:  "ScValTypeScvI32",
	5:  "ScValTypeScvU64",
	6:  "ScValTypeScvI64",
	7:  "ScValTypeScvTimepoint",
	8:  "ScValTypeScvDuration",
	9:  "ScValTypeScvU128",
	10: "ScValTypeScvI128",
	11: "ScValTypeScvU256",
	12: "ScValTypeScvI256",
	13: "ScValTypeScvBytes",
	14: "ScValTypeScvString",
	15: "ScValTypeScvSymbol",
	16: "ScValTypeScvVec",
	17: "ScValTypeScvMap",
	18: "ScValTypeScvAddress",
	19: "ScValTypeScvContractInstance",
	20: "ScValTypeScvLedgerKeyContractInstance",
	21: "ScValTypeScvLedgerKeyNonce",
	22: "ScValTypeScvSparseMap",
}

// ValidEnum validates a proposed value for this enum.  Implements
// the Enum interface for ScValType
func (e ScValType) ValidEnum(v int32) bool {
	_, ok := scValTypeMap[v]
	return ok
}

// String returns the name of `e`
func (e ScValType) String() string {
	name, _ := scValTypeMap[int32(e)]
	return name
}

// EncodeTo encodes this value using the Encoder.
func (e ScValType) EncodeTo(enc *xdr.Encoder) error {
	if _, ok := scValTypeMap[int32(e)]; !ok {
		return fmt.Errorf("'%d' is not a valid ScValType enum value", e)
	}
	_, err := enc.EncodeInt(int32(e))
	return err
}

var _ decoderFrom = (*ScValType)(nil)

// DecodeFrom decodes this value using the Decoder.
func (e *ScValType) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding ScValType: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	v, n, err := d.DecodeInt()
	if err != nil {
		return n, fmt.Errorf("decoding ScValType: %w", err)
	}
	if _, ok := scValTypeMap[v]; !ok {
		return n, fmt.Errorf("'%d' is not a valid ScValType enum value", v)
	}
	*e = ScValType(v)
	return n, nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s ScValType) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *ScValType) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*ScValType)(nil)
	_ encoding.BinaryUnmarshaler = (*ScValType)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s ScValType) xdrType() {}

var _ xdrType = (*ScValType)(nil)

// ScVal is an XDR Union defines as:
//
//	union SCVal switch (SCValType type)
//	 {
//
//	 case SCV_BOOL:
//	     bool b;
//	 case SCV_VOID:
//	     void;
//	 case SCV_ERROR:
//	     SCError error;
//
//	 case SCV_U32:
//	     uint32 u32;
//	 case SCV_I32:
//	     int32 i32;
//
//	 case SCV_U64:
//	     uint64 u64;
//	 case SCV_I64:
//	     int64 i64;
//	 case SCV_TIMEPOINT:
//	     TimePoint timepoint;
//	 case SCV_DURATION:
//	     Duration duration;
//
//	 case SCV_U128:
//	     UInt128Parts u128;
//	 case SCV_I128:
//	     Int128Parts i128;
//
//	 case SCV_U256:
//	     UInt256Parts u256;
//	 case SCV_I256:
//	     Int256Parts i256;
//
//	 case SCV_BYTES:
//	     SCBytes bytes;
//	 case SCV_STRING:
//	     SCString str;
//	 case SCV_SYMBOL:
//	     SCSymbol sym;
//
//	 // Vec and Map are recursive so need to live
//	 // behind an option, due to xdrpp limitations.
//	 case SCV_VEC:
//	     SCVec *vec;
//	 case SCV_MAP:
//	     SCMap *map;
//
//	 case SCV_ADDRESS:
//	     SCAddress address;
//
//	 // Special SCVals reserved for system-constructed contract-data
//	 // ledger keys, not generally usable elsewhere.
//	 case SCV_CONTRACT_INSTANCE:
//	     SCContractInstance instance;
//	 case SCV_LEDGER_KEY_CONTRACT_INSTANCE:
//	     void;
//	 case SCV_LEDGER_KEY_NONCE:
//	     SCNonceKey nonce_key;
//
//
//	 case SCV_SPARSE_MAP:
//	     SCMap *sparseMap;
//
//	 };
type ScVal struct {
	Type      ScValType
	B         *bool
	Error     *ScError
	U32       *Uint32
	I32       *Int32
	U64       *Uint64
	I64       *Int64
	Timepoint *TimePoint
	Duration  *Duration
	U128      *UInt128Parts
	I128      *Int128Parts
	U256      *UInt256Parts
	I256      *Int256Parts
	Bytes     *ScBytes
	Str       *ScString
	Sym       *ScSymbol
	Vec       **ScVec
	Map       **ScMap
	Address   *ScAddress
	Instance  *ScContractInstance
	NonceKey  *ScNonceKey
	SparseMap **ScMap
}

// SwitchFieldName returns the field name in which this union's
// discriminant is stored
func (u ScVal) SwitchFieldName() string {
	return "Type"
}

// ArmForSwitch returns which field name should be used for storing
// the value for an instance of ScVal
func (u ScVal) ArmForSwitch(sw int32) (string, bool) {
	switch ScValType(sw) {
	case ScValTypeScvBool:
		return "B", true
	case ScValTypeScvVoid:
		return "", true
	case ScValTypeScvError:
		return "Error", true
	case ScValTypeScvU32:
		return "U32", true
	case ScValTypeScvI32:
		return "I32", true
	case ScValTypeScvU64:
		return "U64", true
	case ScValTypeScvI64:
		return "I64", true
	case ScValTypeScvTimepoint:
		return "Timepoint", true
	case ScValTypeScvDuration:
		return "Duration", true
	case ScValTypeScvU128:
		return "U128", true
	case ScValTypeScvI128:
		return "I128", true
	case ScValTypeScvU256:
		return "U256", true
	case ScValTypeScvI256:
		return "I256", true
	case ScValTypeScvBytes:
		return "Bytes", true
	case ScValTypeScvString:
		return "Str", true
	case ScValTypeScvSymbol:
		return "Sym", true
	case ScValTypeScvVec:
		return "Vec", true
	case ScValTypeScvMap:
		return "Map", true
	case ScValTypeScvAddress:
		return "Address", true
	case ScValTypeScvContractInstance:
		return "Instance", true
	case ScValTypeScvLedgerKeyContractInstance:
		return "", true
	case ScValTypeScvLedgerKeyNonce:
		return "NonceKey", true
	case ScValTypeScvSparseMap:
		return "SparseMap", true
	}
	return "-", false
}

// NewScVal creates a new  ScVal.
func NewScVal(aType ScValType, value interface{}) (result ScVal, err error) {
	result.Type = aType
	switch ScValType(aType) {
	case ScValTypeScvBool:
		tv, ok := value.(bool)
		if !ok {
			err = errors.New("invalid value, must be bool")
			return
		}
		result.B = &tv
	case ScValTypeScvVoid:
		// void
	case ScValTypeScvError:
		tv, ok := value.(ScError)
		if !ok {
			err = errors.New("invalid value, must be ScError")
			return
		}
		result.Error = &tv
	case ScValTypeScvU32:
		tv, ok := value.(Uint32)
		if !ok {
			err = errors.New("invalid value, must be Uint32")
			return
		}
		result.U32 = &tv
	case ScValTypeScvI32:
		tv, ok := value.(Int32)
		if !ok {
			err = errors.New("invalid value, must be Int32")
			return
		}
		result.I32 = &tv
	case ScValTypeScvU64:
		tv, ok := value.(Uint64)
		if !ok {
			err = errors.New("invalid value, must be Uint64")
			return
		}
		result.U64 = &tv
	case ScValTypeScvI64:
		tv, ok := value.(Int64)
		if !ok {
			err = errors.New("invalid value, must be Int64")
			return
		}
		result.I64 = &tv
	case ScValTypeScvTimepoint:
		tv, ok := value.(TimePoint)
		if !ok {
			err = errors.New("invalid value, must be TimePoint")
			return
		}
		result.Timepoint = &tv
	case ScValTypeScvDuration:
		tv, ok := value.(Duration)
		if !ok {
			err = errors.New("invalid value, must be Duration")
			return
		}
		result.Duration = &tv
	case ScValTypeScvU128:
		tv, ok := value.(UInt128Parts)
		if !ok {
			err = errors.New("invalid value, must be UInt128Parts")
			return
		}
		result.U128 = &tv
	case ScValTypeScvI128:
		tv, ok := value.(Int128Parts)
		if !ok {
			err = errors.New("invalid value, must be Int128Parts")
			return
		}
		result.I128 = &tv
	case ScValTypeScvU256:
		tv, ok := value.(UInt256Parts)
		if !ok {
			err = errors.New("invalid value, must be UInt256Parts")
			return
		}
		result.U256 = &tv
	case ScValTypeScvI256:
		tv, ok := value.(Int256Parts)
		if !ok {
			err = errors.New("invalid value, must be Int256Parts")
			return
		}
		result.I256 = &tv
	case ScValTypeScvBytes:
		tv, ok := value.(ScBytes)
		if !ok {
			err = errors.New("invalid value, must be ScBytes")
			return
		}
		result.Bytes = &tv
	case ScValTypeScvString:
		tv, ok := value.(ScString)
		if !ok {
			err = errors.New("invalid value, must be ScString")
			return
		}
		result.Str = &tv
	case ScValTypeScvSymbol:
		tv, ok := value.(ScSymbol)
		if !ok {
			err = errors.New("invalid value, must be ScSymbol")
			return
		}
		result.Sym = &tv
	case ScValTypeScvVec:
		tv, ok := value.(*ScVec)
		if !ok {
			err = errors.New("invalid value, must be *ScVec")
			return
		}
		result.Vec = &tv
	case ScValTypeScvMap:
		tv, ok := value.(*ScMap)
		if !ok {
			err = errors.New("invalid value, must be *ScMap")
			return
		}
		result.Map = &tv
	case ScValTypeScvAddress:
		tv, ok := value.(ScAddress)
		if !ok {
			err = errors.New("invalid value, must be ScAddress")
			return
		}
		result.Address = &tv
	case ScValTypeScvContractInstance:
		tv, ok := value.(ScContractInstance)
		if !ok {
			err = errors.New("invalid value, must be ScContractInstance")
			return
		}
		result.Instance = &tv
	case ScValTypeScvLedgerKeyContractInstance:
		// void
	case ScValTypeScvLedgerKeyNonce:
		tv, ok := value.(ScNonceKey)
		if !ok {
			err = errors.New("invalid value, must be ScNonceKey")
			return
		}
		result.NonceKey = &tv
	case ScValTypeScvSparseMap:
		tv, ok := value.(*ScMap)
		if !ok {
			err = errors.New("invalid value, must be *ScMap")
			return
		}
		result.SparseMap = &tv
	}
	return
}

// MustB retrieves the B value from the union,
// panicing if the value is not set.
func (u ScVal) MustB() bool {
	val, ok := u.GetB()

	if !ok {
		panic("arm B is not set")
	}

	return val
}

// GetB retrieves the B value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetB() (result bool, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "B" {
		result = *u.B
		ok = true
	}

	return
}

// MustError retrieves the Error value from the union,
// panicing if the value is not set.
func (u ScVal) MustError() ScError {
	val, ok := u.GetError()

	if !ok {
		panic("arm Error is not set")
	}

	return val
}

// GetError retrieves the Error value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetError() (result ScError, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Error" {
		result = *u.Error
		ok = true
	}

	return
}

// MustU32 retrieves the U32 value from the union,
// panicing if the value is not set.
func (u ScVal) MustU32() Uint32 {
	val, ok := u.GetU32()

	if !ok {
		panic("arm U32 is not set")
	}

	return val
}

// GetU32 retrieves the U32 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetU32() (result Uint32, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "U32" {
		result = *u.U32
		ok = true
	}

	return
}

// MustI32 retrieves the I32 value from the union,
// panicing if the value is not set.
func (u ScVal) MustI32() Int32 {
	val, ok := u.GetI32()

	if !ok {
		panic("arm I32 is not set")
	}

	return val
}

// GetI32 retrieves the I32 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetI32() (result Int32, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "I32" {
		result = *u.I32
		ok = true
	}

	return
}

// MustU64 retrieves the U64 value from the union,
// panicing if the value is not set.
func (u ScVal) MustU64() Uint64 {
	val, ok := u.GetU64()

	if !ok {
		panic("arm U64 is not set")
	}

	return val
}

// GetU64 retrieves the U64 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetU64() (result Uint64, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "U64" {
		result = *u.U64
		ok = true
	}

	return
}

// MustI64 retrieves the I64 value from the union,
// panicing if the value is not set.
func (u ScVal) MustI64() Int64 {
	val, ok := u.GetI64()

	if !ok {
		panic("arm I64 is not set")
	}

	return val
}

// GetI64 retrieves the I64 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetI64() (result Int64, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "I64" {
		result = *u.I64
		ok = true
	}

	return
}

// MustTimepoint retrieves the Timepoint value from the union,
// panicing if the value is not set.
func (u ScVal) MustTimepoint() TimePoint {
	val, ok := u.GetTimepoint()

	if !ok {
		panic("arm Timepoint is not set")
	}

	return val
}

// GetTimepoint retrieves the Timepoint value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetTimepoint() (result TimePoint, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Timepoint" {
		result = *u.Timepoint
		ok = true
	}

	return
}

// MustDuration retrieves the Duration value from the union,
// panicing if the value is not set.
func (u ScVal) MustDuration() Duration {
	val, ok := u.GetDuration()

	if !ok {
		panic("arm Duration is not set")
	}

	return val
}

// GetDuration retrieves the Duration value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetDuration() (result Duration, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Duration" {
		result = *u.Duration
		ok = true
	}

	return
}

// MustU128 retrieves the U128 value from the union,
// panicing if the value is not set.
func (u ScVal) MustU128() UInt128Parts {
	val, ok := u.GetU128()

	if !ok {
		panic("arm U128 is not set")
	}

	return val
}

// GetU128 retrieves the U128 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetU128() (result UInt128Parts, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "U128" {
		result = *u.U128
		ok = true
	}

	return
}

// MustI128 retrieves the I128 value from the union,
// panicing if the value is not set.
func (u ScVal) MustI128() Int128Parts {
	val, ok := u.GetI128()

	if !ok {
		panic("arm I128 is not set")
	}

	return val
}

// GetI128 retrieves the I128 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetI128() (result Int128Parts, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "I128" {
		result = *u.I128
		ok = true
	}

	return
}

// MustU256 retrieves the U256 value from the union,
// panicing if the value is not set.
func (u ScVal) MustU256() UInt256Parts {
	val, ok := u.GetU256()

	if !ok {
		panic("arm U256 is not set")
	}

	return val
}

// GetU256 retrieves the U256 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetU256() (result UInt256Parts, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "U256" {
		result = *u.U256
		ok = true
	}

	return
}

// MustI256 retrieves the I256 value from the union,
// panicing if the value is not set.
func (u ScVal) MustI256() Int256Parts {
	val, ok := u.GetI256()

	if !ok {
		panic("arm I256 is not set")
	}

	return val
}

// GetI256 retrieves the I256 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetI256() (result Int256Parts, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "I256" {
		result = *u.I256
		ok = true
	}

	return
}

// MustBytes retrieves the Bytes value from the union,
// panicing if the value is not set.
func (u ScVal) MustBytes() ScBytes {
	val, ok := u.GetBytes()

	if !ok {
		panic("arm Bytes is not set")
	}

	return val
}

// GetBytes retrieves the Bytes value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetBytes() (result ScBytes, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Bytes" {
		result = *u.Bytes
		ok = true
	}

	return
}

// MustStr retrieves the Str value from the union,
// panicing if the value is not set.
func (u ScVal) MustStr() ScString {
	val, ok := u.GetStr()

	if !ok {
		panic("arm Str is not set")
	}

	return val
}

// GetStr retrieves the Str value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetStr() (result ScString, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Str" {
		result = *u.Str
		ok = true
	}

	return
}

// MustSym retrieves the Sym value from the union,
// panicing if the value is not set.
func (u ScVal) MustSym() ScSymbol {
	val, ok := u.GetSym()

	if !ok {
		panic("arm Sym is not set")
	}

	return val
}

// GetSym retrieves the Sym value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetSym() (result ScSymbol, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Sym" {
		result = *u.Sym
		ok = true
	}

	return
}

// MustVec retrieves the Vec value from the union,
// panicing if the value is not set.
func (u ScVal) MustVec() *ScVec {
	val, ok := u.GetVec()

	if !ok {
		panic("arm Vec is not set")
	}

	return val
}

// GetVec retrieves the Vec value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetVec() (result *ScVec, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Vec" {
		result = *u.Vec
		ok = true
	}

	return
}

// MustMap retrieves the Map value from the union,
// panicing if the value is not set.
func (u ScVal) MustMap() *ScMap {
	val, ok := u.GetMap()

	if !ok {
		panic("arm Map is not set")
	}

	return val
}

// GetMap retrieves the Map value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetMap() (result *ScMap, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Map" {
		result = *u.Map
		ok = true
	}

	return
}

// MustAddress retrieves the Address value from the union,
// panicing if the value is not set.
func (u ScVal) MustAddress() ScAddress {
	val, ok := u.GetAddress()

	if !ok {
		panic("arm Address is not set")
	}

	return val
}

// GetAddress retrieves the Address value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetAddress() (result ScAddress, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Address" {
		result = *u.Address
		ok = true
	}

	return
}

// MustInstance retrieves the Instance value from the union,
// panicing if the value is not set.
func (u ScVal) MustInstance() ScContractInstance {
	val, ok := u.GetInstance()

	if !ok {
		panic("arm Instance is not set")
	}

	return val
}

// GetInstance retrieves the Instance value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetInstance() (result ScContractInstance, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Instance" {
		result = *u.Instance
		ok = true
	}

	return
}

// MustNonceKey retrieves the NonceKey value from the union,
// panicing if the value is not set.
func (u ScVal) MustNonceKey() ScNonceKey {
	val, ok := u.GetNonceKey()

	if !ok {
		panic("arm NonceKey is not set")
	}

	return val
}

// GetNonceKey retrieves the NonceKey value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetNonceKey() (result ScNonceKey, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "NonceKey" {
		result = *u.NonceKey
		ok = true
	}

	return
}

// MustSparseMap retrieves the SparseMap value from the union,
// panicing if the value is not set.
func (u ScVal) MustSparseMap() *ScMap {
	val, ok := u.GetSparseMap()

	if !ok {
		panic("arm SparseMap is not set")
	}

	return val
}

// GetSparseMap retrieves the SparseMap value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u ScVal) GetSparseMap() (result *ScMap, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "SparseMap" {
		result = *u.SparseMap
		ok = true
	}

	return
}

// EncodeTo encodes this value using the Encoder.
func (u ScVal) EncodeTo(e *xdr.Encoder) error {
	var err error
	if err = u.Type.EncodeTo(e); err != nil {
		return err
	}
	switch ScValType(u.Type) {
	case ScValTypeScvBool:
		if _, err = e.EncodeBool(bool((*u.B))); err != nil {
			return err
		}
		return nil
	case ScValTypeScvVoid:
		// Void
		return nil
	case ScValTypeScvError:
		if err = (*u.Error).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvU32:
		if err = (*u.U32).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvI32:
		if err = (*u.I32).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvU64:
		if err = (*u.U64).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvI64:
		if err = (*u.I64).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvTimepoint:
		if err = (*u.Timepoint).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvDuration:
		if err = (*u.Duration).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvU128:
		if err = (*u.U128).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvI128:
		if err = (*u.I128).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvU256:
		if err = (*u.U256).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvI256:
		if err = (*u.I256).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvBytes:
		if err = (*u.Bytes).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvString:
		if err = (*u.Str).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvSymbol:
		if err = (*u.Sym).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvVec:
		if _, err = e.EncodeBool((*u.Vec) != nil); err != nil {
			return err
		}
		if (*u.Vec) != nil {
			if err = (*(*u.Vec)).EncodeTo(e); err != nil {
				return err
			}
		}
		return nil
	case ScValTypeScvMap:
		if _, err = e.EncodeBool((*u.Map) != nil); err != nil {
			return err
		}
		if (*u.Map) != nil {
			if err = (*(*u.Map)).EncodeTo(e); err != nil {
				return err
			}
		}
		return nil
	case ScValTypeScvAddress:
		if err = (*u.Address).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvContractInstance:
		if err = (*u.Instance).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvLedgerKeyContractInstance:
		// Void
		return nil
	case ScValTypeScvLedgerKeyNonce:
		if err = (*u.NonceKey).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case ScValTypeScvSparseMap:
		if _, err = e.EncodeBool((*u.SparseMap) != nil); err != nil {
			return err
		}
		if (*u.SparseMap) != nil {
			if err = (*(*u.SparseMap)).EncodeTo(e); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("Type (ScValType) switch value '%d' is not valid for union ScVal", u.Type)
}

var _ decoderFrom = (*ScVal)(nil)

// DecodeFrom decodes this value using the Decoder.
func (u *ScVal) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding ScVal: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	var err error
	var n, nTmp int
	nTmp, err = u.Type.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding ScValType: %w", err)
	}
	switch ScValType(u.Type) {
	case ScValTypeScvBool:
		u.B = new(bool)
		(*u.B), nTmp, err = d.DecodeBool()
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding Bool: %w", err)
		}
		return n, nil
	case ScValTypeScvVoid:
		// Void
		return n, nil
	case ScValTypeScvError:
		u.Error = new(ScError)
		nTmp, err = (*u.Error).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ScError: %w", err)
		}
		return n, nil
	case ScValTypeScvU32:
		u.U32 = new(Uint32)
		nTmp, err = (*u.U32).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding Uint32: %w", err)
		}
		return n, nil
	case ScValTypeScvI32:
		u.I32 = new(Int32)
		nTmp, err = (*u.I32).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding Int32: %w", err)
		}
		return n, nil
	case ScValTypeScvU64:
		u.U64 = new(Uint64)
		nTmp, err = (*u.U64).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding Uint64: %w", err)
		}
		return n, nil
	case ScValTypeScvI64:
		u.I64 = new(Int64)
		nTmp, err = (*u.I64).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding Int64: %w", err)
		}
		return n, nil
	case ScValTypeScvTimepoint:
		u.Timepoint = new(TimePoint)
		nTmp, err = (*u.Timepoint).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding TimePoint: %w", err)
		}
		return n, nil
	case ScValTypeScvDuration:
		u.Duration = new(Duration)
		nTmp, err = (*u.Duration).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding Duration: %w", err)
		}
		return n, nil
	case ScValTypeScvU128:
		u.U128 = new(UInt128Parts)
		nTmp, err = (*u.U128).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding UInt128Parts: %w", err)
		}
		return n, nil
	case ScValTypeScvI128:
		u.I128 = new(Int128Parts)
		nTmp, err = (*u.I128).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding Int128Parts: %w", err)
		}
		return n, nil
	case ScValTypeScvU256:
		u.U256 = new(UInt256Parts)
		nTmp, err = (*u.U256).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding UInt256Parts: %w", err)
		}
		return n, nil
	case ScValTypeScvI256:
		u.I256 = new(Int256Parts)
		nTmp, err = (*u.I256).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding Int256Parts: %w", err)
		}
		return n, nil
	case ScValTypeScvBytes:
		u.Bytes = new(ScBytes)
		nTmp, err = (*u.Bytes).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ScBytes: %w", err)
		}
		return n, nil
	case ScValTypeScvString:
		u.Str = new(ScString)
		nTmp, err = (*u.Str).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ScString: %w", err)
		}
		return n, nil
	case ScValTypeScvSymbol:
		u.Sym = new(ScSymbol)
		nTmp, err = (*u.Sym).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ScSymbol: %w", err)
		}
		return n, nil
	case ScValTypeScvVec:
		u.Vec = new(*ScVec)
		var b bool
		b, nTmp, err = d.DecodeBool()
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ScVec: %w", err)
		}
		(*u.Vec) = nil
		if b {
			(*u.Vec) = new(ScVec)
			nTmp, err = (*u.Vec).DecodeFrom(d, maxDepth)
			n += nTmp
			if err != nil {
				return n, fmt.Errorf("decoding ScVec: %w", err)
			}
		}
		return n, nil
	case ScValTypeScvMap:
		u.Map = new(*ScMap)
		var b bool
		b, nTmp, err = d.DecodeBool()
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ScMap: %w", err)
		}
		(*u.Map) = nil
		if b {
			(*u.Map) = new(ScMap)
			nTmp, err = (*u.Map).DecodeFrom(d, maxDepth)
			n += nTmp
			if err != nil {
				return n, fmt.Errorf("decoding ScMap: %w", err)
			}
		}
		return n, nil
	case ScValTypeScvAddress:
		u.Address = new(ScAddress)
		nTmp, err = (*u.Address).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ScAddress: %w", err)
		}
		return n, nil
	case ScValTypeScvContractInstance:
		u.Instance = new(ScContractInstance)
		nTmp, err = (*u.Instance).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ScContractInstance: %w", err)
		}
		return n, nil
	case ScValTypeScvLedgerKeyContractInstance:
		// Void
		return n, nil
	case ScValTypeScvLedgerKeyNonce:
		u.NonceKey = new(ScNonceKey)
		nTmp, err = (*u.NonceKey).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ScNonceKey: %w", err)
		}
		return n, nil
	case ScValTypeScvSparseMap:
		u.SparseMap = new(*ScMap)
		var b bool
		b, nTmp, err = d.DecodeBool()
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ScMap: %w", err)
		}
		(*u.SparseMap) = nil
		if b {
			(*u.SparseMap) = new(ScMap)
			nTmp, err = (*u.SparseMap).DecodeFrom(d, maxDepth)
			n += nTmp
			if err != nil {
				return n, fmt.Errorf("decoding ScMap: %w", err)
			}
		}
		return n, nil
	}
	return n, fmt.Errorf("union ScVal has invalid Type (ScValType) switch value '%d'", u.Type)
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s ScVal) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *ScVal) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*ScVal)(nil)
	_ encoding.BinaryUnmarshaler = (*ScVal)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s ScVal) xdrType() {}

var _ xdrType = (*ScVal)(nil)
