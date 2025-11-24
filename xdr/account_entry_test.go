package xdr_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	. "github.com/stellar/go-stellar-sdk/xdr"
)

var _ = Describe("xdr.AccountEntry#SignerSummary()", func() {
	const address = "GCR22L3WS7TP72S4Z27YTO6JIQYDJK2KLS2TQNHK6Y7XYPA3AGT3X4FH"
	var account AccountEntry

	BeforeEach(func() {
		account.AccountId.SetAddress(address)
	})

	It("adds the master signer when non-zero", func() {
		account.Thresholds[0] = 1
		summary := account.SignerSummary()
		Expect(summary).To(HaveKey(address))
		Expect(summary[address]).To(Equal(int32(1)))
	})

	It("doesn't have the master signer when zero", func() {
		account.Thresholds[0] = 0
		summary := account.SignerSummary()
		Expect(summary).ToNot(HaveKey(address))
	})

	It("includes every secondary signer", func() {
		account.Signers = []Signer{
			signer("GCNXDL2UN2UOZECXIO3SYDL4FSOLQXBKHKNO4EXKUNY2QBHKNF4K6VKQ", 2),
			signer("GAYLEWCV7LQBIVL7BLJ7NBYBYVKVFB55JWOQMKJQYQ3LBSXSAVFMYNHS", 4),
		}
		summary := account.SignerSummary()
		for _, signer := range account.Signers {
			addy := signer.Key.Address()
			Expect(summary).To(HaveKey(addy))
			Expect(summary[addy]).To(Equal(int32(signer.Weight)))
		}
	})
})

func signer(address string, weight int) (ret Signer) {

	ret.Key.SetAddress(address)
	ret.Weight = Uint32(weight)
	return
}

func TestAccountEntryLiabilities(t *testing.T) {
	account := AccountEntry{}
	liabilities := account.Liabilities()
	assert.Equal(t, Int64(0), liabilities.Buying)
	assert.Equal(t, Int64(0), liabilities.Selling)

	account = AccountEntry{
		Ext: AccountEntryExt{
			V1: &AccountEntryExtensionV1{
				Liabilities: Liabilities{
					Buying:  100,
					Selling: 101,
				},
			},
		},
	}
	liabilities = account.Liabilities()
	assert.Equal(t, Int64(100), liabilities.Buying)
	assert.Equal(t, Int64(101), liabilities.Selling)
}

func TestAccountEntrySponsorships(t *testing.T) {
	account := AccountEntry{}
	sponsored := account.NumSponsored()
	sponsoring := account.NumSponsoring()
	signerIDs := account.SignerSponsoringIDs()
	assert.Equal(t, Uint32(0), sponsored)
	assert.Equal(t, Uint32(0), sponsoring)
	assert.Empty(t, signerIDs)

	signer := MustSigner("GCA4M7QXVBVEVRBU53PJZPXANRNPESGKGOT7UZ4RR4CBVBMQHMFKLZ4W")
	sponsor := MustAddress("GCO26ZSBD63TKYX45H2C7D2WOFWOUSG5BMTNC3BG4QMXM3PAYI6WHKVZ")
	desc := SponsorshipDescriptor(&sponsor)
	account = AccountEntry{
		Signers: []Signer{
			{Key: signer},
		},
		Ext: AccountEntryExt{
			V1: &AccountEntryExtensionV1{
				Ext: AccountEntryExtensionV1Ext{
					V2: &AccountEntryExtensionV2{
						NumSponsored:        1,
						NumSponsoring:       2,
						SignerSponsoringIDs: []SponsorshipDescriptor{desc},
					},
				},
			},
		},
	}
	sponsored = account.NumSponsored()
	sponsoring = account.NumSponsoring()
	signerIDs = account.SignerSponsoringIDs()
	expectedSponsorsForSigners := map[string]AccountId{
		signer.Address(): sponsor,
	}
	assert.Equal(t, Uint32(1), sponsored)
	assert.Equal(t, Uint32(2), sponsoring)
	assert.Len(t, signerIDs, 1)
	assert.Equal(t, desc, signerIDs[0])
	assert.Equal(t, expectedSponsorsForSigners, account.SponsorPerSigner())
}
