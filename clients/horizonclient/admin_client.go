package horizonclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/support/errors"
)

// port - the horizon admin port, zero value defaults to 4200
// host - the host interface name that horizon has bound admin web service, zero value defaults to 'localhost'
// timeout - the length of time for the http client to wait on responses from admin web service
func NewAdminClient(port uint16, host string, timeout time.Duration) (*AdminClient, error) {
	baseURL, err := getAdminBaseURL(port, host)
	if err != nil {
		return nil, err
	}
	if timeout == 0 {
		timeout = HorizonTimeout
	}

	return &AdminClient{
		baseURL:        baseURL,
		http:           http.DefaultClient,
		horizonTimeout: timeout,
	}, nil
}

func getAdminBaseURL(port uint16, host string) (string, error) {
	baseURL, err := url.Parse("http://localhost")
	if err != nil {
		return "", err
	}
	adminPort := uint16(4200)
	if port > 0 {
		adminPort = port
	}
	adminHost := baseURL.Hostname()
	if len(host) > 0 {
		adminHost = host
	}
	baseURL.Host = fmt.Sprintf("%s:%d", adminHost, adminPort)
	return baseURL.String(), nil
}

func (c *AdminClient) sendGetRequest(requestURL string, a interface{}) error {
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return errors.Wrap(err, "error creating Admin HTTP request")
	}
	return c.sendHTTPRequest(req, a)
}

func (c *AdminClient) sendHTTPRequest(req *http.Request, a interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.horizonTimeout)
	defer cancel()

	if resp, err := c.http.Do(req.WithContext(ctx)); err != nil {
		return err
	} else {
		return decodeResponse(resp, a, req.URL.String(), nil)
	}
}

func (c *AdminClient) getIngestionFiltersURL(filter string) string {
	return fmt.Sprintf("%s/ingestion/filters/%s", c.baseURL, filter)
}

func (c *AdminClient) GetIngestionAssetFilter() (hProtocol.AssetFilterConfig, error) {
	var filter hProtocol.AssetFilterConfig
	err := c.sendGetRequest(c.getIngestionFiltersURL("asset"), &filter)
	return filter, err
}

func (c *AdminClient) GetIngestionAccountFilter() (hProtocol.AccountFilterConfig, error) {
	var filter hProtocol.AccountFilterConfig
	err := c.sendGetRequest(c.getIngestionFiltersURL("account"), &filter)
	return filter, err
}

func (c *AdminClient) SetIngestionAssetFilter(filter hProtocol.AssetFilterConfig) error {
	buf := bytes.NewBuffer(nil)
	err := json.NewEncoder(buf).Encode(filter)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, c.getIngestionFiltersURL("asset"), buf)
	if err != nil {
		return errors.Wrap(err, "error creating HTTP request")
	}
	req.Header.Add("Content-Type", "application/json")
	return c.sendHTTPRequest(req, nil)
}

func (c *AdminClient) SetIngestionAccountFilter(filter hProtocol.AccountFilterConfig) error {
	buf := bytes.NewBuffer(nil)
	err := json.NewEncoder(buf).Encode(filter)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, c.getIngestionFiltersURL("account"), buf)
	if err != nil {
		return errors.Wrap(err, "error creating HTTP request")
	}
	req.Header.Add("Content-Type", "application/json")
	return c.sendHTTPRequest(req, nil)
}

// ensure that the horizon admin client implements AdminClientInterface
var _ AdminClientInterface = &AdminClient{}
