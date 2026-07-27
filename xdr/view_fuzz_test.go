package xdr

import (
	"os"
	"testing"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"
)

// FuzzLedgerCloseMetaView asserts that LedgerCloseMetaView's validation,
// raw-read, and walk-assisted navigation paths never panic on arbitrary input
// bytes. Protects against untrusted-input crashes in consumers that expose
// views over network data.
//
// The seed corpus mixes (a) a real captured ledger from testdata/ and
// (b) a handful of randxdr-generated ledgers, so fuzzing starts from
// structurally valid bytes and mutates from there.
func FuzzLedgerCloseMetaView(f *testing.F) {
	if ledger, err := os.ReadFile("testdata/ledger_58752000.bin"); err == nil {
		f.Add(ledger)
	}

	gen := randxdr.NewGenerator()
	for range 8 {
		shape := &gxdr.LedgerCloseMeta{}
		gen.Next(shape, randxdr.LedgerCloseMetaPresets)
		var v LedgerCloseMeta
		if err := gxdr.Convert(shape, &v); err != nil {
			continue
		}
		if data, err := v.MarshalBinary(); err == nil {
			f.Add(data)
		}
	}

	// Empty and trivial inputs — common corner cases.
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		view := ParseLedgerCloseMetaView(data)
		// These may return errors but must never panic on any input.
		// ValidateFull traverses the entire structure, so if a navigation
		// path would panic on this data, this call catches it.
		_ = view.ValidateFull()
		_, _ = view.Raw()

		// Walk-assisted navigation: consume the interior through accessors and
		// iterators (populating iterator state), then re-derive extents. The
		// records left behind must never turn malformed input into a panic.
		fuzzConsumeLCM(view)
		_, _ = view.Raw()
	})
}

// fuzzConsumeLCM drives the public navigation surface over an arbitrary
// input, ignoring all errors — only panics are failures.
// consumeMeta drains a TransactionMetaView's interior through the public
// tier-1 surface, ignoring errors (fuzz coverage of the accessor/iterator
// paths).
func consumeMeta(mv TransactionMetaView) {
	drainChanges := func(c LedgerEntryChangesView) {
		for elem, err := range c.All() {
			if err != nil {
				return
			}
			_, _ = elem.Raw()
		}
	}
	v, err := mv.V()
	if err != nil {
		return
	}
	switch v {
	case 3:
		v3, err := mv.ArmV3()
		if err != nil {
			return
		}
		if c, err := v3.TxChangesBefore(); err == nil {
			drainChanges(c)
		}
		if ops, err := v3.Operations(); err == nil {
			for op, err := range ops.All() {
				if err != nil {
					break
				}
				_, _ = op.Raw()
			}
		}
		if c, err := v3.TxChangesAfter(); err == nil {
			drainChanges(c)
		}
		if sm, err := v3.SorobanMeta(); err == nil {
			_, _ = sm.Raw()
		}
	case 4:
		v4, err := mv.ArmV4()
		if err != nil {
			return
		}
		if ops, err := v4.Operations(); err == nil {
			for op, err := range ops.All() {
				if err != nil {
					break
				}
				if ev, err := op.Events(); err == nil {
					for e, err := range ev.All() {
						if err != nil {
							break
						}
						_, _ = e.Raw()
					}
				}
			}
		}
		if ev, err := v4.Events(); err == nil {
			for e, err := range ev.All() {
				if err != nil {
					break
				}
				_, _ = e.Raw()
			}
		}
	default:
		_, _ = mv.Raw()
	}
}

func fuzzConsumeLCM(view LedgerCloseMetaView) {
	v, err := view.V()
	if err != nil {
		return
	}
	_, _ = view.LedgerSequence()
	_, _ = view.LedgerCloseTime()
	_, _ = view.LedgerHash()
	_, _ = view.PreviousLedgerHash()

	drainResultMeta := func(elem TransactionResultMetaView) {
		if r, err := elem.Result(); err == nil {
			_, _ = r.Raw()
		}
		if m, err := elem.TxApplyProcessing(); err == nil {
			consumeMeta(m)
			_, _ = m.Raw()
		}
		_, _ = elem.Raw()
	}

	switch v {
	case 0:
		v0, err := view.ArmV0()
		if err != nil {
			return
		}
		if tp, err := v0.TxProcessing(); err == nil {
			for elem, err := range tp.All() {
				if err != nil {
					break
				}
				drainResultMeta(elem)
			}
		}
	case 1:
		v1, err := view.ArmV1()
		if err != nil {
			return
		}
		if tp, err := v1.TxProcessing(); err == nil {
			for elem, err := range tp.All() {
				if err != nil {
					break
				}
				drainResultMeta(elem)
			}
		}
	case 2:
		v2, err := view.ArmV2()
		if err != nil {
			return
		}
		if tp, err := v2.TxProcessing(); err == nil {
			for elem, err := range tp.All() {
				if err != nil {
					break
				}
				if m, err := elem.TxApplyProcessing(); err == nil {
					consumeMeta(m)
				}
				_, _ = elem.Raw()
			}
		}
	}
}
