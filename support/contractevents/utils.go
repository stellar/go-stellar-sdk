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

	amount, ok = value.GetI128()
	if !ok {
		return
	}

	return first, second, amount, nil
}
