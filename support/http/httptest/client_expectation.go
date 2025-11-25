package httptest

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jarcoal/httpmock"
	"github.com/stellar/go-stellar-sdk/support/errors"
)

// Return specifies the response for a ClientExpectation, which is then
// committed to the connected mock client.
func (ce *ClientExpectation) Return(r httpmock.Responder) *ClientExpectation {
	ce.Client.MockTransport.RegisterResponder(
		ce.Method,
		ce.URL,
		r,
	)
	return ce
}

// ReturnError causes this expectation to resolve to an error.
func (ce *ClientExpectation) ReturnError(msg string) *ClientExpectation {
	return ce.Return(func(*http.Request) (*http.Response, error) {
		return nil, errors.New(msg)
	})
}

// ReturnString causes this expectation to resolve to a string-based body with
// the provided status code.
func (ce *ClientExpectation) ReturnString(
	status int,
	body string,
) *ClientExpectation {
	return ce.Return(
		httpmock.NewStringResponder(status, body),
	)
}

// ReturnJSON causes this expectation to resolve to a json-based body with the
// provided status code.  Panics when the provided body cannot be encoded to
// JSON.
func (ce *ClientExpectation) ReturnJSON(
	status int,
	body interface{},
) *ClientExpectation {

	r, err := httpmock.NewJsonResponder(status, body)
	if err != nil {
		panic(err)
	}

	return ce.Return(r)
}

// ReturnNotFound is a simple helper that causes this expectation to resolve to
// a 404 error.  If a customized body is needed, use something like
// `ReturnString“ instead.
func (ce *ClientExpectation) ReturnNotFound() *ClientExpectation {
	return ce.ReturnString(http.StatusNotFound, "not found")
}

// ReturnStringWithHeader causes this expectation to resolve to a string-based body with
// the provided status code and response header.
func (ce *ClientExpectation) ReturnStringWithHeader(
	status int,
	body string,
	header http.Header,
) *ClientExpectation {

	req, err := ce.clientRequest()
	if err != nil {
		panic(err)
	}

	cResp := http.Response{
		Status:     strconv.Itoa(status),
		StatusCode: status,
		Body:       httpmock.NewRespBodyFromString(body),
		Header:     header,
		Request:    req,
	}

	return ce.Return(httpmock.ResponderFromResponse(&cResp))
}

// ReturnMultipleResults registers multiple sequential responses for a given client expectation.
// Useful for testing retries
func (ce *ClientExpectation) ReturnMultipleResults(responseSets []ResponseData) *ClientExpectation {
	var allResponses []httpmock.Responder
	for _, response := range responseSets {
		resp := http.Response{
			Status:     strconv.Itoa(response.Status),
			StatusCode: response.Status,
			Body:       httpmock.NewRespBodyFromString(response.Body),
			Header:     response.Header,
		}
		allResponses = append(allResponses, httpmock.ResponderFromResponse(&resp))
	}
	responseIndex := 0
	ce.Client.MockTransport.RegisterResponder(
		ce.Method,
		ce.URL,
		func(req *http.Request) (*http.Response, error) {
			if responseIndex >= len(allResponses) {
				panic(fmt.Errorf("no responses available"))
			}

			resp := allResponses[responseIndex]
			responseIndex++
			return resp(req)
		},
	)

	return ce
}

// ReturnJSONWithHeader causes this expectation to resolve to a json-based body with the provided
// status code and response header.  Panics when the provided body cannot be encoded to JSON.
func (ce *ClientExpectation) ReturnJSONWithHeader(
	status int,
	body interface{},
	header http.Header,
) *ClientExpectation {

	r, err := httpmock.NewJsonResponse(status, body)
	if err != nil {
		panic(err)
	}

	req, err := ce.clientRequest()
	if err != nil {
		panic(err)
	}

	r.Header = header
	r.Request = req
	return ce.Return(httpmock.ResponderFromResponse(r))
}

// clientRequest builds a http.Request struct from the supplied request parameters.
func (ce *ClientExpectation) clientRequest() (*http.Request, error) {
	rurl, err := url.Parse(ce.URL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse request url")
	}

	req := http.Request{
		Method: ce.Method,
		URL:    rurl,
		Host:   rurl.Host,
	}
	return &req, nil
}
