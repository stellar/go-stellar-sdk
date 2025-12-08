package contractevents

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var ErrNotClawbackEvent = errors.New("event is not a valid 'clawback' event")

type ClawbackEvent struct {
	sacEvent

	Admin  string
	From   string
	Amount xdr.Int128Parts
}

// parseClawbackEvent tries to parse the given topics and value as a SAC
// "clawback" event.
//
// Internally, it assumes that the `topics` array has already validated both the
// function name AND the asset <--> contract ID relationship. It will return a
// best-effort parsing even in error cases.
func (event *ClawbackEvent) parse(topics xdr.ScVec, value xdr.ScVal) error {
	//
	// The clawback event format is:
	//
	// 	"clawback" 	Symbol
	//  <admin>		Address
	//  <from> 		Address
	// 	<asset>		Bytes
	//
	// 	<amount> 	i128
	//
	var err error
	event.Admin, event.From, event.Amount, err = parseBalanceChangeEvent(topics, value)
	if err != nil {
		return ErrNotClawbackEvent
	}
	return nil
}
