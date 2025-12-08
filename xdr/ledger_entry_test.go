package xdr

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"

	"github.com/stretchr/testify/assert"
)

func TestLedgerEntrySponsorship(t *testing.T) {
	entry := LedgerEntry{}
	desc := entry.SponsoringID()
	assert.Nil(t, desc)

	sponsor := MustAddress("GCO26ZSBD63TKYX45H2C7D2WOFWOUSG5BMTNC3BG4QMXM3PAYI6WHKVZ")
	desc = SponsorshipDescriptor(&sponsor)

	entry = LedgerEntry{
		Ext: LedgerEntryExt{
			V: 1,
			V1: &LedgerEntryExtensionV1{
				SponsoringId: desc,
			},
		},
	}
	actualDesc := entry.SponsoringID()
	assert.Equal(t, desc, actualDesc)
}

func TestNormalizedClaimableBalance(t *testing.T) {
	input := LedgerEntry{
		LastModifiedLedgerSeq: 20,
		Data: LedgerEntryData{
			Type: LedgerEntryTypeClaimableBalance,
			ClaimableBalance: &ClaimableBalanceEntry{
				Claimants: []Claimant{
					{
						Type: ClaimantTypeClaimantTypeV0,
						V0: &ClaimantV0{
							Destination: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
						},
					},
					{
						Type: ClaimantTypeClaimantTypeV0,
						V0: &ClaimantV0{
							Destination: MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
						},
					},
				},
				Ext: ClaimableBalanceEntryExt{
					V: 0,
				},
			},
		},
	}

	expectedOutput := LedgerEntry{
		LastModifiedLedgerSeq: 20,
		Data: LedgerEntryData{
			Type: LedgerEntryTypeClaimableBalance,
			ClaimableBalance: &ClaimableBalanceEntry{
				Claimants: []Claimant{
					{
						Type: ClaimantTypeClaimantTypeV0,
						V0: &ClaimantV0{
							Destination: MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
						},
					},
					{
						Type: ClaimantTypeClaimantTypeV0,
						V0: &ClaimantV0{
							Destination: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
						},
					},
				},
				Ext: ClaimableBalanceEntryExt{
					V: 1,
					V1: &ClaimableBalanceEntryExtensionV1{
						Flags: 0,
					},
				},
			},
		},
		Ext: LedgerEntryExt{
			V:  1,
			V1: &LedgerEntryExtensionV1{},
		},
	}

	input.Normalize()
	assert.Equal(t, expectedOutput, input)
}

func TestLedgerKeyCoverage(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := 0; i < 10000; i++ {
		ledgerEntry := LedgerEntry{}
		shape := &gxdr.LedgerEntry{}
		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		assert.NoError(t, gxdr.Convert(shape, &ledgerEntry))
		_, err := ledgerEntry.LedgerKey()
		assert.NoError(t, err)
	}
}

func TestLedgerEntryDataLedgerKeyCoverage(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := 0; i < 10000; i++ {
		ledgerEntryData := LedgerEntryData{}

		shape := &gxdr.XdrAnon_LedgerEntry_Data{}
		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		assert.NoError(t, gxdr.Convert(shape, &ledgerEntryData))
		_, err := ledgerEntryData.LedgerKey()
		assert.NoError(t, err)
	}
}
