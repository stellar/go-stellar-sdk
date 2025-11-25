// Package examplehorizonclient provides a dummy client for use with the GoDoc examples.
package examplehorizonclient

import (
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
)

// AccountRequest is a simple mock
type AccountRequest struct {
	AccountID string
}

// Client is a simple mock
type Client struct {
}

// DefaultTestNetClient is a simple mock
var DefaultTestNetClient = Client{}

// AccountDetail returns a minimal, static Account object
func (client *Client) AccountDetail(req AccountRequest) (hProtocol.Account, error) {
	return hProtocol.Account{
		AccountID: req.AccountID,
		Sequence:  3556091187167235,
	}, nil
}

// FeeStats returns mock network fee information
func (client *Client) FeeStats() (hProtocol.FeeStats, error) {
	return hProtocol.FeeStats{
		LastLedger:          22606298,
		LastLedgerBaseFee:   100,
		LedgerCapacityUsage: 0.97,
		MaxFee: hProtocol.FeeDistribution{
			Max:  100,
			Min:  100,
			Mode: 200,
			P10:  250,
			P20:  300,
			P30:  350,
			P40:  500,
			P50:  600,
			P60:  700,
			P70:  800,
			P80:  900,
			P90:  2000,
			P95:  3000,
			P99:  5000,
		},
		FeeCharged: hProtocol.FeeDistribution{
			Max:  100,
			Min:  100,
			Mode: 100,
			P10:  100,
			P20:  100,
			P30:  100,
			P40:  100,
			P50:  100,
			P60:  100,
			P70:  100,
			P80:  100,
			P90:  100,
			P95:  100,
			P99:  100,
		},
	}, nil
}
