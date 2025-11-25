package horizonclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/support/errors"
)

// BuildURL creates the endpoint to be queried based on the data in the OfferRequest struct.
func (or OfferRequest) BuildURL() (endpoint string, err error) {
	if len(or.OfferID) > 0 {
		endpoint = fmt.Sprintf("offers/%s", or.OfferID)
	} else {
		// backwards compatibility  support
		if len(or.ForAccount) > 0 {
			endpoint = fmt.Sprintf("accounts/%s/offers", or.ForAccount)
			queryParams := addQueryParams(cursor(or.Cursor), limit(or.Limit), or.Order)
			if queryParams != "" {
				endpoint = fmt.Sprintf("%s?%s", endpoint, queryParams)
			}
		} else {
			query := url.Values{}
			if len(or.Seller) > 0 {
				query.Add("seller", or.Seller)
			}
			if len(or.Selling) > 0 {
				query.Add("selling", or.Selling)
			}
			if len(or.Buying) > 0 {
				query.Add("buying", or.Buying)
			}

			endpoint = fmt.Sprintf("offers?%s", query.Encode())
			pageParams := addQueryParams(cursor(or.Cursor), limit(or.Limit), or.Order)
			if pageParams != "" {
				endpoint = fmt.Sprintf("%s&%s", endpoint, pageParams)
			}
		}
	}

	_, err = url.Parse(endpoint)
	if err != nil {
		err = errors.Wrap(err, "failed to parse endpoint")
	}

	return endpoint, err
}

// HTTPRequest returns the http request for the offers endpoint
func (or OfferRequest) HTTPRequest(horizonURL string) (*http.Request, error) {
	endpoint, err := or.BuildURL()
	if err != nil {
		return nil, err
	}

	return http.NewRequest("GET", horizonURL+endpoint, nil)
}

// OfferHandler is a function that is called when a new offer is received
type OfferHandler func(hProtocol.Offer)

// StreamOffers streams offers processed by the Stellar network for an account. Use context.WithCancel
// to stop streaming or context.Background() if you want to stream indefinitely.
// OfferHandler is a user-supplied function that is executed for each streamed offer received.
func (or OfferRequest) StreamOffers(ctx context.Context, client *Client, handler OfferHandler) (err error) {
	endpoint, err := or.BuildURL()
	if err != nil {
		return errors.Wrap(err, "unable to build endpoint for offers request")
	}

	url := fmt.Sprintf("%s%s", client.fixHorizonURL(), endpoint)

	return client.stream(ctx, url, func(data []byte) error {
		var offer hProtocol.Offer
		err = json.Unmarshal(data, &offer)
		if err != nil {
			return errors.Wrap(err, "error unmarshaling data for offers request")
		}
		handler(offer)
		return nil
	})
}
