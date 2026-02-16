//go:build !xdr_hello_world

//lint:file-ignore S1005 The issue should be fixed in xdrgen. Unfortunately, there's no way to ignore a single file in staticcheck.

// DO NOT EDIT or your changes may be overwritten
package xdr

import (
	"bytes"
	"encoding"
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
