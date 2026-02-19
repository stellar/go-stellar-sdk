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

// Operation is an XDR Struct defines as:
//
//	struct Operation
//	 {
//	     // sourceAccount is the account used to run the operation
//	     // if not set, the runtime defaults to "sourceAccount" specified at
//	     // the transaction level
//	     MuxedAccount* sourceAccount;
//
//	     union switch (OperationType type)
//	     {
//	     case CREATE_ACCOUNT:
//	         CreateAccountOp createAccountOp;
//	     case PAYMENT:
//	         PaymentOp paymentOp;
//	     case PATH_PAYMENT_STRICT_RECEIVE:
//	         PathPaymentStrictReceiveOp pathPaymentStrictReceiveOp;
//	     case MANAGE_SELL_OFFER:
//	         ManageSellOfferOp manageSellOfferOp;
//	     case CREATE_PASSIVE_SELL_OFFER:
//	         CreatePassiveSellOfferOp createPassiveSellOfferOp;
//	     case SET_OPTIONS:
//	         SetOptionsOp setOptionsOp;
//	     case CHANGE_TRUST:
//	         ChangeTrustOp changeTrustOp;
//	     case ALLOW_TRUST:
//	         AllowTrustOp allowTrustOp;
//	     case ACCOUNT_MERGE:
//	         MuxedAccount destination;
//	     case INFLATION:
//	         void;
//	     case MANAGE_DATA:
//	         ManageDataOp manageDataOp;
//	     case BUMP_SEQUENCE:
//	         BumpSequenceOp bumpSequenceOp;
//	     case MANAGE_BUY_OFFER:
//	         ManageBuyOfferOp manageBuyOfferOp;
//	     case PATH_PAYMENT_STRICT_SEND:
//	         PathPaymentStrictSendOp pathPaymentStrictSendOp;
//	     case CREATE_CLAIMABLE_BALANCE:
//	         CreateClaimableBalanceOp createClaimableBalanceOp;
//	     case CLAIM_CLAIMABLE_BALANCE:
//	         ClaimClaimableBalanceOp claimClaimableBalanceOp;
//	     case BEGIN_SPONSORING_FUTURE_RESERVES:
//	         BeginSponsoringFutureReservesOp beginSponsoringFutureReservesOp;
//	     case END_SPONSORING_FUTURE_RESERVES:
//	         void;
//	     case REVOKE_SPONSORSHIP:
//	         RevokeSponsorshipOp revokeSponsorshipOp;
//	     case CLAWBACK:
//	         ClawbackOp clawbackOp;
//	     case CLAWBACK_CLAIMABLE_BALANCE:
//	         ClawbackClaimableBalanceOp clawbackClaimableBalanceOp;
//	     case SET_TRUST_LINE_FLAGS:
//	         SetTrustLineFlagsOp setTrustLineFlagsOp;
//	     case LIQUIDITY_POOL_DEPOSIT:
//	         LiquidityPoolDepositOp liquidityPoolDepositOp;
//	     case LIQUIDITY_POOL_WITHDRAW:
//	         LiquidityPoolWithdrawOp liquidityPoolWithdrawOp;
//	     case INVOKE_HOST_FUNCTION:
//	         InvokeHostFunctionOp invokeHostFunctionOp;
//	     case EXTEND_FOOTPRINT_TTL:
//	         ExtendFootprintTTLOp extendFootprintTTLOp;
//	     case RESTORE_FOOTPRINT:
//	         RestoreFootprintOp restoreFootprintOp;
//
//	     case HELLO_WORLD:
//	         HelloWorldOp helloWorldOp;
//
//	     }
//	     body;
//	 };
type Operation struct {
	SourceAccount *MuxedAccount
	Body          OperationBody
}

// EncodeTo encodes this value using the Encoder.
func (s *Operation) EncodeTo(e *xdr.Encoder) error {
	var err error
	if _, err = e.EncodeBool(s.SourceAccount != nil); err != nil {
		return err
	}
	if s.SourceAccount != nil {
		if err = (*s.SourceAccount).EncodeTo(e); err != nil {
			return err
		}
	}
	if err = s.Body.EncodeTo(e); err != nil {
		return err
	}
	return nil
}

var _ decoderFrom = (*Operation)(nil)

// DecodeFrom decodes this value using the Decoder.
func (s *Operation) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding Operation: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	var err error
	var n, nTmp int
	var b bool
	b, nTmp, err = d.DecodeBool()
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding MuxedAccount: %w", err)
	}
	s.SourceAccount = nil
	if b {
		s.SourceAccount = new(MuxedAccount)
		nTmp, err = s.SourceAccount.DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding MuxedAccount: %w", err)
		}
	}
	nTmp, err = s.Body.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding OperationBody: %w", err)
	}
	return n, nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s Operation) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *Operation) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*Operation)(nil)
	_ encoding.BinaryUnmarshaler = (*Operation)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s Operation) xdrType() {}

var _ xdrType = (*Operation)(nil)

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

// OperationResult is an XDR Union defines as:
//
//	union OperationResult switch (OperationResultCode code)
//	 {
//	 case opINNER:
//	     union switch (OperationType type)
//	     {
//	     case CREATE_ACCOUNT:
//	         CreateAccountResult createAccountResult;
//	     case PAYMENT:
//	         PaymentResult paymentResult;
//	     case PATH_PAYMENT_STRICT_RECEIVE:
//	         PathPaymentStrictReceiveResult pathPaymentStrictReceiveResult;
//	     case MANAGE_SELL_OFFER:
//	         ManageSellOfferResult manageSellOfferResult;
//	     case CREATE_PASSIVE_SELL_OFFER:
//	         ManageSellOfferResult createPassiveSellOfferResult;
//	     case SET_OPTIONS:
//	         SetOptionsResult setOptionsResult;
//	     case CHANGE_TRUST:
//	         ChangeTrustResult changeTrustResult;
//	     case ALLOW_TRUST:
//	         AllowTrustResult allowTrustResult;
//	     case ACCOUNT_MERGE:
//	         AccountMergeResult accountMergeResult;
//	     case INFLATION:
//	         InflationResult inflationResult;
//	     case MANAGE_DATA:
//	         ManageDataResult manageDataResult;
//	     case BUMP_SEQUENCE:
//	         BumpSequenceResult bumpSeqResult;
//	     case MANAGE_BUY_OFFER:
//	         ManageBuyOfferResult manageBuyOfferResult;
//	     case PATH_PAYMENT_STRICT_SEND:
//	         PathPaymentStrictSendResult pathPaymentStrictSendResult;
//	     case CREATE_CLAIMABLE_BALANCE:
//	         CreateClaimableBalanceResult createClaimableBalanceResult;
//	     case CLAIM_CLAIMABLE_BALANCE:
//	         ClaimClaimableBalanceResult claimClaimableBalanceResult;
//	     case BEGIN_SPONSORING_FUTURE_RESERVES:
//	         BeginSponsoringFutureReservesResult beginSponsoringFutureReservesResult;
//	     case END_SPONSORING_FUTURE_RESERVES:
//	         EndSponsoringFutureReservesResult endSponsoringFutureReservesResult;
//	     case REVOKE_SPONSORSHIP:
//	         RevokeSponsorshipResult revokeSponsorshipResult;
//	     case CLAWBACK:
//	         ClawbackResult clawbackResult;
//	     case CLAWBACK_CLAIMABLE_BALANCE:
//	         ClawbackClaimableBalanceResult clawbackClaimableBalanceResult;
//	     case SET_TRUST_LINE_FLAGS:
//	         SetTrustLineFlagsResult setTrustLineFlagsResult;
//	     case LIQUIDITY_POOL_DEPOSIT:
//	         LiquidityPoolDepositResult liquidityPoolDepositResult;
//	     case LIQUIDITY_POOL_WITHDRAW:
//	         LiquidityPoolWithdrawResult liquidityPoolWithdrawResult;
//	     case INVOKE_HOST_FUNCTION:
//	         InvokeHostFunctionResult invokeHostFunctionResult;
//	     case EXTEND_FOOTPRINT_TTL:
//	         ExtendFootprintTTLResult extendFootprintTTLResult;
//	     case RESTORE_FOOTPRINT:
//	         RestoreFootprintResult restoreFootprintResult;
//
//	     case HELLO_WORLD:
//	         HelloWorldResult helloWorldResult;
//
//	     }
//	     tr;
//	 case opBAD_AUTH:
//	 case opNO_ACCOUNT:
//	 case opNOT_SUPPORTED:
//	 case opTOO_MANY_SUBENTRIES:
//	 case opEXCEEDED_WORK_LIMIT:
//	 case opTOO_MANY_SPONSORING:
//	     void;
//	 };
type OperationResult struct {
	Code OperationResultCode
	Tr   *OperationResultTr
}

// SwitchFieldName returns the field name in which this union's
// discriminant is stored
func (u OperationResult) SwitchFieldName() string {
	return "Code"
}

// ArmForSwitch returns which field name should be used for storing
// the value for an instance of OperationResult
func (u OperationResult) ArmForSwitch(sw int32) (string, bool) {
	switch OperationResultCode(sw) {
	case OperationResultCodeOpInner:
		return "Tr", true
	case OperationResultCodeOpBadAuth:
		return "", true
	case OperationResultCodeOpNoAccount:
		return "", true
	case OperationResultCodeOpNotSupported:
		return "", true
	case OperationResultCodeOpTooManySubentries:
		return "", true
	case OperationResultCodeOpExceededWorkLimit:
		return "", true
	case OperationResultCodeOpTooManySponsoring:
		return "", true
	}
	return "-", false
}

// NewOperationResult creates a new  OperationResult.
func NewOperationResult(code OperationResultCode, value interface{}) (result OperationResult, err error) {
	result.Code = code
	switch OperationResultCode(code) {
	case OperationResultCodeOpInner:
		tv, ok := value.(OperationResultTr)
		if !ok {
			err = errors.New("invalid value, must be OperationResultTr")
			return
		}
		result.Tr = &tv
	case OperationResultCodeOpBadAuth:
		// void
	case OperationResultCodeOpNoAccount:
		// void
	case OperationResultCodeOpNotSupported:
		// void
	case OperationResultCodeOpTooManySubentries:
		// void
	case OperationResultCodeOpExceededWorkLimit:
		// void
	case OperationResultCodeOpTooManySponsoring:
		// void
	}
	return
}

// MustTr retrieves the Tr value from the union,
// panicing if the value is not set.
func (u OperationResult) MustTr() OperationResultTr {
	val, ok := u.GetTr()

	if !ok {
		panic("arm Tr is not set")
	}

	return val
}

// GetTr retrieves the Tr value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResult) GetTr() (result OperationResultTr, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Code))

	if armName == "Tr" {
		result = *u.Tr
		ok = true
	}

	return
}

// EncodeTo encodes this value using the Encoder.
func (u OperationResult) EncodeTo(e *xdr.Encoder) error {
	var err error
	if err = u.Code.EncodeTo(e); err != nil {
		return err
	}
	switch OperationResultCode(u.Code) {
	case OperationResultCodeOpInner:
		if err = (*u.Tr).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationResultCodeOpBadAuth:
		// Void
		return nil
	case OperationResultCodeOpNoAccount:
		// Void
		return nil
	case OperationResultCodeOpNotSupported:
		// Void
		return nil
	case OperationResultCodeOpTooManySubentries:
		// Void
		return nil
	case OperationResultCodeOpExceededWorkLimit:
		// Void
		return nil
	case OperationResultCodeOpTooManySponsoring:
		// Void
		return nil
	}
	return fmt.Errorf("Code (OperationResultCode) switch value '%d' is not valid for union OperationResult", u.Code)
}

