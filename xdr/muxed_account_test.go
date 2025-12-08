package xdr_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/stellar/go-stellar-sdk/xdr"
)

var _ = Describe("xdr.MuxedAccount#Get/SetAddress()", func() {
	It("returns an empty string when muxed account is nil", func() {
		addy := (*MuxedAccount)(nil).Address()
		Expect(addy).To(Equal(""))
	})

	It("returns a strkey string when muxed account is valid", func() {
		var unmuxed MuxedAccount
		err := unmuxed.SetEd25519Address("GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(unmuxed.Type).To(Equal(CryptoKeyTypeKeyTypeEd25519))
		Expect(*unmuxed.Ed25519).To(Equal(Uint256{63, 12, 52, 191, 147, 173, 13, 153, 113, 208, 76, 204, 144, 247, 5, 81, 28, 131, 138, 173, 151, 52, 164, 162, 251, 13, 122, 3, 252, 127, 232, 154}))
		_, err = unmuxed.GetId()
		Expect(err).Should(HaveOccurred())
		muxedy := unmuxed.Address()
		Expect(muxedy).To(Equal("GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"))

		var muxed MuxedAccount
		err = muxed.SetAddress("MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(muxed.Type).To(Equal(CryptoKeyTypeKeyTypeMuxedEd25519))
		Expect(muxed.Med25519.Id).To(Equal(Uint64(9223372036854775808)))
		Expect(muxed.Med25519.Ed25519).To(Equal(*unmuxed.Ed25519))
		muxedy = muxed.Address()
		id, err := muxed.GetId()
		Expect(id).To(Equal(uint64(9223372036854775808)))
		Expect(muxedy).To(Equal("MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK"))

		err = muxed.SetAddress("MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUQ")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(muxed.Type).To(Equal(CryptoKeyTypeKeyTypeMuxedEd25519))
		Expect(muxed.Med25519.Id).To(Equal(Uint64(0)))
		Expect(muxed.Med25519.Ed25519).To(Equal(*unmuxed.Ed25519))
		id, err = muxed.GetId()
		Expect(id).To(Equal(uint64(0)))
		muxedy = muxed.Address()
		Expect(muxedy).To(Equal("MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUQ"))
	})

	It("returns a muxed account from an account ID and memo", func() {
		accountId := "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"
		expectedMuxedId := "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAABUTGI4"

		muxedAccount, err := MuxedAccountFromAccountId(accountId, uint64(420))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(muxedAccount.GetAddress()).To(Equal(expectedMuxedId))
		Expect(muxedAccount.GetId()).To(Equal(uint64(420)))
	})

	It("returns an error when the strkey is invalid", func() {
		var muxed MuxedAccount

		// Test cases from SEP23

		err := muxed.SetEd25519Address("GAAAAAAAACGC6")
		Expect(err).Should(HaveOccurred())

		err = muxed.SetEd25519Address("MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUR")
		Expect(err).Should(HaveOccurred())

		err = muxed.SetEd25519Address("GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZA")
		Expect(err).Should(HaveOccurred())

		err = muxed.SetEd25519Address("GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUACUSI")
		Expect(err).Should(HaveOccurred())

		err = muxed.SetEd25519Address("G47QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVP2I")
		Expect(err).Should(HaveOccurred())

		err = muxed.SetAddress("MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLKA")
		Expect(err).Should(HaveOccurred())

		err = muxed.SetAddress("MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAAV75I")
		Expect(err).Should(HaveOccurred())

		err = muxed.SetAddress("M47QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUQ")
		Expect(err).Should(HaveOccurred())

		err = muxed.SetAddress("MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUK===")
		Expect(err).Should(HaveOccurred())

		err = muxed.SetAddress("MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUO")
		Expect(err).Should(HaveOccurred())
	})
})

var _ = Describe("xdr.AddressToMuxedAccount()", func() {
	It("works", func() {
		address := "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"
		muxedAccount, err := AddressToMuxedAccount(address)

		Expect(muxedAccount.Address()).To(Equal("GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"))
		Expect(err).ShouldNot(HaveOccurred())

		_, err = AddressToAccountId("GCR22L3")

		Expect(err).Should(HaveOccurred())
	})
})

var _ = Describe("xdr.MuxedAccount.ToAccountId()", func() {
	It("works", func() {
		var muxed MuxedAccount

		err := muxed.SetEd25519Address("GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ")
		Expect(err).ShouldNot(HaveOccurred())
		aid := muxed.ToAccountId()
		Expect(aid.Address()).To(Equal("GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"))

		err = muxed.SetAddress("MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK")
		Expect(err).ShouldNot(HaveOccurred())
		aid = muxed.ToAccountId()
		Expect(aid.Address()).To(Equal("GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"))
	})
})
