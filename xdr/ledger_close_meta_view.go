package xdr

// Convenience helpers on LedgerCloseMetaView that decode just the header
// fields commonly needed for streaming validation, without the full XDR
// decode of the body.
//
// Byte-sequence accessors (e.g., LedgerHash) return slices into the source
// bytes — zero-copy, but the slice pins the source bytes alive. Callers
// that need to hold the value past the source's lifetime should copy it
// into a fixed-size type themselves.

func (v LedgerCloseMetaView) ledgerHeaderHistoryEntry() (LedgerHeaderHistoryEntryView, error) {
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
	header, err := v.ledgerHeaderHistoryEntry()
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
	header, err := v.ledgerHeaderHistoryEntry()
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
	header, err := v.ledgerHeaderHistoryEntry()
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
	header, err := v.ledgerHeaderHistoryEntry()
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