var _ decoderFrom = (*OperationResult)(nil)

// DecodeFrom decodes this value using the Decoder.
func (u *OperationResult) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding OperationResult: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	var err error
	var n, nTmp int
	nTmp, err = u.Code.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding OperationResultCode: %w", err)
	}
	switch OperationResultCode(u.Code) {
	case OperationResultCodeOpInner:
		u.Tr = new(OperationResultTr)
		nTmp, err = (*u.Tr).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding OperationResultTr: %w", err)
		}
		return n, nil
	case OperationResultCodeOpBadAuth:
		// Void
		return n, nil
	case OperationResultCodeOpNoAccount:
		// Void
		return n, nil
	case OperationResultCodeOpNotSupported:
		// Void
		return n, nil
	case OperationResultCodeOpTooManySubentries:
		// Void
		return n, nil
	case OperationResultCodeOpExceededWorkLimit:
		// Void
		return n, nil
	case OperationResultCodeOpTooManySponsoring:
		// Void
		return n, nil
	}
	return n, fmt.Errorf("union OperationResult has invalid Code (OperationResultCode) switch value '%d'", u.Code)
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s OperationResult) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *OperationResult) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*OperationResult)(nil)
	_ encoding.BinaryUnmarshaler = (*OperationResult)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s OperationResult) xdrType() {}

var _ xdrType = (*OperationResult)(nil)

// OperationBody is an XDR NestedUnion defines as:
//
//	union switch (OperationType type)
//	     {
//	     case CREATE_ACCOUNT:
//	         CreateAccountOp createAccountOp;
//	     case PAYMENT:
//	         PaymentOp paymentOp;
//	     case PATH_PAYMENT_STRICT_RECEIVE:
//	         PathPaymentStrictReceiveOp pathPaymentStrictReceiveOp;
//	     case MANAGE_SELL_OFFER:
//	         ManageSellOfferOp manageSellOfferOp;
//	     case CREATE_PASSIVE_SELL_OFFER:
//	         CreatePassiveSellOfferOp createPassiveSellOfferOp;
//	     case SET_OPTIONS:
//	         SetOptionsOp setOptionsOp;
//	     case CHANGE_TRUST:
//	         ChangeTrustOp changeTrustOp;
//	     case ALLOW_TRUST:
//	         AllowTrustOp allowTrustOp;
//	     case ACCOUNT_MERGE:
//	         MuxedAccount destination;
//	     case INFLATION:
//	         void;
//	     case MANAGE_DATA:
//	         ManageDataOp manageDataOp;
//	     case BUMP_SEQUENCE:
//	         BumpSequenceOp bumpSequenceOp;
//	     case MANAGE_BUY_OFFER:
//	         ManageBuyOfferOp manageBuyOfferOp;
//	     case PATH_PAYMENT_STRICT_SEND:
//	         PathPaymentStrictSendOp pathPaymentStrictSendOp;
//	     case CREATE_CLAIMABLE_BALANCE:
//	         CreateClaimableBalanceOp createClaimableBalanceOp;
//	     case CLAIM_CLAIMABLE_BALANCE:
//	         ClaimClaimableBalanceOp claimClaimableBalanceOp;
//	     case BEGIN_SPONSORING_FUTURE_RESERVES:
//	         BeginSponsoringFutureReservesOp beginSponsoringFutureReservesOp;
//	     case END_SPONSORING_FUTURE_RESERVES:
//	         void;
//	     case REVOKE_SPONSORSHIP:
//	         RevokeSponsorshipOp revokeSponsorshipOp;
//	     case CLAWBACK:
//	         ClawbackOp clawbackOp;
//	     case CLAWBACK_CLAIMABLE_BALANCE:
//	         ClawbackClaimableBalanceOp clawbackClaimableBalanceOp;
//	     case SET_TRUST_LINE_FLAGS:
//	         SetTrustLineFlagsOp setTrustLineFlagsOp;
//	     case LIQUIDITY_POOL_DEPOSIT:
//	         LiquidityPoolDepositOp liquidityPoolDepositOp;
//	     case LIQUIDITY_POOL_WITHDRAW:
//	         LiquidityPoolWithdrawOp liquidityPoolWithdrawOp;
//	     case INVOKE_HOST_FUNCTION:
//	         InvokeHostFunctionOp invokeHostFunctionOp;
//	     case EXTEND_FOOTPRINT_TTL:
//	         ExtendFootprintTTLOp extendFootprintTTLOp;
//	     case RESTORE_FOOTPRINT:
//	         RestoreFootprintOp restoreFootprintOp;
//
//	     case HELLO_WORLD:
//	         HelloWorldOp helloWorldOp;
//
//	     }
type OperationBody struct {
	Type                            OperationType
	CreateAccountOp                 *CreateAccountOp
	PaymentOp                       *PaymentOp
	PathPaymentStrictReceiveOp      *PathPaymentStrictReceiveOp
	ManageSellOfferOp               *ManageSellOfferOp
	CreatePassiveSellOfferOp        *CreatePassiveSellOfferOp
	SetOptionsOp                    *SetOptionsOp
	ChangeTrustOp                   *ChangeTrustOp
	AllowTrustOp                    *AllowTrustOp
	Destination                     *MuxedAccount
	ManageDataOp                    *ManageDataOp
	BumpSequenceOp                  *BumpSequenceOp
	ManageBuyOfferOp                *ManageBuyOfferOp
	PathPaymentStrictSendOp         *PathPaymentStrictSendOp
	CreateClaimableBalanceOp        *CreateClaimableBalanceOp
	ClaimClaimableBalanceOp         *ClaimClaimableBalanceOp
	BeginSponsoringFutureReservesOp *BeginSponsoringFutureReservesOp
	RevokeSponsorshipOp             *RevokeSponsorshipOp
	ClawbackOp                      *ClawbackOp
	ClawbackClaimableBalanceOp      *ClawbackClaimableBalanceOp
	SetTrustLineFlagsOp             *SetTrustLineFlagsOp
	LiquidityPoolDepositOp          *LiquidityPoolDepositOp
	LiquidityPoolWithdrawOp         *LiquidityPoolWithdrawOp
	InvokeHostFunctionOp            *InvokeHostFunctionOp
	ExtendFootprintTtlOp            *ExtendFootprintTtlOp
	RestoreFootprintOp              *RestoreFootprintOp
	HelloWorldOp                    *HelloWorldOp
}

// SwitchFieldName returns the field name in which this union's
// discriminant is stored
func (u OperationBody) SwitchFieldName() string {
	return "Type"
}

// ArmForSwitch returns which field name should be used for storing
// the value for an instance of OperationBody
func (u OperationBody) ArmForSwitch(sw int32) (string, bool) {
	switch OperationType(sw) {
	case OperationTypeCreateAccount:
		return "CreateAccountOp", true
	case OperationTypePayment:
		return "PaymentOp", true
	case OperationTypePathPaymentStrictReceive:
		return "PathPaymentStrictReceiveOp", true
	case OperationTypeManageSellOffer:
		return "ManageSellOfferOp", true
	case OperationTypeCreatePassiveSellOffer:
		return "CreatePassiveSellOfferOp", true
	case OperationTypeSetOptions:
		return "SetOptionsOp", true
	case OperationTypeChangeTrust:
		return "ChangeTrustOp", true
	case OperationTypeAllowTrust:
		return "AllowTrustOp", true
	case OperationTypeAccountMerge:
		return "Destination", true
	case OperationTypeInflation:
		return "", true
	case OperationTypeManageData:
		return "ManageDataOp", true
	case OperationTypeBumpSequence:
		return "BumpSequenceOp", true
	case OperationTypeManageBuyOffer:
		return "ManageBuyOfferOp", true
	case OperationTypePathPaymentStrictSend:
		return "PathPaymentStrictSendOp", true
	case OperationTypeCreateClaimableBalance:
		return "CreateClaimableBalanceOp", true
	case OperationTypeClaimClaimableBalance:
		return "ClaimClaimableBalanceOp", true
	case OperationTypeBeginSponsoringFutureReserves:
		return "BeginSponsoringFutureReservesOp", true
	case OperationTypeEndSponsoringFutureReserves:
		return "", true
	case OperationTypeRevokeSponsorship:
		return "RevokeSponsorshipOp", true
	case OperationTypeClawback:
		return "ClawbackOp", true
	case OperationTypeClawbackClaimableBalance:
		return "ClawbackClaimableBalanceOp", true
	case OperationTypeSetTrustLineFlags:
		return "SetTrustLineFlagsOp", true
	case OperationTypeLiquidityPoolDeposit:
		return "LiquidityPoolDepositOp", true
	case OperationTypeLiquidityPoolWithdraw:
		return "LiquidityPoolWithdrawOp", true
	case OperationTypeInvokeHostFunction:
		return "InvokeHostFunctionOp", true
	case OperationTypeExtendFootprintTtl:
		return "ExtendFootprintTtlOp", true
	case OperationTypeRestoreFootprint:
		return "RestoreFootprintOp", true
	case OperationTypeHelloWorld:
		return "HelloWorldOp", true
	}
	return "-", false
}

