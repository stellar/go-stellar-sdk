# token_transfer: a void to_muxed_id makes a V4 token event fail to parse

**Repository:** stellar/go-stellar-sdk
**Template:** [`bug_report.md`](https://github.com/stellar/.github/blob/HEAD/.github/ISSUE_TEMPLATE/bug_report.md)
**Labels:** bug
**Status:** draft, not filed

---

### What version are you using?

`github.com/stellar/go-stellar-sdk` at master, commit `91e2cdd`, which is the 0.7.2 line.

### What did you do?

Parsed a protocol 23 (transaction meta V4) SEP-41 transfer event whose data payload is an `ScMap` holding an `amount` entry and a `to_muxed_id` entry whose value is `ScvVoid`. `parseCustomTokenEventV4` in `processors/token_transfer/contract_events.go` routes any map payload to `parseV4MapDataForTokenEvents`, which switches on the type of the `to_muxed_id` value and accepts only `ScvU64`, `ScvBytes` and `ScvString`.

Save the following as `processors/token_transfer/bug_repro_test.go` and run it. It is self contained: it builds its own transaction and event fixtures and depends on nothing else in the test package, so it can sit alongside the existing tests without colliding with them.

```go
package token_transfer

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TestBugVoidToMuxedIdRejected shows that a V4 token event whose to_muxed_id is
// ScvVoid fails to parse, even though omitting the key entirely parses fine.
//
// Drop this file into processors/token_transfer/ and run:
//
//	go test ./processors/token_transfer/ -run TestBugVoidToMuxedIdRejected -v
func TestBugVoidToMuxedIdRejected(t *testing.T) {
	scSymbol := func(s string) xdr.ScVal {
		sym := xdr.ScSymbol(s)
		return xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym}
	}
	scI128 := func(v int64) xdr.ScVal {
		parts := xdr.Int128Parts{Lo: xdr.Uint64(v)}
		return xdr.ScVal{Type: xdr.ScValTypeScvI128, I128: &parts}
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

	// transferEvent builds a V4 transfer event whose data map is {amount, to_muxed_id}.
	// Pass nil to leave to_muxed_id out of the map entirely.
	transferEvent := func(muxedID *xdr.ScVal) xdr.ContractEvent {
		entries := xdr.ScMap{{Key: scSymbol("amount"), Val: scI128(1000)}}
		if muxedID != nil {
			entries = append(entries, xdr.ScMapEntry{Key: scSymbol("to_muxed_id"), Val: *muxedID})
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

	// Control: no to_muxed_id key at all. This parses today.
	event, err := proc.parseEvent(tx, &opIndex, transferEvent(nil))
	if err != nil {
		t.Fatalf("absent to_muxed_id: unexpected error: %v", err)
	}
	if event.Meta.ToMuxedInfo != nil {
		t.Fatalf("absent to_muxed_id: expected no muxed info, got %v", event.Meta.ToMuxedInfo)
	}
	t.Logf("absent to_muxed_id  -> transfer of %s, ToMuxedInfo=nil", event.GetTransfer().Amount)

	// The bug: an explicitly void to_muxed_id means the same thing, but is rejected.
	event, err = proc.parseEvent(tx, &opIndex, transferEvent(&xdr.ScVal{Type: xdr.ScValTypeScvVoid}))
	if err != nil {
		t.Fatalf("void to_muxed_id: expected the event to parse with no muxed info, got error: %v", err)
	}
	if event.Meta.ToMuxedInfo != nil {
		t.Fatalf("void to_muxed_id: expected no muxed info, got %v", event.Meta.ToMuxedInfo)
	}
}
```

```console
$ go test ./processors/token_transfer/ -run TestBugVoidToMuxedIdRejected -v
```

The test parses the same event twice: once with the `to_muxed_id` key omitted, which succeeds, and once with the key present and void, which is the failing case.

### What did you expect to see?

An explicitly void `to_muxed_id` carries the same information as an absent one, namely that the transfer has no muxed destination, so I expected the event to parse into an ordinary transfer event with `ToMuxedInfo` left unset.

Omitting the key entirely already behaves exactly that way. The loop in `parseV4MapDataForTokenEvents` never enters the `to_muxed_id` arm, `muxedInfo` stays nil, and the event is emitted as if the payload had been a bare `i128` amount. The first half of the test above covers this and passes, which is what makes the rejection of the explicit spelling of the same thing look unintended.

### What did you see instead?

`ScvVoid` falls through to the `default` arm of the type switch and the parse fails. No event is produced.

```
--- FAIL: TestBugVoidToMuxedIdRejected (0.00s)
    bug_repro_test.go:82: absent to_muxed_id  -> transfer of 1000, ToMuxedInfo=nil
    bug_repro_test.go:87: void to_muxed_id: expected the event to parse with no muxed info, got error: failed to parse V4 map data: invalid to_muxed_id type for data: ScValTypeScvVoid
```

The failure is invisible to callers, because parse errors on this path are swallowed rather than returned; that half of the problem is written up separately. One option might be to treat a void value the same way an absent key is already treated, though it would be worth first confirming what emitters actually put in this field when there is no muxed destination.
