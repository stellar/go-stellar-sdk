package xdr

import (
	"fmt"
	"math"
)

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

// CloseTimeMs returns the ledger close time in milliseconds since the Unix
// epoch, reading whichever arm of StellarValue.ext carries it.
//
// SIGNED_MS and EMPTY_TX_SET_MS (CAP-0088) carry a millisecond close time
// directly. The older arms BASIC, SIGNED and EMPTY_TX_SET carry whole seconds
// in StellarValue.closeTime, which this scales by 1000, so the value is
// populated across all of history and every pre-Protocol-30 ledger returns a
// multiple of 1000. Top-level closeTime is unchanged on every arm, so callers
// that want whole seconds keep reading it directly.
//
// An arm this package does not recognise is an error rather than a fall back
// to whole seconds. A future millisecond arm that silently returned
// closeTime*1000 would be wrong by up to 999ms, would be indistinguishable
// from a genuine pre-Protocol-30 value, and would leave nothing in the result
// to show that precision had been lost.
func (s StellarValue) CloseTimeMs() (int64, error) {
	switch s.Ext.V {
	case StellarValueTypeStellarValueBasic,
		StellarValueTypeStellarValueSigned,
		StellarValueTypeStellarValueEmptyTxSet:
		return closeTimeSecondsToMs(s.CloseTime)

	case StellarValueTypeStellarValueSignedMs:
		if s.Ext.SignedMsValue == nil {
			return 0, fmt.Errorf("STELLAR_VALUE_SIGNED_MS with no signedMsValue")
		}
		return closeTimeMsToInt64(s.Ext.SignedMsValue.CloseTimeMs)

	case StellarValueTypeStellarValueEmptyTxSetMs:
		if s.Ext.ProposedMsValue == nil {
			return 0, fmt.Errorf("STELLAR_VALUE_EMPTY_TX_SET_MS with no proposedMsValue")
		}
		return closeTimeMsToInt64(s.Ext.ProposedMsValue.CloseTimeMs)

	default:
		return 0, fmt.Errorf("unknown StellarValue.ext arm: %d", s.Ext.V)
	}
}

func closeTimeSecondsToMs(t TimePoint) (int64, error) {
	if uint64(t) > uint64(math.MaxInt64)/1000 {
		return 0, fmt.Errorf("close time out of range: %d", uint64(t))
	}
	return int64(t) * 1000, nil //nolint:gosec // bounded above
}

func closeTimeMsToInt64(t TimePointMs) (int64, error) {
	if uint64(t) > uint64(math.MaxInt64) {
		return 0, fmt.Errorf("close time in ms out of range: %d", uint64(t))
	}
	return int64(t), nil //nolint:gosec // bounded above
}
