# token_transfer: a bytes to_muxed_id is padded or truncated to 32 bytes without a length check

**Repository:** stellar/go-stellar-sdk
**Template:** [`bug_report.md`](https://github.com/stellar/.github/blob/HEAD/.github/ISSUE_TEMPLATE/bug_report.md)
**Labels:** bug
**Status:** draft, not filed

---

### What version are you using?

`github.com/stellar/go-stellar-sdk` at master, commit `91e2cdd`, which is the 0.7.2 line.

### What did you do?

Parsed protocol 23 (transaction meta V4) SEP-41 token events whose `to_muxed_id` is an `ScvBytes` value that is not 32 bytes long. `parseV4MapDataForTokenEvents` in `processors/token_transfer/contract_events.go` allocates a fixed 32 byte buffer and copies the value into it without checking the value's length:

```go
hashBytes := make([]byte, 32)
copy(hashBytes, val)
```

`copy` stops at whichever slice is shorter, so a short value is zero-padded up to 32 bytes and a long value loses everything past byte 32. Both cases parse without error.

Save the following as `processors/token_transfer/bug_repro_test.go` and run it. It calls `parseV4MapDataForTokenEvents` directly, so it needs no transaction or ledger fixtures and depends on nothing else in the test package.

```go
package token_transfer

import (
	"bytes"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// TestBugToMuxedIdBytesLengthNotChecked shows that parseV4MapDataForTokenEvents
// forces any ScvBytes to_muxed_id into a 32 byte hash: shorter values are
// zero-padded, longer values are truncated. Neither is reported as an error.
//
// Drop this file into processors/token_transfer/ and run:
//
//	go test ./processors/token_transfer/ -run TestBugToMuxedIdBytesLengthNotChecked -v
func TestBugToMuxedIdBytesLengthNotChecked(t *testing.T) {
	// parse builds the {amount, to_muxed_id} data map of a V4 token event with
	// the given to_muxed_id bytes, and returns the parsed muxed id hash.
	parse := func(muxedID []byte) []byte {
		t.Helper()
		amountKey, muxedKey := xdr.ScSymbol("amount"), xdr.ScSymbol("to_muxed_id")
		amount := xdr.Int128Parts{Lo: 1000}
		raw := xdr.ScBytes(muxedID)
		_, info, err := parseV4MapDataForTokenEvents(xdr.ScMap{
			{
				Key: xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &amountKey},
				Val: xdr.ScVal{Type: xdr.ScValTypeScvI128, I128: &amount},
			},
			{
				Key: xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &muxedKey},
				Val: xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &raw},
			},
		})
		if err != nil {
			t.Fatalf("to_muxed_id of %d bytes: %v", len(muxedID), err)
		}
		return info.GetHash()
	}

	// Too short: zero-padded up to 32 bytes, so a 4 byte value is reported as a
	// 32 byte hash no emitter sent.
	if got := parse([]byte{1, 2, 3, 4}); len(got) == 32 {
		t.Errorf("4 byte to_muxed_id parsed to a 32 byte hash: %v", got)
	}

	// Too long: truncated to 32 bytes, so two ids differing only past byte 32
	// collapse onto the same hash.
	a := parse(append(bytes.Repeat([]byte{0xcd}, 32), 1))
	b := parse(append(bytes.Repeat([]byte{0xcd}, 32), 2))
	if bytes.Equal(a, b) {
		t.Errorf("two different 33 byte to_muxed_ids both parsed to %v", a)
	}
}
```

```console
$ go test ./processors/token_transfer/ -run TestBugToMuxedIdBytesLengthNotChecked -v
```

### What did you expect to see?

A hash muxed id is a 32 byte quantity, so a value of any other length is not one. I expected a parse error naming `to_muxed_id`, rather than a value of one length reshaped into a value of another.

### What did you see instead?

Both lengths parse successfully, and the resulting `ToMuxedInfo.Hash` is a well formed 32 byte value that the emitter never sent:

```
=== RUN   TestBugToMuxedIdBytesLengthNotChecked
    bug_repro_test.go:44: 4 byte to_muxed_id parsed to a 32 byte hash: [1 2 3 4 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0]
    bug_repro_test.go:52: two different 33 byte to_muxed_ids both parsed to [205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205 205]
--- FAIL: TestBugToMuxedIdBytesLengthNotChecked (0.00s)
```

Nothing downstream can distinguish either result from an emitter that genuinely sent those 32 bytes. The truncating case is worse than the data loss alone suggests: two distinct ids that agree on their first 32 bytes produce byte identical `MuxedInfo`, so a consumer keying on the muxed id would treat two different destinations as the same one.

A length check before the copy would cover both, though whether the right response is an error or verbatim preservation depends on how strictly this field is meant to be read.
