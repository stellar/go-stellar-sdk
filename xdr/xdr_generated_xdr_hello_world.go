//go:build xdr_hello_world

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

// OperationType is an XDR Enum defines as:
//
//	enum OperationType
//	 {
//	     CREATE_ACCOUNT = 0,
//	     PAYMENT = 1,
//	     PATH_PAYMENT_STRICT_RECEIVE = 2,
//	     MANAGE_SELL_OFFER = 3,
//	     CREATE_PASSIVE_SELL_OFFER = 4,
//	     SET_OPTIONS = 5,
//	     CHANGE_TRUST = 6,
//	     ALLOW_TRUST = 7,
//	     ACCOUNT_MERGE = 8,
//	     INFLATION = 9,
//	     MANAGE_DATA = 10,
//	     BUMP_SEQUENCE = 11,
//	     MANAGE_BUY_OFFER = 12,
//	     PATH_PAYMENT_STRICT_SEND = 13,
//	     CREATE_CLAIMABLE_BALANCE = 14,
//	     CLAIM_CLAIMABLE_BALANCE = 15,
//	     BEGIN_SPONSORING_FUTURE_RESERVES = 16,
//	     END_SPONSORING_FUTURE_RESERVES = 17,
//	     REVOKE_SPONSORSHIP = 18,
//	     CLAWBACK = 19,
//	     CLAWBACK_CLAIMABLE_BALANCE = 20,
//	     SET_TRUST_LINE_FLAGS = 21,
//	     LIQUIDITY_POOL_DEPOSIT = 22,
//	     LIQUIDITY_POOL_WITHDRAW = 23,
//	     INVOKE_HOST_FUNCTION = 24,
//	     EXTEND_FOOTPRINT_TTL = 25,
//
//	     RESTORE_FOOTPRINT = 26,
//	     HELLO_WORLD = 27
//
//	     RESTORE_FOOTPRINT = 26
//
//	 };
type OperationType int32

const (
	OperationTypeCreateAccount                 OperationType = 0
	OperationTypePayment                       OperationType = 1
	OperationTypePathPaymentStrictReceive      OperationType = 2
	OperationTypeManageSellOffer               OperationType = 3
	OperationTypeCreatePassiveSellOffer        OperationType = 4
	OperationTypeSetOptions                    OperationType = 5
	OperationTypeChangeTrust                   OperationType = 6
	OperationTypeAllowTrust                    OperationType = 7
	OperationTypeAccountMerge                  OperationType = 8
	OperationTypeInflation                     OperationType = 9
	OperationTypeManageData                    OperationType = 10
	OperationTypeBumpSequence                  OperationType = 11
	OperationTypeManageBuyOffer                OperationType = 12
	OperationTypePathPaymentStrictSend         OperationType = 13
	OperationTypeCreateClaimableBalance        OperationType = 14
	OperationTypeClaimClaimableBalance         OperationType = 15
	OperationTypeBeginSponsoringFutureReserves OperationType = 16
	OperationTypeEndSponsoringFutureReserves   OperationType = 17
	OperationTypeRevokeSponsorship             OperationType = 18
	OperationTypeClawback                      OperationType = 19
	OperationTypeClawbackClaimableBalance      OperationType = 20
	OperationTypeSetTrustLineFlags             OperationType = 21
	OperationTypeLiquidityPoolDeposit          OperationType = 22
	OperationTypeLiquidityPoolWithdraw         OperationType = 23
	OperationTypeInvokeHostFunction            OperationType = 24
	OperationTypeExtendFootprintTtl            OperationType = 25
	OperationTypeRestoreFootprint              OperationType = 26
	OperationTypeHelloWorld                    OperationType = 27
)

