package xdr_test

import (
	"testing"

	. "github.com/stellar/go-stellar-sdk/xdr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ = Describe("xdr.Asset#Extract()", func() {
	var asset Asset

	Context("asset is native", func() {
		BeforeEach(func() {
			var err error
			asset, err = NewAsset(AssetTypeAssetTypeNative, nil)
			Expect(err).To(BeNil())
		})

		It("can extract to AssetType", func() {
			var typ AssetType
			err := asset.Extract(&typ, nil, nil)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal(AssetTypeAssetTypeNative))
		})

		It("can extract to string", func() {
			var typ string
			err := asset.Extract(&typ, nil, nil)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal("native"))
		})
	})

	Context("asset is credit_alphanum4", func() {
		BeforeEach(func() {
			var err error
			an := AlphaNum4{}
			err = an.Issuer.SetAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
			Expect(err).To(BeNil())
			copy(an.AssetCode[:], []byte("USD"))

			asset, err = NewAsset(AssetTypeAssetTypeCreditAlphanum4, an)
			Expect(err).To(BeNil())
		})

		It("can extract when typ is AssetType", func() {
			var typ AssetType
			var code, issuer string

			err := asset.Extract(&typ, &code, &issuer)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal(AssetTypeAssetTypeCreditAlphanum4))
			Expect(code).To(Equal("USD"))
			Expect(issuer).To(Equal("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"))
		})

		It("can extract to strings", func() {
			var typ, code, issuer string

			err := asset.Extract(&typ, &code, &issuer)
			Expect(err).To(BeNil())
			Expect(typ).To(Equal("credit_alphanum4"))
			Expect(code).To(Equal("USD"))
			Expect(issuer).To(Equal("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"))
		})

	})
})

var _ = Describe("xdr.Asset#String()", func() {
	var asset Asset

	Context("asset is native", func() {
		BeforeEach(func() {
			var err error
			asset, err = NewAsset(AssetTypeAssetTypeNative, nil)
			Expect(err).To(BeNil())
		})

		It("returns 'native'", func() {
			Expect(asset.String()).To(Equal("native"))
		})
	})

	Context("asset is credit_alphanum4", func() {
		BeforeEach(func() {
			var err error
			an := AlphaNum4{}
			err = an.Issuer.SetAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
			Expect(err).To(BeNil())
			copy(an.AssetCode[:], []byte("USD"))

			asset, err = NewAsset(AssetTypeAssetTypeCreditAlphanum4, an)
			Expect(err).To(BeNil())
		})

		It("returns 'type/code/issuer'", func() {
			Expect(asset.String()).To(Equal("credit_alphanum4/USD/GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"))
		})
	})
})

