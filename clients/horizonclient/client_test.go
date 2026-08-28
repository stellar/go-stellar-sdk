package horizonclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupClient() *Client {
	return &Client{
		HorizonURL: "https://localhost/",
		AppName:    "client_test",
		AppVersion: "4.5.7",
	}
}

func TestSetClientAppHeaders_DefaultLogic(t *testing.T) {
	client := setupClient()

	request := LedgerRequest{
		Order: OrderAsc,
		Limit: 5,
	}

	req, err := request.HTTPRequest(client.HorizonURL)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(req.Header))

	client.setClientAppHeaders(req)
	assert.Equal(t, 4, len(req.Header))

	assert.Equal(t, "go-stellar-sdk", req.Header.Get("X-Client-Name"))
	assert.Equal(t, version, req.Header.Get("X-Client-Version"))
	assert.Equal(t, "client_test", req.Header.Get("X-App-Name"))
	assert.Equal(t, "4.5.7", req.Header.Get("X-App-Version"))
}

func TestSetClientAppHeaders_CustomHeadersLogic(t *testing.T) {
	client := setupClient()

	client.Headers = make(map[string]string)
	client.Headers["X-Api-Key"] = "abcde"

	request := LedgerRequest{
		Order: OrderAsc,
		Limit: 5,
	}

	req, err := request.HTTPRequest(client.HorizonURL)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(req.Header))

	client.setClientAppHeaders(req)
	assert.Equal(t, 5, len(req.Header))

	assert.Equal(t, "go-stellar-sdk", req.Header.Get("X-Client-Name"))
	assert.Equal(t, version, req.Header.Get("X-Client-Version"))
	assert.Equal(t, "client_test", req.Header.Get("X-App-Name"))
	assert.Equal(t, "4.5.7", req.Header.Get("X-App-Version"))
	assert.Equal(t, "abcde", req.Header.Get("X-Api-Key"))
}
