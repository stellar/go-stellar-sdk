package contract

import (
	"github.com/stellar/go-stellar-sdk/xdr"
)

// IsReadCall reports whether the simulated transaction is a read-only view
// call: simulation returned no Address-credentialed authorization entries
// (only SourceAccount, or none at all), and the SorobanTransactionData
// footprint touches no read-write ledger keys. Such calls can be served
// directly from ReturnValue without submission.
//
// IsReadCall is conservative: when no simulation has been run yet (or the
// transaction carries no SorobanData), it returns false.
func (a *AssembledTransaction) IsReadCall() bool {
	if a == nil || a.Simulation == nil {
		return false
	}
	// Any Address-credentialed auth entry means signatures are required.
	for _, entry := range a.AuthEntries {
		if entry.Credentials.Type == xdr.SorobanCredentialsTypeSorobanCredentialsAddress {
			return false
		}
	}
	// A non-empty ReadWrite footprint means the call writes state.
	if a.op == nil || a.op.Ext.SorobanData == nil {
		return false
	}
	return len(a.op.Ext.SorobanData.Resources.Footprint.ReadWrite) == 0
}

// NeedsNonInvokerSigningBy returns addresses, other than the invoker/source
// account used for simulation, whose Address-credentialed authorization entries
// still need signatures before the transaction can be submitted. SourceAccount
// credentials are omitted because they are authorized by the envelope signature
// itself, and already-signed entries are omitted. The result is deduplicated
// and preserves first-seen order.
//
// Like IsReadCall, it is conservative: before Simulate has run (Simulation is
// nil) it returns nil.
func (a *AssembledTransaction) NeedsNonInvokerSigningBy() []xdr.ScAddress {
	if a == nil || a.Simulation == nil {
		return nil
	}
	var addrs []xdr.ScAddress
	seen := make(map[string]struct{})
	for _, entry := range a.AuthEntries {
		if entry.Credentials.Type != xdr.SorobanCredentialsTypeSorobanCredentialsAddress {
			continue
		}
		if entry.Credentials.Address == nil {
			continue
		}
		if entry.Credentials.Address.Signature.Type != xdr.ScValTypeScvVoid {
			continue
		}
		addr := entry.Credentials.Address.Address
		key, err := addr.String()
		if err != nil {
			// Unexpected for a well-formed address, but never silently drop a
			// required signer: include it without participating in dedup.
			addrs = append(addrs, addr)
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		addrs = append(addrs, addr)
	}
	return addrs
}

// RequiresEnforcingResimulation reports whether any recorded authorization
// entry is for a contract address (custom account / smart wallet). Contract
// addresses authorize via __check_auth, which recording-mode simulation does
// not run, so their footprint and resource fees require signing the entry and
// re-simulating in enforcing mode. Plain Ed25519 account-address signers do not
// need this.
func (a *AssembledTransaction) RequiresEnforcingResimulation() bool {
	if a == nil || a.Simulation == nil {
		return false
	}
	for _, entry := range a.AuthEntries {
		if entry.Credentials.Type == xdr.SorobanCredentialsTypeSorobanCredentialsAddress &&
			entry.Credentials.Address != nil &&
			entry.Credentials.Address.Address.Type == xdr.ScAddressTypeScAddressTypeContract {
			return true
		}
	}
	return false
}
