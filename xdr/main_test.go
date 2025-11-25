package xdr

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ExampleUnmarshal shows the lowest-level process to decode a base64
// envelope encoded in base64.
func ExampleUnmarshal() {
	data := "AAAAAgAAAABi/B0L0JGythwN1lY0aypo19NHxvLCyO5tBEcCVvwF9wAAAAoAAAAAAAAAAQAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAK6jei3jmoI8TGlD/egc37PXtHKKzWV8wViZBaCu5L5MAAAAADuaygAAAAAAAAAAAVb8BfcAAABACmeyD4/+Oj7llOmTrcjKLHLTQJF0TV/VggCOUZ30ZPgMsQy6A2T//Zdzb7MULVo/Y7kDrqAZRS51rvIp7YMUAA=="

	rawr := strings.NewReader(data)
	b64r := base64.NewDecoder(base64.StdEncoding, rawr)

	var tx TransactionEnvelope
	bytesRead, err := Unmarshal(b64r, &tx)

	fmt.Printf("read %d bytes\n", bytesRead)

	if err != nil {
		log.Fatal(err)
	}

	operations := tx.Operations()
	fmt.Printf("This tx has %d operations\n", len(operations))
	// Output: read 196 bytes
	// This tx has 1 operations
}

func TestSafeUnmarshalHex(t *testing.T) {
	accountID := MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML")
	hex, err := MarshalHex(accountID)
	assert.NoError(t, err)
	assert.Equal(t, "00000000b62e01510c1677279da72e6df492ada2320aceedd63360037786f8ed7f52075a", hex)
	var parsed AccountId
	err = SafeUnmarshalHex(hex, &parsed)
	assert.NoError(t, err)
	assert.True(t, accountID.Equals(parsed))
}

var _ = Describe("xdr.SafeUnmarshal", func() {
	var (
		result int32
		data   []byte
		err    error
	)

	JustBeforeEach(func() {
		err = SafeUnmarshal(data, &result)
	})

	Context("input data is a single xdr value", func() {
		BeforeEach(func() {
			data = []byte{0x00, 0x00, 0x00, 0x01}
		})

		It("succeeds", func() {
			Expect(err).To(BeNil())
		})

		It("decodes the data correctly", func() {
			Expect(result).To(Equal(int32(1)))
		})
	})

	Context("when the input data contains more than one encoded struct", func() {
		BeforeEach(func() {
			data = []byte{
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x01,
			}
		})
		It("errors", func() {
			Expect(err).ToNot(BeNil())
		})
	})
})

var _ = Describe("xdr.SafeUnmarshalBase64", func() {
	var (
		result int32
		data   string
		err    error
	)

	JustBeforeEach(func() {
		err = SafeUnmarshalBase64(data, &result)
	})

	Context("input data is a single xdr value", func() {
		BeforeEach(func() {
			data = "AAAAAQ=="
		})

		It("succeeds", func() {
			Expect(err).To(BeNil())
		})

		It("decodes the data correctly", func() {
			Expect(result).To(Equal(int32(1)))
		})
	})

	Context("when the input data contains more than one encoded struct", func() {
		BeforeEach(func() {
			data = "AAAAAQAAAAI="
		})
		It("errors", func() {
			Expect(err).ToNot(BeNil())
		})
	})
})

