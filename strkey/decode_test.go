package strkey

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecode(t *testing.T) {
	cases := []struct {
		Name                string
		Address             string
		ExpectedVersionByte VersionByte
		ExpectedPayload     []byte
	}{
		{
			Name:                "AccountID",
			Address:             "GA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQHES5",
			ExpectedVersionByte: VersionByteAccountID,
			ExpectedPayload: []byte{
				0x36, 0x3e, 0xaa, 0x38, 0x67, 0x84, 0x1f, 0xba,
				0xd0, 0xf4, 0xed, 0x88, 0xc7, 0x79, 0xe4, 0xfe,
				0x66, 0xe5, 0x6a, 0x24, 0x70, 0xdc, 0x98, 0xc0,
				0xec, 0x9c, 0x07, 0x3d, 0x05, 0xc7, 0xb1, 0x03,
			},
		},
		{
			Name:                "MuxedAccount",
			Address:             "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK",
			ExpectedVersionByte: VersionByteMuxedAccount,
			ExpectedPayload: []byte{
				0x3f, 0x0c, 0x34, 0xbf, 0x93, 0xad, 0x0d, 0x99,
				0x71, 0xd0, 0x4c, 0xcc, 0x90, 0xf7, 0x05, 0x51,
				0x1c, 0x83, 0x8a, 0xad, 0x97, 0x34, 0xa4, 0xa2,
				0xfb, 0x0d, 0x7a, 0x03, 0xfc, 0x7f, 0xe8, 0x9a,
				0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			Name:                "Seed",
			Address:             "SBU2RRGLXH3E5CQHTD3ODLDF2BWDCYUSSBLLZ5GNW7JXHDIYKXZWHOKR",
			ExpectedVersionByte: VersionByteSeed,
			ExpectedPayload: []byte{
				0x69, 0xa8, 0xc4, 0xcb, 0xb9, 0xf6, 0x4e, 0x8a,
				0x07, 0x98, 0xf6, 0xe1, 0xac, 0x65, 0xd0, 0x6c,
				0x31, 0x62, 0x92, 0x90, 0x56, 0xbc, 0xf4, 0xcd,
				0xb7, 0xd3, 0x73, 0x8d, 0x18, 0x55, 0xf3, 0x63,
			},
		},
		{
			Name:                "HashTx",
			Address:             "TBU2RRGLXH3E5CQHTD3ODLDF2BWDCYUSSBLLZ5GNW7JXHDIYKXZWHXL7",
			ExpectedVersionByte: VersionByteHashTx,
			ExpectedPayload: []byte{
				0x69, 0xa8, 0xc4, 0xcb, 0xb9, 0xf6, 0x4e, 0x8a,
				0x07, 0x98, 0xf6, 0xe1, 0xac, 0x65, 0xd0, 0x6c,
				0x31, 0x62, 0x92, 0x90, 0x56, 0xbc, 0xf4, 0xcd,
				0xb7, 0xd3, 0x73, 0x8d, 0x18, 0x55, 0xf3, 0x63,
			},
		},
		{
			Name:                "HashX",
			Address:             "XBU2RRGLXH3E5CQHTD3ODLDF2BWDCYUSSBLLZ5GNW7JXHDIYKXZWGTOG",
			ExpectedVersionByte: VersionByteHashX,
			ExpectedPayload: []byte{
				0x69, 0xa8, 0xc4, 0xcb, 0xb9, 0xf6, 0x4e, 0x8a,
				0x07, 0x98, 0xf6, 0xe1, 0xac, 0x65, 0xd0, 0x6c,
				0x31, 0x62, 0x92, 0x90, 0x56, 0xbc, 0xf4, 0xcd,
				0xb7, 0xd3, 0x73, 0x8d, 0x18, 0x55, 0xf3, 0x63,
			},
		},
		{
			Name:                "SignedPayload",
			Address:             "PA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAAAAAEQAAAAAAAAAAAAAAAAAABBXA",
			ExpectedVersionByte: VersionByteSignedPayload,
			ExpectedPayload: []byte{
				// ed25519 (same as test case 1)
				0x36, 0x3e, 0xaa, 0x38, 0x67, 0x84, 0x1f, 0xba,
				0xd0, 0xf4, 0xed, 0x88, 0xc7, 0x79, 0xe4, 0xfe,
				0x66, 0xe5, 0x6a, 0x24, 0x70, 0xdc, 0x98, 0xc0,
				0xec, 0x9c, 0x07, 0x3d, 0x05, 0xc7, 0xb1, 0x03,
				// payload length
				0x00, 0x00, 0x00, 0x09,
				// payload
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00,
				// padding
				0x00, 0x00, 0x00,
			},
		},
		{
			Name:                "Contract",
			ExpectedVersionByte: VersionByteContract,
			ExpectedPayload: []byte{
				0x3f, 0x0c, 0x34, 0xbf, 0x93, 0xad, 0x0d, 0x99,
				0x71, 0xd0, 0x4c, 0xcc, 0x90, 0xf7, 0x05, 0x51,
				0x1c, 0x83, 0x8a, 0xad, 0x97, 0x34, 0xa4, 0xa2,
				0xfb, 0x0d, 0x7a, 0x03, 0xfc, 0x7f, 0xe8, 0x9a,
			},
			Address: "CA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUWDA",
		},
		{
			Name:                "Liquidity Pool",
			ExpectedVersionByte: VersionByteLiquidityPool,
			ExpectedPayload: []byte{
				0x3f, 0x0c, 0x34, 0xbf, 0x93, 0xad, 0x0d, 0x99,
				0x71, 0xd0, 0x4c, 0xcc, 0x90, 0xf7, 0x05, 0x51,
				0x1c, 0x83, 0x8a, 0xad, 0x97, 0x34, 0xa4, 0xa2,
				0xfb, 0x0d, 0x7a, 0x03, 0xfc, 0x7f, 0xe8, 0x9a,
			},
			Address: "LA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUPJN",
		},
		{
			Name:                "Claimable Balance",
			ExpectedVersionByte: VersionByteClaimableBalance,
			ExpectedPayload: []byte{
				// first byte represents ClaimableBalanceIDType (in this case it's V0)
				0x00,
				// the remaining 32 bytes are the contents of the claimable balance hash
				0x3f, 0x0c, 0x34, 0xbf, 0x93, 0xad, 0x0d, 0x99,
				0x71, 0xd0, 0x4c, 0xcc, 0x90, 0xf7, 0x05, 0x51,
				0x1c, 0x83, 0x8a, 0xad, 0x97, 0x34, 0xa4, 0xa2,
				0xfb, 0x0d, 0x7a, 0x03, 0xfc, 0x7f, 0xe8, 0x9a,
			},
			Address: "BAAD6DBUX6J22DMZOHIEZTEQ64CVCHEDRKWZONFEUL5Q26QD7R76RGR4TU",
		},
	}

	for _, kase := range cases {
		payload, err := Decode(kase.ExpectedVersionByte, kase.Address)
		if assert.NoError(t, err, "An error occurred decoding case %s", kase.Name) {
			assert.Equal(t, kase.ExpectedPayload, payload, "Output mismatch in case %s", kase.Name)
		}
	}

	// the expected version byte doesn't match the actual version byte
	_, err := Decode(VersionByteSeed, cases[0].Address)
	assert.Error(t, err)

	// invalid version byte
	_, err = Decode(VersionByte(2), cases[0].Address)
	assert.Error(t, err)

	// empty input
	_, err = Decode(VersionByteAccountID, "")
	assert.Error(t, err)

	// corrupted checksum
	_, err = Decode(VersionByteAccountID, "GA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQHE55")
	assert.Error(t, err)

	// corrupted payload
	_, err = Decode(VersionByteAccountID, "GA3D5KRYM6CB7OWOOOORR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQHES5")
	assert.Error(t, err)

	// non-canonical representation due to extra character
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLKA")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unused leftover character")
	}

	// non-canonical representation due to leftover bits set to 1 (some of the test strkeys are too short for a muxed account
	// but they comply with the test's purpose all the same)

	// 1 unused bit (length 69)
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLH")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unused bits should be set to 0")
	}
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUR")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unused bits should be set to 0")
	}
	// 4 unused bits (length 68)

	// 'B' is equivalent to 0b00001
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJB")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unused bits should be set to 0")
	}
	// 'C' is equivalent to 0b00010
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJC")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unused bits should be set to 0")
	}
	// 'E' is equivalent to 0b00100
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJE")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unused bits should be set to 0")
	}
	// 'I' is equivalent to 0b01000
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJI")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unused bits should be set to 0")
	}
	// '7' is equivalent to 0b11111
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJ7")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unused bits should be set to 0")
	}
	// '6' is equivalent to 0b11110
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJ6")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unused bits should be set to 0")
	}
	// '4' is equivalent to 0b11100
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJ4")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unused bits should be set to 0")
	}
	// 'Y' is equivalent to 0b11000
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJY")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "unused bits should be set to 0")
	}

	// 'Q' is equivalent to 0b10000, so there should be no error
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJQ")
	assert.NotContains(t, err.Error(), "unused bits should be set to 0")

	// Padding bytes are not allowed
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK===")
	assert.Contains(t, err.Error(), "illegal base32 data")

	// Invalid algorithm (low 3 bits of version byte are 7)
	_, err = Decode(VersionByteMuxedAccount, "M47QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUQ")
	assert.Contains(t, err.Error(), "invalid version byte")

	// Invalid checksum
	_, err = Decode(VersionByteMuxedAccount, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUO")
	assert.Contains(t, err.Error(), "invalid checksum")
}

func TestMalformed(t *testing.T) {
	// found by go-fuzz
	crashers := []string{
		"\n\n5JY",
		"UURL\xff\xff\xff\xff",
		"\r\r222",
	}

	for _, c := range crashers {
		t.Run("crashers "+c, func(t *testing.T) {
			assert.NotPanics(t, func() {
				_, err := Decode(VersionByteAccountID, c)
				assert.Error(t, err)
			})
		})
	}
}

func TestDecodeRejectsOversizedInput(t *testing.T) {
	oversized := strings.Repeat("A", maxEncodedSize+1)

	_, err := Decode(VersionByteAccountID, oversized)
	assert.ErrorContains(t, err, "maximum valid length")

	_, _, err = DecodeAny(oversized)
	assert.ErrorContains(t, err, "maximum valid length")

	_, err = Version(oversized)
	assert.ErrorContains(t, err, "maximum valid length")

	// Exactly-maxEncodedSize input must pass the length guard and reach
	// downstream validation (which will reject it for other reasons).
	boundary := strings.Repeat("A", maxEncodedSize)
	_, err = Decode(VersionByteAccountID, boundary)
	if assert.Error(t, err) {
		assert.NotContains(t, err.Error(), "maximum valid length")
	}
}
