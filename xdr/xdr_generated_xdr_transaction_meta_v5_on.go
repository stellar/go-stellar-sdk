//go:build xdr_transaction_meta_v5

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

// TransactionMetaV5 is an XDR Struct defines as:
//
//	struct TransactionMetaV5
//	 {
//	     ExtensionPoint ext;
//
//	     LedgerEntryChanges txChangesBefore;  // tx level changes before operations
//	                                          // are applied if any
//	     OperationMetaV2 operations<>;        // meta for each operation
//	     LedgerEntryChanges txChangesAfter;   // tx level changes after operations are
//	                                          // applied if any
//	     SorobanTransactionMetaV2* sorobanMeta; // Soroban-specific meta (only for
//	                                            // Soroban transactions).
//
//	     TransactionEvent events<>; // Used for transaction-level events (like fee payment)
//	     DiagnosticEvent diagnosticEvents<>; // Used for all diagnostic information
//	     uint32 txIndex; // Index of the transaction in the ledger
//	 };
type TransactionMetaV5 struct {
	Ext              ExtensionPoint
	TxChangesBefore  LedgerEntryChanges
	Operations       []OperationMetaV2
	TxChangesAfter   LedgerEntryChanges
	SorobanMeta      *SorobanTransactionMetaV2
	Events           []TransactionEvent
	DiagnosticEvents []DiagnosticEvent
	TxIndex          Uint32
}

// EncodeTo encodes this value using the Encoder.
func (s *TransactionMetaV5) EncodeTo(e *xdr.Encoder) error {
	var err error
	if err = s.Ext.EncodeTo(e); err != nil {
		return err
	}
	if err = s.TxChangesBefore.EncodeTo(e); err != nil {
		return err
	}
	if _, err = e.EncodeUint(uint32(len(s.Operations))); err != nil {
		return err
	}
	for i := 0; i < len(s.Operations); i++ {
		if err = s.Operations[i].EncodeTo(e); err != nil {
			return err
		}
	}
	if err = s.TxChangesAfter.EncodeTo(e); err != nil {
		return err
	}
	if _, err = e.EncodeBool(s.SorobanMeta != nil); err != nil {
		return err
	}
	if s.SorobanMeta != nil {
		if err = (*s.SorobanMeta).EncodeTo(e); err != nil {
			return err
		}
	}
	if _, err = e.EncodeUint(uint32(len(s.Events))); err != nil {
		return err
	}
	for i := 0; i < len(s.Events); i++ {
		if err = s.Events[i].EncodeTo(e); err != nil {
			return err
		}
	}
	if _, err = e.EncodeUint(uint32(len(s.DiagnosticEvents))); err != nil {
		return err
	}
	for i := 0; i < len(s.DiagnosticEvents); i++ {
		if err = s.DiagnosticEvents[i].EncodeTo(e); err != nil {
			return err
		}
	}
	if err = s.TxIndex.EncodeTo(e); err != nil {
		return err
	}
	return nil
}

var _ decoderFrom = (*TransactionMetaV5)(nil)

