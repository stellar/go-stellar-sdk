# token_transfer: a to_muxed_id shorter than 32 bytes is zero-padded into a hash

**Repository:** stellar/go-stellar-sdk
**Template:** [`bug_report.md`](https://github.com/stellar/.github/blob/HEAD/.github/ISSUE_TEMPLATE/bug_report.md)
**Labels:** bug
**Status:** draft, not filed

---

### What version are you using?

`github.com/stellar/go-stellar-sdk` at master, commit `91e2cdd`, which is the 0.7.2 line.

### What did you do?

Parsed a protocol 23 (transaction meta V4) SEP-41 transfer event whose `to_muxed_id` is an `ScvBytes` value of 4 bytes, `[1 2 3 4]`. `parseV4MapDataForTokenEvents` in `processors/token_transfer/contract_events.go` handles the bytes form by allocating a fixed 32 byte buffer and copying the value into it, without checking the value's length.

```go
hashBytes := make([]byte, 32)
copy(hashBytes, val)
```

Save the following as `processors/token_transfer/bug_repro_test.go` and run it. It is self contained: it builds its own transaction and event fixtures and depends on nothing else in the test package, so it can sit alongside the existing tests without colliding with them.

```go
package token_transfer

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TestBugShortToMuxedIdZeroPadded shows that a to_muxed_id of fewer than 32
// bytes is zero-padded into a 32 byte hash instead of being rejected.
//
// Drop this file into processors/token_transfer/ and run:
//
//	go test ./processors/token_transfer/ -run TestBugShortToMuxedIdZeroPadded -v
func TestBugShortToMuxedIdZeroPadded(t *testing.T) {
	scSymbol := func(s string) xdr.ScVal {
		sym := xdr.ScSymbol(s)
		return xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym}
	}
	scI128 := func(v int64) xdr.ScVal {
		parts := xdr.Int128Parts{Lo: xdr.Uint64(v)}
		return xdr.ScVal{Type: xdr.ScValTypeScvI128, I128: &parts}
	}
	scBytes := func(b []byte) xdr.ScVal {
		raw := xdr.ScBytes(b)
		return xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &raw}
	}
	scContractAddr := func(id xdr.ContractId) xdr.ScVal {
		cid := id
		return xdr.ScVal{
			Type:    xdr.ScValTypeScvAddress,
			Address: &xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: &cid},
		}
	}

	tokenContract := xdr.ContractId{1}
	fromContract := xdr.ContractId{2}
	toContract := xdr.ContractId{3}

	transferEvent := func(muxedID xdr.ScVal) xdr.ContractEvent {
		entries := xdr.ScMap{
			{Key: scSymbol("amount"), Val: scI128(1000)},
			{Key: scSymbol("to_muxed_id"), Val: muxedID},
		}
		data := &entries
		return xdr.ContractEvent{
			Type:       xdr.ContractEventTypeContract,
			ContractId: &tokenContract,
			Body: xdr.ContractEventBody{V: 0, V0: &xdr.ContractEventV0{
				Topics: []xdr.ScVal{scSymbol("transfer"), scContractAddr(fromContract), scContractAddr(toContract)},
				Data:   xdr.ScVal{Type: xdr.ScValTypeScvMap, Map: &data},
			}},
		}
	}

	// A minimal protocol 23 transaction (transaction meta V4).
	tx := ingest.LedgerTransaction{
		Index:    1,
		Envelope: xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &xdr.TransactionV1Envelope{}},
		UnsafeMeta: xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{{}},
		}},
		Ledger: xdr.LedgerCloseMeta{V: 0, V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{
				LedgerVersion: 23,
				LedgerSeq:     12345,
				ScpValue:      xdr.StellarValue{CloseTime: 1234500},
			}},
		}},
	}

	opIndex := uint32(0)
	proc := NewEventsProcessor("Test SDF Network ; September 2015")

	// Control: a real 32 byte hash muxed id round-trips unchanged.
	full := make([]byte, 32)
	for i := range full {
		full[i] = 0xab
	}
	event, err := proc.parseEvent(tx, &opIndex, transferEvent(scBytes(full)))
	if err != nil {
		t.Fatalf("32 byte to_muxed_id: unexpected error: %v", err)
	}
	if got := event.Meta.ToMuxedInfo.GetHash(); len(got) != 32 || got[0] != 0xab {
		t.Fatalf("32 byte to_muxed_id: got %v", got)
	}
	t.Logf("32 byte to_muxed_id -> hash preserved")

	// The bug: 4 bytes in, 32 bytes out. The event parses, and what it reports
	// is not what the emitter sent.
	short := []byte{1, 2, 3, 4}
	event, err = proc.parseEvent(tx, &opIndex, transferEvent(scBytes(short)))
	if err == nil {
		t.Fatalf("4 byte to_muxed_id: expected a parse error, but the event parsed with hash %v",
			event.Meta.ToMuxedInfo.GetHash())
	}
	t.Logf("4 byte to_muxed_id rejected with: %v", err)
}
```

```console
$ go test ./processors/token_transfer/ -run TestBugShortToMuxedIdZeroPadded -v
```

The test first parses a real 32 byte hash to show that the well formed case round-trips, then parses the 4 byte value.

### What did you expect to see?

A hash muxed id is a 32 byte quantity, so a 4 byte value is not one. I expected either a parse error naming `to_muxed_id`, or the bytes preserved exactly as sent, but not a value of one length reshaped into a value of another.

### What did you see instead?

The parse succeeds and the emitted event carries a 32 byte `ToMuxedInfo.Hash`.

```
--- FAIL: TestBugShortToMuxedIdZeroPadded (0.00s)
    bug_repro_test.go:88: 32 byte to_muxed_id -> hash preserved
    bug_repro_test.go:95: 4 byte to_muxed_id: expected a parse error, but the event parsed with hash [1 2 3 4 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0]
```

Nothing downstream can distinguish that from an emitter that genuinely sent those 32 bytes, so a malformed event is turned into a well formed one carrying a muxed id that was never on the ledger.

One option might be to validate the length before the copy, though whether the right response is an error or verbatim preservation probably depends on how strictly this field is meant to be read.
