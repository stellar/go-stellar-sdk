package horizonclient

import (
	"testing"

	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/support/http/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiquidityPoolsRequestBuildUrl(t *testing.T) {
	// It should return valid liquidity_pools endpoint and no errors
	endpoint, err := LiquidityPoolsRequest{}.BuildURL()
	require.NoError(t, err)
	assert.Equal(t, "liquidity_pools", endpoint)

	// It should return valid liquidity_pools endpoint and no errors
	endpoint, err = LiquidityPoolsRequest{Order: OrderDesc}.BuildURL()
	require.NoError(t, err)
	assert.Equal(t, "liquidity_pools?order=desc", endpoint)

	// It should return valid liquidity_pools endpoint and no errors
	endpoint, err = LiquidityPoolsRequest{Reserves: []string{
		"EURT:GAP5LETOV6YIE62YAM56STDANPRDO7ZFDBGSNHJQIYGGKSMOZAHOOS2S",
		"PHP:GBUQWP3BOUZX34TOND2QV7QQ7K7VJTG6VSE7WMLBTMDJLLAW7YKGU6EP",
	}}.BuildURL()
	require.NoError(t, err)
	assert.Equal(t, "liquidity_pools?reserves=EURT%3AGAP5LETOV6YIE62YAM56STDANPRDO7ZFDBGSNHJQIYGGKSMOZAHOOS2S%2CPHP%3AGBUQWP3BOUZX34TOND2QV7QQ7K7VJTG6VSE7WMLBTMDJLLAW7YKGU6EP", endpoint)
}

func TestLiquidityPoolsRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	request := LiquidityPoolsRequest{}

	hmock.On(
		"GET",
		"https://localhost/liquidity_pools",
	).ReturnString(200, liquidityPoolsResponse)

	response, err := client.LiquidityPools(request)
	if assert.NoError(t, err) {
		assert.IsType(t, response, hProtocol.LiquidityPoolsPage{})
		links := response.Links
		assert.Equal(t, links.Self.Href, "https://horizon.stellar.org/liquidity_pools?limit=200\u0026order=asc")

		assert.Equal(t, links.Next.Href, "https://horizon.stellar.org/liquidity_pools?limit=200\u0026order=asc")

		record := response.Embedded.Records[0]
		assert.IsType(t, record, hProtocol.LiquidityPool{})
		assert.Equal(t, "abcdef", record.ID)
		assert.Equal(t, uint32(30), record.FeeBP)
		assert.Equal(t, uint64(300), record.TotalTrustlines)
		assert.Equal(t, "5000.0000000", record.TotalShares)
	}

	// failure response
	request = LiquidityPoolsRequest{}

	hmock.On(
		"GET",
		"https://localhost/liquidity_pools",
	).ReturnString(400, badRequestResponse)

	_, err = client.LiquidityPools(request)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "horizon error")
		horizonError, ok := err.(*Error)
		assert.Equal(t, ok, true)
		assert.Equal(t, horizonError.Problem.Title, "Bad Request")
	}
}

var liquidityPoolsResponse = `{
  "_links": {
    "self": {
      "href": "https://horizon.stellar.org/liquidity_pools?limit=200\u0026order=asc"
    },
    "next": {
      "href": "https://horizon.stellar.org/liquidity_pools?limit=200\u0026order=asc"
    }
  },
  "_embedded": {
    "records": [
			{
				"id": "abcdef",
				"paging_token": "abcdef",
				"fee_bp": 30,
				"type": "constant_product",
				"total_trustlines": "300",
				"total_shares": "5000.0000000",
				"reserves": [
					{
						"amount": "1000.0000005",
						"asset": "EURT:GAP5LETOV6YIE62YAM56STDANPRDO7ZFDBGSNHJQIYGGKSMOZAHOOS2S"
					},
					{
						"amount": "2000.0000000",
						"asset": "PHP:GBUQWP3BOUZX34TOND2QV7QQ7K7VJTG6VSE7WMLBTMDJLLAW7YKGU6EP"
					}
				]
			}
    ]
  }
}`
