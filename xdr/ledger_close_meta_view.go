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
		return LedgerHeaderHistoryEntryView{}, err
	}
	switch value {
	case 0:
		v0, err := v.ArmV0()
		if err != nil {
			return LedgerHeaderHistoryEntryView{}, err
		}
		f, err := v0.Fields()
		if err != nil {
			return LedgerHeaderHistoryEntryView{}, err
		}
		return f.LedgerHeader()
	case 1:
		v1, err := v.ArmV1()
		if err != nil {
			return LedgerHeaderHistoryEntryView{}, err
		}
		f, err := v1.Fields()
		if err != nil {
			return LedgerHeaderHistoryEntryView{}, err
		}
		return f.LedgerHeader()
	case 2:
		v2, err := v.ArmV2()
		if err != nil {
			return LedgerHeaderHistoryEntryView{}, err
		}
		f, err := v2.Fields()
		if err != nil {
			return LedgerHeaderHistoryEntryView{}, err
		}
		return f.LedgerHeader()
	default:
		return LedgerHeaderHistoryEntryView{}, viewErrUnknownDiscriminant(0, value)
	}
}

// ledgerHeader navigates to the inner LedgerHeader view.
func (v LedgerCloseMetaView) ledgerHeader() (LedgerHeaderView, error) {
	header, err := v.ledgerHeaderHistoryEntry()
	if err != nil {
		return LedgerHeaderView{}, err
	}
	f, err := header.Fields()
	if err != nil {
		return LedgerHeaderView{}, err
	}
	return f.Header()
}

// LedgerSequence returns the sequence number of this LedgerCloseMeta.
func (v LedgerCloseMetaView) LedgerSequence() (uint32, error) {
	header, err := v.ledgerHeader()
	if err != nil {
		return 0, err
	}
	f, err := header.Fields()
	if err != nil {
		return 0, err
	}
	seqView, err := f.LedgerSeq()
	if err != nil {
		return 0, err
	}
	return seqView.Value()
}

// LedgerCloseTime returns the close time (Unix seconds) of this
// LedgerCloseMeta, mirroring LedgerCloseMeta.LedgerCloseTime on the parsed
// type.
func (v LedgerCloseMetaView) LedgerCloseTime() (int64, error) {
	header, err := v.ledgerHeader()
	if err != nil {
		return 0, err
	}
	f, err := header.Fields()
	if err != nil {
		return 0, err
	}
	scpValue, err := f.ScpValue()
	if err != nil {
		return 0, err
	}
	sf, err := scpValue.Fields()
	if err != nil {
		return 0, err
	}
	ctView, err := sf.CloseTime()
	if err != nil {
		return 0, err
	}
	ct, err := ctView.Value()
	if err != nil {
		return 0, err
	}
	return int64(ct), nil //nolint:gosec // TimePoint is uint64; real close times fit int64
}

// LedgerHash returns the 32-byte hash of the closed ledger as a slice into
// the source bytes. Zero copy; the slice is valid as long as the source
// LedgerCloseMetaView's bytes are.
func (v LedgerCloseMetaView) LedgerHash() ([]byte, error) {
	header, err := v.ledgerHeaderHistoryEntry()
	if err != nil {
		return nil, err
	}
	f, err := header.Fields()
	if err != nil {
		return nil, err
	}
	hashView, err := f.Hash()
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
	header, err := v.ledgerHeader()
	if err != nil {
		return nil, err
	}
	f, err := header.Fields()
	if err != nil {
		return nil, err
	}
	hashView, err := f.PreviousLedgerHash()
	if err != nil {
		return nil, err
	}
	// Raw() returns the zero-copy []byte alias of the source; the fixed-opaque
	// Value() would return a [32]byte that escapes to the heap as a copy.
	return hashView.Raw()
}
