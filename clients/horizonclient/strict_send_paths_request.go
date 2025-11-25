package horizonclient

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// BuildURL creates the endpoint to be queried based on the data in the PathsRequest struct.
func (pr StrictSendPathsRequest) BuildURL() (endpoint string, err error) {
	endpoint = "paths/strict-send"

	// add the parameters to a map here so it is easier for addQueryParams to populate the parameter list
	// We can't use assetCode and assetIssuer types here because the parameter names are different
	paramMap := make(map[string]string)
	paramMap["destination_assets"] = pr.DestinationAssets
	paramMap["destination_account"] = pr.DestinationAccount
	paramMap["source_asset_type"] = string(pr.SourceAssetType)
	paramMap["source_asset_code"] = pr.SourceAssetCode
	paramMap["source_asset_issuer"] = pr.SourceAssetIssuer
	paramMap["source_amount"] = pr.SourceAmount

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

// HTTPRequest returns the http request for the strict send path payment endpoint
func (pr StrictSendPathsRequest) HTTPRequest(horizonURL string) (*http.Request, error) {
	endpoint, err := pr.BuildURL()
	if err != nil {
		return nil, err
	}

	return http.NewRequest("GET", horizonURL+endpoint, nil)
}
