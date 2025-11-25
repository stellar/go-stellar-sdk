package txnbuild

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
)

var signers = []xdr.SignerKey{
	xdr.MustSigner("GAOQJGUAB7NI7K7I62ORBXMN3J4SSWQUQ7FOEPSDJ322W2HMCNWPHXFB"),
	xdr.MustSigner("GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"),
	xdr.MustSigner("PA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAQACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB6IBZGM"),
}

// TestPreconditionClassification ensures that Preconditions will correctly
// differentiate V1 (timebounds-only) or V2 (all other) preconditions correctly.
func TestPreconditionClassification(t *testing.T) {
	tbpc := Preconditions{TimeBounds: NewTimebounds(1, 2)}
	assert.False(t, (&Preconditions{}).hasV2Conditions())
	assert.False(t, tbpc.hasV2Conditions())

	tbpc.MinSequenceNumberLedgerGap = 2
	assert.True(t, tbpc.hasV2Conditions())
}

// TestPreconditionValidation ensures that errors occur when necessary.
func TestPreconditionValidation(t *testing.T) {
	t.Run("invalid signer", func(t *testing.T) {
		pc := Preconditions{
			TimeBounds:   NewTimebounds(27, 42),
			ExtraSigners: []string{"literally just some nonsense"},
		}

		_, err := pc.BuildXDR()
		assert.Error(t, err)
	})

	t.Run("too many signers", func(t *testing.T) {
		pc := Preconditions{
			TimeBounds:   NewTimebounds(27, 42),
			ExtraSigners: signerKeysToStrings(signers),
		}

		assert.Error(t, pc.Validate())
	})

	t.Run("nonsense ledgerbounds", func(t *testing.T) {
		pc := Preconditions{TimeBounds: NewTimebounds(27, 42)}
		pc.LedgerBounds = &LedgerBounds{MinLedger: 42, MaxLedger: 1}
		assert.Error(t, pc.Validate())
	})
}

// TestPreconditionEncoding ensures correct XDR is generated for a
// (non-exhaustive) handful of precondition combinations. It generates XDR and
// txnbuild structures that match semantically, then makes sure they translate
// between each other (encode/decode round trips, etc.).
func TestPreconditionEncoding(t *testing.T) {
	modifiers := []struct {
		Name     string
		Modifier func() (xdr.Preconditions, Preconditions)
	}{
		{
			"unchanged",
			func() (xdr.Preconditions, Preconditions) {
				return createPreconditionFixtures()
			},
		},
		{
			"only timebounds",
			func() (xdr.Preconditions, Preconditions) {
				return xdr.Preconditions{
					Type: xdr.PreconditionTypePrecondTime,
					TimeBounds: &xdr.TimeBounds{
						MinTime: xdr.TimePoint(1),
						MaxTime: xdr.TimePoint(2),
					},
				}, Preconditions{TimeBounds: NewTimebounds(1, 2)}
			},
		},
		{
			"unbounded ledgerbounds",
			func() (xdr.Preconditions, Preconditions) {
				xdrPc, pc := createPreconditionFixtures()
				xdrPc.V2.LedgerBounds.MaxLedger = 0
				pc.LedgerBounds.MaxLedger = 0
				return xdrPc, pc
			},
		},
		{
			"nil ledgerbounds",
			func() (xdr.Preconditions, Preconditions) {
				xdrPc, pc := createPreconditionFixtures()
				xdrPc.V2.LedgerBounds = nil
				pc.LedgerBounds = nil
				return xdrPc, pc
			},
		},
		{
			"nil minSeq",
			func() (xdr.Preconditions, Preconditions) {
				xdrPc, pc := createPreconditionFixtures()
				xdrPc.V2.MinSeqNum = nil
				pc.MinSequenceNumber = nil
				return xdrPc, pc
			},
		},
		{
			"no signers",
			func() (xdr.Preconditions, Preconditions) {
				xdrPc, pc := createPreconditionFixtures()
				xdrPc.V2.ExtraSigners = nil
				pc.ExtraSigners = nil
				return xdrPc, pc
			},
		},
	}
	for _, testCase := range modifiers {
		t.Run(testCase.Name, func(t *testing.T) {
			xdrPrecond, precond := testCase.Modifier()

			assert.NoError(t, precond.Validate())

			expectedBytes, err := xdrPrecond.MarshalBinary()
			assert.NoError(t, err)
			encodedPrecond, err := precond.BuildXDR()
			assert.NoError(t, err)
			actualBytes, err := encodedPrecond.MarshalBinary()
			assert.NoError(t, err)

			// building the struct should result in identical XDR!
			assert.Equal(t, expectedBytes, actualBytes)

			// unpacking the XDR should result in identical structs!
			roundTripXdr := xdr.Preconditions{}
			err = roundTripXdr.UnmarshalBinary(actualBytes)
			assert.NoError(t, err)
			assert.Equal(t, xdrPrecond, roundTripXdr)

			roundTripPrecond := Preconditions{}
			roundTripPrecond.FromXDR(roundTripXdr)
			assert.Equal(t, precond, roundTripPrecond)
		})
	}
}

// createPreconditionFixtures returns some initial, sensible XDR and txnbuild
// precondition structures with all fields set and matching semantics.
func createPreconditionFixtures() (xdr.Preconditions, Preconditions) {
	seqNum := int64(42)
	xdrSeqNum := xdr.SequenceNumber(seqNum)
	xdrCond := xdr.Preconditions{
		Type: xdr.PreconditionTypePrecondV2,
		V2: &xdr.PreconditionsV2{
			TimeBounds: &xdr.TimeBounds{
				MinTime: xdr.TimePoint(27),
				MaxTime: xdr.TimePoint(42),
			},
			LedgerBounds: &xdr.LedgerBounds{
				MinLedger: xdr.Uint32(27),
				MaxLedger: xdr.Uint32(42),
			},
			MinSeqNum:       &xdrSeqNum,
			MinSeqAge:       xdr.Duration(27),
			MinSeqLedgerGap: xdr.Uint32(42),
			ExtraSigners:    []xdr.SignerKey{signers[0]},
		},
	}
	pc := Preconditions{
		TimeBounds:                 NewTimebounds(27, 42),
		LedgerBounds:               &LedgerBounds{27, 42},
		MinSequenceNumber:          &seqNum,
		MinSequenceNumberAge:       27,
		MinSequenceNumberLedgerGap: 42,
		ExtraSigners:               []string{signers[0].Address()},
	}

	return xdrCond, pc
}

func signerKeysToStrings(signerKeys []xdr.SignerKey) []string {
	strSigners := make([]string, len(signers))
	for i, signerKey := range signers {
		strSigners[i] = signerKey.Address()
	}
	return strSigners
}
