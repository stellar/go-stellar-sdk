package horizonclient

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// LiquidityPoolRequest struct contains data for getting liquidity pool details from a horizon server.
type LiquidityPoolRequest struct {
	LiquidityPoolID string
}

// BuildURL creates the endpoint to be queried based on the data in the LiquidityPoolRequest struct.
// If no data is set, it defaults to the build the URL for all assets
func (r LiquidityPoolRequest) BuildURL() (endpoint string, err error) {

	nParams := countParams(r.LiquidityPoolID)
	if nParams <= 0 {
		err = errors.New("invalid request: no parameters")
	}
	if err != nil {
		return endpoint, err
	}

	endpoint = fmt.Sprintf(
		"liquidity_pools/%s",
		r.LiquidityPoolID,
	)

	_, err = url.Parse(endpoint)
	if err != nil {
		err = errors.Wrap(err, "failed to parse endpoint")
	}

	return endpoint, err
}

// HTTPRequest returns the http request for the liquidity pool endpoint
func (r LiquidityPoolRequest) HTTPRequest(horizonURL string) (*http.Request, error) {
	endpoint, err := r.BuildURL()
	if err != nil {
		return nil, err
	}

	return http.NewRequest("GET", horizonURL+endpoint, nil)
}