// NewOperationBody creates a new  OperationBody.
func NewOperationBody(aType OperationType, value interface{}) (result OperationBody, err error) {
	result.Type = aType
	switch OperationType(aType) {
	case OperationTypeCreateAccount:
		tv, ok := value.(CreateAccountOp)
		if !ok {
			err = errors.New("invalid value, must be CreateAccountOp")
			return
		}
		result.CreateAccountOp = &tv
	case OperationTypePayment:
		tv, ok := value.(PaymentOp)
		if !ok {
			err = errors.New("invalid value, must be PaymentOp")
			return
		}
		result.PaymentOp = &tv
	case OperationTypePathPaymentStrictReceive:
		tv, ok := value.(PathPaymentStrictReceiveOp)
		if !ok {
			err = errors.New("invalid value, must be PathPaymentStrictReceiveOp")
			return
		}
		result.PathPaymentStrictReceiveOp = &tv
	case OperationTypeManageSellOffer:
		tv, ok := value.(ManageSellOfferOp)
		if !ok {
			err = errors.New("invalid value, must be ManageSellOfferOp")
			return
		}
		result.ManageSellOfferOp = &tv
	case OperationTypeCreatePassiveSellOffer:
		tv, ok := value.(CreatePassiveSellOfferOp)
		if !ok {
			err = errors.New("invalid value, must be CreatePassiveSellOfferOp")
			return
		}
		result.CreatePassiveSellOfferOp = &tv
	case OperationTypeSetOptions:
		tv, ok := value.(SetOptionsOp)
		if !ok {
			err = errors.New("invalid value, must be SetOptionsOp")
			return
		}
		result.SetOptionsOp = &tv
	case OperationTypeChangeTrust:
		tv, ok := value.(ChangeTrustOp)
		if !ok {
			err = errors.New("invalid value, must be ChangeTrustOp")
			return
		}
		result.ChangeTrustOp = &tv
	case OperationTypeAllowTrust:
		tv, ok := value.(AllowTrustOp)
		if !ok {
			err = errors.New("invalid value, must be AllowTrustOp")
			return
		}
		result.AllowTrustOp = &tv
	case OperationTypeAccountMerge:
		tv, ok := value.(MuxedAccount)
		if !ok {
			err = errors.New("invalid value, must be MuxedAccount")
			return
		}
		result.Destination = &tv
	case OperationTypeInflation:
		// void
	case OperationTypeManageData:
		tv, ok := value.(ManageDataOp)
		if !ok {
			err = errors.New("invalid value, must be ManageDataOp")
			return
		}
		result.ManageDataOp = &tv
	case OperationTypeBumpSequence:
		tv, ok := value.(BumpSequenceOp)
		if !ok {
			err = errors.New("invalid value, must be BumpSequenceOp")
			return
		}
		result.BumpSequenceOp = &tv
	case OperationTypeManageBuyOffer:
		tv, ok := value.(ManageBuyOfferOp)
		if !ok {
			err = errors.New("invalid value, must be ManageBuyOfferOp")
			return
		}
		result.ManageBuyOfferOp = &tv
	case OperationTypePathPaymentStrictSend:
		tv, ok := value.(PathPaymentStrictSendOp)
		if !ok {
			err = errors.New("invalid value, must be PathPaymentStrictSendOp")
			return
		}
		result.PathPaymentStrictSendOp = &tv
	case OperationTypeCreateClaimableBalance:
		tv, ok := value.(CreateClaimableBalanceOp)
		if !ok {
			err = errors.New("invalid value, must be CreateClaimableBalanceOp")
			return
		}
		result.CreateClaimableBalanceOp = &tv
	case OperationTypeClaimClaimableBalance:
		tv, ok := value.(ClaimClaimableBalanceOp)
		if !ok {
			err = errors.New("invalid value, must be ClaimClaimableBalanceOp")
			return
		}
		result.ClaimClaimableBalanceOp = &tv
	case OperationTypeBeginSponsoringFutureReserves:
		tv, ok := value.(BeginSponsoringFutureReservesOp)
		if !ok {
			err = errors.New("invalid value, must be BeginSponsoringFutureReservesOp")
			return
		}
		result.BeginSponsoringFutureReservesOp = &tv
	case OperationTypeEndSponsoringFutureReserves:
		// void
	case OperationTypeRevokeSponsorship:
		tv, ok := value.(RevokeSponsorshipOp)
		if !ok {
			err = errors.New("invalid value, must be RevokeSponsorshipOp")
			return
		}
		result.RevokeSponsorshipOp = &tv
	case OperationTypeClawback:
		tv, ok := value.(ClawbackOp)
		if !ok {
			err = errors.New("invalid value, must be ClawbackOp")
			return
		}
		result.ClawbackOp = &tv
	case OperationTypeClawbackClaimableBalance:
		tv, ok := value.(ClawbackClaimableBalanceOp)
		if !ok {
			err = errors.New("invalid value, must be ClawbackClaimableBalanceOp")
			return
		}
		result.ClawbackClaimableBalanceOp = &tv
	case OperationTypeSetTrustLineFlags:
		tv, ok := value.(SetTrustLineFlagsOp)
		if !ok {
			err = errors.New("invalid value, must be SetTrustLineFlagsOp")
			return
		}
		result.SetTrustLineFlagsOp = &tv
	case OperationTypeLiquidityPoolDeposit:
		tv, ok := value.(LiquidityPoolDepositOp)
		if !ok {
			err = errors.New("invalid value, must be LiquidityPoolDepositOp")
			return
		}
		result.LiquidityPoolDepositOp = &tv
	case OperationTypeLiquidityPoolWithdraw:
		tv, ok := value.(LiquidityPoolWithdrawOp)
		if !ok {
			err = errors.New("invalid value, must be LiquidityPoolWithdrawOp")
			return
		}
		result.LiquidityPoolWithdrawOp = &tv
	case OperationTypeInvokeHostFunction:
		tv, ok := value.(InvokeHostFunctionOp)
		if !ok {
			err = errors.New("invalid value, must be InvokeHostFunctionOp")
			return
		}
		result.InvokeHostFunctionOp = &tv
	case OperationTypeExtendFootprintTtl:
		tv, ok := value.(ExtendFootprintTtlOp)
		if !ok {
			err = errors.New("invalid value, must be ExtendFootprintTtlOp")
			return
		}
		result.ExtendFootprintTtlOp = &tv
	case OperationTypeRestoreFootprint:
		tv, ok := value.(RestoreFootprintOp)
		if !ok {
			err = errors.New("invalid value, must be RestoreFootprintOp")
			return
		}
		result.RestoreFootprintOp = &tv
	case OperationTypeHelloWorld:
		tv, ok := value.(HelloWorldOp)
		if !ok {
			err = errors.New("invalid value, must be HelloWorldOp")
			return
		}
		result.HelloWorldOp = &tv
	}
	return
}

// MustCreateAccountOp retrieves the CreateAccountOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustCreateAccountOp() CreateAccountOp {
	val, ok := u.GetCreateAccountOp()

	if !ok {
		panic("arm CreateAccountOp is not set")
	}

	return val
}

// GetCreateAccountOp retrieves the CreateAccountOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetCreateAccountOp() (result CreateAccountOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "CreateAccountOp" {
		result = *u.CreateAccountOp
		ok = true
	}

	return
}

// MustPaymentOp retrieves the PaymentOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustPaymentOp() PaymentOp {
	val, ok := u.GetPaymentOp()

	if !ok {
		panic("arm PaymentOp is not set")
	}

	return val
}

// GetPaymentOp retrieves the PaymentOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetPaymentOp() (result PaymentOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "PaymentOp" {
		result = *u.PaymentOp
		ok = true
	}

	return
}

// MustPathPaymentStrictReceiveOp retrieves the PathPaymentStrictReceiveOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustPathPaymentStrictReceiveOp() PathPaymentStrictReceiveOp {
	val, ok := u.GetPathPaymentStrictReceiveOp()

	if !ok {
		panic("arm PathPaymentStrictReceiveOp is not set")
	}

	return val
}

// GetPathPaymentStrictReceiveOp retrieves the PathPaymentStrictReceiveOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetPathPaymentStrictReceiveOp() (result PathPaymentStrictReceiveOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "PathPaymentStrictReceiveOp" {
		result = *u.PathPaymentStrictReceiveOp
		ok = true
	}

	return
}

// MustManageSellOfferOp retrieves the ManageSellOfferOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustManageSellOfferOp() ManageSellOfferOp {
	val, ok := u.GetManageSellOfferOp()

	if !ok {
		panic("arm ManageSellOfferOp is not set")
	}

	return val
}

// GetManageSellOfferOp retrieves the ManageSellOfferOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetManageSellOfferOp() (result ManageSellOfferOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ManageSellOfferOp" {
		result = *u.ManageSellOfferOp
		ok = true
	}

	return
}

// MustCreatePassiveSellOfferOp retrieves the CreatePassiveSellOfferOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustCreatePassiveSellOfferOp() CreatePassiveSellOfferOp {
	val, ok := u.GetCreatePassiveSellOfferOp()

	if !ok {
		panic("arm CreatePassiveSellOfferOp is not set")
	}

	return val
}

// GetCreatePassiveSellOfferOp retrieves the CreatePassiveSellOfferOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetCreatePassiveSellOfferOp() (result CreatePassiveSellOfferOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "CreatePassiveSellOfferOp" {
		result = *u.CreatePassiveSellOfferOp
		ok = true
	}

	return
}

// MustSetOptionsOp retrieves the SetOptionsOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustSetOptionsOp() SetOptionsOp {
	val, ok := u.GetSetOptionsOp()

	if !ok {
		panic("arm SetOptionsOp is not set")
	}

	return val
}

// GetSetOptionsOp retrieves the SetOptionsOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetSetOptionsOp() (result SetOptionsOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "SetOptionsOp" {
		result = *u.SetOptionsOp
		ok = true
	}

	return
}

// MustChangeTrustOp retrieves the ChangeTrustOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustChangeTrustOp() ChangeTrustOp {
	val, ok := u.GetChangeTrustOp()

	if !ok {
		panic("arm ChangeTrustOp is not set")
	}

	return val
}

// GetChangeTrustOp retrieves the ChangeTrustOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetChangeTrustOp() (result ChangeTrustOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ChangeTrustOp" {
		result = *u.ChangeTrustOp
		ok = true
	}

	return
}

// MustAllowTrustOp retrieves the AllowTrustOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustAllowTrustOp() AllowTrustOp {
	val, ok := u.GetAllowTrustOp()

	if !ok {
		panic("arm AllowTrustOp is not set")
	}

	return val
}

// GetAllowTrustOp retrieves the AllowTrustOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetAllowTrustOp() (result AllowTrustOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "AllowTrustOp" {
		result = *u.AllowTrustOp
		ok = true
	}

	return
}

// MustDestination retrieves the Destination value from the union,
// panicing if the value is not set.
func (u OperationBody) MustDestination() MuxedAccount {
	val, ok := u.GetDestination()

	if !ok {
		panic("arm Destination is not set")
	}

	return val
}

// GetDestination retrieves the Destination value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetDestination() (result MuxedAccount, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "Destination" {
		result = *u.Destination
		ok = true
	}

	return
}

// MustManageDataOp retrieves the ManageDataOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustManageDataOp() ManageDataOp {
	val, ok := u.GetManageDataOp()

	if !ok {
		panic("arm ManageDataOp is not set")
	}

	return val
}

// GetManageDataOp retrieves the ManageDataOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetManageDataOp() (result ManageDataOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ManageDataOp" {
		result = *u.ManageDataOp
		ok = true
	}

	return
}

// MustBumpSequenceOp retrieves the BumpSequenceOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustBumpSequenceOp() BumpSequenceOp {
	val, ok := u.GetBumpSequenceOp()

	if !ok {
		panic("arm BumpSequenceOp is not set")
	}

	return val
}

// GetBumpSequenceOp retrieves the BumpSequenceOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetBumpSequenceOp() (result BumpSequenceOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "BumpSequenceOp" {
		result = *u.BumpSequenceOp
		ok = true
	}

	return
}

// MustManageBuyOfferOp retrieves the ManageBuyOfferOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustManageBuyOfferOp() ManageBuyOfferOp {
	val, ok := u.GetManageBuyOfferOp()

	if !ok {
		panic("arm ManageBuyOfferOp is not set")
	}

	return val
}

// GetManageBuyOfferOp retrieves the ManageBuyOfferOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetManageBuyOfferOp() (result ManageBuyOfferOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ManageBuyOfferOp" {
		result = *u.ManageBuyOfferOp
		ok = true
	}

	return
}

// MustPathPaymentStrictSendOp retrieves the PathPaymentStrictSendOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustPathPaymentStrictSendOp() PathPaymentStrictSendOp {
	val, ok := u.GetPathPaymentStrictSendOp()

	if !ok {
		panic("arm PathPaymentStrictSendOp is not set")
	}

	return val
}

// GetPathPaymentStrictSendOp retrieves the PathPaymentStrictSendOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetPathPaymentStrictSendOp() (result PathPaymentStrictSendOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "PathPaymentStrictSendOp" {
		result = *u.PathPaymentStrictSendOp
		ok = true
	}

	return
}

// MustCreateClaimableBalanceOp retrieves the CreateClaimableBalanceOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustCreateClaimableBalanceOp() CreateClaimableBalanceOp {
	val, ok := u.GetCreateClaimableBalanceOp()

	if !ok {
		panic("arm CreateClaimableBalanceOp is not set")
	}

	return val
}

// GetCreateClaimableBalanceOp retrieves the CreateClaimableBalanceOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetCreateClaimableBalanceOp() (result CreateClaimableBalanceOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "CreateClaimableBalanceOp" {
		result = *u.CreateClaimableBalanceOp
		ok = true
	}

	return
}

