package horizonclient

import (
	"testing"

	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/support/http/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiquidityPoolRequestBuildUrl(t *testing.T) {
	// It should return valid liquidity_pool endpoint and no errors
	endpoint, err := LiquidityPoolRequest{}.BuildURL()
	assert.EqualError(t, err, "invalid request: no parameters")
	assert.Equal(t, "", endpoint)

	// It should return valid liquidity_pool endpoint and no errors
	endpoint, err = LiquidityPoolRequest{LiquidityPoolID: "abcdef"}.BuildURL()
	require.NoError(t, err)
	assert.Equal(t, "liquidity_pools/abcdef", endpoint)
}

func TestLiquidityPoolDetailRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	request := LiquidityPoolRequest{LiquidityPoolID: "abcdef"}

	hmock.On(
		"GET",
		"https://localhost/liquidity_pools/abcdef",
	).ReturnString(200, liquidityPoolResponse)

	response, err := client.LiquidityPoolDetail(request)
	if assert.NoError(t, err) {
		assert.IsType(t, response, hProtocol.LiquidityPool{})
		assert.Equal(t, "abcdef", response.ID)
		assert.Equal(t, uint32(30), response.FeeBP)
		assert.Equal(t, uint64(300), response.TotalTrustlines)
		assert.Equal(t, "5000.0000000", response.TotalShares)
	}

	// failure response
	request = LiquidityPoolRequest{LiquidityPoolID: "abcdef"}

	hmock.On(
		"GET",
		"https://localhost/liquidity_pools/abcdef",
	).ReturnString(400, badRequestResponse)

	_, err = client.LiquidityPoolDetail(request)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "horizon error")
		horizonError, ok := err.(*Error)
		assert.Equal(t, ok, true)
		assert.Equal(t, horizonError.Problem.Title, "Bad Request")
	}
}

var liquidityPoolResponse = `{
  "_links": {
    "self": {
      "href": "https://horizon.stellar.org/liquidity_pools/abcdef"
    }
  },
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
}`
