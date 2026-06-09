package contract

import (
	"github.com/stellar/go-stellar-sdk/support/collections/set"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// addressCredentials returns the SorobanAddressCredentials embedded in an
// Address or AddressV2 credential, or nil otherwise.
func addressCredentials(c xdr.SorobanCredentials) *xdr.SorobanAddressCredentials {
	switch c.Type {
	case xdr.SorobanCredentialsTypeSorobanCredentialsAddress:
		return c.Address
	case xdr.SorobanCredentialsTypeSorobanCredentialsAddressV2:
		return c.AddressV2
	default:
		return nil
	}
}

// IsReadCall reports whether the simulated transaction is a read-only view
// call: simulation returned no address-credentialed authorization entries
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
	// Any entry not authorized by the source account itself means signatures
	// are required.
	for _, entry := range a.AuthEntries {
		if entry.Credentials.Type != xdr.SorobanCredentialsTypeSorobanCredentialsSourceAccount {
			return false
		}
	}
	// A non-empty ReadWrite footprint means the call writes state.
	if a.op == nil || a.op.Ext.SorobanData == nil {
		return false
	}
	return len(a.op.Ext.SorobanData.Resources.Footprint.ReadWrite) == 0
}

// NeedsNonSourceAccountSigningBy returns addresses, other than the invoker/source
// account used for simulation, whose address-credentialed authorization entries
// (Address or AddressV2) still need signatures before the transaction can be
// submitted. SourceAccount credentials are omitted
// because they are authorized by the envelope signature itself, and
// already-signed entries are omitted. The result is deduplicated and preserves
// first-seen order.
//
// It returns nil before Simulate has run.
func (a *AssembledTransaction) NeedsNonSourceAccountSigningBy() []xdr.ScAddress {
	if a == nil || a.Simulation == nil {
		return nil
	}
	var addrs []xdr.ScAddress
	seen := set.NewSet[string](len(a.AuthEntries))
	for _, entry := range a.AuthEntries {
		creds := addressCredentials(entry.Credentials)
		if creds == nil || creds.Signature.Type != xdr.ScValTypeScvVoid {
			continue
		}
		addr := creds.Address
		key, err := addr.String()
		if err != nil {
			// Unexpected for a well-formed address, but never silently drop a
			// required signer: include it without participating in dedup.
			addrs = append(addrs, addr)
			continue
		}
		if seen.Contains(key) {
			continue
		}
		seen.Add(key)
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
		creds := addressCredentials(entry.Credentials)
		if creds != nil && creds.Address.Type == xdr.ScAddressTypeScAddressTypeContract {
			return true
		}
	}
	return false
}
