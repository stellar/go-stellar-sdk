package xdr

import "slices"

// LedgerKey returns the key of the ledger entry without decoding the entry:
// for a valid entry, the key that LedgerEntry.LedgerKey returns. It copies
// the entry type and the key fields out of v, checking only what it needs to
// find them, into a new buffer that holds exactly the key's XDR.
func (v LedgerEntryView) LedgerKey() (LedgerKeyView, error) {
	return Try(func() LedgerKeyView {
		data := v.MustData()
		// data is the entry type followed by the entry, and most entries start
		// with their key fields, so data starts with the key. Contract data and
		// contract code entries start with ext, and their key fields follow it.
		switch data.MustType() {
		case LedgerEntryTypeContractData:
			e := data.MustContractData()
			return slices.Concat(LedgerEntryTypeView(data).MustRaw(),
				e.MustContract().MustRaw(), e.MustKey().MustRaw(), e.MustDurability().MustRaw())
		case LedgerEntryTypeContractCode:
			return slices.Concat(LedgerEntryTypeView(data).MustRaw(), data.MustContractCode().MustHash().MustRaw())
		default:
			return LedgerKeyView(data).MustCopy()
		}
	})
}
