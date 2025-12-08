//go:build gofuzz
// +build gofuzz

package jsonclaimpredicate

import (
	"bytes"
	"encoding/json"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// Fuzz is go-fuzz function for fuzzing xdr.ClaimPredicate JSON
// marshaller and unmarshaller.
func Fuzz(data []byte) int {
	// Ignore malformed ClaimPredicate
	var p xdr.ClaimPredicate
	err := xdr.SafeUnmarshal(data, &p)
	if err != nil {
		return -1
	}

	// Ignore invalid predicates: (and/or length != 2, nil not)
	if !validate(p) {
		return -1
	}

	j, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}

	var p2 xdr.ClaimPredicate
	err = json.Unmarshal(j, &p2)
	if err != nil {
		panic(err)
	}

	j2, err := json.Marshal(p2)
	if err != nil {
		panic(err)
	}

	if !bytes.Equal(j, j2) {
		panic("not equal " + string(j) + " " + string(j2))
	}

	return 1
}

func validate(p xdr.ClaimPredicate) bool {
	switch p.Type {
	case xdr.ClaimPredicateTypeClaimPredicateUnconditional:
		return true
	case xdr.ClaimPredicateTypeClaimPredicateAnd:
		and := *p.AndPredicates
		if len(and) != 2 {
			return false
		}
		return validate(and[0]) && validate(and[1])
	case xdr.ClaimPredicateTypeClaimPredicateOr:
		or := *p.OrPredicates
		if len(or) != 2 {
			return false
		}
		return validate(or[0]) && validate(or[1])
	case xdr.ClaimPredicateTypeClaimPredicateNot:
		if *p.NotPredicate == nil {
			return false
		}
		return validate(**p.NotPredicate)
	case xdr.ClaimPredicateTypeClaimPredicateBeforeAbsoluteTime:
		return *p.AbsBefore >= 0
	case xdr.ClaimPredicateTypeClaimPredicateBeforeRelativeTime:
		return *p.RelBefore >= 0
	}

	panic("Invalid type")
}