// MustClaimClaimableBalanceOp retrieves the ClaimClaimableBalanceOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustClaimClaimableBalanceOp() ClaimClaimableBalanceOp {
	val, ok := u.GetClaimClaimableBalanceOp()

	if !ok {
		panic("arm ClaimClaimableBalanceOp is not set")
	}

	return val
}

// GetClaimClaimableBalanceOp retrieves the ClaimClaimableBalanceOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetClaimClaimableBalanceOp() (result ClaimClaimableBalanceOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ClaimClaimableBalanceOp" {
		result = *u.ClaimClaimableBalanceOp
		ok = true
	}

	return
}

// MustBeginSponsoringFutureReservesOp retrieves the BeginSponsoringFutureReservesOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustBeginSponsoringFutureReservesOp() BeginSponsoringFutureReservesOp {
	val, ok := u.GetBeginSponsoringFutureReservesOp()

	if !ok {
		panic("arm BeginSponsoringFutureReservesOp is not set")
	}

	return val
}

// GetBeginSponsoringFutureReservesOp retrieves the BeginSponsoringFutureReservesOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetBeginSponsoringFutureReservesOp() (result BeginSponsoringFutureReservesOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "BeginSponsoringFutureReservesOp" {
		result = *u.BeginSponsoringFutureReservesOp
		ok = true
	}

	return
}

// MustRevokeSponsorshipOp retrieves the RevokeSponsorshipOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustRevokeSponsorshipOp() RevokeSponsorshipOp {
	val, ok := u.GetRevokeSponsorshipOp()

	if !ok {
		panic("arm RevokeSponsorshipOp is not set")
	}

	return val
}

// GetRevokeSponsorshipOp retrieves the RevokeSponsorshipOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetRevokeSponsorshipOp() (result RevokeSponsorshipOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "RevokeSponsorshipOp" {
		result = *u.RevokeSponsorshipOp
		ok = true
	}

	return
}

// MustClawbackOp retrieves the ClawbackOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustClawbackOp() ClawbackOp {
	val, ok := u.GetClawbackOp()

	if !ok {
		panic("arm ClawbackOp is not set")
	}

	return val
}

// GetClawbackOp retrieves the ClawbackOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetClawbackOp() (result ClawbackOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ClawbackOp" {
		result = *u.ClawbackOp
		ok = true
	}

	return
}

// MustClawbackClaimableBalanceOp retrieves the ClawbackClaimableBalanceOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustClawbackClaimableBalanceOp() ClawbackClaimableBalanceOp {
	val, ok := u.GetClawbackClaimableBalanceOp()

	if !ok {
		panic("arm ClawbackClaimableBalanceOp is not set")
	}

	return val
}

// GetClawbackClaimableBalanceOp retrieves the ClawbackClaimableBalanceOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetClawbackClaimableBalanceOp() (result ClawbackClaimableBalanceOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ClawbackClaimableBalanceOp" {
		result = *u.ClawbackClaimableBalanceOp
		ok = true
	}

	return
}

// MustSetTrustLineFlagsOp retrieves the SetTrustLineFlagsOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustSetTrustLineFlagsOp() SetTrustLineFlagsOp {
	val, ok := u.GetSetTrustLineFlagsOp()

	if !ok {
		panic("arm SetTrustLineFlagsOp is not set")
	}

	return val
}

// GetSetTrustLineFlagsOp retrieves the SetTrustLineFlagsOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetSetTrustLineFlagsOp() (result SetTrustLineFlagsOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "SetTrustLineFlagsOp" {
		result = *u.SetTrustLineFlagsOp
		ok = true
	}

	return
}

// MustLiquidityPoolDepositOp retrieves the LiquidityPoolDepositOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustLiquidityPoolDepositOp() LiquidityPoolDepositOp {
	val, ok := u.GetLiquidityPoolDepositOp()

	if !ok {
		panic("arm LiquidityPoolDepositOp is not set")
	}

	return val
}

// GetLiquidityPoolDepositOp retrieves the LiquidityPoolDepositOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetLiquidityPoolDepositOp() (result LiquidityPoolDepositOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "LiquidityPoolDepositOp" {
		result = *u.LiquidityPoolDepositOp
		ok = true
	}

	return
}

// MustLiquidityPoolWithdrawOp retrieves the LiquidityPoolWithdrawOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustLiquidityPoolWithdrawOp() LiquidityPoolWithdrawOp {
	val, ok := u.GetLiquidityPoolWithdrawOp()

	if !ok {
		panic("arm LiquidityPoolWithdrawOp is not set")
	}

	return val
}

// GetLiquidityPoolWithdrawOp retrieves the LiquidityPoolWithdrawOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetLiquidityPoolWithdrawOp() (result LiquidityPoolWithdrawOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "LiquidityPoolWithdrawOp" {
		result = *u.LiquidityPoolWithdrawOp
		ok = true
	}

	return
}

// MustInvokeHostFunctionOp retrieves the InvokeHostFunctionOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustInvokeHostFunctionOp() InvokeHostFunctionOp {
	val, ok := u.GetInvokeHostFunctionOp()

	if !ok {
		panic("arm InvokeHostFunctionOp is not set")
	}

	return val
}

// GetInvokeHostFunctionOp retrieves the InvokeHostFunctionOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetInvokeHostFunctionOp() (result InvokeHostFunctionOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "InvokeHostFunctionOp" {
		result = *u.InvokeHostFunctionOp
		ok = true
	}

	return
}

// MustExtendFootprintTtlOp retrieves the ExtendFootprintTtlOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustExtendFootprintTtlOp() ExtendFootprintTtlOp {
	val, ok := u.GetExtendFootprintTtlOp()

	if !ok {
		panic("arm ExtendFootprintTtlOp is not set")
	}

	return val
}

// GetExtendFootprintTtlOp retrieves the ExtendFootprintTtlOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetExtendFootprintTtlOp() (result ExtendFootprintTtlOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ExtendFootprintTtlOp" {
		result = *u.ExtendFootprintTtlOp
		ok = true
	}

	return
}

// MustRestoreFootprintOp retrieves the RestoreFootprintOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustRestoreFootprintOp() RestoreFootprintOp {
	val, ok := u.GetRestoreFootprintOp()

	if !ok {
		panic("arm RestoreFootprintOp is not set")
	}

	return val
}

// GetRestoreFootprintOp retrieves the RestoreFootprintOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetRestoreFootprintOp() (result RestoreFootprintOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "RestoreFootprintOp" {
		result = *u.RestoreFootprintOp
		ok = true
	}

	return
}

// MustHelloWorldOp retrieves the HelloWorldOp value from the union,
// panicing if the value is not set.
func (u OperationBody) MustHelloWorldOp() HelloWorldOp {
	val, ok := u.GetHelloWorldOp()

	if !ok {
		panic("arm HelloWorldOp is not set")
	}

	return val
}

// GetHelloWorldOp retrieves the HelloWorldOp value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationBody) GetHelloWorldOp() (result HelloWorldOp, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "HelloWorldOp" {
		result = *u.HelloWorldOp
		ok = true
	}

	return
}

