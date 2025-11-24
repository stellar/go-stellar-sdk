package horizonclient

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/support/http/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrictSendPathsRequestBuildUrl(t *testing.T) {
	pr := StrictSendPathsRequest{}
	endpoint, err := pr.BuildURL()

	// It should return no errors and paths endpoint
	// Horizon will return an error though because there are no parameters
	require.NoError(t, err)
	assert.Equal(t, "paths/strict-send", endpoint)

	pr = StrictSendPathsRequest{
		SourceAmount:       "100",
		SourceAssetCode:    "NGN",
		SourceAssetIssuer:  "GDZST3XVCDTUJ76ZAV2HA72KYQODXXZ5PTMAPZGDHZ6CS7RO7MGG3DBM",
		SourceAssetType:    AssetType4,
		DestinationAccount: "GDZST3XVCDTUJ76ZAV2HA72KYQODXXZ5PTMAPZGDHZ6CS7RO7MGG3DBM",
	}

	endpoint, err = pr.BuildURL()

	// It should return a valid endpoint and no errors
	require.NoError(t, err)
	assert.Equal(
		t,
		"paths/strict-send?destination_account=GDZST3XVCDTUJ76ZAV2HA72KYQODXXZ5PTMAPZGDHZ6CS7RO7MGG3DBM&source_amount=100&source_asset_code=NGN&source_asset_issuer=GDZST3XVCDTUJ76ZAV2HA72KYQODXXZ5PTMAPZGDHZ6CS7RO7MGG3DBM&source_asset_type=credit_alphanum4",
		endpoint,
	)

	pr = StrictSendPathsRequest{
		SourceAmount:      "100",
		SourceAssetCode:   "USD",
		SourceAssetIssuer: "GDUKMGUGDZQK6YHYA5Z6AY2G4XDSZPSZ3SW5UN3ARVMO6QSRDWP5YLEX",
		SourceAssetType:   AssetType4,
		DestinationAssets: "EURT:GAP5LETOV6YIE62YAM56STDANPRDO7ZFDBGSNHJQIYGGKSMOZAHOOS2S,native",
	}

	endpoint, err = pr.BuildURL()

	require.NoError(t, err)
	assert.Equal(
		t,
		"paths/strict-send?destination_assets=EURT%3AGAP5LETOV6YIE62YAM56STDANPRDO7ZFDBGSNHJQIYGGKSMOZAHOOS2S%2Cnative&source_amount=100&source_asset_code=USD&source_asset_issuer=GDUKMGUGDZQK6YHYA5Z6AY2G4XDSZPSZ3SW5UN3ARVMO6QSRDWP5YLEX&source_asset_type=credit_alphanum4",
		endpoint,
	)
}
func TestStrictSendPathsRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	pr := StrictSendPathsRequest{
		SourceAmount:       "20",
		SourceAssetCode:    "USD",
		SourceAssetIssuer:  "GDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN",
		SourceAssetType:    AssetType4,
		DestinationAccount: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
	}

	hmock.On(
		"GET",
		"https://localhost/paths/strict-send?destination_account=GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU&source_amount=20&source_asset_code=USD&source_asset_issuer=GDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN&source_asset_type=credit_alphanum4",
	).ReturnString(200, pathsResponse)

	paths, err := client.StrictSendPaths(pr)
	assert.NoError(t, err)
	assert.Len(t, paths.Embedded.Records, 3)

	pr = StrictSendPathsRequest{
		SourceAmount:      "20",
		SourceAssetCode:   "USD",
		SourceAssetIssuer: "GDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN",
		SourceAssetType:   AssetType4,
		DestinationAssets: "EUR:GDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN",
	}

	hmock.On(
		"GET",
		"https://localhost/paths/strict-send?destination_assets=EUR%3AGDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN&source_amount=20&source_asset_code=USD&source_asset_issuer=GDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN&source_asset_type=credit_alphanum4",
	).ReturnString(200, pathsResponse)

	paths, err = client.StrictSendPaths(pr)
	assert.NoError(t, err)
	assert.Len(t, paths.Embedded.Records, 3)

	pr = StrictSendPathsRequest{
		SourceAmount:       "20",
		SourceAssetCode:    "USD",
		SourceAssetIssuer:  "GDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN",
		SourceAssetType:    AssetType4,
		DestinationAssets:  "EUR:GDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN",
		DestinationAccount: "GDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN",
	}

	hmock.On(
		"GET",
		"https://localhost/paths/strict-send?destination_account=GDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN&destination_assets=EUR%3AGDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN&source_amount=20&source_asset_code=USD&source_asset_issuer=GDSBCQO34HWPGUGQSP3QBFEXVTSR2PW46UIGTHVWGWJGQKH3AFNHXHXN&source_asset_type=credit_alphanum4",
	).ReturnString(400, badRequestResponse)

	_, err = client.StrictSendPaths(pr)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "horizon error")
		horizonError, ok := err.(*Error)
		assert.Equal(t, ok, true)
		assert.Equal(t, horizonError.Problem.Title, "Bad Request")
	}
}
