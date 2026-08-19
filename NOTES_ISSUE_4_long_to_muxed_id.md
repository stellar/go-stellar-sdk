# token_transfer: a to_muxed_id longer than 32 bytes is silently truncated

**Repository:** stellar/go-stellar-sdk
**Template:** [`bug_report.md`](https://github.com/stellar/.github/blob/HEAD/.github/ISSUE_TEMPLATE/bug_report.md)
**Labels:** bug
**Status:** draft, not filed

---

### What version are you using?

`github.com/stellar/go-stellar-sdk` at master, commit `91e2cdd`, which is the 0.7.2 line.

### What did you do?

Parsed a protocol 23 (transaction meta V4) SEP-41 transfer event whose `to_muxed_id` is an `ScvBytes` value of 40 bytes. `parseV4MapDataForTokenEvents` in `processors/token_transfer/contract_events.go` copies the value into a fixed 32 byte buffer, and `copy` stops at the destination length, so the trailing 8 bytes are discarded.

```go
hashBytes := make([]byte, 32)
copy(hashBytes, val)
```

Save the following as `processors/token_transfer/bug_repro_test.go` and run it. It is self contained: it builds its own transaction and event fixtures and depends on nothing else in the test package, so it can sit alongside the existing tests without colliding with them.

```go
package token_transfer

import (
	"bytes"
	"testing"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TestBugLongToMuxedIdTruncated shows that a to_muxed_id of more than 32 bytes
// is truncated to its first 32 bytes, which both loses data and collapses two
// distinct identifiers onto the same MuxedInfo.
//
// Drop this file into processors/token_transfer/ and run:
//
//	go test ./processors/token_transfer/ -run TestBugLongToMuxedIdTruncated -v
func TestBugLongToMuxedIdTruncated(t *testing.T) {
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

	// Two 40 byte values that agree on their first 32 bytes and differ after it.
	first := append(bytes.Repeat([]byte{0xcd}, 32), 1, 1, 1, 1, 1, 1, 1, 1)
	second := append(bytes.Repeat([]byte{0xcd}, 32), 2, 2, 2, 2, 2, 2, 2, 2)

	firstEvent, err := proc.parseEvent(tx, &opIndex, transferEvent(scBytes(first)))
	if err == nil {
		t.Errorf("40 byte to_muxed_id: expected a parse error, but the event parsed with hash %v",
			firstEvent.Meta.ToMuxedInfo.GetHash())
	} else {
		t.Logf("40 byte to_muxed_id rejected with: %v", err)
		return
	}

	secondEvent, err := proc.parseEvent(tx, &opIndex, transferEvent(scBytes(second)))
	if err != nil {
		t.Fatalf("second 40 byte to_muxed_id: unexpected error: %v", err)
	}

	// Worse than losing the tail: the two distinct ids are now indistinguishable.
	if bytes.Equal(firstEvent.Meta.ToMuxedInfo.GetHash(), secondEvent.Meta.ToMuxedInfo.GetHash()) {
		t.Errorf("two different 40 byte to_muxed_ids both parsed to the same hash %v",
			firstEvent.Meta.ToMuxedInfo.GetHash())
	}
}
```

```console
$ go test ./processors/token_transfer/ -run TestBugLongToMuxedIdTruncated -v
```

The test parses two 40 byte values that agree on their first 32 bytes and differ after it, so it demonstrates both the truncation and its consequence.

### What did you expect to see?

An oversized value is malformed, so I expected a parse error naming `to_muxed_id` rather than a successful parse of a value the emitter did not send.

### What did you see instead?

The parse succeeds and the emitted event carries only the first 32 bytes as `ToMuxedInfo.Hash`, with no indication that anything was dropped. Beyond losing the tail, this collapses distinct identifiers: the two 40 byte values in the reproduction differ, but produce byte identical `MuxedInfo`, so a consumer keying on the muxed id would treat two different destinations as the same one.

```
--- FAIL: TestBugLongToMuxedIdTruncated (0.00s)
    bug_repro_test.go:84: 40 byte to_muxed_id: expected a parse error, but the event parsed with hash [205 205 ... 205]
    bug_repro_test.go:98: two different 40 byte to_muxed_ids both parsed to the same hash [205 205 ... 205]
```

This shares a line with the short value case, which is padded rather than truncated, so the two may well want a single length check between them.