// EncodeTo encodes this value using the Encoder.
func (u OperationBody) EncodeTo(e *xdr.Encoder) error {
	var err error
	if err = u.Type.EncodeTo(e); err != nil {
		return err
	}
	switch OperationType(u.Type) {
	case OperationTypeCreateAccount:
		if err = (*u.CreateAccountOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypePayment:
		if err = (*u.PaymentOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypePathPaymentStrictReceive:
		if err = (*u.PathPaymentStrictReceiveOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeManageSellOffer:
		if err = (*u.ManageSellOfferOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeCreatePassiveSellOffer:
		if err = (*u.CreatePassiveSellOfferOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeSetOptions:
		if err = (*u.SetOptionsOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeChangeTrust:
		if err = (*u.ChangeTrustOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeAllowTrust:
		if err = (*u.AllowTrustOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeAccountMerge:
		if err = (*u.Destination).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeInflation:
		// Void
		return nil
	case OperationTypeManageData:
		if err = (*u.ManageDataOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeBumpSequence:
		if err = (*u.BumpSequenceOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeManageBuyOffer:
		if err = (*u.ManageBuyOfferOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypePathPaymentStrictSend:
		if err = (*u.PathPaymentStrictSendOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeCreateClaimableBalance:
		if err = (*u.CreateClaimableBalanceOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeClaimClaimableBalance:
		if err = (*u.ClaimClaimableBalanceOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeBeginSponsoringFutureReserves:
		if err = (*u.BeginSponsoringFutureReservesOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeEndSponsoringFutureReserves:
		// Void
		return nil
	case OperationTypeRevokeSponsorship:
		if err = (*u.RevokeSponsorshipOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeClawback:
		if err = (*u.ClawbackOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeClawbackClaimableBalance:
		if err = (*u.ClawbackClaimableBalanceOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeSetTrustLineFlags:
		if err = (*u.SetTrustLineFlagsOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeLiquidityPoolDeposit:
		if err = (*u.LiquidityPoolDepositOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeLiquidityPoolWithdraw:
		if err = (*u.LiquidityPoolWithdrawOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeInvokeHostFunction:
		if err = (*u.InvokeHostFunctionOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeExtendFootprintTtl:
		if err = (*u.ExtendFootprintTtlOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeRestoreFootprint:
		if err = (*u.RestoreFootprintOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeHelloWorld:
		if err = (*u.HelloWorldOp).EncodeTo(e); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("Type (OperationType) switch value '%d' is not valid for union OperationBody", u.Type)
}

var _ decoderFrom = (*OperationBody)(nil)

// DecodeFrom decodes this value using the Decoder.
func (u *OperationBody) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding OperationBody: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	var err error
	var n, nTmp int
	nTmp, err = u.Type.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding OperationType: %w", err)
	}
	switch OperationType(u.Type) {
	case OperationTypeCreateAccount:
		u.CreateAccountOp = new(CreateAccountOp)
		nTmp, err = (*u.CreateAccountOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding CreateAccountOp: %w", err)
		}
		return n, nil
	case OperationTypePayment:
		u.PaymentOp = new(PaymentOp)
		nTmp, err = (*u.PaymentOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding PaymentOp: %w", err)
		}
		return n, nil
	case OperationTypePathPaymentStrictReceive:
		u.PathPaymentStrictReceiveOp = new(PathPaymentStrictReceiveOp)
		nTmp, err = (*u.PathPaymentStrictReceiveOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding PathPaymentStrictReceiveOp: %w", err)
		}
		return n, nil
	case OperationTypeManageSellOffer:
		u.ManageSellOfferOp = new(ManageSellOfferOp)
		nTmp, err = (*u.ManageSellOfferOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ManageSellOfferOp: %w", err)
		}
		return n, nil
	case OperationTypeCreatePassiveSellOffer:
		u.CreatePassiveSellOfferOp = new(CreatePassiveSellOfferOp)
		nTmp, err = (*u.CreatePassiveSellOfferOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding CreatePassiveSellOfferOp: %w", err)
		}
		return n, nil
	case OperationTypeSetOptions:
		u.SetOptionsOp = new(SetOptionsOp)
		nTmp, err = (*u.SetOptionsOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding SetOptionsOp: %w", err)
		}
		return n, nil
	case OperationTypeChangeTrust:
		u.ChangeTrustOp = new(ChangeTrustOp)
		nTmp, err = (*u.ChangeTrustOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ChangeTrustOp: %w", err)
		}
		return n, nil
	case OperationTypeAllowTrust:
		u.AllowTrustOp = new(AllowTrustOp)
		nTmp, err = (*u.AllowTrustOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding AllowTrustOp: %w", err)
		}
		return n, nil
	case OperationTypeAccountMerge:
		u.Destination = new(MuxedAccount)
		nTmp, err = (*u.Destination).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding MuxedAccount: %w", err)
		}
		return n, nil
	case OperationTypeInflation:
		// Void
		return n, nil
	case OperationTypeManageData:
		u.ManageDataOp = new(ManageDataOp)
		nTmp, err = (*u.ManageDataOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ManageDataOp: %w", err)
		}
		return n, nil
	case OperationTypeBumpSequence:
		u.BumpSequenceOp = new(BumpSequenceOp)
		nTmp, err = (*u.BumpSequenceOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding BumpSequenceOp: %w", err)
		}
		return n, nil
	case OperationTypeManageBuyOffer:
		u.ManageBuyOfferOp = new(ManageBuyOfferOp)
		nTmp, err = (*u.ManageBuyOfferOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ManageBuyOfferOp: %w", err)
		}
		return n, nil
	case OperationTypePathPaymentStrictSend:
		u.PathPaymentStrictSendOp = new(PathPaymentStrictSendOp)
		nTmp, err = (*u.PathPaymentStrictSendOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding PathPaymentStrictSendOp: %w", err)
		}
		return n, nil
	case OperationTypeCreateClaimableBalance:
		u.CreateClaimableBalanceOp = new(CreateClaimableBalanceOp)
		nTmp, err = (*u.CreateClaimableBalanceOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding CreateClaimableBalanceOp: %w", err)
		}
		return n, nil
	case OperationTypeClaimClaimableBalance:
		u.ClaimClaimableBalanceOp = new(ClaimClaimableBalanceOp)
		nTmp, err = (*u.ClaimClaimableBalanceOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ClaimClaimableBalanceOp: %w", err)
		}
		return n, nil
	case OperationTypeBeginSponsoringFutureReserves:
		u.BeginSponsoringFutureReservesOp = new(BeginSponsoringFutureReservesOp)
		nTmp, err = (*u.BeginSponsoringFutureReservesOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding BeginSponsoringFutureReservesOp: %w", err)
		}
		return n, nil
	case OperationTypeEndSponsoringFutureReserves:
		// Void
		return n, nil
	case OperationTypeRevokeSponsorship:
		u.RevokeSponsorshipOp = new(RevokeSponsorshipOp)
		nTmp, err = (*u.RevokeSponsorshipOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding RevokeSponsorshipOp: %w", err)
		}
		return n, nil
	case OperationTypeClawback:
		u.ClawbackOp = new(ClawbackOp)
		nTmp, err = (*u.ClawbackOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ClawbackOp: %w", err)
		}
		return n, nil
	case OperationTypeClawbackClaimableBalance:
		u.ClawbackClaimableBalanceOp = new(ClawbackClaimableBalanceOp)
		nTmp, err = (*u.ClawbackClaimableBalanceOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ClawbackClaimableBalanceOp: %w", err)
		}
		return n, nil
	case OperationTypeSetTrustLineFlags:
		u.SetTrustLineFlagsOp = new(SetTrustLineFlagsOp)
		nTmp, err = (*u.SetTrustLineFlagsOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding SetTrustLineFlagsOp: %w", err)
		}
		return n, nil
	case OperationTypeLiquidityPoolDeposit:
		u.LiquidityPoolDepositOp = new(LiquidityPoolDepositOp)
		nTmp, err = (*u.LiquidityPoolDepositOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding LiquidityPoolDepositOp: %w", err)
		}
		return n, nil
	case OperationTypeLiquidityPoolWithdraw:
		u.LiquidityPoolWithdrawOp = new(LiquidityPoolWithdrawOp)
		nTmp, err = (*u.LiquidityPoolWithdrawOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding LiquidityPoolWithdrawOp: %w", err)
		}
		return n, nil
	case OperationTypeInvokeHostFunction:
		u.InvokeHostFunctionOp = new(InvokeHostFunctionOp)
		nTmp, err = (*u.InvokeHostFunctionOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding InvokeHostFunctionOp: %w", err)
		}
		return n, nil
	case OperationTypeExtendFootprintTtl:
		u.ExtendFootprintTtlOp = new(ExtendFootprintTtlOp)
		nTmp, err = (*u.ExtendFootprintTtlOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ExtendFootprintTtlOp: %w", err)
		}
		return n, nil
	case OperationTypeRestoreFootprint:
		u.RestoreFootprintOp = new(RestoreFootprintOp)
		nTmp, err = (*u.RestoreFootprintOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding RestoreFootprintOp: %w", err)
		}
		return n, nil
	case OperationTypeHelloWorld:
		u.HelloWorldOp = new(HelloWorldOp)
		nTmp, err = (*u.HelloWorldOp).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding HelloWorldOp: %w", err)
		}
		return n, nil
	}
	return n, fmt.Errorf("union OperationBody has invalid Type (OperationType) switch value '%d'", u.Type)
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s OperationBody) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *OperationBody) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*OperationBody)(nil)
	_ encoding.BinaryUnmarshaler = (*OperationBody)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s OperationBody) xdrType() {}

var _ xdrType = (*OperationBody)(nil)

// OperationResultTr is an XDR NestedUnion defines as:
//
//	union switch (OperationType type)
//	     {
//	     case CREATE_ACCOUNT:
//	         CreateAccountResult createAccountResult;
//	     case PAYMENT:
//	         PaymentResult paymentResult;
//	     case PATH_PAYMENT_STRICT_RECEIVE:
//	         PathPaymentStrictReceiveResult pathPaymentStrictReceiveResult;
//	     case MANAGE_SELL_OFFER:
//	         ManageSellOfferResult manageSellOfferResult;
//	     case CREATE_PASSIVE_SELL_OFFER:
//	         ManageSellOfferResult createPassiveSellOfferResult;
//	     case SET_OPTIONS:
//	         SetOptionsResult setOptionsResult;
//	     case CHANGE_TRUST:
//	         ChangeTrustResult changeTrustResult;
//	     case ALLOW_TRUST:
//	         AllowTrustResult allowTrustResult;
//	     case ACCOUNT_MERGE:
//	         AccountMergeResult accountMergeResult;
//	     case INFLATION:
//	         InflationResult inflationResult;
//	     case MANAGE_DATA:
//	         ManageDataResult manageDataResult;
//	     case BUMP_SEQUENCE:
//	         BumpSequenceResult bumpSeqResult;
//	     case MANAGE_BUY_OFFER:
//	         ManageBuyOfferResult manageBuyOfferResult;
//	     case PATH_PAYMENT_STRICT_SEND:
//	         PathPaymentStrictSendResult pathPaymentStrictSendResult;
//	     case CREATE_CLAIMABLE_BALANCE:
//	         CreateClaimableBalanceResult createClaimableBalanceResult;
//	     case CLAIM_CLAIMABLE_BALANCE:
//	         ClaimClaimableBalanceResult claimClaimableBalanceResult;
//	     case BEGIN_SPONSORING_FUTURE_RESERVES:
//	         BeginSponsoringFutureReservesResult beginSponsoringFutureReservesResult;
//	     case END_SPONSORING_FUTURE_RESERVES:
//	         EndSponsoringFutureReservesResult endSponsoringFutureReservesResult;
//	     case REVOKE_SPONSORSHIP:
//	         RevokeSponsorshipResult revokeSponsorshipResult;
//	     case CLAWBACK:
//	         ClawbackResult clawbackResult;
//	     case CLAWBACK_CLAIMABLE_BALANCE:
//	         ClawbackClaimableBalanceResult clawbackClaimableBalanceResult;
//	     case SET_TRUST_LINE_FLAGS:
//	         SetTrustLineFlagsResult setTrustLineFlagsResult;
//	     case LIQUIDITY_POOL_DEPOSIT:
//	         LiquidityPoolDepositResult liquidityPoolDepositResult;
//	     case LIQUIDITY_POOL_WITHDRAW:
//	         LiquidityPoolWithdrawResult liquidityPoolWithdrawResult;
//	     case INVOKE_HOST_FUNCTION:
//	         InvokeHostFunctionResult invokeHostFunctionResult;
//	     case EXTEND_FOOTPRINT_TTL:
//	         ExtendFootprintTTLResult extendFootprintTTLResult;
//	     case RESTORE_FOOTPRINT:
//	         RestoreFootprintResult restoreFootprintResult;
//
//	     case HELLO_WORLD:
//	         HelloWorldResult helloWorldResult;
//
//	     }
type OperationResultTr struct {
	Type                                OperationType
	CreateAccountResult                 *CreateAccountResult
	PaymentResult                       *PaymentResult
	PathPaymentStrictReceiveResult      *PathPaymentStrictReceiveResult
	ManageSellOfferResult               *ManageSellOfferResult
	CreatePassiveSellOfferResult        *ManageSellOfferResult
	SetOptionsResult                    *SetOptionsResult
	ChangeTrustResult                   *ChangeTrustResult
	AllowTrustResult                    *AllowTrustResult
	AccountMergeResult                  *AccountMergeResult
	InflationResult                     *InflationResult
	ManageDataResult                    *ManageDataResult
	BumpSeqResult                       *BumpSequenceResult
	ManageBuyOfferResult                *ManageBuyOfferResult
	PathPaymentStrictSendResult         *PathPaymentStrictSendResult
	CreateClaimableBalanceResult        *CreateClaimableBalanceResult
	ClaimClaimableBalanceResult         *ClaimClaimableBalanceResult
	BeginSponsoringFutureReservesResult *BeginSponsoringFutureReservesResult
	EndSponsoringFutureReservesResult   *EndSponsoringFutureReservesResult
	RevokeSponsorshipResult             *RevokeSponsorshipResult
	ClawbackResult                      *ClawbackResult
	ClawbackClaimableBalanceResult      *ClawbackClaimableBalanceResult
	SetTrustLineFlagsResult             *SetTrustLineFlagsResult
	LiquidityPoolDepositResult          *LiquidityPoolDepositResult
	LiquidityPoolWithdrawResult         *LiquidityPoolWithdrawResult
	InvokeHostFunctionResult            *InvokeHostFunctionResult
	ExtendFootprintTtlResult            *ExtendFootprintTtlResult
	RestoreFootprintResult              *RestoreFootprintResult
	HelloWorldResult                    *HelloWorldResult
}

// SwitchFieldName returns the field name in which this union's
// discriminant is stored
func (u OperationResultTr) SwitchFieldName() string {
	return "Type"
}

// ArmForSwitch returns which field name should be used for storing
// the value for an instance of OperationResultTr
func (u OperationResultTr) ArmForSwitch(sw int32) (string, bool) {
	switch OperationType(sw) {
	case OperationTypeCreateAccount:
		return "CreateAccountResult", true
	case OperationTypePayment:
		return "PaymentResult", true
	case OperationTypePathPaymentStrictReceive:
		return "PathPaymentStrictReceiveResult", true
	case OperationTypeManageSellOffer:
		return "ManageSellOfferResult", true
	case OperationTypeCreatePassiveSellOffer:
		return "CreatePassiveSellOfferResult", true
	case OperationTypeSetOptions:
		return "SetOptionsResult", true
	case OperationTypeChangeTrust:
		return "ChangeTrustResult", true
	case OperationTypeAllowTrust:
		return "AllowTrustResult", true
	case OperationTypeAccountMerge:
		return "AccountMergeResult", true
	case OperationTypeInflation:
		return "InflationResult", true
	case OperationTypeManageData:
		return "ManageDataResult", true
	case OperationTypeBumpSequence:
		return "BumpSeqResult", true
	case OperationTypeManageBuyOffer:
		return "ManageBuyOfferResult", true
	case OperationTypePathPaymentStrictSend:
		return "PathPaymentStrictSendResult", true
	case OperationTypeCreateClaimableBalance:
		return "CreateClaimableBalanceResult", true
	case OperationTypeClaimClaimableBalance:
		return "ClaimClaimableBalanceResult", true
	case OperationTypeBeginSponsoringFutureReserves:
		return "BeginSponsoringFutureReservesResult", true
	case OperationTypeEndSponsoringFutureReserves:
		return "EndSponsoringFutureReservesResult", true
	case OperationTypeRevokeSponsorship:
		return "RevokeSponsorshipResult", true
	case OperationTypeClawback:
		return "ClawbackResult", true
	case OperationTypeClawbackClaimableBalance:
		return "ClawbackClaimableBalanceResult", true
	case OperationTypeSetTrustLineFlags:
		return "SetTrustLineFlagsResult", true
	case OperationTypeLiquidityPoolDeposit:
		return "LiquidityPoolDepositResult", true
	case OperationTypeLiquidityPoolWithdraw:
		return "LiquidityPoolWithdrawResult", true
	case OperationTypeInvokeHostFunction:
		return "InvokeHostFunctionResult", true
	case OperationTypeExtendFootprintTtl:
		return "ExtendFootprintTtlResult", true
	case OperationTypeRestoreFootprint:
		return "RestoreFootprintResult", true
	case OperationTypeHelloWorld:
		return "HelloWorldResult", true
	}
	return "-", false
}

// NewOperationResultTr creates a new  OperationResultTr.
func NewOperationResultTr(aType OperationType, value interface{}) (result OperationResultTr, err error) {
	result.Type = aType
	switch OperationType(aType) {
	case OperationTypeCreateAccount:
		tv, ok := value.(CreateAccountResult)
		if !ok {
			err = errors.New("invalid value, must be CreateAccountResult")
			return
		}
		result.CreateAccountResult = &tv
	case OperationTypePayment:
		tv, ok := value.(PaymentResult)
		if !ok {
			err = errors.New("invalid value, must be PaymentResult")
			return
		}
		result.PaymentResult = &tv
	case OperationTypePathPaymentStrictReceive:
		tv, ok := value.(PathPaymentStrictReceiveResult)
		if !ok {
			err = errors.New("invalid value, must be PathPaymentStrictReceiveResult")
			return
		}
		result.PathPaymentStrictReceiveResult = &tv
	case OperationTypeManageSellOffer:
		tv, ok := value.(ManageSellOfferResult)
		if !ok {
			err = errors.New("invalid value, must be ManageSellOfferResult")
			return
		}
		result.ManageSellOfferResult = &tv
	case OperationTypeCreatePassiveSellOffer:
		tv, ok := value.(ManageSellOfferResult)
		if !ok {
			err = errors.New("invalid value, must be ManageSellOfferResult")
			return
		}
		result.CreatePassiveSellOfferResult = &tv
	case OperationTypeSetOptions:
		tv, ok := value.(SetOptionsResult)
		if !ok {
			err = errors.New("invalid value, must be SetOptionsResult")
			return
		}
		result.SetOptionsResult = &tv
	case OperationTypeChangeTrust:
		tv, ok := value.(ChangeTrustResult)
		if !ok {
			err = errors.New("invalid value, must be ChangeTrustResult")
			return
		}
		result.ChangeTrustResult = &tv
	case OperationTypeAllowTrust:
		tv, ok := value.(AllowTrustResult)
		if !ok {
			err = errors.New("invalid value, must be AllowTrustResult")
			return
		}
		result.AllowTrustResult = &tv
	case OperationTypeAccountMerge:
		tv, ok := value.(AccountMergeResult)
		if !ok {
			err = errors.New("invalid value, must be AccountMergeResult")
			return
		}
		result.AccountMergeResult = &tv
	case OperationTypeInflation:
		tv, ok := value.(InflationResult)
		if !ok {
			err = errors.New("invalid value, must be InflationResult")
			return
		}
		result.InflationResult = &tv
	case OperationTypeManageData:
		tv, ok := value.(ManageDataResult)
		if !ok {
			err = errors.New("invalid value, must be ManageDataResult")
			return
		}
		result.ManageDataResult = &tv
	case OperationTypeBumpSequence:
		tv, ok := value.(BumpSequenceResult)
		if !ok {
			err = errors.New("invalid value, must be BumpSequenceResult")
			return
		}
		result.BumpSeqResult = &tv
	case OperationTypeManageBuyOffer:
		tv, ok := value.(ManageBuyOfferResult)
		if !ok {
			err = errors.New("invalid value, must be ManageBuyOfferResult")
			return
		}
		result.ManageBuyOfferResult = &tv
	case OperationTypePathPaymentStrictSend:
		tv, ok := value.(PathPaymentStrictSendResult)
		if !ok {
			err = errors.New("invalid value, must be PathPaymentStrictSendResult")
			return
		}
		result.PathPaymentStrictSendResult = &tv
	case OperationTypeCreateClaimableBalance:
		tv, ok := value.(CreateClaimableBalanceResult)
		if !ok {
			err = errors.New("invalid value, must be CreateClaimableBalanceResult")
			return
		}
		result.CreateClaimableBalanceResult = &tv
	case OperationTypeClaimClaimableBalance:
		tv, ok := value.(ClaimClaimableBalanceResult)
		if !ok {
			err = errors.New("invalid value, must be ClaimClaimableBalanceResult")
			return
		}
		result.ClaimClaimableBalanceResult = &tv
	case OperationTypeBeginSponsoringFutureReserves:
		tv, ok := value.(BeginSponsoringFutureReservesResult)
		if !ok {
			err = errors.New("invalid value, must be BeginSponsoringFutureReservesResult")
			return
		}
		result.BeginSponsoringFutureReservesResult = &tv
	case OperationTypeEndSponsoringFutureReserves:
		tv, ok := value.(EndSponsoringFutureReservesResult)
		if !ok {
			err = errors.New("invalid value, must be EndSponsoringFutureReservesResult")
			return
		}
		result.EndSponsoringFutureReservesResult = &tv
	case OperationTypeRevokeSponsorship:
		tv, ok := value.(RevokeSponsorshipResult)
		if !ok {
			err = errors.New("invalid value, must be RevokeSponsorshipResult")
			return
		}
		result.RevokeSponsorshipResult = &tv
	case OperationTypeClawback:
		tv, ok := value.(ClawbackResult)
		if !ok {
			err = errors.New("invalid value, must be ClawbackResult")
			return
		}
		result.ClawbackResult = &tv
	case OperationTypeClawbackClaimableBalance:
		tv, ok := value.(ClawbackClaimableBalanceResult)
		if !ok {
			err = errors.New("invalid value, must be ClawbackClaimableBalanceResult")
			return
		}
		result.ClawbackClaimableBalanceResult = &tv
	case OperationTypeSetTrustLineFlags:
		tv, ok := value.(SetTrustLineFlagsResult)
		if !ok {
			err = errors.New("invalid value, must be SetTrustLineFlagsResult")
			return
		}
		result.SetTrustLineFlagsResult = &tv
	case OperationTypeLiquidityPoolDeposit:
		tv, ok := value.(LiquidityPoolDepositResult)
		if !ok {
			err = errors.New("invalid value, must be LiquidityPoolDepositResult")
			return
		}
		result.LiquidityPoolDepositResult = &tv
	case OperationTypeLiquidityPoolWithdraw:
		tv, ok := value.(LiquidityPoolWithdrawResult)
		if !ok {
			err = errors.New("invalid value, must be LiquidityPoolWithdrawResult")
			return
		}
		result.LiquidityPoolWithdrawResult = &tv
	case OperationTypeInvokeHostFunction:
		tv, ok := value.(InvokeHostFunctionResult)
		if !ok {
			err = errors.New("invalid value, must be InvokeHostFunctionResult")
			return
		}
		result.InvokeHostFunctionResult = &tv
	case OperationTypeExtendFootprintTtl:
		tv, ok := value.(ExtendFootprintTtlResult)
		if !ok {
			err = errors.New("invalid value, must be ExtendFootprintTtlResult")
			return
		}
		result.ExtendFootprintTtlResult = &tv
	case OperationTypeRestoreFootprint:
		tv, ok := value.(RestoreFootprintResult)
		if !ok {
			err = errors.New("invalid value, must be RestoreFootprintResult")
			return
		}
		result.RestoreFootprintResult = &tv
	case OperationTypeHelloWorld:
		tv, ok := value.(HelloWorldResult)
		if !ok {
			err = errors.New("invalid value, must be HelloWorldResult")
			return
		}
		result.HelloWorldResult = &tv
	}
	return
}

// MustCreateAccountResult retrieves the CreateAccountResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustCreateAccountResult() CreateAccountResult {
	val, ok := u.GetCreateAccountResult()

	if !ok {
		panic("arm CreateAccountResult is not set")
	}

	return val
}

// GetCreateAccountResult retrieves the CreateAccountResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetCreateAccountResult() (result CreateAccountResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "CreateAccountResult" {
		result = *u.CreateAccountResult
		ok = true
	}

	return
}

// MustPaymentResult retrieves the PaymentResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustPaymentResult() PaymentResult {
	val, ok := u.GetPaymentResult()

	if !ok {
		panic("arm PaymentResult is not set")
	}

	return val
}

// GetPaymentResult retrieves the PaymentResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetPaymentResult() (result PaymentResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "PaymentResult" {
		result = *u.PaymentResult
		ok = true
	}

	return
}

// MustPathPaymentStrictReceiveResult retrieves the PathPaymentStrictReceiveResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustPathPaymentStrictReceiveResult() PathPaymentStrictReceiveResult {
	val, ok := u.GetPathPaymentStrictReceiveResult()

	if !ok {
		panic("arm PathPaymentStrictReceiveResult is not set")
	}

	return val
}

// GetPathPaymentStrictReceiveResult retrieves the PathPaymentStrictReceiveResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetPathPaymentStrictReceiveResult() (result PathPaymentStrictReceiveResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "PathPaymentStrictReceiveResult" {
		result = *u.PathPaymentStrictReceiveResult
		ok = true
	}

	return
}

// MustManageSellOfferResult retrieves the ManageSellOfferResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustManageSellOfferResult() ManageSellOfferResult {
	val, ok := u.GetManageSellOfferResult()

	if !ok {
		panic("arm ManageSellOfferResult is not set")
	}

	return val
}

// GetManageSellOfferResult retrieves the ManageSellOfferResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetManageSellOfferResult() (result ManageSellOfferResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ManageSellOfferResult" {
		result = *u.ManageSellOfferResult
		ok = true
	}

	return
}

// MustCreatePassiveSellOfferResult retrieves the CreatePassiveSellOfferResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustCreatePassiveSellOfferResult() ManageSellOfferResult {
	val, ok := u.GetCreatePassiveSellOfferResult()

	if !ok {
		panic("arm CreatePassiveSellOfferResult is not set")
	}

	return val
}

// GetCreatePassiveSellOfferResult retrieves the CreatePassiveSellOfferResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetCreatePassiveSellOfferResult() (result ManageSellOfferResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "CreatePassiveSellOfferResult" {
		result = *u.CreatePassiveSellOfferResult
		ok = true
	}

	return
}

// MustSetOptionsResult retrieves the SetOptionsResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustSetOptionsResult() SetOptionsResult {
	val, ok := u.GetSetOptionsResult()

	if !ok {
		panic("arm SetOptionsResult is not set")
	}

	return val
}

// GetSetOptionsResult retrieves the SetOptionsResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetSetOptionsResult() (result SetOptionsResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "SetOptionsResult" {
		result = *u.SetOptionsResult
		ok = true
	}

	return
}