func TestStringCanonical(t *testing.T) {
	asset := MustNewNativeAsset()
	require.Equal(t, "native", asset.StringCanonical())

	asset = MustNewCreditAsset("USD", "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	require.Equal(t, "USD:GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H", asset.StringCanonical())
}

var _ = Describe("xdr.Asset#Equals()", func() {
	var (
		issuer1       AccountId
		issuer2       AccountId
		usd4          [4]byte
		usd12         [12]byte
		eur4          [4]byte
		native        Asset
		usd4_issuer1  Asset
		usd4_issuer2  Asset
		usd12_issuer1 Asset
		eur4_issuer1  Asset
	)

	BeforeEach(func() {
		err := issuer1.SetAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
		Expect(err).To(BeNil())
		err = issuer2.SetAddress("GCCB23N7VU2U3JHZYSX7HKK6WBBUMHFP4GOXYOZDETXTICY6BR26EGJY")
		Expect(err).To(BeNil())

		copy(usd4[:], []byte("USD"))
		copy(usd12[:], []byte("USD"))
		copy(eur4[:], []byte("EUR"))

		native, err = NewAsset(AssetTypeAssetTypeNative, nil)
		Expect(err).To(BeNil())

		usd4_issuer1, err = NewAsset(AssetTypeAssetTypeCreditAlphanum4, AlphaNum4{
			Issuer:    issuer1,
			AssetCode: usd4,
		})
		Expect(err).To(BeNil())

		usd4_issuer2, err = NewAsset(AssetTypeAssetTypeCreditAlphanum4, AlphaNum4{
			Issuer:    issuer2,
			AssetCode: usd4,
		})
		Expect(err).To(BeNil())

		usd12_issuer1, err = NewAsset(AssetTypeAssetTypeCreditAlphanum12, AlphaNum12{
			Issuer:    issuer1,
			AssetCode: usd12,
		})
		Expect(err).To(BeNil())

		eur4_issuer1, err = NewAsset(AssetTypeAssetTypeCreditAlphanum4, AlphaNum4{
			Issuer:    issuer1,
			AssetCode: eur4,
		})
		Expect(err).To(BeNil())
	})

	It("returns true for self comparisons", func() {
		Expect(native.Equals(native)).To(BeTrue())
		Expect(usd4_issuer1.Equals(usd4_issuer1)).To(BeTrue())
		Expect(usd4_issuer2.Equals(usd4_issuer2)).To(BeTrue())
		Expect(usd12_issuer1.Equals(usd12_issuer1)).To(BeTrue())
		Expect(eur4_issuer1.Equals(eur4_issuer1)).To(BeTrue())
	})

	It("returns false for differences", func() {
		// type mismatch
		Expect(native.Equals(usd4_issuer1)).To(BeFalse())
		Expect(native.Equals(usd12_issuer1)).To(BeFalse())
		Expect(usd4_issuer1.Equals(usd12_issuer1)).To(BeFalse())

		// issuer mismatch
		Expect(usd4_issuer1.Equals(usd12_issuer1)).To(BeFalse())

		// code mismatch
		Expect(usd4_issuer1.Equals(eur4_issuer1)).To(BeFalse())
	})

})

func TestAssetSetCredit(t *testing.T) {
	issuer := AccountId{}
	issuer.SetAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")

	a := &Asset{}
	a.SetCredit("USD", issuer)
	assert.Nil(t, a.AlphaNum12)
	assert.NotNil(t, a.AlphaNum4)
	assert.Equal(t, AssetTypeAssetTypeCreditAlphanum4, a.Type)
	assert.Equal(t, issuer, a.AlphaNum4.Issuer)
	assert.Equal(t, AssetCode4{'U', 'S', 'D', 0}, a.AlphaNum4.AssetCode)

	a = &Asset{}
	a.SetCredit("USDUSD", issuer)
	assert.Nil(t, a.AlphaNum4)
	assert.NotNil(t, a.AlphaNum12)
	assert.Equal(t, AssetTypeAssetTypeCreditAlphanum12, a.Type)
	assert.Equal(t, issuer, a.AlphaNum12.Issuer)
	assert.Equal(t, AssetCode12{'U', 'S', 'D', 'U', 'S', 'D', 0, 0, 0, 0, 0, 0}, a.AlphaNum12.AssetCode)
}

func TestToAllowTrustOpAsset_AlphaNum4(t *testing.T) {
	a := &Asset{}
	at, err := a.ToAssetCode("ABCD")
	if assert.NoError(t, err) {
		code, ok := at.GetAssetCode4()
		assert.True(t, ok)
		var expected AssetCode4
		copy(expected[:], "ABCD")
		assert.Equal(t, expected, code)
	}
}

func TestToAllowTrustOpAsset_AlphaNum12(t *testing.T) {
	a := &Asset{}
	at, err := a.ToAssetCode("ABCDEFGHIJKL")
	if assert.NoError(t, err) {
		code, ok := at.GetAssetCode12()
		assert.True(t, ok)
		var expected AssetCode12
		copy(expected[:], "ABCDEFGHIJKL")
		assert.Equal(t, expected, code)
	}
}

func TestToAllowTrustOpAsset_Error(t *testing.T) {
	a := &Asset{}
	_, err := a.ToAssetCode("")
	assert.EqualError(t, err, "Asset code length is invalid")
}

func TestBuildAssets(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		value          string
		expectedAssets []Asset
		expectedError  string
	}{
		{
			"empty list",
			"",
			[]Asset{},
			"",
		},
		{
			"native",
			"native",
			[]Asset{MustNewNativeAsset()},
			"",
		},
		{
			"asset does not contain :",
			"invalid-asset",
			[]Asset{},
			"invalid-asset is not a valid asset",
		},
		{
			"asset contains more than one :",
			"usd:GAEDTJ4PPEFVW5XV2S7LUXBEHNQMX5Q2GM562RJGOQG7GVCE5H3HIB4V:",
			[]Asset{},
			"is not a valid asset",
		},
		{
			"unicode asset code",
			"üsd:GAEDTJ4PPEFVW5XV2S7LUXBEHNQMX5Q2GM562RJGOQG7GVCE5H3HIB4V",
			[]Asset{},
			"contains an invalid asset code",
		},
		{
			"asset code must be alpha numeric",
			"!usd:GAEDTJ4PPEFVW5XV2S7LUXBEHNQMX5Q2GM562RJGOQG7GVCE5H3HIB4V",
			[]Asset{},
			"contains an invalid asset code",
		},
		{
			"asset code contains backslash",
			"usd\\x23:GAEDTJ4PPEFVW5XV2S7LUXBEHNQMX5Q2GM562RJGOQG7GVCE5H3HIB4V",
			[]Asset{},
			"contains an invalid asset code",
		},
		{
			"contains null characters",
			"abcde\\x00:GAEDTJ4PPEFVW5XV2S7LUXBEHNQMX5Q2GM562RJGOQG7GVCE5H3HIB4V",
			[]Asset{},
			"contains an invalid asset code",
		},
		{
			"asset code is too short",
			":GAEDTJ4PPEFVW5XV2S7LUXBEHNQMX5Q2GM562RJGOQG7GVCE5H3HIB4V",
			[]Asset{},
			"is not a valid asset",
		},
		{
			"asset code is too long",
			"0123456789abc:GAEDTJ4PPEFVW5XV2S7LUXBEHNQMX5Q2GM562RJGOQG7GVCE5H3HIB4V",
			[]Asset{},
			"is not a valid asset",
		},
		{
			"issuer is empty",
			"usd:",
			[]Asset{},
			"contains an invalid issuer",
		},
		{
			"issuer is invalid",
			"usd:kkj9808;l",
			[]Asset{},
			"contains an invalid issuer",
		},
		{
			"validation succeeds",
			"usd:GAEDTJ4PPEFVW5XV2S7LUXBEHNQMX5Q2GM562RJGOQG7GVCE5H3HIB4V,usdabc:GAEDTJ4PPEFVW5XV2S7LUXBEHNQMX5Q2GM562RJGOQG7GVCE5H3HIB4V",
			[]Asset{
				MustNewCreditAsset("usd", "GAEDTJ4PPEFVW5XV2S7LUXBEHNQMX5Q2GM562RJGOQG7GVCE5H3HIB4V"),
				MustNewCreditAsset("usdabc", "GAEDTJ4PPEFVW5XV2S7LUXBEHNQMX5Q2GM562RJGOQG7GVCE5H3HIB4V"),
			},
			"",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			tt := assert.New(t)
			assets, err := BuildAssets(testCase.value)
			if testCase.expectedError == "" {
				tt.NoError(err)
				tt.Len(assets, len(testCase.expectedAssets))
				for i := range assets {
					tt.Equal(testCase.expectedAssets[i], assets[i])
				}
			} else {
				tt.Error(err)
				tt.Contains(err.Error(), testCase.expectedError)
			}
		})
	}
}

