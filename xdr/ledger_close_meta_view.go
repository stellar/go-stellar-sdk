package xdr

import "fmt"

// Convenience helpers on LedgerCloseMetaView that decode just the header
// fields commonly needed for streaming validation, without the full XDR
// decode of the body.
//
// Byte-sequence accessors (e.g., LedgerHash) return slices into the source
// bytes — zero-copy, but the slice pins the source bytes alive. Callers
// that need to hold the value past the source's lifetime should copy it
// into a fixed-size type themselves.

// LedgerHeader returns a view of this ledger's LedgerHeaderHistoryEntry,
// resolving the LedgerCloseMeta version (V0/V1/V2) internally. It lets
// callers read header fields without decoding the rest of the meta.
func (v LedgerCloseMetaView) LedgerHeader() (LedgerHeaderHistoryEntryView, error) {
	value, err := v.V()
	if err != nil {
		return nil, err
	}
	switch value {
	case 0:
		v0, err := v.V0()
		if err != nil {
			return nil, err
		}
		return v0.LedgerHeader()
	case 1:
		v1, err := v.V1()
		if err != nil {
			return nil, err
		}
		return v1.LedgerHeader()
	case 2:
		v2, err := v.V2()
		if err != nil {
			return nil, err
		}
		return v2.LedgerHeader()
	default:
		return nil, viewErrUnknownDiscriminant(0, value)
	}
}

// LedgerSequence returns the sequence number of this LedgerCloseMeta.
func (v LedgerCloseMetaView) LedgerSequence() (uint32, error) {
	header, err := v.LedgerHeader()
	if err != nil {
		return 0, err
	}
	headerInner, err := header.Header()
	if err != nil {
		return 0, err
	}
	seqView, err := headerInner.LedgerSeq()
	if err != nil {
		return 0, err
	}
	seq, err := seqView.Value()
	if err != nil {
		return 0, err
	}
	return uint32(seq), nil
}

// LedgerCloseTime returns the close time (Unix seconds) of this
// LedgerCloseMeta, mirroring LedgerCloseMeta.LedgerCloseTime on the parsed
// type.
func (v LedgerCloseMetaView) LedgerCloseTime() (int64, error) {
	header, err := v.LedgerHeader()
	if err != nil {
		return 0, err
	}
	// The Must* accessors panic with *ViewError on the first malformed field
	// and Try recovers it, so the chain needs only one error check.
	ct, err := Try(func() uint64 {
		return header.MustHeader().MustScpValue().MustCloseTime().MustValue()
	})
	return int64(ct), err //nolint:gosec // TimePoint is uint64; real close times fit int64
}

// LedgerHash returns the 32-byte hash of the closed ledger as a slice into
// the source bytes. Zero copy; the slice is valid as long as the source
// LedgerCloseMetaView's bytes are.
func (v LedgerCloseMetaView) LedgerHash() ([]byte, error) {
	header, err := v.LedgerHeader()
	if err != nil {
		return nil, err
	}
	hashView, err := header.Hash()
	if err != nil {
		return nil, err
	}
	// Raw() returns the zero-copy []byte alias of the source; the fixed-opaque
	// Value() would return a [32]byte that escapes to the heap as a copy.
	return hashView.Raw()
}

// PreviousLedgerHash returns the 32-byte hash of the parent ledger as a
// slice into the source bytes. Zero copy; the slice is valid as long as
// the source LedgerCloseMetaView's bytes are.
func (v LedgerCloseMetaView) PreviousLedgerHash() ([]byte, error) {
	header, err := v.LedgerHeader()
	if err != nil {
		return nil, err
	}
	headerInner, err := header.Header()
	if err != nil {
		return nil, err
	}
	hashView, err := headerInner.PreviousLedgerHash()
	if err != nil {
		return nil, err
	}
	// Raw() returns the zero-copy []byte alias of the source; the fixed-opaque
	// Value() would return a [32]byte that escapes to the heap as a copy.
	return hashView.Raw()
}

// LedgerCloseTimeMs returns the close time in milliseconds since the Unix
// epoch. For every arm the decoder knows it returns the same value as
// StellarValue.CloseTimeMs on the parsed type, which a test asserts by reading
// the same bytes both ways. Pre-Protocol-30 ledgers carry whole seconds and
// return a multiple of 1000; an ext arm this package does not recognise is an
// error rather than a silent fall back to whole seconds.
//
// The two paths agree on values, not on error types. A discriminant the
// decoder has never heard of is rejected by StellarValueExtView.V as a
// *ViewError of kind ViewErrUnknownDiscriminant, where the parsed path returns
// a plain error from its default case. Treat any error here as "no close
// time" rather than matching on its type.
func (v LedgerCloseMetaView) LedgerCloseTimeMs() (int64, error) {
	header, err := v.LedgerHeader()
	if err != nil {
		return 0, err
	}

	// One Try around the whole read, as LedgerEntryView.LedgerKey does: the
	// Must* accessors panic with *ViewError on the first malformed field and
	// Try recovers it, so the chain needs a single error check.
	var arm StellarValueType
	raw, err := Try(func() uint64 {
		scpValue := header.MustHeader().MustScpValue()
		ext := scpValue.MustExt()
		arm = ext.MustV()
		switch arm {
		case StellarValueTypeStellarValueSignedMs:
			return ext.MustSignedMsValue().MustCloseTimeMs().MustValue()
		case StellarValueTypeStellarValueEmptyTxSetMs:
			return ext.MustProposedMsValue().MustCloseTimeMs().MustValue()
		default:
			// Whole-second arms, and any arm this package does not know, which
			// the switch below rejects. closeTime is a top-level field, so
			// reading it is safe whatever the arm turns out to be.
			return scpValue.MustCloseTime().MustValue()
		}
	})
	if err != nil {
		return 0, err
	}

	switch arm {
	case StellarValueTypeStellarValueSignedMs,
		StellarValueTypeStellarValueEmptyTxSetMs:
		return closeTimeMsToInt64(TimePointMs(raw))

	case StellarValueTypeStellarValueBasic,
		StellarValueTypeStellarValueSigned,
		StellarValueTypeStellarValueEmptyTxSet:
		return closeTimeSecondsToMs(TimePoint(raw))

	default:
		// Reachable only once the XDR is regenerated with a new arm:
		// StellarValueExtView.V rejects a discriminant it has never heard of
		// before this point, so anything arriving here is an arm the decoder
		// understands and this function has not been taught. That is exactly
		// when viewErrUnknownDiscriminant would mislead, by reporting an
		// unknown union discriminant for an arm the decoder knows, so name the
		// arm the way the parsed path does.
		return 0, fmt.Errorf("unknown StellarValue.ext arm: %d", arm)
	}
}
