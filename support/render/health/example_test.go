package health_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	supporthttp "github.com/stellar/go-stellar-sdk/support/http"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/support/render/health"
	"github.com/stellar/go-stellar-sdk/support/render/httpjson"
)

func ExampleResponse() {
	mux := supporthttp.NewAPIMux(log.DefaultLogger)

	mux.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		healthCheckResult := false
		response := health.Response{}
		if healthCheckResult {
			response.Status = health.StatusPass
		} else {
			response.Status = health.StatusFail
		}
		httpjson.Render(w, response, httpjson.HEALTHJSON)
	})

	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	resp := w.Result()

	fmt.Println("Content Type:", resp.Header.Get("Content-Type"))
	fmt.Println("Status Code:", resp.StatusCode)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("Body:", string(body))

	// Output:
	// Content Type: application/health+json; charset=utf-8
	// Status Code: 200
	// Body: {
	//   "status": "fail"
	// }
}

func ExampleHandler() {
	mux := supporthttp.NewAPIMux(log.DefaultLogger)

	mux.Get("/health", health.PassHandler{}.ServeHTTP)

	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	resp := w.Result()

	fmt.Println("Content Type:", resp.Header.Get("Content-Type"))
	fmt.Println("Status Code:", resp.StatusCode)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("Body:", string(body))

	// Output:
	// Content Type: application/health+json; charset=utf-8
	// Status Code: 200
	// Body: {
	//   "status": "pass"
	// }
}
