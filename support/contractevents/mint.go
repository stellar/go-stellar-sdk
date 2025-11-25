package contractevents

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var ErrNotMintEvent = errors.New("event is not a valid 'mint' event")

type MintEvent struct {
	sacEvent

	Admin  string
	To     string
	Amount xdr.Int128Parts
}

// parseMintEvent tries to parse the given topics and value as a SAC "mint"
// event.
//
// Internally, it assumes that the `topics` array has already validated both the
// function name AND the asset <--> contract ID relationship. It will return a
// best-effort parsing even in error cases.
func (event *MintEvent) parse(topics xdr.ScVec, value xdr.ScVal) error {
	//
	// The mint event format is:
	//
	// 	"mint"  	Symbol
	//  <admin>		Address
	//  <to> 		Address
	// 	<asset>		Bytes
	//
	// 	<amount> 	i128
	//
	var err error
	event.Admin, event.To, event.Amount, err = parseBalanceChangeEvent(topics, value)
	if err != nil {
		return ErrNotMintEvent
	}
	return nil
}
