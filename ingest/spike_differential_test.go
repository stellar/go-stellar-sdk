package ingest

// Stage-1 spike gates for the two-tier visitor: the hand-written
// xdr.SpikeExtract* collectors (both delivery variants) must be
// byte-identical to the frozen oracle over the FULL extended corpus, and
// must fail-vs-succeed at exactly the oracle's truncation boundaries (the
// "truncation stops round up to element/array advance boundaries" decision,
// recorded executable). The zero-subscription decision is pinned here too.

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// diffCompareSpike runs the frozen oracle and a spike variant on raw and
// requires full agreement, mirroring diffCompareExtractors (error parity;
// identical hashes/flags; pointer-identical raw event slices; spine
// nil-ness).
func diffCompareSpike(raw []byte, perArray bool) error {
	ov, oerr := oracleExtractLedgerEvents(xdr.ParseLedgerCloseMetaView(raw))
	sv, serr := xdr.SpikeExtractLedgerEvents(raw, perArray)
	if (oerr != nil) != (serr != nil) {
		return fmt.Errorf("error parity diverges: oracle=%v spike=%v", oerr, serr)
	}
	if oerr != nil {
		return nil
	}
	if len(ov) != len(sv) {
		return fmt.Errorf("tx count diverges: %d vs %d", len(ov), len(sv))
	}
	for i := range ov {
		if ov[i].Hash != sv[i].Hash {
			return fmt.Errorf("tx %d: Hash diverges", i)
		}
		if ov[i].InnerHash != sv[i].InnerHash {
			return fmt.Errorf("tx %d: InnerHash diverges", i)
		}
		if ov[i].FeeBump != sv[i].FeeBump {
			return fmt.Errorf("tx %d: FeeBump diverges", i)
		}
		if err := diffRawSpine(ov[i].TransactionEvents, sv[i].TransactionEvents); err != nil {
			return fmt.Errorf("tx %d: TransactionEvents: %w", i, err)
		}
		if (ov[i].OperationEvents == nil) != (sv[i].OperationEvents == nil) {
			return fmt.Errorf("tx %d: OperationEvents nil-ness diverges", i)
		}
		if len(ov[i].OperationEvents) != len(sv[i].OperationEvents) {
			return fmt.Errorf("tx %d: OperationEvents op count diverges: %d vs %d",
				i, len(ov[i].OperationEvents), len(sv[i].OperationEvents))
		}
		for op := range ov[i].OperationEvents {
			if err := diffRawSpine(ov[i].OperationEvents[op], sv[i].OperationEvents[op]); err != nil {
				return fmt.Errorf("tx %d: OperationEvents[%d]: %w", i, op, err)
			}
		}
	}
	return nil
}

// TestSpikeExtract_DifferentialCorpus: byte-identity vs the frozen oracle on
// the full extended corpus, both variants.
func TestSpikeExtract_DifferentialCorpus(t *testing.T) {
	for _, fx := range differentialCorpus(t) {
		t.Run(fx.name, func(t *testing.T) {
			require.NoError(t, diffCompareSpike(fx.raw, false), "per-event variant")
			require.NoError(t, diffCompareSpike(fx.raw, true), "per-array variant")

			// Hashes: identical to the oracle-derived hashes.
			ov, oerr := oracleExtractLedgerEvents(xdr.ParseLedgerCloseMetaView(fx.raw))
			hv, herr := xdr.SpikeExtractTxHashes(fx.raw)
			require.Equal(t, oerr != nil, herr != nil, "hash error parity: oracle=%v spike=%v", oerr, herr)
			if oerr == nil {
				require.Equal(t, len(ov), len(hv))
				for i := range ov {
					require.Equal(t, ov[i].Hash, [32]byte(hv[i]), "tx %d hash", i)
				}
			}
		})
	}
}

// TestSpikeExtract_TruncationSweep records the truncation decision as a test:
// on every prefix of every sweep fixture, the spike walk must fail-vs-succeed
// exactly where the frozen oracle path does — truncation stops round up to
// element/array advance boundaries, never mid-element leniency and never
// extra validation.
func TestSpikeExtract_TruncationSweep(t *testing.T) {
	for _, fx := range differentialCorpus(t) {
		if !fx.sweep {
			continue
		}
		t.Run(fx.name, func(t *testing.T) {
			step := 1
			if len(fx.raw) > 8192 {
				step = ((len(fx.raw) / 8192) + 1) * 4
			}
			for n := 0; n <= len(fx.raw); n += step {
				p := fx.raw[:n:n]
				if err := diffCompareSpike(p, false); err != nil {
					t.Fatalf("per-event variant, prefix %d of %d: %v", n, len(fx.raw), err)
				}
				if err := diffCompareSpike(p, true); err != nil {
					t.Fatalf("per-array variant, prefix %d of %d: %v", n, len(fx.raw), err)
				}
			}
		})
	}
}

// TestSpikeWalk_ZeroSubscription pins the panel decision: a zero-subscription
// walk returns immediately, validating nothing — even over garbage.
func TestSpikeWalk_ZeroSubscription(t *testing.T) {
	for _, raw := range [][]byte{nil, {}, {0xff}, {0xff, 0xff, 0xff, 0xff, 1, 2, 3}} {
		require.NoError(t, xdr.SpikeWalkNothing(raw))
	}
	// And on a truncated real fixture (which every subscribed walk rejects).
	fx := differentialCorpus(t)[0]
	require.NoError(t, xdr.SpikeWalkNothing(fx.raw[:5:5]))
	_, err := xdr.SpikeExtractTxHashes(fx.raw[:5:5])
	require.Error(t, err, "a subscribed walk must still validate")
}
