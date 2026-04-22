package randxdr

import (
	"math"
	"regexp"
	"strings"

	goxdr "github.com/xdrpp/goxdr/xdr"
)

// Selector is function used to match fields of a goxdr.XdrType
type Selector func(string, goxdr.XdrType) bool

// Setter is a function used to set field values for a goxdr.XdrType
type Setter func(*randMarshaller, string, goxdr.XdrType)

// Preset can be used to restrict values for specific fields of a goxdr.XdrType.
type Preset struct {
	Selector Selector
	Setter   Setter
}

// FieldEquals returns a Selector which matches on a field name by equality
func FieldEquals(toMatch string) Selector {
	return func(name string, xdrType goxdr.XdrType) bool {
		return name == toMatch
	}
}

// FieldMatches returns a Selector which matches on a field name by regexp
func FieldMatches(r *regexp.Regexp) Selector {
	return func(name string, xdrType goxdr.XdrType) bool {
		return r.MatchString(name)
	}
}

// And is a Selector which returns true if the given pair of selectors
// match the field.
func And(a, b Selector) Selector {
	return func(s string, xdrType goxdr.XdrType) bool {
		return a(s, xdrType) && b(s, xdrType)
	}
}

// IsPtr is a Selector which matches on all XDR pointer fields
var IsPtr Selector = func(name string, xdrType goxdr.XdrType) bool {
	_, ok := goxdr.XdrBaseType(xdrType).(goxdr.XdrPtr)
	return ok
}

// IsNestedInnerSet is a Selector which identifies nesting for the following xdr type:
//
//	struct SCPQuorumSet
//	{
//		uint32 threshold;
//		PublicKey validators<>;
//		SCPQuorumSet innerSets<>;
//	};
//
// supports things like: A,B,C,(D,E,F),(G,H,(I,J,K,L))
// only allows 2 levels of nesting
var IsNestedInnerSet Selector = func(name string, xdrType goxdr.XdrType) bool {
	if strings.HasSuffix(name, ".innerSets") && strings.Count(name, ".innerSets[") > 0 {
		_, ok := goxdr.XdrBaseType(xdrType).(goxdr.XdrVec)
		return ok
	}
	return false
}

// IsDeepAuthorizedInvocationTree is a Selector which identifies deep trees of the following xdr type:
//
//	struct AuthorizedInvocation
//	{
//		Hash contractID;
//		SCSymbol functionName;
//		SCVec args;
//		AuthorizedInvocation subInvocations<>;
//	};
//
// only allows trees of height up to 2
var IsDeepAuthorizedInvocationTree Selector = func(name string, xdrType goxdr.XdrType) bool {
	if strings.HasSuffix(name, "subInvocations") && strings.Count(name, ".subInvocations[") > 0 {
		_, ok := goxdr.XdrBaseType(xdrType).(goxdr.XdrVec)
		return ok
	}
	return false
}

// LedgerCloseMetaPresets is the canonical preset list for randxdr generators
// producing LedgerCloseMeta values. It collapses the two recursive subtrees
// (nested inner transaction sets, deep authorized-invocation trees) that
// would otherwise explode combinatorially under default generator parameters.
var LedgerCloseMetaPresets = []Preset{
	{Selector: IsNestedInnerSet, Setter: SetVecLen(0)},
	{Selector: IsDeepAuthorizedInvocationTree, Setter: SetVecLen(0)},
}

// SetPtr is a Setter which sets the xdr pointer to null if present is false
func SetPtr(present bool) Setter {
	return func(m *randMarshaller, name string, xdrType goxdr.XdrType) {
		p := goxdr.XdrBaseType(xdrType).(goxdr.XdrPtr)
		p.SetPresent(present)
		p.XdrMarshalValue(m, name)
	}
}

// SetVecLen returns a Setter which sets the length of a variable length
// array ( https://tools.ietf.org/html/rfc4506#section-4.13 ) to a fixed value
func SetVecLen(vecLen uint32) Setter {
	return func(x *randMarshaller, field string, xdrType goxdr.XdrType) {
		v := goxdr.XdrBaseType(xdrType).(goxdr.XdrVec)
		v.SetVecLen(vecLen)
		v.XdrMarshalN(x, field, vecLen)
	}
}

// SetU32 returns a Setter which sets a uint32 XDR field to a randomly selected
// element from vals
func SetU32(vals ...uint32) Setter {
	return func(x *randMarshaller, field string, xdrType goxdr.XdrType) {
		f := goxdr.XdrBaseType(xdrType).(goxdr.XdrNum32)
		f.SetU32(vals[x.rand.Intn(len(vals))])
	}
}

// SetPositiveNum64 returns a Setter which sets a uint64 XDR field to a random positive value
var SetPositiveNum64 Setter = func(x *randMarshaller, field string, xdrType goxdr.XdrType) {
	f := goxdr.XdrBaseType(xdrType).(goxdr.XdrNum64)
	f.SetU64(uint64(x.rand.Int63n(math.MaxInt64)))
}

// SetPositiveNum32 returns a Setter which sets a uint32 XDR field to a random positive value
var SetPositiveNum32 Setter = func(x *randMarshaller, field string, xdrType goxdr.XdrType) {

	f := goxdr.XdrBaseType(xdrType).(goxdr.XdrNum32)
	f.SetU32(uint32(x.rand.Int31n(math.MaxInt32)))
}

const alphaNumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// SetAssetCode returns a Setter which sets an asset code XDR field to a
// random alphanumeric string right-padded with 0 bytes
var SetAssetCode Setter = func(x *randMarshaller, field string, xdrType goxdr.XdrType) {
	f := goxdr.XdrBaseType(xdrType).(goxdr.XdrBytes)
	slice := f.GetByteSlice()
	var end int
	switch len(slice) {
	case 4:
		end = int(x.rand.Int31n(4))
	case 12:
		end = int(4 + x.rand.Int31n(8))
	}

	for i := 0; i <= end; i++ {
		slice[i] = alphaNumeric[x.rand.Int31n(int32(len(alphaNumeric)))]
	}
}

// SetPrintableASCII returns a Setter which sets a home domain string32 with a random
// printable ascii string
var SetPrintableASCII Setter = func(x *randMarshaller, field string, xdrType goxdr.XdrType) {
	f := goxdr.XdrBaseType(xdrType).(goxdr.XdrString)
	end := int(x.rand.Int31n(int32(f.Bound)))
	var text []byte
	for i := 0; i <= end; i++ {
		// printable ascii range is from 32 - 127
		printableChar := byte(32 + x.rand.Int31n(95))
		text = append(text, printableChar)
	}
	f.SetString(string(text))
}
