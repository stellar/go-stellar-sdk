package horizonclient

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// BuildURL creates the endpoint to be queried based on the data in the AccountsRequest struct.
// Either "Signer" or "Asset" fields should be set when retrieving Accounts.
// At the moment, you can't use both filters at the same time.
func (r AccountsRequest) BuildURL() (endpoint string, err error) {

	nParams := countParams(r.Signer, r.Asset, r.Sponsor, r.LiquidityPool)

	if nParams <= 0 {
		err = errors.New("invalid request: no parameters - Signer, Asset, Sponsor, or LiquidityPool must be provided")
	}

	if nParams > 1 {
		err = errors.New("invalid request: too many parameters - Multiple filters provided, provide a single filter")
	}

	if err != nil {
		return endpoint, err
	}
	query := url.Values{}
	switch {
	case len(r.Signer) > 0:
		query.Add("signer", r.Signer)

	case len(r.Asset) > 0:
		query.Add("asset", r.Asset)

	case len(r.Sponsor) > 0:
		query.Add("sponsor", r.Sponsor)

	case len(r.LiquidityPool) > 0:
		query.Add("liquidity_pool", r.LiquidityPool)
	}

	endpoint = fmt.Sprintf(
		"accounts?%s",
		query.Encode(),
	)

	if pageParams := addQueryParams(cursor(r.Cursor), limit(r.Limit), r.Order); len(pageParams) > 0 {
		endpoint = fmt.Sprintf(
			"%s&%s",
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

// HTTPRequest returns the http request for the accounts endpoint
func (r AccountsRequest) HTTPRequest(horizonURL string) (*http.Request, error) {
	endpoint, err := r.BuildURL()
	if err != nil {
		return nil, err
	}

	return http.NewRequest("GET", horizonURL+endpoint, nil)
}
