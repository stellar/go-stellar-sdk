package xdr

// LedgerCloseValueSignature returns the close-value signature from whichever
// StellarValue arm carries it: SIGNED, EMPTY_TX_SET (CAP-0083), or their
// millisecond close-time counterparts SIGNED_MS and EMPTY_TX_SET_MS
// (CAP-0088). Only BASIC has no signature.
//
// The generated GetLcValueSignature only reads the SIGNED arm, so it finds no
// signature on ledgers closed with an empty transaction set, or on any ledger
// after the Protocol 30 upgrade.
func (e StellarValueExt) LedgerCloseValueSignature() (LedgerCloseValueSignature, bool) {
	if sig, ok := e.GetLcValueSignature(); ok {
		return sig, true
	}
	if v, ok := e.GetProposedValue(); ok {
		return v.LcValueSignature, true
	}
	if v, ok := e.GetSignedMsValue(); ok {
		return v.LcValueSignature, true
	}
	if v, ok := e.GetProposedMsValue(); ok {
		return v.LcValueSignature, true
	}
	return LedgerCloseValueSignature{}, false
}
