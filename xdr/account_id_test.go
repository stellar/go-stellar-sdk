package xdr_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/stellar/go-stellar-sdk/xdr"
)

var _ = Describe("xdr.AccountId#Address()", func() {
	It("returns a strkey string when account id is valid", func() {
		var aid AccountId
		err := aid.SetAddress("GCR22L3WS7TP72S4Z27YTO6JIQYDJK2KLS2TQNHK6Y7XYPA3AGT3X4FH")
		Expect(err).ShouldNot(HaveOccurred())
		addy := aid.Address()
		Expect(addy).To(Equal("GCR22L3WS7TP72S4Z27YTO6JIQYDJK2KLS2TQNHK6Y7XYPA3AGT3X4FH"))
	})
})

var _ = Describe("xdr.AccountId#Equals()", func() {
	It("returns true when the account ids have equivalent values", func() {
		var l, r AccountId
		err := l.SetAddress("GCR22L3WS7TP72S4Z27YTO6JIQYDJK2KLS2TQNHK6Y7XYPA3AGT3X4FH")
		Expect(err).ShouldNot(HaveOccurred())
		err = r.SetAddress("GCR22L3WS7TP72S4Z27YTO6JIQYDJK2KLS2TQNHK6Y7XYPA3AGT3X4FH")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(l.Equals(r)).To(BeTrue())
	})

	It("returns false when the account ids have different values", func() {
		var l, r AccountId
		err := l.SetAddress("GCR22L3WS7TP72S4Z27YTO6JIQYDJK2KLS2TQNHK6Y7XYPA3AGT3X4FH")
		Expect(err).ShouldNot(HaveOccurred())
		err = r.SetAddress("GBTBXQEVDNVUEESCTPUT3CHJDVNG44EMPMBELH5F7H3YPHXPZXOTEWB4")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(l.Equals(r)).To(BeFalse())
	})
})

var _ = Describe("xdr.AccountId#LedgerKey()", func() {
	It("works", func() {
		var aid AccountId
		err := aid.SetAddress("GCR22L3WS7TP72S4Z27YTO6JIQYDJK2KLS2TQNHK6Y7XYPA3AGT3X4FH")
		Expect(err).ShouldNot(HaveOccurred())

		key, err := aid.LedgerKey()
		Expect(err).ShouldNot(HaveOccurred())
		packed := key.MustAccount().AccountId
		Expect(packed.Equals(aid)).To(BeTrue())
	})
})

var _ = Describe("xdr.AddressToAccountID()", func() {
	It("works", func() {
		address := "GCR22L3WS7TP72S4Z27YTO6JIQYDJK2KLS2TQNHK6Y7XYPA3AGT3X4FH"
		accountID, err := AddressToAccountId(address)

		Expect(accountID.Address()).To(Equal("GCR22L3WS7TP72S4Z27YTO6JIQYDJK2KLS2TQNHK6Y7XYPA3AGT3X4FH"))
		Expect(err).ShouldNot(HaveOccurred())

		_, err = AddressToAccountId("GCR22L3")

		Expect(err).Should(HaveOccurred())
	})
})

var _ = Describe("xdr.AccountId#ToMuxedAccount()", func() {
	It("works", func() {
		address := "GCR22L3WS7TP72S4Z27YTO6JIQYDJK2KLS2TQNHK6Y7XYPA3AGT3X4FH"
		accountID, err := AddressToAccountId(address)
		Expect(err).ShouldNot(HaveOccurred())
		muxed := accountID.ToMuxedAccount()

		Expect(muxed.Type).To(Equal(CryptoKeyTypeKeyTypeEd25519))
		Expect(*muxed.Ed25519).To(Equal(*accountID.Ed25519))
	})
})
