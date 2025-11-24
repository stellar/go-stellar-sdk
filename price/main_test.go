package price

import (
	"math"
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
)

var Tests = []struct {
	S string
	P xdr.Price
	V bool
}{
	{"0.1", xdr.Price{1, 10}, true},
	{"0.01", xdr.Price{1, 100}, true},
	{"0.001", xdr.Price{1, 1000}, true},
	{"543.017930", xdr.Price{54301793, 100000}, true},
	{"319.69983", xdr.Price{31969983, 100000}, true},
	{"0.93", xdr.Price{93, 100}, true},
	{"0.5", xdr.Price{1, 2}, true},
	{"1.730", xdr.Price{173, 100}, true},
	{"0.85334384", xdr.Price{5333399, 6250000}, true},
	{"5.5", xdr.Price{11, 2}, true},
	{"2.72783", xdr.Price{272783, 100000}, true},
	{"638082.0", xdr.Price{638082, 1}, true},
	{"2.93850088", xdr.Price{36731261, 12500000}, true},
	{"58.04", xdr.Price{1451, 25}, true},
	{"41.265", xdr.Price{8253, 200}, true},
	{"5.1476", xdr.Price{12869, 2500}, true},
	{"95.14", xdr.Price{4757, 50}, true},
	{"0.74580", xdr.Price{3729, 5000}, true},
	{"4119.0", xdr.Price{4119, 1}, true},

	// Expensive inputs:
	{strings.Repeat("1", 22), xdr.Price{}, false},
	{strings.Repeat("1", 1000000), xdr.Price{}, false},
	{"0." + strings.Repeat("1", 1000000), xdr.Price{}, false},
	{"1E9223372036854775807", xdr.Price{}, false},
	{"1e9223372036854775807", xdr.Price{}, false},
}

func TestParse(t *testing.T) {
	for _, v := range Tests {
		o, err := Parse(v.S)
		if v.V && err != nil {
			t.Errorf("Couldn't parse %s: %v+", v.S, err)
			continue
		}

		o, err = Parse(v.S)
		if !v.V && err == nil {
			t.Errorf("expected err for input %s", v.S)
			continue
		}

		if o.N != v.P.N || o.D != v.P.D {
			t.Errorf("%s parsed to %d, not %d", v.S, o, v.P)
		}
	}

	_, err := Parse("0.0000000003")
	if err == nil {
		t.Error("Expected error")
	}

	_, err = Parse("2147483649")
	if err == nil {
		t.Error("Expected error")
	}
}

func TestStringFromFloat64(t *testing.T) {

	tests := map[float64]string{
		0:         "0.0000000",
		0.0000001: "0.0000001",
		1.0000001: "1.0000001",
		123:       "123.0000000",
	}

	for f, s := range tests {
		assert.Equal(t, s, StringFromFloat64(f))
	}
}

func TestConvertToBuyingUnits(t *testing.T) {
	testCases := []struct {
		sellingOfferAmount int64
		sellingUnitsNeeded int64
		pricen             int64
		priced             int64
		wantBuyingUnits    int64
		wantSellingUnits   int64
	}{
		{7, 2, 3, 7, 1, 2},
		{math.MaxInt64, 2, 3, 7, 1, 2},
		{20, 20, 1, 4, 5, 20},
		{20, 100, 1, 4, 5, 20},
		{20, 20, 7, 11, 13, 19},
		{20, 20, 11, 7, 32, 20},
		{20, 100, 7, 11, 13, 19},
		{20, 100, 11, 7, 32, 20},
		{1, 0, 3, 7, 0, 0},
		{1, 0, 7, 3, 0, 0},
		{math.MaxInt64, 0, 3, 7, 0, 0},
	}
	for _, kase := range testCases {
		t.Run(t.Name(), func(t *testing.T) {
			buyingUnits, sellingUnits, e := ConvertToBuyingUnits(kase.sellingOfferAmount, kase.sellingUnitsNeeded, kase.pricen, kase.priced)
			if !assert.Nil(t, e) {
				return
			}
			assert.Equal(t, kase.wantBuyingUnits, buyingUnits)
			assert.Equal(t, kase.wantSellingUnits, sellingUnits)
		})
	}
}

func TestMulFractionOverflow(t *testing.T) {
	_, e := MulFractionRoundDown(math.MaxInt64/2+1, 2, 1)
	if e != ErrOverflow {
		t.Fatal("expected overflow error")
	}

	_, e = mulFractionRoundUp(math.MaxInt64/2+1, 2, 1)
	if e != ErrOverflow {
		t.Fatal("expected overflow error")
	}
}