func TestLedgerKeyBinaryCompress(t *testing.T) {
	e := NewEncodingBuffer()
	for _, tc := range []struct {
		key         LedgerKey
		expectedOut []byte
	}{
		{
			key: LedgerKey{Type: LedgerEntryTypeAccount,
				Account: &LedgerKeyAccount{
					AccountId: MustAddress("GAOQJGUAB7NI7K7I62ORBXMN3J4SSWQUQ7FOEPSDJ322W2HMCNWPHXFB"),
				},
			},
			expectedOut: []byte{0x0, 0x0, 0x1d, 0x4, 0x9a, 0x80, 0xf, 0xda, 0x8f, 0xab, 0xe8, 0xf6, 0x9d, 0x10, 0xdd, 0x8d, 0xda, 0x79, 0x29, 0x5a, 0x14, 0x87, 0xca, 0xe2, 0x3e, 0x43, 0x4e, 0xf5, 0xab, 0x68, 0xec, 0x13, 0x6c, 0xf3},
		},
		{
			key: LedgerKey{
				Type: LedgerEntryTypeTrustline,
				TrustLine: &LedgerKeyTrustLine{
					AccountId: MustAddress("GAOQJGUAB7NI7K7I62ORBXMN3J4SSWQUQ7FOEPSDJ322W2HMCNWPHXFB"),
					Asset:     MustNewCreditAsset("EUR", "GAOQJGUAB7NI7K7I62ORBXMN3J4SSWQUQ7FOEPSDJ322W2HMCNWPHXFB").ToTrustLineAsset(),
				},
			},
			expectedOut: []byte{0x1, 0x0, 0x1d, 0x4, 0x9a, 0x80, 0xf, 0xda, 0x8f, 0xab, 0xe8, 0xf6, 0x9d, 0x10, 0xdd, 0x8d, 0xda, 0x79, 0x29, 0x5a, 0x14, 0x87, 0xca, 0xe2, 0x3e, 0x43, 0x4e, 0xf5, 0xab, 0x68, 0xec, 0x13, 0x6c, 0xf3, 0x1, 0x45, 0x55, 0x52, 0x0, 0x1d, 0x4, 0x9a, 0x80, 0xf, 0xda, 0x8f, 0xab, 0xe8, 0xf6, 0x9d, 0x10, 0xdd, 0x8d, 0xda, 0x79, 0x29, 0x5a, 0x14, 0x87, 0xca, 0xe2, 0x3e, 0x43, 0x4e, 0xf5, 0xab, 0x68, 0xec, 0x13, 0x6c, 0xf3},
		},
		{
			key: LedgerKey{
				Type: LedgerEntryTypeOffer,
				Offer: &LedgerKeyOffer{
					SellerId: MustAddress("GAOQJGUAB7NI7K7I62ORBXMN3J4SSWQUQ7FOEPSDJ322W2HMCNWPHXFB"),
					OfferId:  Int64(3),
				},
			},
			expectedOut: []byte{0x2, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x3},
		},
		{
			key: LedgerKey{
				Type: LedgerEntryTypeData,
				Data: &LedgerKeyData{
					AccountId: MustAddress("GAOQJGUAB7NI7K7I62ORBXMN3J4SSWQUQ7FOEPSDJ322W2HMCNWPHXFB"),
					DataName:  "foobar",
				},
			},
			expectedOut: []byte{0x3, 0x0, 0x1d, 0x4, 0x9a, 0x80, 0xf, 0xda, 0x8f, 0xab, 0xe8, 0xf6, 0x9d, 0x10, 0xdd, 0x8d, 0xda, 0x79, 0x29, 0x5a, 0x14, 0x87, 0xca, 0xe2, 0x3e, 0x43, 0x4e, 0xf5, 0xab, 0x68, 0xec, 0x13, 0x6c, 0xf3, 0x66, 0x6f, 0x6f, 0x62, 0x61, 0x72},
		},
		{
			key: LedgerKey{
				Type: LedgerEntryTypeClaimableBalance,
				ClaimableBalance: &LedgerKeyClaimableBalance{
					BalanceId: ClaimableBalanceId{
						Type: 0,
						V0:   &Hash{0xca, 0xfe, 0xba, 0xbe},
					},
				},
			},
			expectedOut: []byte{0x4, 0x0, 0xca, 0xfe, 0xba, 0xbe, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		},
		{
			key: LedgerKey{
				Type: LedgerEntryTypeLiquidityPool,
				LiquidityPool: &LedgerKeyLiquidityPool{
					LiquidityPoolId: PoolId{0xca, 0xfe, 0xba, 0xbe},
				},
			},
			expectedOut: []byte{0x5, 0xca, 0xfe, 0xba, 0xbe, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		},
		{
			key: LedgerKey{
				Type: LedgerEntryTypeConfigSetting,
				ConfigSetting: &LedgerKeyConfigSetting{
					ConfigSettingId: ConfigSettingIdConfigSettingContractMaxSizeBytes,
				},
			},
			expectedOut: []byte{0x8, 0x0, 0x0, 0x0, 0x0},
		},
	} {
		b, err := e.LedgerKeyUnsafeMarshalBinaryCompress(tc.key)
		assert.NoError(t, err)
		assert.Equal(t, tc.expectedOut, b)
	}
}

func TestLedgerKeyBinaryCompressCoverage(t *testing.T) {
	e := NewEncodingBuffer()
	gen := randxdr.NewGenerator()
	for i := 0; i < 10000; i++ {
		ledgerKey := LedgerKey{}

		shape := &gxdr.LedgerKey{}
		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		assert.NoError(t, gxdr.Convert(shape, &ledgerKey))

		compressed, err := e.LedgerKeyUnsafeMarshalBinaryCompress(ledgerKey)
		assert.NoError(t, err)
		keyType, err := GetBinaryCompressedLedgerKeyType(compressed)
		assert.NoError(t, err)
		assert.Equal(t, ledgerKey.Type, keyType)
	}
}

func TestCommitHashLength(t *testing.T) {
	require.Equal(t, 40, len(CommitHash))
}
