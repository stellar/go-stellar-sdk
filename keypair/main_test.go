package keypair

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func TestBuild(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Package: github.com/stellar/go-stellar-sdk/keypair")
}

var (
	address     = "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"
	seed        = "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4"
	hint        = [4]byte{0x56, 0xfc, 0x05, 0xf7}
	payloadHint = [4]byte{0x33, 0x90, 0x69, 0x98}
	message     = []byte("hello")
	signature   = []byte{
		0x2e, 0x75, 0xcc, 0x20, 0xd5, 0x19, 0x11, 0x1c, 0xaa, 0xaa, 0xdd, 0xdf,
		0x46, 0x4b, 0xb6, 0x50, 0xd2, 0xea, 0xf0, 0xa5, 0xd1, 0x8d, 0x74, 0x56,
		0x93, 0xa1, 0x61, 0x00, 0xf2, 0xa4, 0x93, 0x7b, 0xc1, 0xdf, 0xfa, 0x8b,
		0x0b, 0x1f, 0x61, 0xa2, 0x76, 0x99, 0x6d, 0x7e, 0xe8, 0xde, 0xb2, 0xd0,
		0xdd, 0x9e, 0xe5, 0x10, 0x55, 0x60, 0x77, 0xb0, 0x2d, 0xec, 0x16, 0x79,
		0x2e, 0x91, 0x5c, 0x0a,
	}
)

func ItBehavesLikeAKP(subject *KP) {
	// NOTE: subject will only be valid to dereference when inside am "It"
	// example.

	Describe("Address()", func() {
		It("returns the correct address", func() {
			Expect((*subject).Address()).To(Equal(address))
		})
	})

	Describe("FromAddress()", func() {
		It("returns an address-only representation, or public key, of this key", func() {
			fromAddress := (*subject).FromAddress()
			Expect(fromAddress.Address()).To(Equal(address))
		})
	})

	Describe("Hint()", func() {
		It("returns the correct hint", func() {
			Expect((*subject).Hint()).To(Equal(hint))
		})
	})

	type VerifyCase struct {
		Message   []byte
		Signature []byte
		Case      types.GomegaMatcher
	}

	DescribeTable("Verify()",
		func(vc VerifyCase) {
			Expect((*subject).Verify(vc.Message, vc.Signature)).To(vc.Case)
		},
		Entry("correct", VerifyCase{message, signature, BeNil()}),
		Entry("empty signature", VerifyCase{message, []byte{}, Equal(ErrInvalidSignature)}),
		Entry("empty message", VerifyCase{[]byte{}, signature, Equal(ErrInvalidSignature)}),
		Entry("different message", VerifyCase{[]byte("diff"), signature, Equal(ErrInvalidSignature)}),
		Entry("malformed signature", VerifyCase{message, signature[0:10], Equal(ErrInvalidSignature)}),
	)
}

type ParseCase struct {
	Input    string
	TypeCase types.GomegaMatcher
	ErrCase  types.GomegaMatcher
}

var _ = DescribeTable("keypair.Parse()",
	func(c ParseCase) {
		kp, err := Parse(c.Input)

		Expect(kp).To(c.TypeCase)
		Expect(err).To(c.ErrCase)
	},

	Entry("a valid address", ParseCase{
		Input:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		TypeCase: BeAssignableToTypeOf(&FromAddress{}),
		ErrCase:  BeNil(),
	}),
	Entry("a corrupted address", ParseCase{
		Input:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7O32H",
		TypeCase: BeNil(),
		ErrCase:  HaveOccurred(),
	}),
	Entry("a valid seed", ParseCase{
		Input:    "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4",
		TypeCase: BeAssignableToTypeOf(&Full{}),
		ErrCase:  BeNil(),
	}),
	Entry("a corrupted seed", ParseCase{
		Input:    "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL3",
		TypeCase: BeNil(),
		ErrCase:  HaveOccurred(),
	}),
	Entry("a blank string", ParseCase{
		Input:    "",
		TypeCase: BeNil(),
		ErrCase:  HaveOccurred(),
	}),
)

type ParseFullCase struct {
	Input    string
	FullCase types.GomegaMatcher
	ErrCase  types.GomegaMatcher
}

