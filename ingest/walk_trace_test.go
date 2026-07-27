package ingest

// Walk trace equivalence (design §4.1): callback traces captured from the
// walker over the full differential corpus, pinned as compact per-fixture
// digests. The derived walker must reproduce every trace exactly; a
// divergence is escalated, never silently conformed (review amendment 1).
// Regenerate with -run TestWalkLCM_TraceEquivalence -update-walk-traces.

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

var updateWalkTraces = flag.Bool("update-walk-traces", false, "regenerate testdata/walk_traces.txt from the current walker")

// traceWalk runs a full subscription over raw, serializing every fire —
// position, context args, and payload identity (buffer-relative offset+len
// for views; the value for counts/discriminants) — and returns the fire
// count and the SHA-256 of the serialized stream. Errors end the trace with
// a terminal "err" record (error TEXT is excluded: it is not a compatibility
// surface; the stop POINT is).
func traceWalk(raw []byte) (int, string) {
	h := sha256.New()
	fires := 0
	base := uintptr(unsafe.Pointer(unsafe.SliceData(raw)))
	rec := func(format string, args ...any) {
		fires++
		fmt.Fprintf(h, format+"\n", args...)
	}
	span := func(b []byte) string {
		if len(raw) == 0 || len(b) == 0 {
			return fmt.Sprintf("len%d", len(b))
		}
		off := uintptr(unsafe.Pointer(unsafe.SliceData(b))) - base
		return fmt.Sprintf("%d+%d", off, len(b))
	}
	w := xdr.LedgerCloseMetaWalk{
		TxProcessingBegin: func(count int) error { rec("tpb %d", count); return nil },
		ResultPair: func(tx int, pair xdr.TransactionResultPairView) error {
			rec("rp %d %s", tx, span(pair.MustRaw()))
			return nil
		},
		MetaVersion:   func(tx int, v int32) error { rec("mv %d %d", tx, v); return nil },
		V4Ops:         func(tx, n int) error { rec("v4o %d %d", tx, n); return nil },
		OpEventsBegin: func(tx, op, n int) error { rec("oeb %d %d %d", tx, op, n); return nil },
		OpEvent: func(tx, op, ev int, e xdr.ContractEventView) error {
			rec("oe %d %d %d %s", tx, op, ev, span(e.MustRaw()))
			return nil
		},
		TxEventsBegin: func(tx, n int) error { rec("teb %d %d", tx, n); return nil },
		TxEvent: func(tx, ev int, e xdr.TransactionEventView) error {
			rec("te %d %d %s", tx, ev, span(e.MustRaw()))
			return nil
		},
		DiagEventsBegin: func(tx, n int) error { rec("deb %d %d", tx, n); return nil },
		DiagEvent: func(tx, ev int, e xdr.DiagnosticEventView) error {
			rec("de %d %d %s", tx, ev, span(e.MustRaw()))
			return nil
		},
		TxMeta: func(tx int, m xdr.TransactionMetaView) error {
			rec("tm %d %s", tx, span(m.MustRaw()))
			return nil
		},
	}
	if err := xdr.WalkLedgerCloseMeta(xdr.NewLedgerCloseMetaView(raw), &w); err != nil {
		rec("err")
	}
	return fires, hex.EncodeToString(h.Sum(nil))
}

const walkTracePath = "testdata/walk_traces.txt"

// TestWalkLCM_TraceEquivalence pins the walker's exact callback behavior —
// fire order, context args, payload extents, stop points — against digests
// captured from the walker that defined the contract. Truncation prefixes of
// every sweep fixture are included (4-byte stride, capped work), so
// element-boundary stops are pinned too.
func TestWalkLCM_TraceEquivalence(t *testing.T) {
	type entry struct {
		name, digest string
		fires        int
	}
	var got []entry
	for _, fx := range differentialCorpus(t) {
		fires, digest := traceWalk(fx.raw)
		got = append(got, entry{fx.name, digest, fires})
		if !fx.sweep {
			continue
		}
		step := 4
		if len(fx.raw) > 8192 {
			step = ((len(fx.raw) / 8192) + 1) * 4
		}
		for n := 0; n < len(fx.raw); n += step {
			fires, digest := traceWalk(fx.raw[:n:n])
			got = append(got, entry{fmt.Sprintf("%s@%d", fx.name, n), digest, fires})
		}
	}

	if *updateWalkTraces {
		var b strings.Builder
		b.WriteString("# walk callback trace digests; regenerate with -update-walk-traces\n")
		for _, e := range got {
			fmt.Fprintf(&b, "%s %d %s\n", e.name, e.fires, e.digest)
		}
		require.NoError(t, os.WriteFile(walkTracePath, []byte(b.String()), 0644))
		t.Logf("wrote %d trace digests", len(got))
		return
	}

	f, err := os.Open(walkTracePath)
	require.NoError(t, err, "trace fixture missing; run with -update-walk-traces")
	defer f.Close()
	want := map[string]entry{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		var e entry
		_, err := fmt.Sscanf(line, "%s %d %s", &e.name, &e.fires, &e.digest)
		require.NoError(t, err, "bad trace line %q", line)
		want[e.name] = e
	}
	require.Len(t, want, len(got), "trace fixture entry count")
	for _, e := range got {
		w, ok := want[e.name]
		require.True(t, ok, "no captured trace for %s", e.name)
		require.Equal(t, w.fires, e.fires, "%s: fire count diverges from captured trace", e.name)
		require.Equal(t, w.digest, e.digest, "%s: callback trace diverges from captured trace (ESCALATE, do not conform)", e.name)
	}
}
