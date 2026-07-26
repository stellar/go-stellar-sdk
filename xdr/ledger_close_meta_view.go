package xdr

// Convenience helpers on LedgerCloseMetaView that decode just the header
// fields commonly needed for streaming validation, without the full XDR
// decode of the body. Byte-sequence accessors (e.g., LedgerHash) return
// slices into the source bytes — zero-copy; callers copy what they retain.

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
		return v0.LedgerHeader()
	case 1:
		v1, err := v.ArmV1()
		if err != nil {
			return LedgerHeaderHistoryEntryView{}, err
		}
		return v1.LedgerHeader()
	case 2:
		v2, err := v.ArmV2()
		if err != nil {
			return LedgerHeaderHistoryEntryView{}, err
		}
		return v2.LedgerHeader()
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
	return header.Header()
}

// LedgerSequence returns the sequence number of this LedgerCloseMeta.
func (v LedgerCloseMetaView) LedgerSequence() (uint32, error) {
	return Try(func() uint32 {
		header, err := v.ledgerHeader()
		if err != nil {
			mustView(err)
		}
		return header.MustLedgerSeq().MustValue()
	})
}

// LedgerCloseTime returns the close time (Unix seconds) of this
// LedgerCloseMeta, mirroring LedgerCloseMeta.LedgerCloseTime.
func (v LedgerCloseMetaView) LedgerCloseTime() (int64, error) {
	ct, err := Try(func() uint64 {
		header, err := v.ledgerHeader()
		if err != nil {
			mustView(err)
		}
		return header.MustScpValue().MustCloseTime().MustValue()
	})
	return int64(ct), err //nolint:gosec // TimePoint is uint64; real close times fit int64
}

// LedgerHash returns the 32-byte hash of the closed ledger as a slice into
// the source bytes (zero copy).
func (v LedgerCloseMetaView) LedgerHash() ([]byte, error) {
	return Try(func() []byte {
		header, err := v.ledgerHeaderHistoryEntry()
		if err != nil {
			mustView(err)
		}
		return header.MustHash().MustRaw()
	})
}

// PreviousLedgerHash returns the 32-byte hash of the parent ledger as a
// slice into the source bytes (zero copy).
func (v LedgerCloseMetaView) PreviousLedgerHash() ([]byte, error) {
	return Try(func() []byte {
		header, err := v.ledgerHeader()
		if err != nil {
			mustView(err)
		}
		return header.MustPreviousLedgerHash().MustRaw()
	})
}
