package xdr

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"

	"github.com/stretchr/testify/assert"
)

func TestClaimPredicateJSON(t *testing.T) {
	unconditional := &ClaimPredicate{
		Type: ClaimPredicateTypeClaimPredicateUnconditional,
	}
	relBefore := Int64(12)
	absBefore := Int64(1598440539)

	source := ClaimPredicate{
		Type: ClaimPredicateTypeClaimPredicateAnd,
		AndPredicates: &[]ClaimPredicate{
			{
				Type: ClaimPredicateTypeClaimPredicateOr,
				OrPredicates: &[]ClaimPredicate{
					{
						Type:      ClaimPredicateTypeClaimPredicateBeforeRelativeTime,
						RelBefore: &relBefore,
					},
					{
						Type:      ClaimPredicateTypeClaimPredicateBeforeAbsoluteTime,
						AbsBefore: &absBefore,
					},
				},
			},
			{
				Type:         ClaimPredicateTypeClaimPredicateNot,
				NotPredicate: &unconditional,
			},
		},
	}

	serialized, err := json.Marshal(source)
	assert.NoError(t, err)
	assert.JSONEq(
		t,
		`{"and":[{"or":[{"rel_before":"12"},{"abs_before":"2020-08-26T11:15:39Z","abs_before_epoch":"1598440539"}]},{"not":{"unconditional":true}}]}`,
		string(serialized),
	)

	var parsed ClaimPredicate
	assert.NoError(t, json.Unmarshal(serialized, &parsed))

	var serializedBase64, parsedBase64 string
	serializedBase64, err = MarshalBase64(source)
	assert.NoError(t, err)

	parsedBase64, err = MarshalBase64(parsed)
	assert.NoError(t, err)

	assert.Equal(t, serializedBase64, parsedBase64)
}

func TestRandClaimPredicateJSON(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := 0; i < 10000; i++ {
		cp := &ClaimPredicate{}
		shape := &gxdr.ClaimPredicate{}
		gen.Next(
			shape,
			[]randxdr.Preset{
				{Selector: randxdr.IsPtr, Setter: randxdr.SetPtr(true)},
			},
		)
		assert.NoError(t, gxdr.Convert(shape, cp))

		serializedJSON, err := json.Marshal(cp)
		assert.NoError(t, err)

		serializedBytes, err := cp.MarshalBinary()
		assert.NoError(t, err)

		var parsed ClaimPredicate
		assert.NoError(t, json.Unmarshal(serializedJSON, &parsed))
		parsedBin, err := parsed.MarshalBinary()
		assert.NoError(t, err)

		assert.True(t, bytes.Equal(serializedBytes, parsedBin))
	}
}

func TestAbsBeforeTimestamps(t *testing.T) {
	const year = 365 * 24 * 60 * 60
	for _, testCase := range []struct {
		unix     int64
		expected string
	}{
		{
			0,
			`{"abs_before":"1970-01-01T00:00:00Z","abs_before_epoch":"0"}`,
		},
		{
			900 * year,
			`{"abs_before":"2869-05-27T00:00:00Z","abs_before_epoch":"28382400000"}`,
		},
		{
			math.MaxInt64,
			`{"abs_before":"+292277026596-12-04T15:30:07Z","abs_before_epoch":"9223372036854775807"}`,
		},
		{
			-10,
			`{"abs_before":"1969-12-31T23:59:50Z","abs_before_epoch":"-10"}`,
		},
		{
			-9000 * year,
			`{"abs_before":"-7025-12-23T00:00:00Z","abs_before_epoch":"-283824000000"}`,
		},
		{
			math.MinInt64,
			// this serialization doesn't make sense but at least it doesn't crash the marshaller
			`{"abs_before":"+292277026596-12-04T15:30:08Z","abs_before_epoch":"-9223372036854775808"}`,
		},
	} {
		xdrSec := Int64(testCase.unix)
		source := ClaimPredicate{
			Type:      ClaimPredicateTypeClaimPredicateBeforeAbsoluteTime,
			AbsBefore: &xdrSec,
		}

		serialized, err := json.Marshal(source)
		assert.NoError(t, err)
		assert.JSONEq(t, testCase.expected, string(serialized))

		var parsed ClaimPredicate
		assert.NoError(t, json.Unmarshal(serialized, &parsed))
		assert.Equal(t, *parsed.AbsBefore, *source.AbsBefore)
	}
}

func TestISO8601Time_UnmarshalJSON(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		timestamp      string
		expectedParsed iso8601Time
		expectedError  string
	}{
		{
			"null timestamp",
			"null",
			iso8601Time{},
			"",
		},
		{
			"empty string",
			"",
			iso8601Time{},
			"unexpected end of JSON input",
		},
		{
			"not string",
			"1",
			iso8601Time{},
			"json: cannot unmarshal number into Go value of type string",
		},
		{
			"does not begin with double quotes",
			"'1\"",
			iso8601Time{},
			"invalid character '\\'' looking for beginning of value",
		},
		{
			"does not end with double quotes",
			"\"1",
			iso8601Time{},
			"unexpected end of JSON input",
		},
		{
			"could not extract time",
			"\"2006-01-02aldfd\"",
			iso8601Time{},
			"Could not extract time: parsing time \"2006-01-02aldfd\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"aldfd\" as \"T\"",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ts := &iso8601Time{}
			err := ts.UnmarshalJSON([]byte(testCase.timestamp))
			if len(testCase.expectedError) == 0 {
				assert.NoError(t, err)
				assert.Equal(t, *ts, testCase.expectedParsed)
			} else {
				assert.EqualError(t, err, testCase.expectedError)
			}
		})
	}
}
