package xdr

// Convenience helpers on LedgerCloseMetaView that decode just the header
// fields commonly needed for streaming validation, without the full XDR
// decode of the body. Mirrors the equivalent methods on LedgerCloseMeta
// (in ledger_close_meta.go) so callers can switch between decoded and
// view forms without churn.

func (v LedgerCloseMetaView) ledgerHeaderHistoryEntry() (LedgerHeaderHistoryEntryView, error) {
	disc, err := v.V()
	if err != nil {
		return nil, err
	}
	value, err := disc.Value()
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

// LedgerHash returns the hash of the closed ledger.
func (v LedgerCloseMetaView) LedgerHash() (Hash, error) {
	header, err := v.ledgerHeaderHistoryEntry()
	if err != nil {
		return Hash{}, err
	}
	hashView, err := header.Hash()
	if err != nil {
		return Hash{}, err
	}
	bytes, err := hashView.Value()
	if err != nil {
		return Hash{}, err
	}
	var h Hash
	copy(h[:], bytes)
	return h, nil
}

// PreviousLedgerHash returns the hash of the parent ledger.
func (v LedgerCloseMetaView) PreviousLedgerHash() (Hash, error) {
	header, err := v.ledgerHeaderHistoryEntry()
	if err != nil {
		return Hash{}, err
	}
	headerInner, err := header.Header()
	if err != nil {
		return Hash{}, err
	}
	hashView, err := headerInner.PreviousLedgerHash()
	if err != nil {
		return Hash{}, err
	}
	bytes, err := hashView.Value()
	if err != nil {
		return Hash{}, err
	}
	var h Hash
	copy(h[:], bytes)
	return h, nil
}
