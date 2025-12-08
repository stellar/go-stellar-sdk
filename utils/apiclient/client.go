package apiclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/stellar/go-stellar-sdk/support/log"
)

const (
	defaultMaxRetries         = 5
	defaultInitialBackoffTime = 1 * time.Second
)

func isRetryableStatusCode(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode == http.StatusServiceUnavailable
}

func (c *APIClient) GetURL(endpoint string, queryParams url.Values) string {
	return fmt.Sprintf("%s/%s?%s", c.BaseURL, endpoint, queryParams.Encode())
}

func (c *APIClient) CallAPI(reqParams RequestParams) (interface{}, error) {
	if reqParams.QueryParams == nil {
		reqParams.QueryParams = url.Values{}
	}

	if reqParams.Headers == nil {
		reqParams.Headers = map[string]interface{}{}
	}

	if c.MaxRetries == 0 {
		c.MaxRetries = defaultMaxRetries
	}

	if c.InitialBackoffTime == 0 {
		c.InitialBackoffTime = defaultInitialBackoffTime
	}

	if reqParams.Endpoint == "" {
		return nil, fmt.Errorf("Please set endpoint to query")
	}

	url := c.GetURL(reqParams.Endpoint, reqParams.QueryParams)
	reqBody, err := CreateRequestBody(reqParams.RequestType, url)
	if err != nil {
		return nil, fmt.Errorf("http request creation failed")
	}

	SetAuthHeaders(reqBody, c.AuthType, c.AuthHeaders)
	SetHeaders(reqBody, reqParams.Headers)
	client := c.HTTP
	if client == nil {
		client = &http.Client{}
	}

	var result interface{}
	retries := 0

	for retries <= c.MaxRetries {
		resp, err := client.Do(reqBody)
		if err != nil {
			return nil, fmt.Errorf("http request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read response body: %w", err)
			}

			if err := json.Unmarshal(body, &result); err != nil {
				return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
			}

			return result, nil
		} else if isRetryableStatusCode(resp.StatusCode) {
			retries++
			backoffDuration := c.InitialBackoffTime * time.Duration(1<<retries)
			if retries <= c.MaxRetries {
				log.Debugf("Received retryable status %d. Retrying in %v...\n", resp.StatusCode, backoffDuration)
				time.Sleep(backoffDuration)
			} else {
				return nil, fmt.Errorf("maximum retries reached after receiving status %d", resp.StatusCode)
			}
		} else {
			return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
		}
	}

	return nil, fmt.Errorf("API request failed after %d retries", retries)
}
