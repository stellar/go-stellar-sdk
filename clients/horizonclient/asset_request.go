package horizonclient

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// BuildURL creates the endpoint to be queried based on the data in the AssetRequest struct.
// If no data is set, it defaults to the build the URL for all assets
func (ar AssetRequest) BuildURL() (endpoint string, err error) {
	endpoint = "assets"
	queryParams := addQueryParams(assetCode(ar.ForAssetCode), assetIssuer(ar.ForAssetIssuer), cursor(ar.Cursor), limit(ar.Limit), ar.Order)
	if queryParams != "" {
		endpoint = fmt.Sprintf(
			"%s?%s",
			endpoint,
			queryParams,
		)
	}

	_, err = url.Parse(endpoint)
	if err != nil {
		err = errors.Wrap(err, "failed to parse endpoint")
	}

	return endpoint, err
}

// HTTPRequest returns the http request for the assets endpoint
func (ar AssetRequest) HTTPRequest(horizonURL string) (*http.Request, error) {
	endpoint, err := ar.BuildURL()
	if err != nil {
		return nil, err
	}

	return http.NewRequest("GET", horizonURL+endpoint, nil)
}