// MustChangeTrustResult retrieves the ChangeTrustResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustChangeTrustResult() ChangeTrustResult {
	val, ok := u.GetChangeTrustResult()

	if !ok {
		panic("arm ChangeTrustResult is not set")
	}

	return val
}

// GetChangeTrustResult retrieves the ChangeTrustResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetChangeTrustResult() (result ChangeTrustResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ChangeTrustResult" {
		result = *u.ChangeTrustResult
		ok = true
	}

	return
}

// MustAllowTrustResult retrieves the AllowTrustResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustAllowTrustResult() AllowTrustResult {
	val, ok := u.GetAllowTrustResult()

	if !ok {
		panic("arm AllowTrustResult is not set")
	}

	return val
}

// GetAllowTrustResult retrieves the AllowTrustResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetAllowTrustResult() (result AllowTrustResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "AllowTrustResult" {
		result = *u.AllowTrustResult
		ok = true
	}

	return
}

// MustAccountMergeResult retrieves the AccountMergeResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustAccountMergeResult() AccountMergeResult {
	val, ok := u.GetAccountMergeResult()

	if !ok {
		panic("arm AccountMergeResult is not set")
	}

	return val
}

// GetAccountMergeResult retrieves the AccountMergeResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetAccountMergeResult() (result AccountMergeResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "AccountMergeResult" {
		result = *u.AccountMergeResult
		ok = true
	}

	return
}

