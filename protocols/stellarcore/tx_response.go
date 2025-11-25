package stellarcore

import (
	"github.com/stellar/go-stellar-sdk/xdr"
)

const (
	// TXStatusError represents the status value returned by stellar-core when an error occurred from
	// submitting a transaction
	TXStatusError = "ERROR"

	// TXStatusPending represents the status value returned by stellar-core when a transaction has
	// been accepted for processing
	TXStatusPending = "PENDING"

	// TXStatusDuplicate represents the status value returned by stellar-core when a submitted
	// transaction is a duplicate
	TXStatusDuplicate = "DUPLICATE"

	// TXStatusTryAgainLater represents the status value returned by stellar-core when a submitted
	// transaction was not included in the previous 4 ledgers and get banned for being added in the
	// next few ledgers.
	TXStatusTryAgainLater = "TRY_AGAIN_LATER"
)

// TXResponse represents the response returned from a submission request sent to stellar-core's /tx
// endpoint
type TXResponse struct {
	Exception string `json:"exception"`
	Error     string `json:"error"`
	Status    string `json:"status"`
	// DiagnosticEvents is an optional base64-encoded XDR Variable-Length Array of DiagnosticEvents
	DiagnosticEvents string `json:"diagnostic_events,omitempty"`
}

// IsException returns true if the response represents an exception response from stellar-core
func (resp *TXResponse) IsException() bool {
	return resp.Exception != ""
}

// DecodeDiagnosticEvents returns the decoded events
func DecodeDiagnosticEvents(events string) ([]xdr.DiagnosticEvent, error) {
	var ret []xdr.DiagnosticEvent
	if events == "" {
		return ret, nil
	}
	err := xdr.SafeUnmarshalBase64(events, &ret)
	if err != nil {
		return nil, err
	}
	return ret, err
}

// DiagnosticEventsToSlice transforms the base64 diagnostic events into a slice of individual
// base64-encoded diagnostic events
func DiagnosticEventsToSlice(events string) ([]string, error) {
	decoded, err := DecodeDiagnosticEvents(events)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(decoded))
	for i := 0; i < len(decoded); i++ {
		encoded, err := xdr.MarshalBase64(decoded[i])
		if err != nil {
			return nil, err
		}
		result[i] = encoded
	}
	return result, nil
}
