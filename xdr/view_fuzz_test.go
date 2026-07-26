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

		// Walk-assisted navigation: consume the interior through bundles and
		// iterators (populating walk records), then re-derive extents. The
		// records left behind must never turn malformed input into a panic.
		fuzzConsumeLCM(view)
		_, _ = view.Raw()
	})
}

// fuzzConsumeLCM drives the public navigation surface over an arbitrary
// input, ignoring all errors — only panics are failures.
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
		f, _ := elem.Fields()
		if r, err := f.Result(); err == nil {
			_, _ = r.Raw()
		}
		if m, err := f.TxApplyProcessing(); err == nil {
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
		f, _ := v0.Fields()
		if tp, err := f.TxProcessing(); err == nil {
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
		f, _ := v1.Fields()
		if tp, err := f.TxProcessing(); err == nil {
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
		f, _ := v2.Fields()
		if tp, err := f.TxProcessing(); err == nil {
			for elem, err := range tp.All() {
				if err != nil {
					break
				}
				ef, _ := elem.Fields()
				if m, err := ef.TxApplyProcessing(); err == nil {
					consumeMeta(m)
				}
				_, _ = elem.Raw()
			}
		}
	}
}