// MustInflationResult retrieves the InflationResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustInflationResult() InflationResult {
	val, ok := u.GetInflationResult()

	if !ok {
		panic("arm InflationResult is not set")
	}

	return val
}

// GetInflationResult retrieves the InflationResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetInflationResult() (result InflationResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "InflationResult" {
		result = *u.InflationResult
		ok = true
	}

	return
}

// MustManageDataResult retrieves the ManageDataResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustManageDataResult() ManageDataResult {
	val, ok := u.GetManageDataResult()

	if !ok {
		panic("arm ManageDataResult is not set")
	}

	return val
}

// GetManageDataResult retrieves the ManageDataResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetManageDataResult() (result ManageDataResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ManageDataResult" {
		result = *u.ManageDataResult
		ok = true
	}

	return
}

// MustBumpSeqResult retrieves the BumpSeqResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustBumpSeqResult() BumpSequenceResult {
	val, ok := u.GetBumpSeqResult()

	if !ok {
		panic("arm BumpSeqResult is not set")
	}

	return val
}

// GetBumpSeqResult retrieves the BumpSeqResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetBumpSeqResult() (result BumpSequenceResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "BumpSeqResult" {
		result = *u.BumpSeqResult
		ok = true
	}

	return
}

// MustManageBuyOfferResult retrieves the ManageBuyOfferResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustManageBuyOfferResult() ManageBuyOfferResult {
	val, ok := u.GetManageBuyOfferResult()

	if !ok {
		panic("arm ManageBuyOfferResult is not set")
	}

	return val
}

// GetManageBuyOfferResult retrieves the ManageBuyOfferResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetManageBuyOfferResult() (result ManageBuyOfferResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ManageBuyOfferResult" {
		result = *u.ManageBuyOfferResult
		ok = true
	}

	return
}

// MustPathPaymentStrictSendResult retrieves the PathPaymentStrictSendResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustPathPaymentStrictSendResult() PathPaymentStrictSendResult {
	val, ok := u.GetPathPaymentStrictSendResult()

	if !ok {
		panic("arm PathPaymentStrictSendResult is not set")
	}

	return val
}

// GetPathPaymentStrictSendResult retrieves the PathPaymentStrictSendResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetPathPaymentStrictSendResult() (result PathPaymentStrictSendResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "PathPaymentStrictSendResult" {
		result = *u.PathPaymentStrictSendResult
		ok = true
	}

	return
}

// MustCreateClaimableBalanceResult retrieves the CreateClaimableBalanceResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustCreateClaimableBalanceResult() CreateClaimableBalanceResult {
	val, ok := u.GetCreateClaimableBalanceResult()

	if !ok {
		panic("arm CreateClaimableBalanceResult is not set")
	}

	return val
}

// GetCreateClaimableBalanceResult retrieves the CreateClaimableBalanceResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetCreateClaimableBalanceResult() (result CreateClaimableBalanceResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "CreateClaimableBalanceResult" {
		result = *u.CreateClaimableBalanceResult
		ok = true
	}

	return
}

// MustClaimClaimableBalanceResult retrieves the ClaimClaimableBalanceResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustClaimClaimableBalanceResult() ClaimClaimableBalanceResult {
	val, ok := u.GetClaimClaimableBalanceResult()

	if !ok {
		panic("arm ClaimClaimableBalanceResult is not set")
	}

	return val
}

// GetClaimClaimableBalanceResult retrieves the ClaimClaimableBalanceResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetClaimClaimableBalanceResult() (result ClaimClaimableBalanceResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ClaimClaimableBalanceResult" {
		result = *u.ClaimClaimableBalanceResult
		ok = true
	}

	return
}

// MustBeginSponsoringFutureReservesResult retrieves the BeginSponsoringFutureReservesResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustBeginSponsoringFutureReservesResult() BeginSponsoringFutureReservesResult {
	val, ok := u.GetBeginSponsoringFutureReservesResult()

	if !ok {
		panic("arm BeginSponsoringFutureReservesResult is not set")
	}

	return val
}

// GetBeginSponsoringFutureReservesResult retrieves the BeginSponsoringFutureReservesResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetBeginSponsoringFutureReservesResult() (result BeginSponsoringFutureReservesResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "BeginSponsoringFutureReservesResult" {
		result = *u.BeginSponsoringFutureReservesResult
		ok = true
	}

	return
}

// MustEndSponsoringFutureReservesResult retrieves the EndSponsoringFutureReservesResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustEndSponsoringFutureReservesResult() EndSponsoringFutureReservesResult {
	val, ok := u.GetEndSponsoringFutureReservesResult()

	if !ok {
		panic("arm EndSponsoringFutureReservesResult is not set")
	}

	return val
}

// GetEndSponsoringFutureReservesResult retrieves the EndSponsoringFutureReservesResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetEndSponsoringFutureReservesResult() (result EndSponsoringFutureReservesResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "EndSponsoringFutureReservesResult" {
		result = *u.EndSponsoringFutureReservesResult
		ok = true
	}

	return
}

// MustRevokeSponsorshipResult retrieves the RevokeSponsorshipResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustRevokeSponsorshipResult() RevokeSponsorshipResult {
	val, ok := u.GetRevokeSponsorshipResult()

	if !ok {
		panic("arm RevokeSponsorshipResult is not set")
	}

	return val
}

// GetRevokeSponsorshipResult retrieves the RevokeSponsorshipResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetRevokeSponsorshipResult() (result RevokeSponsorshipResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "RevokeSponsorshipResult" {
		result = *u.RevokeSponsorshipResult
		ok = true
	}

	return
}

// MustClawbackResult retrieves the ClawbackResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustClawbackResult() ClawbackResult {
	val, ok := u.GetClawbackResult()

	if !ok {
		panic("arm ClawbackResult is not set")
	}

	return val
}

// GetClawbackResult retrieves the ClawbackResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetClawbackResult() (result ClawbackResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ClawbackResult" {
		result = *u.ClawbackResult
		ok = true
	}

	return
}

// MustClawbackClaimableBalanceResult retrieves the ClawbackClaimableBalanceResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustClawbackClaimableBalanceResult() ClawbackClaimableBalanceResult {
	val, ok := u.GetClawbackClaimableBalanceResult()

	if !ok {
		panic("arm ClawbackClaimableBalanceResult is not set")
	}

	return val
}

// GetClawbackClaimableBalanceResult retrieves the ClawbackClaimableBalanceResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetClawbackClaimableBalanceResult() (result ClawbackClaimableBalanceResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ClawbackClaimableBalanceResult" {
		result = *u.ClawbackClaimableBalanceResult
		ok = true
	}

	return
}

// MustSetTrustLineFlagsResult retrieves the SetTrustLineFlagsResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustSetTrustLineFlagsResult() SetTrustLineFlagsResult {
	val, ok := u.GetSetTrustLineFlagsResult()

	if !ok {
		panic("arm SetTrustLineFlagsResult is not set")
	}

	return val
}

// GetSetTrustLineFlagsResult retrieves the SetTrustLineFlagsResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetSetTrustLineFlagsResult() (result SetTrustLineFlagsResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "SetTrustLineFlagsResult" {
		result = *u.SetTrustLineFlagsResult
		ok = true
	}

	return
}

// MustLiquidityPoolDepositResult retrieves the LiquidityPoolDepositResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustLiquidityPoolDepositResult() LiquidityPoolDepositResult {
	val, ok := u.GetLiquidityPoolDepositResult()

	if !ok {
		panic("arm LiquidityPoolDepositResult is not set")
	}

	return val
}

// GetLiquidityPoolDepositResult retrieves the LiquidityPoolDepositResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetLiquidityPoolDepositResult() (result LiquidityPoolDepositResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "LiquidityPoolDepositResult" {
		result = *u.LiquidityPoolDepositResult
		ok = true
	}

	return
}

// MustLiquidityPoolWithdrawResult retrieves the LiquidityPoolWithdrawResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustLiquidityPoolWithdrawResult() LiquidityPoolWithdrawResult {
	val, ok := u.GetLiquidityPoolWithdrawResult()

	if !ok {
		panic("arm LiquidityPoolWithdrawResult is not set")
	}

	return val
}

// GetLiquidityPoolWithdrawResult retrieves the LiquidityPoolWithdrawResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetLiquidityPoolWithdrawResult() (result LiquidityPoolWithdrawResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "LiquidityPoolWithdrawResult" {
		result = *u.LiquidityPoolWithdrawResult
		ok = true
	}

	return
}

// MustInvokeHostFunctionResult retrieves the InvokeHostFunctionResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustInvokeHostFunctionResult() InvokeHostFunctionResult {
	val, ok := u.GetInvokeHostFunctionResult()

	if !ok {
		panic("arm InvokeHostFunctionResult is not set")
	}

	return val
}

// GetInvokeHostFunctionResult retrieves the InvokeHostFunctionResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetInvokeHostFunctionResult() (result InvokeHostFunctionResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "InvokeHostFunctionResult" {
		result = *u.InvokeHostFunctionResult
		ok = true
	}

	return
}

// MustExtendFootprintTtlResult retrieves the ExtendFootprintTtlResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustExtendFootprintTtlResult() ExtendFootprintTtlResult {
	val, ok := u.GetExtendFootprintTtlResult()

	if !ok {
		panic("arm ExtendFootprintTtlResult is not set")
	}

	return val
}

