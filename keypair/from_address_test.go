package keypair

import (
	"crypto/ed25519"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromAddress_Hint(t *testing.T) {
	kp := MustParseAddress("GAYUB4KATGTUZEGUMJEOZDPPWM4MQLHCIKC4T55YSXHN234WI6BJMIY2")
	assert.Equal(t, [4]byte{0x96, 0x47, 0x82, 0x96}, kp.Hint())
}

func TestParseAddress_RejectsShortPayload(t *testing.T) {
	shortPayload := make([]byte, 5)
	shortAddress, err := strkey.Encode(strkey.VersionByteAccountID, shortPayload)
	require.NoError(t, err)

	_, err = ParseAddress(shortAddress)
	assert.ErrorIs(t, err, ErrInvalidKey)
}

func TestParseAddress_RejectsLongPayload(t *testing.T) {
	longPayload := make([]byte, 64)
	longAddress, err := strkey.Encode(strkey.VersionByteAccountID, longPayload)
	require.NoError(t, err)

	_, err = ParseAddress(longAddress)
	assert.ErrorIs(t, err, ErrInvalidKey)
}

func TestParseAddress_AcceptsValid32BytePayload(t *testing.T) {
	validPayload := make([]byte, ed25519.PublicKeySize)
	validAddress, err := strkey.Encode(strkey.VersionByteAccountID, validPayload)
	require.NoError(t, err)

	kp, err := ParseAddress(validAddress)
	require.NoError(t, err)
	assert.Equal(t, validAddress, kp.Address())
}

func TestFromAddress_Equal(t *testing.T) {
	// A nil FromAddress.
	var kp0 *FromAddress

	// A FromAddress with a value.
	kp1 := MustParseAddress("GAYUB4KATGTUZEGUMJEOZDPPWM4MQLHCIKC4T55YSXHN234WI6BJMIY2")

	// Another FromAddress with a value.
	kp2 := MustParseAddress("GD5II5W6KQTJPES32LL6VJK6PLOHMEKYUXJPLERXUKR3MCLM3TNFSIPW")

	// A nil FromAddress should be equal to a nil FromAddress.
	assert.True(t, kp0.Equal(nil))

	// A non-nil FromAddress is not equal to a nil KP with no type.
	assert.False(t, kp1.Equal(nil))

	// A non-nil FromAddress is not equal to a nil FromAddress.
	assert.False(t, kp1.Equal(nil))

	// A non-nil FromAddress is equal to itself.
	assert.True(t, kp1.Equal(kp1))

	// A non-nil FromAddress is equal to another FromAddress containing the same address.
	assert.True(t, kp1.Equal(MustParseAddress(kp1.address)))

	// A non-nil FromAddress is not equal a non-nil FromAddress of different value.
	assert.False(t, kp1.Equal(kp2))
	assert.False(t, kp2.Equal(kp1))
}

var _ = Describe("keypair.FromAddress", func() {
	var subject KP

	JustBeforeEach(func() {
		subject = MustParse(address)
	})

	ItBehavesLikeAKP(&subject)

	Describe("Sign()", func() {
		It("fails", func() {
			_, err := subject.Sign(message)
			Expect(err).To(HaveOccurred())
		})
	})
	Describe("SignBase64()", func() {
		It("fails", func() {
			_, err := subject.SignBase64(message)
			Expect(err).To(HaveOccurred())
		})
	})
	Describe("SignDecorated()", func() {
		It("fails", func() {
			_, err := subject.SignDecorated(message)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("MarshalText()", func() {
		type Case struct {
			Input     *FromAddress
			BytesCase types.GomegaMatcher
			ErrCase   types.GomegaMatcher
		}
		DescribeTable("MarshalText()",
			func(c Case) {
				bytes, err := c.Input.MarshalText()
				Expect(bytes).To(c.BytesCase)
				Expect(err).To(c.ErrCase)
			},
			Entry("a valid address", Case{
				Input:     MustParseAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"),
				BytesCase: Equal([]byte("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")),
				ErrCase:   Not(HaveOccurred()),
			}),
			Entry("an empty address", Case{
				Input:     &FromAddress{},
				BytesCase: Equal([]byte("")),
				ErrCase:   Not(HaveOccurred()),
			}),
		)
	})

	Describe("UnmarshalText()", func() {
		type Case struct {
			Address     *FromAddress
			Input       []byte
			AddressCase types.GomegaMatcher
			ErrCase     types.GomegaMatcher
			FuncCase    types.GomegaMatcher
		}
		DescribeTable("UnmarshalText()",
			func(c Case) {
				f := func() {
					err := c.Address.UnmarshalText(c.Input)
					Expect(c.Address).To(c.AddressCase)
					Expect(err).To(c.ErrCase)
				}
				Expect(f).To(c.FuncCase)
			},
			Entry("a valid address into an empty FromAddress", Case{
				Address:     &FromAddress{},
				Input:       []byte("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"),
				AddressCase: Equal(MustParseAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")),
				ErrCase:     Not(HaveOccurred()),
				FuncCase:    Not(Panic()),
			}),
			Entry("an invalid address into an empty FromAddress", Case{
				Address:     &FromAddress{},
				Input:       []byte("?BRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"),
				AddressCase: Equal(&FromAddress{}),
				ErrCase:     HaveOccurred(),
				FuncCase:    Not(Panic()),
			}),
			Entry("a valid address into a nil FromAddress", Case{
				// This test case is included to indicate nil handling is not
				// supported. Handling this case is unnecessary because the
				// encoding packages in the stdlib protect against unmarshaling
				// into nil objects when calling Unmarshal directly on a nil
				// object and by allocating a new value in other cases.
				Address:     nil,
				Input:       []byte("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"),
				AddressCase: Equal(MustParseAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")),
				ErrCase:     Not(HaveOccurred()),
				FuncCase:    Panic(),
			}),
		)
	})

	Describe("MarshalBinary()", func() {
		type Case struct {
			Input     *FromAddress
			BytesCase types.GomegaMatcher
			ErrCase   types.GomegaMatcher
		}
		DescribeTable("MarshalBinary()",
			func(c Case) {
				bytes, err := c.Input.MarshalBinary()
				Expect(bytes).To(c.BytesCase)
				Expect(err).To(c.ErrCase)
			},
			Entry("a valid address", Case{
				Input:     MustParseAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"),
				BytesCase: Equal([]byte{0, 0, 0, 0, 98, 252, 29, 11, 208, 145, 178, 182, 28, 13, 214, 86, 52, 107, 42, 104, 215, 211, 71, 198, 242, 194, 200, 238, 109, 4, 71, 2, 86, 252, 5, 247}),
				ErrCase:   Not(HaveOccurred()),
			}),
			Entry("an empty address", Case{
				Input:     &FromAddress{},
				BytesCase: Equal([]byte("")),
				ErrCase:   HaveOccurred(),
			}),
		)
	})

	Describe("UnmarshalBinary()", func() {
		type Case struct {
			Address     *FromAddress
			Input       []byte
			AddressCase types.GomegaMatcher
			ErrCase     types.GomegaMatcher
			FuncCase    types.GomegaMatcher
		}
		DescribeTable("UnmarshalBinary()",
			func(c Case) {
				f := func() {
					err := c.Address.UnmarshalBinary(c.Input)
					Expect(c.Address).To(c.AddressCase)
					Expect(err).To(c.ErrCase)
				}
				Expect(f).To(c.FuncCase)
			},
			Entry("a valid address into an empty FromAddress", Case{
				Address:     &FromAddress{},
				Input:       []byte{0, 0, 0, 0, 98, 252, 29, 11, 208, 145, 178, 182, 28, 13, 214, 86, 52, 107, 42, 104, 215, 211, 71, 198, 242, 194, 200, 238, 109, 4, 71, 2, 86, 252, 5, 247},
				AddressCase: Equal(MustParseAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")),
				ErrCase:     Not(HaveOccurred()),
				FuncCase:    Not(Panic()),
			}),
			Entry("an invalid address into an empty FromAddress", Case{
				Address:     &FromAddress{},
				Input:       []byte{0, 0, 0, 1, 98, 252, 29, 11, 208, 145, 178, 182, 28, 13, 214, 86, 52, 107, 42, 104, 215, 211, 71, 198, 242, 194, 200, 238, 109, 4, 71, 2, 86, 252, 5, 247},
				AddressCase: Equal(&FromAddress{}),
				ErrCase:     HaveOccurred(),
				FuncCase:    Not(Panic()),
			}),
			Entry("a valid address into a nil FromAddress", Case{
				// This test case is included to indicate nil handling is not
				// supported. Handling this case is unnecessary because the
				// encoding packages in the stdlib protect against unmarshaling
				// into nil objects when calling Unmarshal directly on a nil
				// object and by allocating a new value in other cases.
				Address:     nil,
				Input:       []byte{0, 0, 0, 0, 98, 252, 29, 11, 208, 145, 178, 182, 28, 13, 214, 86, 52, 107, 42, 104, 215, 211, 71, 198, 242, 194, 200, 238, 109, 4, 71, 2, 86, 252, 5, 247},
				AddressCase: Equal(MustParseAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")),
				ErrCase:     Not(HaveOccurred()),
				FuncCase:    Panic(),
			}),
		)
	})
})
