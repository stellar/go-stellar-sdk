package apiclient

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stellar/go-stellar-sdk/support/http/httptest"
	"github.com/stretchr/testify/assert"
)

func TestGetURL(t *testing.T) {
	c := &APIClient{
		BaseURL: "https://stellar.org",
	}

	queryParams := url.Values{}
	queryParams.Add("type", "forward")
	queryParams.Add("federation_type", "bank_account")
	queryParams.Add("swift", "BOPBPHMM")
	queryParams.Add("acct", "2382376")
	furl := c.GetURL("federation", queryParams)
	assert.Equal(t, "https://stellar.org/federation?acct=2382376&federation_type=bank_account&swift=BOPBPHMM&type=forward", furl)
}

type testCase struct {
	name          string
	mockResponses []httptest.ResponseData
	responseType  string
	expected      interface{}
	expectedError string
}

func TestCallAPI(t *testing.T) {
	tomlBody := "NETWORK_PASSPHRASE=\"Public Global Stellar Network ; September 2015\"\nFEDERATION_SERVER=\"https://aqua.network/federation\"\n"
	testCases := []testCase{
		{
			name: "status 200 - Success",
			mockResponses: []httptest.ResponseData{
				{Status: http.StatusOK, Body: `{"data": "Okay Response"}`, Header: nil},
			},
			expected:      map[string]interface{}{"data": "Okay Response"},
			expectedError: "",
		},
		{
			name: "success with retries - status 429 and 503 then 200",
			mockResponses: []httptest.ResponseData{
				{Status: http.StatusTooManyRequests, Body: `{"data": "First Response"}`, Header: nil},
				{Status: http.StatusServiceUnavailable, Body: `{"data": "Second Response"}`, Header: nil},
				{Status: http.StatusOK, Body: `{"data": "Third Response"}`, Header: nil},
				{Status: http.StatusOK, Body: `{"data": "Fourth Response"}`, Header: nil},
			},
			expected:      map[string]interface{}{"data": "Third Response"},
			expectedError: "",
		},
		{
			name: "failure - status 500",
			mockResponses: []httptest.ResponseData{
				{Status: http.StatusInternalServerError, Body: `{"error": "Internal Server Error"}`, Header: nil},
			},
			expected:      nil,
			expectedError: "API request failed with status 500",
		},
		{
			name: "failure - status 401",
			mockResponses: []httptest.ResponseData{
				{Status: http.StatusUnauthorized, Body: `{"error": "Bad authorization"}`, Header: nil},
			},
			expected:      nil,
			expectedError: "API request failed with status 401",
		},
		{
			name: "raw response - non-JSON body (stellar.toml)",
			mockResponses: []httptest.ResponseData{
				{Status: http.StatusOK, Body: tomlBody, Header: nil},
			},
			responseType:  ResponseTypeRaw,
			expected:      []byte(tomlBody),
			expectedError: "",
		},
		{
			name: "json response type - invalid JSON body fails",
			mockResponses: []httptest.ResponseData{
				{Status: http.StatusOK, Body: tomlBody, Header: nil},
			},
			responseType:  ResponseTypeJSON,
			expected:      nil,
			expectedError: "failed to unmarshal JSON: invalid character 'N' looking for beginning of value",
		},
		{
			name: "unsupported response type",
			mockResponses: []httptest.ResponseData{
				{Status: http.StatusOK, Body: `{"data": "x"}`, Header: nil},
			},
			responseType:  "xml",
			expected:      nil,
			expectedError: "unsupported response type: xml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hmock := httptest.NewClient()
			hmock.On("GET", "https://stellar.org/federation?acct=2382376").
				ReturnMultipleResults(tc.mockResponses)

			c := &APIClient{
				BaseURL: "https://stellar.org",
				HTTP:    hmock,
			}

			queryParams := url.Values{}
			queryParams.Add("acct", "2382376")

			reqParams := RequestParams{
				RequestType:  "GET",
				Endpoint:     "federation",
				QueryParams:  queryParams,
				ResponseType: tc.responseType,
			}

			result, err := c.CallAPI(reqParams)

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}

			if tc.expected != nil {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