var _ = DescribeTable("keypair.ParseFull()",
	func(c ParseFullCase) {
		kp, err := ParseFull(c.Input)

		Expect(kp).To(c.FullCase)
		Expect(err).To(c.ErrCase)
	},

	Entry("a valid address", ParseFullCase{
		Input:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		FullCase: BeNil(),
		ErrCase:  HaveOccurred(),
	}),
	Entry("a corrupted address", ParseFullCase{
		Input:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7O32H",
		FullCase: BeNil(),
		ErrCase:  HaveOccurred(),
	}),
	Entry("a valid seed", ParseFullCase{
		Input: "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4",
		FullCase: Equal(&Full{
			address:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			seed:       "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4",
			publicKey:  ed25519.PublicKey{98, 252, 29, 11, 208, 145, 178, 182, 28, 13, 214, 86, 52, 107, 42, 104, 215, 211, 71, 198, 242, 194, 200, 238, 109, 4, 71, 2, 86, 252, 5, 247},
			privateKey: ed25519.PrivateKey{206, 224, 48, 45, 89, 132, 77, 50, 189, 202, 145, 92, 130, 3, 221, 68, 179, 63, 187, 126, 220, 25, 5, 30, 163, 122, 190, 223, 40, 236, 212, 114, 98, 252, 29, 11, 208, 145, 178, 182, 28, 13, 214, 86, 52, 107, 42, 104, 215, 211, 71, 198, 242, 194, 200, 238, 109, 4, 71, 2, 86, 252, 5, 247},
		}),
		ErrCase: BeNil(),
	}),
	Entry("a corrupted seed", ParseFullCase{
		Input:    "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL3",
		FullCase: BeNil(),
		ErrCase:  HaveOccurred(),
	}),
	Entry("a blank string", ParseFullCase{
		Input:    "",
		FullCase: BeNil(),
		ErrCase:  HaveOccurred(),
	}),
)

type MustParseFullCase struct {
	Input    string
	FullCase types.GomegaMatcher
	FuncCase types.GomegaMatcher
}

var _ = DescribeTable("keypair.MustParseFull()",
	func(c MustParseFullCase) {
		f := func() {
			kp := MustParseFull(c.Input)
			Expect(kp).To(c.FullCase)
		}
		Expect(f).To(c.FuncCase)
	},

	Entry("a valid address", MustParseFullCase{
		Input:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		FullCase: BeNil(),
		FuncCase: Panic(),
	}),
	Entry("a corrupted address", MustParseFullCase{
		Input:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7O32H",
		FullCase: BeNil(),
		FuncCase: Panic(),
	}),
	Entry("a valid seed", MustParseFullCase{
		Input: "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4",
		FullCase: Equal(&Full{
			address:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			seed:       "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4",
			publicKey:  ed25519.PublicKey{98, 252, 29, 11, 208, 145, 178, 182, 28, 13, 214, 86, 52, 107, 42, 104, 215, 211, 71, 198, 242, 194, 200, 238, 109, 4, 71, 2, 86, 252, 5, 247},
			privateKey: ed25519.PrivateKey{206, 224, 48, 45, 89, 132, 77, 50, 189, 202, 145, 92, 130, 3, 221, 68, 179, 63, 187, 126, 220, 25, 5, 30, 163, 122, 190, 223, 40, 236, 212, 114, 98, 252, 29, 11, 208, 145, 178, 182, 28, 13, 214, 86, 52, 107, 42, 104, 215, 211, 71, 198, 242, 194, 200, 238, 109, 4, 71, 2, 86, 252, 5, 247},
		}),
		FuncCase: Not(Panic()),
	}),
	Entry("a corrupted seed", MustParseFullCase{
		Input:    "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL3",
		FullCase: BeNil(),
		FuncCase: Panic(),
	}),
	Entry("a blank string", MustParseFullCase{
		Input:    "",
		FullCase: BeNil(),
		FuncCase: Panic(),
	}),
)

type ParseAddressCase struct {
	Input       string
	AddressCase types.GomegaMatcher
	ErrCase     types.GomegaMatcher
}