func TestBuildAsset(t *testing.T) {
	testCases := []struct {
		assetType string
		code      string
		issuer    string
		valid     bool
	}{
		{
			assetType: "native",
			valid:     true,
		},
		{
			assetType: "credit_alphanum4",
			code:      "USD",
			issuer:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			valid:     true,
		},
		{
			assetType: "credit_alphanum12",
			code:      "SPOOON",
			issuer:    "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
			valid:     true,
		},
		{
			assetType: "invalid",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.assetType, func(t *testing.T) {
			asset, err := BuildAsset(tc.assetType, tc.issuer, tc.code)

			if tc.valid {
				assert.NoError(t, err)
				var assetType, code, issuer string
				asset.Extract(&assetType, &code, &issuer)
				assert.Equal(t, tc.assetType, assetType)
				assert.Equal(t, tc.code, code)
				assert.Equal(t, tc.issuer, issuer)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAssetLessThan(t *testing.T) {
	xlm := MustNewNativeAsset()

	t.Run("returns false if assets are equal", func(t *testing.T) {
		assetA, err := NewCreditAsset(
			"ARST",
			"GB7TAYRUZGE6TVT7NHP5SMIZRNQA6PLM423EYISAOAP3MKYIQMVYP2JO",
		)
		require.NoError(t, err)

		assetB, err := NewCreditAsset(
			"USD",
			"GCEZWKCA5VLDNRLN3RPRJMRZOX3Z6G5CHCGSNFHEYVXM3XOJMDS674JZ",
		)
		require.NoError(t, err)

		assert.False(t, xlm.LessThan(xlm))
		assert.False(t, assetA.LessThan(assetA))
		assert.False(t, assetB.LessThan(assetB))
	})

	t.Run("test if asset types are being validated as native < anum4 < anum12", func(t *testing.T) {
		anum4, err := NewCreditAsset(
			"ARST",
			"GB7TAYRUZGE6TVT7NHP5SMIZRNQA6PLM423EYISAOAP3MKYIQMVYP2JO",
		)
		require.NoError(t, err)
		anum12, err := NewCreditAsset(
			"ARSTANUM12",
			"GB7TAYRUZGE6TVT7NHP5SMIZRNQA6PLM423EYISAOAP3MKYIQMVYP2JO",
		)
		require.NoError(t, err)

		assert.False(t, xlm.LessThan(xlm))
		assert.True(t, xlm.LessThan(anum4))
		assert.True(t, xlm.LessThan(anum12))

		assert.False(t, anum4.LessThan(xlm))
		assert.False(t, anum4.LessThan(anum4))
		assert.True(t, anum4.LessThan(anum12))

		assert.False(t, anum12.LessThan(xlm))
		assert.False(t, anum12.LessThan(anum4))
		assert.False(t, anum12.LessThan(anum12))
	})

	t.Run("test if asset codes are being validated as assetCodeA < assetCodeB", func(t *testing.T) {
		assetARST, err := NewCreditAsset(
			"ARST",
			"GB7TAYRUZGE6TVT7NHP5SMIZRNQA6PLM423EYISAOAP3MKYIQMVYP2JO",
		)
		require.NoError(t, err)
		assetUSDX, err := NewCreditAsset(
			"USDX",
			"GB7TAYRUZGE6TVT7NHP5SMIZRNQA6PLM423EYISAOAP3MKYIQMVYP2JO",
		)
		require.NoError(t, err)

		assert.False(t, assetARST.LessThan(assetARST))
		assert.True(t, assetARST.LessThan(assetUSDX))

		assert.False(t, assetUSDX.LessThan(assetARST))
		assert.False(t, assetUSDX.LessThan(assetUSDX))
	})

	t.Run("test if asset issuers are being validated as assetIssuerA < assetIssuerB", func(t *testing.T) {
		assetIssuerA, err := NewCreditAsset(
			"ARST",
			"GB7TAYRUZGE6TVT7NHP5SMIZRNQA6PLM423EYISAOAP3MKYIQMVYP2JO",
		)
		require.NoError(t, err)
		assetIssuerB, err := NewCreditAsset(
			"ARST",
			"GCEZWKCA5VLDNRLN3RPRJMRZOX3Z6G5CHCGSNFHEYVXM3XOJMDS674JZ",
		)
		require.NoError(t, err)

		assert.True(t, assetIssuerA.LessThan(assetIssuerB))
		assert.False(t, assetIssuerA.LessThan(assetIssuerA))

		assert.False(t, assetIssuerB.LessThan(assetIssuerA))
		assert.False(t, assetIssuerB.LessThan(assetIssuerB))
	})

	t.Run("test if codes with upper-case letters are sorted before lower-case letters", func(t *testing.T) {
		// All upper-case letters should come before any lower-case ones
		assetA, err := NewCreditAsset("B", "GA7NLOF4EHWMJF6DBXXV2H6AYI7IHYWNFZR6R52BYBLY7TE5Q74AIDRA")
		require.NoError(t, err)
		assetB, err := NewCreditAsset("a", "GA7NLOF4EHWMJF6DBXXV2H6AYI7IHYWNFZR6R52BYBLY7TE5Q74AIDRA")
		require.NoError(t, err)

		assert.True(t, assetA.LessThan(assetB))
	})
}

func BenchmarkAssetString(b *testing.B) {
	n := MustNewNativeAsset()
	a, err := NewCreditAsset(
		"ARST",
		"GB7TAYRUZGE6TVT7NHP5SMIZRNQA6PLM423EYISAOAP3MKYIQMVYP2JO",
	)
	require.NoError(b, err)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = n.String()
		_ = a.String()
	}
}

func BenchmarkAssetStringCanonical(b *testing.B) {
	n := MustNewNativeAsset()
	a, err := NewCreditAsset(
		"ARST",
		"GB7TAYRUZGE6TVT7NHP5SMIZRNQA6PLM423EYISAOAP3MKYIQMVYP2JO",
	)
	require.NoError(b, err)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = n.StringCanonical()
		_ = a.StringCanonical()
	}
}