// GetExtendFootprintTtlResult retrieves the ExtendFootprintTtlResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetExtendFootprintTtlResult() (result ExtendFootprintTtlResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "ExtendFootprintTtlResult" {
		result = *u.ExtendFootprintTtlResult
		ok = true
	}

	return
}

// MustRestoreFootprintResult retrieves the RestoreFootprintResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustRestoreFootprintResult() RestoreFootprintResult {
	val, ok := u.GetRestoreFootprintResult()

	if !ok {
		panic("arm RestoreFootprintResult is not set")
	}

	return val
}

// GetRestoreFootprintResult retrieves the RestoreFootprintResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetRestoreFootprintResult() (result RestoreFootprintResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "RestoreFootprintResult" {
		result = *u.RestoreFootprintResult
		ok = true
	}

	return
}

// MustHelloWorldResult retrieves the HelloWorldResult value from the union,
// panicing if the value is not set.
func (u OperationResultTr) MustHelloWorldResult() HelloWorldResult {
	val, ok := u.GetHelloWorldResult()

	if !ok {
		panic("arm HelloWorldResult is not set")
	}

	return val
}

// GetHelloWorldResult retrieves the HelloWorldResult value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u OperationResultTr) GetHelloWorldResult() (result HelloWorldResult, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.Type))

	if armName == "HelloWorldResult" {
		result = *u.HelloWorldResult
		ok = true
	}

	return
}

// EncodeTo encodes this value using the Encoder.
func (u OperationResultTr) EncodeTo(e *xdr.Encoder) error {
	var err error
	if err = u.Type.EncodeTo(e); err != nil {
		return err
	}
	switch OperationType(u.Type) {
	case OperationTypeCreateAccount:
		if err = (*u.CreateAccountResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypePayment:
		if err = (*u.PaymentResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypePathPaymentStrictReceive:
		if err = (*u.PathPaymentStrictReceiveResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeManageSellOffer:
		if err = (*u.ManageSellOfferResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeCreatePassiveSellOffer:
		if err = (*u.CreatePassiveSellOfferResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeSetOptions:
		if err = (*u.SetOptionsResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeChangeTrust:
		if err = (*u.ChangeTrustResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeAllowTrust:
		if err = (*u.AllowTrustResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeAccountMerge:
		if err = (*u.AccountMergeResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeInflation:
		if err = (*u.InflationResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeManageData:
		if err = (*u.ManageDataResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeBumpSequence:
		if err = (*u.BumpSeqResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeManageBuyOffer:
		if err = (*u.ManageBuyOfferResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypePathPaymentStrictSend:
		if err = (*u.PathPaymentStrictSendResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeCreateClaimableBalance:
		if err = (*u.CreateClaimableBalanceResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeClaimClaimableBalance:
		if err = (*u.ClaimClaimableBalanceResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeBeginSponsoringFutureReserves:
		if err = (*u.BeginSponsoringFutureReservesResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeEndSponsoringFutureReserves:
		if err = (*u.EndSponsoringFutureReservesResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeRevokeSponsorship:
		if err = (*u.RevokeSponsorshipResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeClawback:
		if err = (*u.ClawbackResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeClawbackClaimableBalance:
		if err = (*u.ClawbackClaimableBalanceResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeSetTrustLineFlags:
		if err = (*u.SetTrustLineFlagsResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeLiquidityPoolDeposit:
		if err = (*u.LiquidityPoolDepositResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeLiquidityPoolWithdraw:
		if err = (*u.LiquidityPoolWithdrawResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeInvokeHostFunction:
		if err = (*u.InvokeHostFunctionResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeExtendFootprintTtl:
		if err = (*u.ExtendFootprintTtlResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeRestoreFootprint:
		if err = (*u.RestoreFootprintResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case OperationTypeHelloWorld:
		if err = (*u.HelloWorldResult).EncodeTo(e); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("Type (OperationType) switch value '%d' is not valid for union OperationResultTr", u.Type)
}

var _ decoderFrom = (*OperationResultTr)(nil)

// DecodeFrom decodes this value using the Decoder.
func (u *OperationResultTr) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding OperationResultTr: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	var err error
	var n, nTmp int
	nTmp, err = u.Type.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding OperationType: %w", err)
	}
	switch OperationType(u.Type) {
	case OperationTypeCreateAccount:
		u.CreateAccountResult = new(CreateAccountResult)
		nTmp, err = (*u.CreateAccountResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding CreateAccountResult: %w", err)
		}
		return n, nil
	case OperationTypePayment:
		u.PaymentResult = new(PaymentResult)
		nTmp, err = (*u.PaymentResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding PaymentResult: %w", err)
		}
		return n, nil
	case OperationTypePathPaymentStrictReceive:
		u.PathPaymentStrictReceiveResult = new(PathPaymentStrictReceiveResult)
		nTmp, err = (*u.PathPaymentStrictReceiveResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding PathPaymentStrictReceiveResult: %w", err)
		}
		return n, nil
	case OperationTypeManageSellOffer:
		u.ManageSellOfferResult = new(ManageSellOfferResult)
		nTmp, err = (*u.ManageSellOfferResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ManageSellOfferResult: %w", err)
		}
		return n, nil
	case OperationTypeCreatePassiveSellOffer:
		u.CreatePassiveSellOfferResult = new(ManageSellOfferResult)
		nTmp, err = (*u.CreatePassiveSellOfferResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ManageSellOfferResult: %w", err)
		}
		return n, nil
	case OperationTypeSetOptions:
		u.SetOptionsResult = new(SetOptionsResult)
		nTmp, err = (*u.SetOptionsResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding SetOptionsResult: %w", err)
		}
		return n, nil
	case OperationTypeChangeTrust:
		u.ChangeTrustResult = new(ChangeTrustResult)
		nTmp, err = (*u.ChangeTrustResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ChangeTrustResult: %w", err)
		}
		return n, nil
	case OperationTypeAllowTrust:
		u.AllowTrustResult = new(AllowTrustResult)
		nTmp, err = (*u.AllowTrustResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding AllowTrustResult: %w", err)
		}
		return n, nil
	case OperationTypeAccountMerge:
		u.AccountMergeResult = new(AccountMergeResult)
		nTmp, err = (*u.AccountMergeResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding AccountMergeResult: %w", err)
		}
		return n, nil
	case OperationTypeInflation:
		u.InflationResult = new(InflationResult)
		nTmp, err = (*u.InflationResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding InflationResult: %w", err)
		}
		return n, nil
	case OperationTypeManageData:
		u.ManageDataResult = new(ManageDataResult)
		nTmp, err = (*u.ManageDataResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ManageDataResult: %w", err)
		}
		return n, nil
	case OperationTypeBumpSequence:
		u.BumpSeqResult = new(BumpSequenceResult)
		nTmp, err = (*u.BumpSeqResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding BumpSequenceResult: %w", err)
		}
		return n, nil
	case OperationTypeManageBuyOffer:
		u.ManageBuyOfferResult = new(ManageBuyOfferResult)
		nTmp, err = (*u.ManageBuyOfferResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ManageBuyOfferResult: %w", err)
		}
		return n, nil
	case OperationTypePathPaymentStrictSend:
		u.PathPaymentStrictSendResult = new(PathPaymentStrictSendResult)
		nTmp, err = (*u.PathPaymentStrictSendResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding PathPaymentStrictSendResult: %w", err)
		}
		return n, nil
	case OperationTypeCreateClaimableBalance:
		u.CreateClaimableBalanceResult = new(CreateClaimableBalanceResult)
		nTmp, err = (*u.CreateClaimableBalanceResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding CreateClaimableBalanceResult: %w", err)
		}
		return n, nil
	case OperationTypeClaimClaimableBalance:
		u.ClaimClaimableBalanceResult = new(ClaimClaimableBalanceResult)
		nTmp, err = (*u.ClaimClaimableBalanceResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ClaimClaimableBalanceResult: %w", err)
		}
		return n, nil
	case OperationTypeBeginSponsoringFutureReserves:
		u.BeginSponsoringFutureReservesResult = new(BeginSponsoringFutureReservesResult)
		nTmp, err = (*u.BeginSponsoringFutureReservesResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding BeginSponsoringFutureReservesResult: %w", err)
		}
		return n, nil
	case OperationTypeEndSponsoringFutureReserves:
		u.EndSponsoringFutureReservesResult = new(EndSponsoringFutureReservesResult)
		nTmp, err = (*u.EndSponsoringFutureReservesResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding EndSponsoringFutureReservesResult: %w", err)
		}
		return n, nil
	case OperationTypeRevokeSponsorship:
		u.RevokeSponsorshipResult = new(RevokeSponsorshipResult)
		nTmp, err = (*u.RevokeSponsorshipResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding RevokeSponsorshipResult: %w", err)
		}
		return n, nil
	case OperationTypeClawback:
		u.ClawbackResult = new(ClawbackResult)
		nTmp, err = (*u.ClawbackResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ClawbackResult: %w", err)
		}
		return n, nil
	case OperationTypeClawbackClaimableBalance:
		u.ClawbackClaimableBalanceResult = new(ClawbackClaimableBalanceResult)
		nTmp, err = (*u.ClawbackClaimableBalanceResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ClawbackClaimableBalanceResult: %w", err)
		}
		return n, nil
	case OperationTypeSetTrustLineFlags:
		u.SetTrustLineFlagsResult = new(SetTrustLineFlagsResult)
		nTmp, err = (*u.SetTrustLineFlagsResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding SetTrustLineFlagsResult: %w", err)
		}
		return n, nil
	case OperationTypeLiquidityPoolDeposit:
		u.LiquidityPoolDepositResult = new(LiquidityPoolDepositResult)
		nTmp, err = (*u.LiquidityPoolDepositResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding LiquidityPoolDepositResult: %w", err)
		}
		return n, nil
	case OperationTypeLiquidityPoolWithdraw:
		u.LiquidityPoolWithdrawResult = new(LiquidityPoolWithdrawResult)
		nTmp, err = (*u.LiquidityPoolWithdrawResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding LiquidityPoolWithdrawResult: %w", err)
		}
		return n, nil
	case OperationTypeInvokeHostFunction:
		u.InvokeHostFunctionResult = new(InvokeHostFunctionResult)
		nTmp, err = (*u.InvokeHostFunctionResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding InvokeHostFunctionResult: %w", err)
		}
		return n, nil
	case OperationTypeExtendFootprintTtl:
		u.ExtendFootprintTtlResult = new(ExtendFootprintTtlResult)
		nTmp, err = (*u.ExtendFootprintTtlResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding ExtendFootprintTtlResult: %w", err)
		}
		return n, nil
	case OperationTypeRestoreFootprint:
		u.RestoreFootprintResult = new(RestoreFootprintResult)
		nTmp, err = (*u.RestoreFootprintResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding RestoreFootprintResult: %w", err)
		}
		return n, nil
	case OperationTypeHelloWorld:
		u.HelloWorldResult = new(HelloWorldResult)
		nTmp, err = (*u.HelloWorldResult).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding HelloWorldResult: %w", err)
		}
		return n, nil
	}
	return n, fmt.Errorf("union OperationResultTr has invalid Type (OperationType) switch value '%d'", u.Type)
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s OperationResultTr) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *OperationResultTr) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*OperationResultTr)(nil)
	_ encoding.BinaryUnmarshaler = (*OperationResultTr)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s OperationResultTr) xdrType() {}

var _ xdrType = (*OperationResultTr)(nil)
