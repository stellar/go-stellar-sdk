package apiclient

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/stellar/go-stellar-sdk/support/log"
)

func CreateRequestBody(requestType string, url string) (*http.Request, error) {
	req, err := http.NewRequest(requestType, url, nil)
	if err != nil {
		return nil, fmt.Errorf("http GET request creation failed: %w", err)
	}
	return req, nil
}

func SetHeaders(req *http.Request, args map[string]interface{}) {
	for key, value := range args {
		strValue, ok := value.(string)
		if !ok {
			log.Debugf("Skipping non-string value for header %s\n", key)
			continue
		}

		req.Header.Set(key, strValue)
	}
}

func SetAuthHeaders(req *http.Request, authType string, args map[string]interface{}) error {
	switch authType {
	case "basic":
		username, ok := args["username"].(string)
		if !ok {
			return fmt.Errorf("missing or invalid username")
		}
		password, ok := args["password"].(string)
		if !ok {
			return fmt.Errorf("missing or invalid password")
		}

		authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
		SetHeaders(req, map[string]interface{}{
			"Authorization": authHeader,
		})

	case "api_key":
		apiKey, ok := args["api_key"].(string)
		if !ok {
			return fmt.Errorf("missing or invalid API key")
		}
		SetHeaders(req, map[string]interface{}{
			"Authorization": apiKey,
		})

	default:
		return fmt.Errorf("unsupported auth type: %s", authType)
	}
	return nil
}