// DecodeFrom decodes this value using the Decoder.
func (s *TransactionMetaV5) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding TransactionMetaV5: %w", ErrMaxDecodingDepthReached)
	}
	maxDepth -= 1
	var err error
	var n, nTmp int
	nTmp, err = s.Ext.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding ExtensionPoint: %w", err)
	}
	nTmp, err = s.TxChangesBefore.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding LedgerEntryChanges: %w", err)
	}
	var l uint32
	l, nTmp, err = d.DecodeUint()
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding OperationMetaV2: %w", err)
	}
	s.Operations = nil
	if l > 0 {
		if il, ok := d.InputLen(); ok && uint(il) < uint(l) {
			return n, fmt.Errorf("decoding OperationMetaV2: length (%d) exceeds remaining input length (%d)", l, il)
		}
		s.Operations = make([]OperationMetaV2, l)
		for i := uint32(0); i < l; i++ {
			nTmp, err = s.Operations[i].DecodeFrom(d, maxDepth)
			n += nTmp
			if err != nil {
				return n, fmt.Errorf("decoding OperationMetaV2: %w", err)
			}
		}
	}
	nTmp, err = s.TxChangesAfter.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding LedgerEntryChanges: %w", err)
	}
	var b bool
	b, nTmp, err = d.DecodeBool()
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding SorobanTransactionMetaV2: %w", err)
	}
	s.SorobanMeta = nil
	if b {
		s.SorobanMeta = new(SorobanTransactionMetaV2)
		nTmp, err = s.SorobanMeta.DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding SorobanTransactionMetaV2: %w", err)
		}
	}
	l, nTmp, err = d.DecodeUint()
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding TransactionEvent: %w", err)
	}
	s.Events = nil
	if l > 0 {
		if il, ok := d.InputLen(); ok && uint(il) < uint(l) {
			return n, fmt.Errorf("decoding TransactionEvent: length (%d) exceeds remaining input length (%d)", l, il)
		}
		s.Events = make([]TransactionEvent, l)
		for i := uint32(0); i < l; i++ {
			nTmp, err = s.Events[i].DecodeFrom(d, maxDepth)
			n += nTmp
			if err != nil {
				return n, fmt.Errorf("decoding TransactionEvent: %w", err)
			}
		}
	}
	l, nTmp, err = d.DecodeUint()
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding DiagnosticEvent: %w", err)
	}
	s.DiagnosticEvents = nil
	if l > 0 {
		if il, ok := d.InputLen(); ok && uint(il) < uint(l) {
			return n, fmt.Errorf("decoding DiagnosticEvent: length (%d) exceeds remaining input length (%d)", l, il)
		}
		s.DiagnosticEvents = make([]DiagnosticEvent, l)
		for i := uint32(0); i < l; i++ {
			nTmp, err = s.DiagnosticEvents[i].DecodeFrom(d, maxDepth)
			n += nTmp
			if err != nil {
				return n, fmt.Errorf("decoding DiagnosticEvent: %w", err)
			}
		}
	}
	nTmp, err = s.TxIndex.DecodeFrom(d, maxDepth)
	n += nTmp
	if err != nil {
		return n, fmt.Errorf("decoding Uint32: %w", err)
	}
	return n, nil
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s TransactionMetaV5) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *TransactionMetaV5) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*TransactionMetaV5)(nil)
	_ encoding.BinaryUnmarshaler = (*TransactionMetaV5)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s TransactionMetaV5) xdrType() {}

var _ xdrType = (*TransactionMetaV5)(nil)

// TransactionMeta is an XDR Union defines as:
//
//	union TransactionMeta switch (int v)
//	 {
//	 case 0:
//	     OperationMeta operations<>;
//	 case 1:
//	     TransactionMetaV1 v1;
//	 case 2:
//	     TransactionMetaV2 v2;
//	 case 3:
//	     TransactionMetaV3 v3;
//	 case 4:
//	     TransactionMetaV4 v4;
//
//	 case 5:
//	     TransactionMetaV5 v5;
//
//	 };
type TransactionMeta struct {
	V          int32
	Operations *[]OperationMeta
	V1         *TransactionMetaV1
	V2         *TransactionMetaV2
	V3         *TransactionMetaV3
	V4         *TransactionMetaV4
	V5         *TransactionMetaV5
}

// SwitchFieldName returns the field name in which this union's
// discriminant is stored
func (u TransactionMeta) SwitchFieldName() string {
	return "V"
}

// ArmForSwitch returns which field name should be used for storing
// the value for an instance of TransactionMeta
func (u TransactionMeta) ArmForSwitch(sw int32) (string, bool) {
	switch int32(sw) {
	case 0:
		return "Operations", true
	case 1:
		return "V1", true
	case 2:
		return "V2", true
	case 3:
		return "V3", true
	case 4:
		return "V4", true
	case 5:
		return "V5", true
	}
	return "-", false
}