var operationTypeMap = map[int32]string{
	0:  "OperationTypeCreateAccount",
	1:  "OperationTypePayment",
	2:  "OperationTypePathPaymentStrictReceive",
	3:  "OperationTypeManageSellOffer",
	4:  "OperationTypeCreatePassiveSellOffer",
	5:  "OperationTypeSetOptions",
	6:  "OperationTypeChangeTrust",
	7:  "OperationTypeAllowTrust",
	8:  "OperationTypeAccountMerge",
	9:  "OperationTypeInflation",
	10: "OperationTypeManageData",
	11: "OperationTypeBumpSequence",
	12: "OperationTypeManageBuyOffer",
	13: "OperationTypePathPaymentStrictSend",
	14: "OperationTypeCreateClaimableBalance",
	15: "OperationTypeClaimClaimableBalance",
	16: "OperationTypeBeginSponsoringFutureReserves",
	17: "OperationTypeEndSponsoringFutureReserves",
	18: "OperationTypeRevokeSponsorship",
	19: "OperationTypeClawback",
	20: "OperationTypeClawbackClaimableBalance",
	21: "OperationTypeSetTrustLineFlags",
	22: "OperationTypeLiquidityPoolDeposit",
	23: "OperationTypeLiquidityPoolWithdraw",
	24: "OperationTypeInvokeHostFunction",
	25: "OperationTypeExtendFootprintTtl",
	26: "OperationTypeRestoreFootprint",
	27: "OperationTypeHelloWorld",
}

// ValidEnum validates a proposed value for this enum.  Implements
// the Enum interface for OperationType
func (e OperationType) ValidEnum(v int32) bool {
	_, ok := operationTypeMap[v]
	return ok
}

// String returns the name of `e`
func (e OperationType) String() string {
	name, _ := operationTypeMap[int32(e)]
	return name
}

// EncodeTo encodes this value using the Encoder.
func (e OperationType) EncodeTo(enc *xdr.Encoder) error {
	if _, ok := operationTypeMap[int32(e)]; !ok {
		return fmt.Errorf("'%d' is not a valid OperationType enum value", e)
	}
	_, err := enc.EncodeInt(int32(e))
	return err
}

var _ decoderFrom = (*OperationType)(nil)

// DecodeFrom decodes this value using the Decoder.
func (e *OperationType) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding OperationType: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	v, n, err := d.DecodeInt()
	if err != nil {
		return n, fmt.Errorf("decoding OperationType: %w", err)
	}
	if _, ok := operationTypeMap[v]; !ok {
		return n, fmt.Errorf("'%d' is not a valid OperationType enum value", v)
	}
	*e = OperationType(v)
	return n, nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s OperationType) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *OperationType) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*OperationType)(nil)
	_ encoding.BinaryUnmarshaler = (*OperationType)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s OperationType) xdrType() {}

var _ xdrType = (*OperationType)(nil)

// HelloWorldOp is an XDR Struct defines as:
//
//	struct HelloWorldOp
//	 {
//	     AccountID helloTo;
//	 };
type HelloWorldOp struct {
	HelloTo AccountId
}

// EncodeTo encodes this value using the Encoder.
func (s *HelloWorldOp) EncodeTo(e *xdr.Encoder) error {
	var err error
	if err = s.HelloTo.EncodeTo(e); err != nil {
		return err
	}
	return nil
}

var _ decoderFrom = (*HelloWorldOp)(nil)

// DecodeFrom decodes this value using the Decoder.
func (s *HelloWorldOp) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding HelloWorldOp: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	var err error
	var n, nTmp int
	nTmp, err = s.HelloTo.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding AccountId: %w", err)
	}
	return n, nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s HelloWorldOp) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *HelloWorldOp) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*HelloWorldOp)(nil)
	_ encoding.BinaryUnmarshaler = (*HelloWorldOp)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s HelloWorldOp) xdrType() {}

var _ xdrType = (*HelloWorldOp)(nil)

// HelloWorldResultCode is an XDR Enum defines as:
//
//	enum HelloWorldResultCode
//	 {
//	     HELLO_WORLD_SUCCESS = 0,
//	     HELLO_WORLD_MALFORMED = -1
//	 };
type HelloWorldResultCode int32

const (
	HelloWorldResultCodeHelloWorldSuccess   HelloWorldResultCode = 0
	HelloWorldResultCodeHelloWorldMalformed HelloWorldResultCode = -1
)

var helloWorldResultCodeMap = map[int32]string{
	0:  "HelloWorldResultCodeHelloWorldSuccess",
	-1: "HelloWorldResultCodeHelloWorldMalformed",
}

// ValidEnum validates a proposed value for this enum.  Implements
// the Enum interface for HelloWorldResultCode
func (e HelloWorldResultCode) ValidEnum(v int32) bool {
	_, ok := helloWorldResultCodeMap[v]
	return ok
}

