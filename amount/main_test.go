package amount_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/amount"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var Tests = []struct {
	S     string
	I     xdr.Int64
	valid bool
}{
	{"100.0000000", 1000000000, true},
	{"-100.0000000", -1000000000, true},
	{"100.0000001", 1000000001, true},
	{"123.0000001", 1230000001, true},
	{"123.00000001", 0, false},
	{"922337203685.4775807", 9223372036854775807, true},
	{"922337203685.4775808", 0, false},
	{"922337203686", 0, false},
	{"-922337203685.4775808", -9223372036854775808, true},
	{"-922337203685.4775809", 0, false},
	{"-922337203686", 0, false},
	{"1000000000000.0000000", 0, false},
	{"1000000000000", 0, false},
	{"-0.5000000", -5000000, true},
	{"0.5000000", 5000000, true},
	{"0.12345678", 0, false},
	// Expensive inputs:
	{strings.Repeat("1", 1000000), 0, false},
	{"1E9223372036854775807", 0, false},
	{"1e9223372036854775807", 0, false},
	{"Inf", 0, false},
}

func TestParse(t *testing.T) {
	for _, v := range Tests {
		o, err := amount.Parse(v.S)
		if !v.valid && err == nil {
			t.Errorf("expected err for input %s", v.S)
			continue
		}
		if v.valid && err != nil {
			t.Errorf("couldn't parse %s: %v", v.S, err)
			continue
		}

		if o != v.I {
			t.Errorf("%s parsed to %d, not %d", v.S, o, v.I)
		}
	}
}

func TestString(t *testing.T) {
	for _, v := range Tests {
		if !v.valid {
			continue
		}

		o := amount.String(v.I)

		if o != v.S {
			t.Errorf("%d stringified to %s, not %s", v.I, o, v.S)
		}
	}
}

func TestIntStringToAmount(t *testing.T) {
	var testCases = []struct {
		Output string
		Input  string
		Valid  bool
	}{
		{"100.0000000", "1000000000", true},
		{"-100.0000000", "-1000000000", true},
		{"100.0000001", "1000000001", true},
		{"123.0000001", "1230000001", true},
		{"922337203685.4775807", "9223372036854775807", true},
		{"922337203685.4775808", "9223372036854775808", true},
		{"92233.7203686", "922337203686", true},
		{"-922337203685.4775808", "-9223372036854775808", true},
		{"-922337203685.4775809", "-9223372036854775809", true},
		{"-92233.7203686", "-922337203686", true},
		{"1000000000000.0000000", "10000000000000000000", true},
		{"0.0000000", "0", true},
		// Expensive inputs when using big.Rat:
		{"10000000000000.0000000", "1" + strings.Repeat("0", 20), true},
		{"-10000000000000.0000000", "-1" + strings.Repeat("0", 20), true},
		{"1" + strings.Repeat("0", 1000-7) + ".0000000", "1" + strings.Repeat("0", 1000), true},
		{"1" + strings.Repeat("0", 1000000-7) + ".0000000", "1" + strings.Repeat("0", 1000000), true},
		// Invalid inputs
		{"", "nan", false},
		{"", "", false},
		{"", "-", false},
		{"", "1E9223372036854775807", false},
		{"", "1e9223372036854775807", false},
		{"", "Inf", false},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s to %s (valid = %t)", tc.Input, tc.Output, tc.Valid), func(t *testing.T) {
			o, err := amount.IntStringToAmount(tc.Input)

			if !tc.Valid && err == nil {
				t.Errorf("expected err for input %s (output: %s)", tc.Input, tc.Output)
				return
			}
			if tc.Valid && err != nil {
				t.Errorf("couldn't parse %s: %v", tc.Input, err)
				return
			}

			if o != tc.Output {
				t.Errorf("%s converted to %s, not %s", tc.Input, o, tc.Output)
			}
		})
	}

}
