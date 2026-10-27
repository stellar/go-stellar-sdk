package apiclient

import (
	"net/http"
	"net/url"
	"time"
)

type HTTP interface {
	Do(req *http.Request) (resp *http.Response, err error)
	Get(url string) (resp *http.Response, err error)
	PostForm(url string, data url.Values) (resp *http.Response, err error)
}

type APIClient struct {
	BaseURL            string
	HTTP               HTTP
	AuthType           string
	AuthHeaders        map[string]interface{}
	MaxRetries         int
	InitialBackoffTime time.Duration
}

type RequestParams struct {
	RequestType  string
	Endpoint     string
	QueryParams  url.Values
	Headers      map[string]interface{}
	ResponseType string
}

const (
	ResponseTypeJSON = "json"
	ResponseTypeRaw  = "raw"
)
