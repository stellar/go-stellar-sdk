package xdr

import (
	"os"
	"testing"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"
)

// FuzzLedgerCloseMetaView asserts that LedgerCloseMetaView's validation and
// raw-read paths never panic on arbitrary input bytes. Protects against
// untrusted-input crashes in consumers that expose views over network data.
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
		view := LedgerCloseMetaView(data)
		// These may return errors but must never panic on any input.
		// ValidateFull traverses the entire structure, so if a navigation
		// path would panic on this data, this call catches it.
		_ = view.ValidateFull()
		_, _ = view.Raw()
	})
}
