package httpjson

import (
	"encoding/json"
	"net/http"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

type contentType int

const (
	JSON contentType = iota
	HALJSON
	HEALTHJSON
)

// renderToString renders the provided data as a json string
func renderToString(data interface{}, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(data, "", "  ")
	}

	return json.Marshal(data)
}

// Render write data to w, after marshaling to json. The response header is
// set based on cType.
func Render(w http.ResponseWriter, data interface{}, cType contentType) {
	RenderStatus(w, http.StatusOK, data, cType)
}

// RenderStatus write data to w, after marshaling to json.
// The response header is set based on cType.
// The response status code is set to the statusCode.
func RenderStatus(w http.ResponseWriter, statusCode int, data interface{}, cType contentType) {
	js, err := renderToString(data, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "inline")
	switch cType {
	case HALJSON:
		w.Header().Set("Content-Type", "application/hal+json; charset=utf-8")
	case HEALTHJSON:
		w.Header().Set("Content-Type", "application/health+json; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.WriteHeader(statusCode)
	w.Write(js)
}

var ErrBadRequest = errors.New("bad request")
