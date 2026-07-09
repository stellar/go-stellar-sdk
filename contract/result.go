package contract

import (
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Result returns the contract invocation's return value as a raw xdr.ScVal.
//
//   - For a read call (IsReadCall == true), Result returns the simulated
//     ReturnValue captured by Simulate.
//   - Before Simulate has run, Result returns an *Error matching
//     ErrNotYetSimulated.
//   - For a write call, the decodable result lives in the submitted
//     transaction's on-chain meta and requires the sign/send/poll path, which
//     is not yet supported; Result reports ErrNotYetSimulated.
//
// Result is read-only with respect to the AT's lifecycle state and may be
// called any number of times.
func (a *AssembledTransaction) Result() (any, error) {
	if a == nil {
		return nil, invalidArgsf("AssembledTransaction not initialized")
	}
	if a.Simulation == nil {
		return nil, &Error{Kind: KindNotYetSimulated, Details: "Result"}
	}

	if a.IsReadCall() {
		if a.ReturnValue == nil {
			// Soroban convention: void-returning fns omit the ScVal entirely.
			return xdr.ScVal{Type: xdr.ScValTypeScvVoid}, nil
		}
		return *a.ReturnValue, nil
	}

	// Write-call results come from the submitted tx's on-chain meta, which
	// requires the sign/send/poll path.
	return nil, &Error{
		Kind:    KindNotYetSimulated,
		Details: "Result: write-call results require submission (sign/send/poll)",
	}
}
