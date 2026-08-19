# token_transfer: malformed token events are dropped with no error and no log

**Repository:** stellar/go-stellar-sdk
**Template:** [`bug_report.md`](https://github.com/stellar/.github/blob/HEAD/.github/ISSUE_TEMPLATE/bug_report.md)
**Labels:** bug
**Status:** draft, not filed

---

### What version are you using?

`github.com/stellar/go-stellar-sdk` at master, commit `91e2cdd`, which is the 0.7.2 line.

### What did you do?

Called `EventsFromOperation` for an operation carrying a contract event that is unambiguously a SEP-41 transfer, with the correct event name, the correct topics and a valid amount, but whose data payload fails validation partway through. In the reproduction the payload is a V4 map whose `to_muxed_id` is void, which `parseEvent` rejects. `contractEventsFromOperation` (`processors/token_transfer/token_transfer_processor.go:351`) keeps only the events that parsed and returns no error for the rest.

Save the following as `processors/token_transfer/bug_repro_test.go` and run it. It is self contained: it builds its own transaction and event fixtures and depends on nothing else in the test package, so it can sit alongside the existing tests without colliding with them.

```go
package token_transfer

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TestBugMalformedEventSilentlyDropped shows that a token event the SDK cannot
// parse is discarded with no error and no log, so a consumer cannot tell it
// apart from an operation that moved no tokens.
//
// Drop this file into processors/token_transfer/ and run:
//
//	go test ./processors/token_transfer/ -run TestBugMalformedEventSilentlyDropped -v
func TestBugMalformedEventSilentlyDropped(t *testing.T) {
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

	// An unambiguous SEP-41 transfer of 1000 units: right event name, right
	// topics, valid amount. Only the to_muxed_id value is one the SDK rejects.
	entries := xdr.ScMap{
		{Key: scSymbol("amount"), Val: scI128(1000)},
		{Key: scSymbol("to_muxed_id"), Val: xdr.ScVal{Type: xdr.ScValTypeScvVoid}},
	}
	data := &entries
	transferEvent := xdr.ContractEvent{
		Type:       xdr.ContractEventTypeContract,
		ContractId: &tokenContract,
		Body: xdr.ContractEventBody{V: 0, V0: &xdr.ContractEventV0{
			Topics: []xdr.ScVal{scSymbol("transfer"), scContractAddr(fromContract), scContractAddr(toContract)},
			Data:   xdr.ScVal{Type: xdr.ScValTypeScvMap, Map: &data},
		}},
	}

	// A minimal protocol 23 transaction (transaction meta V4) carrying that event
	// on its single operation.
	tx := ingest.LedgerTransaction{
		Index:    1,
		Envelope: xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &xdr.TransactionV1Envelope{}},
		UnsafeMeta: xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{{Events: []xdr.ContractEvent{transferEvent}}},
		}},
		Ledger: xdr.LedgerCloseMeta{V: 0, V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{
				LedgerVersion: 23,
				LedgerSeq:     12345,
				ScpValue:      xdr.StellarValue{CloseTime: 1234500},
			}},
		}},
	}

	op := xdr.Operation{Body: xdr.OperationBody{
		Type:                 xdr.OperationTypeInvokeHostFunction,
		InvokeHostFunctionOp: &xdr.InvokeHostFunctionOp{},
	}}

	proc := NewEventsProcessor("Test SDF Network ; September 2015")

	// This is the public entry point a consumer uses.
	events, err := proc.EventsFromOperation(tx, 0, op, xdr.OperationResult{})

	// The parse failure is reported as "not a SEP-41 token event" and swallowed:
	// no error reaches the caller, and nothing is logged.
	if err != nil {
		t.Logf("error returned to the caller: %v", err)
	} else {
		t.Logf("error returned to the caller: none")
	}
	if len(events) != 1 {
		t.Fatalf("expected the transfer to be reported (as an event or as an error), got %d events and err=%v",
			len(events), err)
	}
}
```

```console
$ go test ./processors/token_transfer/ -run TestBugMalformedEventSilentlyDropped -v
```

Any parse failure reproduces this; the void `to_muxed_id` is just a convenient one that is itself the subject of a separate report.

### What did you expect to see?

Two quite different situations reach this loop and I expected them to be distinguishable. One is a contract event that simply is not a token event, which is routine and should be skipped quietly. The other is an event that is clearly a token event but whose payload the SDK cannot make sense of, which is a transfer disappearing from the output.

For the second case I expected something observable: an error returned to the caller, or failing that a counter or a log line, so that a consumer reconstructing balances from this stream can tell the difference between an operation that moved no tokens and an operation whose token movements were discarded.

### What did you see instead?

Both cases are reported as `ErrNotSep41TokenEvent`, and the loop keeps only events where `err == nil`, so the malformed transfer is discarded exactly like an unrelated contract event.

```go
ev, err := p.parseEvent(tx, &opIndex, contractEvent)

// You dont bail on error here, since error here means that it is not a sep-41 compliant token event.
if err == nil {
    events = append(events, ev)
}
```

`EventsFromOperation` returns an empty slice and a nil error, and nothing is logged. In the reproduction a valid 1000 unit transfer between two known contract addresses vanishes with no trace.

```
--- FAIL: TestBugMalformedEventSilentlyDropped (0.00s)
    bug_repro_test.go:86: error returned to the caller: none
    bug_repro_test.go:89: expected the transfer to be reported (as an event or as an error), got 0 events and err=<nil>
```

This is what turns every parse-strictness bug on this path into silent data loss rather than a visible failure, so it may be worth deciding separately from any individual parsing fix.
