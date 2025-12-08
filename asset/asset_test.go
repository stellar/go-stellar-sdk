package asset

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

var (
	assetCode = "USDC"
	issuer    = "GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN"
	usdcAsset = xdr.MustNewCreditAsset(assetCode, issuer)
)

func TestNewNativeAsset(t *testing.T) {
	nativeAsset := NewNativeAsset()

	assert.NotNil(t, nativeAsset, "Native asset should not be nil")
	assert.IsType(t, &Asset{}, nativeAsset, "Native asset should be of type *Asset")
	assert.IsType(t, &Asset_Native{}, nativeAsset.AssetType, "AssetType should be *Asset_Native")
	assert.True(t, nativeAsset.GetNative(), "Native asset should have Native set to true")
}

func TestNewProtoAsset(t *testing.T) {

	usdcProtoAsset := NewProtoAsset(usdcAsset)

	assert.NotNil(t, usdcProtoAsset, "asset should not be nil")
	assert.IsType(t, &Asset{}, usdcProtoAsset, "asset should be of type *Asset")
	assert.IsType(t, &Asset_IssuedAsset{}, usdcProtoAsset.AssetType, "AssetType should be *Asset_IssuedAsset")
	// Check issued asset fields
	assert.Equal(t, assetCode, usdcProtoAsset.GetIssuedAsset().AssetCode, "Asset code should match")
	assert.Equal(t, issuer, usdcProtoAsset.GetIssuedAsset().Issuer, "Issuer should match")

	xlmProtoAsset := NewProtoAsset(xdr.MustNewNativeAsset())
	assert.NotNil(t, xlmProtoAsset, "asset should not be nil")
	assert.IsType(t, &Asset{}, xlmProtoAsset, "asset should be of type *Asset")
	assert.IsType(t, &Asset_Native{}, xlmProtoAsset.AssetType, "AssetType should be *Asset_Native")
	assert.True(t, xlmProtoAsset.GetNative(), "Native asset should have Native set to true")
}

func TestAssetSerialization(t *testing.T) {
	original := NewProtoAsset(usdcAsset)

	serializedAsset, err := proto.Marshal(original)
	assert.NoError(t, err, "Failed to marshal asset")

	var deserializedAsset Asset
	err = proto.Unmarshal(serializedAsset, &deserializedAsset)
	assert.NoError(t, err, "Failed to unmarshal asset")

	assert.True(t, proto.Equal(original, &deserializedAsset), "Deserialized asset does not match the original")
}
