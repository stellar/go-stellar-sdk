package horizonclient

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// BuildURL creates the endpoint to be queried based on the data in the PathsRequest struct.
func (pr PathsRequest) BuildURL() (endpoint string, err error) {
	endpoint = "paths"

	// add the parameters to a map here so it is easier for addQueryParams to populate the parameter list
	// We can't use assetCode and assetIssuer types here because the paremeter names are different
	paramMap := make(map[string]string)
	paramMap["destination_account"] = pr.DestinationAccount
	paramMap["destination_asset_type"] = string(pr.DestinationAssetType)
	paramMap["destination_asset_code"] = pr.DestinationAssetCode
	paramMap["destination_asset_issuer"] = pr.DestinationAssetIssuer
	paramMap["destination_amount"] = pr.DestinationAmount
	paramMap["source_account"] = pr.SourceAccount
	paramMap["source_assets"] = pr.SourceAssets

	queryParams := addQueryParams(paramMap)
	if queryParams != "" {
		endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams)
	}

	_, err = url.Parse(endpoint)
	if err != nil {
		err = errors.Wrap(err, "failed to parse endpoint")
	}

	return endpoint, err
}

// HTTPRequest returns the http request for the path payment endpoint
func (pr PathsRequest) HTTPRequest(horizonURL string) (*http.Request, error) {
	endpoint, err := pr.BuildURL()
	if err != nil {
		return nil, err
	}

	return http.NewRequest("GET", horizonURL+endpoint, nil)
}
