package contractevents

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var ErrNotBalanceChangeEvent = errors.New("event doesn't represent a balance change")

// parseBalanceChangeEvent is a generalization of a subset of the Stellar Asset
// Contract events. Transfer, mint, clawback, and burn events all have two
// addresses and an amount involved. The addresses represent different things in
// different event types (e.g. "from" or "admin"), but the parsing is identical.
// This helper extracts all three parts or returns a generic error if it can't.
func parseBalanceChangeEvent(topics xdr.ScVec, value xdr.ScVal) (
	first string,
	second string,
	amount xdr.Int128Parts,
	err error,
) {
	err = ErrNotBalanceChangeEvent
	if len(topics) != 4 {
		return
	}

	firstSc, ok := topics[1].GetAddress()
	if !ok {
		return
	}
	first, err = firstSc.String()
	if err != nil {
		err = errors.Wrap(err, ErrNotBalanceChangeEvent.Error())
		return
	}

	secondSc, ok := topics[2].GetAddress()
	if !ok {
		return
	}
	second, err = secondSc.String()
	if err != nil {
		err = errors.Wrap(err, ErrNotBalanceChangeEvent.Error())
		return
	}

	amount, ok = eventAmount(value)
	if !ok {
		return first, second, amount, ErrNotBalanceChangeEvent
	}

	return first, second, amount, nil
}

// eventAmount reads the amount from SAC event data. The data is either a bare
// i128, or (CAP-0067, and CAP-0084 for muxed contract destinations) a map
// whose "amount" key holds the i128, alongside keys such as "to_muxed_id".
func eventAmount(value xdr.ScVal) (xdr.Int128Parts, bool) {
	if amount, ok := value.GetI128(); ok {
		return amount, true
	}
	entries, ok := value.GetMap()
	if !ok || entries == nil {
		return xdr.Int128Parts{}, false
	}
	for _, entry := range *entries {
		if key, ok := entry.Key.GetSym(); ok && key == "amount" {
			return entry.Val.GetI128()
		}
	}
	return xdr.Int128Parts{}, false
}