// String returns the name of `e`
func (e HelloWorldResultCode) String() string {
	name, _ := helloWorldResultCodeMap[int32(e)]
	return name
}

// EncodeTo encodes this value using the Encoder.
func (e HelloWorldResultCode) EncodeTo(enc *xdr.Encoder) error {
	if _, ok := helloWorldResultCodeMap[int32(e)]; !ok {
		return fmt.Errorf("'%d' is not a valid HelloWorldResultCode enum value", e)
	}
	_, err := enc.EncodeInt(int32(e))
	return err
}

var _ decoderFrom = (*HelloWorldResultCode)(nil)

// DecodeFrom decodes this value using the Decoder.
func (e *HelloWorldResultCode) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding HelloWorldResultCode: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	v, n, err := d.DecodeInt()
	if err != nil {
		return n, fmt.Errorf("decoding HelloWorldResultCode: %w", err)
	}
	if _, ok := helloWorldResultCodeMap[v]; !ok {
		return n, fmt.Errorf("'%d' is not a valid HelloWorldResultCode enum value", v)
	}
	*e = HelloWorldResultCode(v)
	return n, nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s HelloWorldResultCode) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *HelloWorldResultCode) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*HelloWorldResultCode)(nil)
	_ encoding.BinaryUnmarshaler = (*HelloWorldResultCode)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s HelloWorldResultCode) xdrType() {}

var _ xdrType = (*HelloWorldResultCode)(nil)

// HelloWorldResult is an XDR Union defines as:
//
//	union HelloWorldResult switch (HelloWorldResultCode code)
//	 {
//	 case HELLO_WORLD_SUCCESS:
//	     void;
//	 case HELLO_WORLD_MALFORMED:
//	     void;
//	 };
type HelloWorldResult struct {
	Code HelloWorldResultCode
}

// SwitchFieldName returns the field name in which this union's
// discriminant is stored
func (u HelloWorldResult) SwitchFieldName() string {
	return "Code"
}

// ArmForSwitch returns which field name should be used for storing
// the value for an instance of HelloWorldResult
func (u HelloWorldResult) ArmForSwitch(sw int32) (string, bool) {
	switch HelloWorldResultCode(sw) {
	case HelloWorldResultCodeHelloWorldSuccess:
		return "", true
	case HelloWorldResultCodeHelloWorldMalformed:
		return "", true
	}
	return "-", false
}

// NewHelloWorldResult creates a new  HelloWorldResult.
func NewHelloWorldResult(code HelloWorldResultCode, value interface{}) (result HelloWorldResult, err error) {
	result.Code = code
	switch HelloWorldResultCode(code) {
	case HelloWorldResultCodeHelloWorldSuccess:
		// void
	case HelloWorldResultCodeHelloWorldMalformed:
		// void
	}
	return
}

// EncodeTo encodes this value using the Encoder.
func (u HelloWorldResult) EncodeTo(e *xdr.Encoder) error {
	var err error
	if err = u.Code.EncodeTo(e); err != nil {
		return err
	}
	switch HelloWorldResultCode(u.Code) {
	case HelloWorldResultCodeHelloWorldSuccess:
		// Void
		return nil
	case HelloWorldResultCodeHelloWorldMalformed:
		// Void
		return nil
	}
	return fmt.Errorf("Code (HelloWorldResultCode) switch value '%d' is not valid for union HelloWorldResult", u.Code)
}

var _ decoderFrom = (*HelloWorldResult)(nil)

// DecodeFrom decodes this value using the Decoder.
func (u *HelloWorldResult) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding HelloWorldResult: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	var err error
	var n, nTmp int
	nTmp, err = u.Code.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding HelloWorldResultCode: %w", err)
	}
	switch HelloWorldResultCode(u.Code) {
	case HelloWorldResultCodeHelloWorldSuccess:
		// Void
		return n, nil
	case HelloWorldResultCodeHelloWorldMalformed:
		// Void
		return n, nil
	}
	return n, fmt.Errorf("union HelloWorldResult has invalid Code (HelloWorldResultCode) switch value '%d'", u.Code)
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s HelloWorldResult) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *HelloWorldResult) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*HelloWorldResult)(nil)
	_ encoding.BinaryUnmarshaler = (*HelloWorldResult)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s HelloWorldResult) xdrType() {}

var _ xdrType = (*HelloWorldResult)(nil)