// NewTransactionMeta creates a new  TransactionMeta.
func NewTransactionMeta(v int32, value interface{}) (result TransactionMeta, err error) {
	result.V = v
	switch int32(v) {
	case 0:
		tv, ok := value.([]OperationMeta)
		if !ok {
			err = errors.New("invalid value, must be []OperationMeta")
			return
		}
		result.Operations = &tv
	case 1:
		tv, ok := value.(TransactionMetaV1)
		if !ok {
			err = errors.New("invalid value, must be TransactionMetaV1")
			return
		}
		result.V1 = &tv
	case 2:
		tv, ok := value.(TransactionMetaV2)
		if !ok {
			err = errors.New("invalid value, must be TransactionMetaV2")
			return
		}
		result.V2 = &tv
	case 3:
		tv, ok := value.(TransactionMetaV3)
		if !ok {
			err = errors.New("invalid value, must be TransactionMetaV3")
			return
		}
		result.V3 = &tv
	case 4:
		tv, ok := value.(TransactionMetaV4)
		if !ok {
			err = errors.New("invalid value, must be TransactionMetaV4")
			return
		}
		result.V4 = &tv
	case 5:
		tv, ok := value.(TransactionMetaV5)
		if !ok {
			err = errors.New("invalid value, must be TransactionMetaV5")
			return
		}
		result.V5 = &tv
	}
	return
}

// MustOperations retrieves the Operations value from the union,
// panicing if the value is not set.
func (u TransactionMeta) MustOperations() []OperationMeta {
	val, ok := u.GetOperations()

	if !ok {
		panic("arm Operations is not set")
	}

	return val
}

// GetOperations retrieves the Operations value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u TransactionMeta) GetOperations() (result []OperationMeta, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.V))

	if armName == "Operations" {
		result = *u.Operations
		ok = true
	}

	return
}

// MustV1 retrieves the V1 value from the union,
// panicing if the value is not set.
func (u TransactionMeta) MustV1() TransactionMetaV1 {
	val, ok := u.GetV1()

	if !ok {
		panic("arm V1 is not set")
	}

	return val
}

// GetV1 retrieves the V1 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u TransactionMeta) GetV1() (result TransactionMetaV1, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.V))

	if armName == "V1" {
		result = *u.V1
		ok = true
	}

	return
}

// MustV2 retrieves the V2 value from the union,
// panicing if the value is not set.
func (u TransactionMeta) MustV2() TransactionMetaV2 {
	val, ok := u.GetV2()

	if !ok {
		panic("arm V2 is not set")
	}

	return val
}

// GetV2 retrieves the V2 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u TransactionMeta) GetV2() (result TransactionMetaV2, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.V))

	if armName == "V2" {
		result = *u.V2
		ok = true
	}

	return
}

// MustV3 retrieves the V3 value from the union,
// panicing if the value is not set.
func (u TransactionMeta) MustV3() TransactionMetaV3 {
	val, ok := u.GetV3()

	if !ok {
		panic("arm V3 is not set")
	}

	return val
}

// GetV3 retrieves the V3 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u TransactionMeta) GetV3() (result TransactionMetaV3, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.V))

	if armName == "V3" {
		result = *u.V3
		ok = true
	}

	return
}

// MustV4 retrieves the V4 value from the union,
// panicing if the value is not set.
func (u TransactionMeta) MustV4() TransactionMetaV4 {
	val, ok := u.GetV4()

	if !ok {
		panic("arm V4 is not set")
	}

	return val
}

// GetV4 retrieves the V4 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u TransactionMeta) GetV4() (result TransactionMetaV4, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.V))

	if armName == "V4" {
		result = *u.V4
		ok = true
	}

	return
}

// MustV5 retrieves the V5 value from the union,
// panicing if the value is not set.
func (u TransactionMeta) MustV5() TransactionMetaV5 {
	val, ok := u.GetV5()

	if !ok {
		panic("arm V5 is not set")
	}

	return val
}

// GetV5 retrieves the V5 value from the union,
// returning ok if the union's switch indicated the value is valid.
func (u TransactionMeta) GetV5() (result TransactionMetaV5, ok bool) {
	armName, _ := u.ArmForSwitch(int32(u.V))

	if armName == "V5" {
		result = *u.V5
		ok = true
	}

	return
}