var _ = DescribeTable("keypair.ParseAddress()",
	func(c ParseAddressCase) {
		kp, err := ParseAddress(c.Input)

		Expect(kp).To(c.AddressCase)
		Expect(err).To(c.ErrCase)
	},

	Entry("a valid address", ParseAddressCase{
		Input: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		AddressCase: Equal(&FromAddress{
			address:   "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			publicKey: ed25519.PublicKey{98, 252, 29, 11, 208, 145, 178, 182, 28, 13, 214, 86, 52, 107, 42, 104, 215, 211, 71, 198, 242, 194, 200, 238, 109, 4, 71, 2, 86, 252, 5, 247},
		}),
		ErrCase: BeNil(),
	}),
	Entry("a corrupted address", ParseAddressCase{
		Input:       "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7O32H",
		AddressCase: BeNil(),
		ErrCase:     HaveOccurred(),
	}),
	Entry("a valid seed", ParseAddressCase{
		Input:       "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4",
		AddressCase: BeNil(),
		ErrCase:     HaveOccurred(),
	}),
	Entry("a corrupted seed", ParseAddressCase{
		Input:       "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL3",
		AddressCase: BeNil(),
		ErrCase:     HaveOccurred(),
	}),
	Entry("a blank string", ParseAddressCase{
		Input:       "",
		AddressCase: BeNil(),
		ErrCase:     HaveOccurred(),
	}),
)

type MustParseAddressCase struct {
	Input       string
	AddressCase types.GomegaMatcher
	FuncCase    types.GomegaMatcher
}

var _ = DescribeTable("keypair.MustParseAddress()",
	func(c MustParseAddressCase) {
		f := func() {
			kp := MustParseAddress(c.Input)
			Expect(kp).To(c.AddressCase)
		}
		Expect(f).To(c.FuncCase)
	},

	Entry("a valid address", MustParseAddressCase{
		Input: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		AddressCase: Equal(&FromAddress{
			address:   "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			publicKey: ed25519.PublicKey{98, 252, 29, 11, 208, 145, 178, 182, 28, 13, 214, 86, 52, 107, 42, 104, 215, 211, 71, 198, 242, 194, 200, 238, 109, 4, 71, 2, 86, 252, 5, 247},
		}),
		FuncCase: Not(Panic()),
	}),
	Entry("a corrupted address", MustParseAddressCase{
		Input:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7O32H",
		FuncCase: Panic(),
	}),
	Entry("a valid seed", MustParseAddressCase{
		Input:    "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4",
		FuncCase: Panic(),
	}),
	Entry("a corrupted seed", MustParseAddressCase{
		Input:    "SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL3",
		FuncCase: Panic(),
	}),
	Entry("a blank string", MustParseAddressCase{
		Input:    "",
		FuncCase: Panic(),
	}),
)

var _ = Describe("keypair.Random()", func() {
	It("does not return the same value twice", func() {
		seen := map[string]bool{}
		for i := 0; i < 1000; i++ {
			kp, err := Random()
			Expect(err).To(BeNil())
			seed := kp.Seed()
			Expect(seen).ToNot(ContainElement(seed))
			seen[seed] = true
		}
	})
})

type errReader struct {
	Err error
}

func (r errReader) Read(_ []byte) (n int, err error) {
	return 0, r.Err
}

var _ = Describe("keypair.MustRandom()", func() {
	It("does not return the same value twice", func() {
		seen := map[string]bool{}
		for i := 0; i < 1000; i++ {
			kp := MustRandom()
			seed := kp.Seed()
			Expect(seen).ToNot(ContainElement(seed))
			seen[seed] = true
		}
	})

	Describe("when error", func() {
		var originalRandReader io.Reader
		BeforeEach(func() {
			originalRandReader = rand.Reader
			rand.Reader = errReader{Err: errors.New("an error")}
		})
		AfterEach(func() {
			rand.Reader = originalRandReader
		})
		It("panics", func() {
			defer func() {
				r := recover()
				Expect(r).ToNot(BeNil())
				Expect(r).To(Equal(errors.New("an error")))
			}()
			MustRandom()
		})
	})
})

var _ = Describe("keypair.Root()", func() {
	It("returns the root key pair for the passphrase", func() {
		networkPassphrase := "Standalone Network ; February 2017"
		kp := Root(networkPassphrase)
		seed := kp.Seed()
		Expect(seed).To(Equal("SC5O7VZUXDJ6JBDSZ74DSERXL7W3Y5LTOAMRF7RQRL3TAGAPS7LUVG3L"))
	})
})
