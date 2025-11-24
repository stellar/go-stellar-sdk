package loadtest

import (
	"encoding"
	"fmt"

	goxdr "github.com/xdrpp/goxdr/xdr"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type updateLedgerSeqMarshaller struct {
	getUpdatedLedger func(uint32) uint32
}

func (updateLedgerSeqMarshaller) Sprintf(f string, args ...interface{}) string {
	return fmt.Sprintf(f, args...)
}

func (u updateLedgerSeqMarshaller) Marshal(field string, i goxdr.XdrType) {
	switch t := goxdr.XdrBaseType(i).(type) {
	case goxdr.XdrBytes, goxdr.XdrNum32, goxdr.XdrNum64, *goxdr.XdrBool, goxdr.XdrEnum:
	case *gxdr.AccountEntryExtensionV3:
		t.SeqLedger = u.getUpdatedLedger(t.SeqLedger)
		t.XdrRecurse(u, field)
	case *gxdr.TTLEntry:
		t.LiveUntilLedgerSeq = u.getUpdatedLedger(t.LiveUntilLedgerSeq)
		t.XdrRecurse(u, field)
	case *gxdr.LedgerEntry:
		t.LastModifiedLedgerSeq = u.getUpdatedLedger(t.LastModifiedLedgerSeq)
		t.XdrRecurse(u, field)
	case goxdr.XdrAggregate:
		t.XdrRecurse(u, field)
	default:
		panic(fmt.Sprintf("field %s has unexpected xdr type %v", field, t))
	}
}

type XDR interface {
	encoding.BinaryUnmarshaler
	encoding.BinaryMarshaler
}

// UpdateLedgerSeqInLedgerEntries will traverse the ledger entries contained within dest and update
// any ledger sequence values that are found in the ledger entries. The new
// ledger sequence values will be determined by calling getUpdatedLedger().
func UpdateLedgerSeqInLedgerEntries(dest XDR, getUpdatedLedger func(uint32) uint32) error {
	raw, err := dest.MarshalBinary()
	if err != nil {
		return err
	}
	var destGoxdr goxdr.XdrType
	switch t := dest.(type) {
	case *xdr.LedgerCloseMeta:
		destGoxdr = &gxdr.LedgerCloseMeta{}
		*t = xdr.LedgerCloseMeta{}
	case *xdr.TransactionMeta:
		destGoxdr = &gxdr.TransactionMeta{}
		*t = xdr.TransactionMeta{}
	case *xdr.LedgerEntryChange:
		destGoxdr = &gxdr.LedgerEntryChange{}
		*t = xdr.LedgerEntryChange{}
	case *xdr.LedgerEntry:
		destGoxdr = &gxdr.LedgerEntry{}
		*t = xdr.LedgerEntry{}
	default:
		return fmt.Errorf("unsupported XDR type %T", t)
	}
	gxdr.Parse(destGoxdr, raw)

	updateLedgerSeqMarshaller{getUpdatedLedger: getUpdatedLedger}.Marshal("", destGoxdr)
	return gxdr.Convert(destGoxdr, dest)
}