// EncodeTo encodes this value using the Encoder.
func (u TransactionMeta) EncodeTo(e *xdr.Encoder) error {
	var err error
	if _, err = e.EncodeInt(int32(u.V)); err != nil {
		return err
	}
	switch int32(u.V) {
	case 0:
		if _, err = e.EncodeUint(uint32(len((*u.Operations)))); err != nil {
			return err
		}
		for i := 0; i < len((*u.Operations)); i++ {
			if err = (*u.Operations)[i].EncodeTo(e); err != nil {
				return err
			}
		}
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
	case 3:
		if err = (*u.V3).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case 4:
		if err = (*u.V4).EncodeTo(e); err != nil {
			return err
		}
		return nil
	case 5:
		if err = (*u.V5).EncodeTo(e); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("V (int32) switch value '%d' is not valid for union TransactionMeta", u.V)
}

var _ decoderFrom = (*TransactionMeta)(nil)

// DecodeFrom decodes this value using the Decoder.
func (u *TransactionMeta) DecodeFrom(d *xdr.Decoder, maxDepth uint) (int, error) {
	if maxDepth == 0 {
		return 0, fmt.Errorf("decoding TransactionMeta: %w", ErrMaxDecodingDepthReached)
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
		u.Operations = new([]OperationMeta)
		var l uint32
		l, nTmp, err = d.DecodeUint()
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding OperationMeta: %w", err)
		}
		(*u.Operations) = nil
		if l > 0 {
			if il, ok := d.InputLen(); ok && uint(il) < uint(l) {
				return n, fmt.Errorf("decoding OperationMeta: length (%d) exceeds remaining input length (%d)", l, il)
			}
			(*u.Operations) = make([]OperationMeta, l)
			for i := uint32(0); i < l; i++ {
				nTmp, err = (*u.Operations)[i].DecodeFrom(d, maxDepth)
				n += nTmp
				if err != nil {
					return n, fmt.Errorf("decoding OperationMeta: %w", err)
				}
			}
		}
		return n, nil
	case 1:
		u.V1 = new(TransactionMetaV1)
		nTmp, err = (*u.V1).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding TransactionMetaV1: %w", err)
		}
		return n, nil
	case 2:
		u.V2 = new(TransactionMetaV2)
		nTmp, err = (*u.V2).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding TransactionMetaV2: %w", err)
		}
		return n, nil
	case 3:
		u.V3 = new(TransactionMetaV3)
		nTmp, err = (*u.V3).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding TransactionMetaV3: %w", err)
		}
		return n, nil
	case 4:
		u.V4 = new(TransactionMetaV4)
		nTmp, err = (*u.V4).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding TransactionMetaV4: %w", err)
		}
		return n, nil
	case 5:
		u.V5 = new(TransactionMetaV5)
		nTmp, err = (*u.V5).DecodeFrom(d, maxDepth)
		n += nTmp
		if err != nil {
			return n, fmt.Errorf("decoding TransactionMetaV5: %w", err)
		}
		return n, nil
	}
	return n, fmt.Errorf("union TransactionMeta has invalid V (int32) switch value '%d'", u.V)
}

// MarshalBinary implements encoding.BinaryMarshaler.
func (s TransactionMeta) MarshalBinary() ([]byte, error) {
	b := bytes.Buffer{}
	e := xdr.NewEncoder(&b)
	err := s.EncodeTo(e)
	return b.Bytes(), err
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (s *TransactionMeta) UnmarshalBinary(inp []byte) error {
	r := bytes.NewReader(inp)
	o := xdr.DefaultDecodeOptions
	o.MaxInputLen = len(inp)
	d := xdr.NewDecoderWithOptions(r, o)
	_, err := s.DecodeFrom(d, o.MaxDepth)
	return err
}

var (
	_ encoding.BinaryMarshaler   = (*TransactionMeta)(nil)
	_ encoding.BinaryUnmarshaler = (*TransactionMeta)(nil)
)

// xdrType signals that this type represents XDR values defined by this package.
func (s TransactionMeta) xdrType() {}

var _ xdrType = (*TransactionMeta)(nil)
