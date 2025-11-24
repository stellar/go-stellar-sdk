package horizonclient

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// LiquidityPoolsRequest struct contains data for getting pool details from a horizon server.
// If "Reserves" is not set, it returns all liquidity pools.
// The query parameters (Order, Cursor and Limit) are optional. All or none can be set.
type LiquidityPoolsRequest struct {
	Cursor   string
	Limit    uint
	Order    Order
	Reserves []string
}

// BuildURL creates the endpoint to be queried based on the data in the LiquidityPoolRequest struct.
// If no data is set, it defaults to the build the URL for all assets
func (r LiquidityPoolsRequest) BuildURL() (endpoint string, err error) {
	endpoint = "liquidity_pools"

	if pageParams := addQueryParams(
		cursor(r.Cursor),
		limit(r.Limit),
		r.Order,
		reserves(r.Reserves),
	); len(pageParams) > 0 {
		endpoint = fmt.Sprintf(
			"%s?%s",
			endpoint,
			pageParams,
		)
	}

	_, err = url.Parse(endpoint)
	if err != nil {
		err = errors.Wrap(err, "failed to parse endpoint")
	}

	return endpoint, err
}

// HTTPRequest returns the http request for the pool endpoint
func (r LiquidityPoolsRequest) HTTPRequest(horizonURL string) (*http.Request, error) {
	endpoint, err := r.BuildURL()
	if err != nil {
		return nil, err
	}

	return http.NewRequest("GET", horizonURL+endpoint, nil)
}
