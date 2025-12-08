package problem

import (
	"context"
	"net/http"

	"github.com/stellar/go-stellar-sdk/support/log"
)

// DefaultServiceHost is the default service host used with the default problem
// instance.
var DefaultServiceHost = "https://stellar.org/horizon-errors/"

// DefaultLogger is the default logger used with the default problem instance.
var DefaultLogger = log.DefaultLogger

// Default is the problem instance used by the package functions providing a
// global state registry and rendering of problems for an application. For a
// non-global state registry instantiate a new Problem with New.
var Default = New(DefaultServiceHost, DefaultLogger, LogAllErrors)

// RegisterError records an error -> P mapping, allowing the app to register
// specific errors that may occur in other packages to be rendered as a specific
// P instance.
//
// For example, you might want to render any sql.ErrNoRows errors as a
// problem.NotFound, and you would do so by calling:
//
// problem.RegisterError(sql.ErrNoRows, problem.NotFound) in you application
// initialization sequence
func RegisterError(err error, p P) {
	Default.RegisterError(err, p)
}

// IsKnownError maps an error to a list of known errors
func IsKnownError(err error) error {
	return Default.IsKnownError(err)
}

// SetLogFilter sets log filter of the default Problem
func SetLogFilter(filter LogFilter) {
	Default.SetLogFilter(filter)
}

// UnRegisterErrors removes all registered errors
func UnRegisterErrors() {
	Default.UnRegisterErrors()
}

// RegisterHost registers the service host url. It is used to prepend the host
// url to the error type. If you don't wish to prepend anything to the error
// type, register host as an empty string.
// The default service host points to `https://stellar.org/horizon-errors/`.
func RegisterHost(host string) {
	Default.RegisterHost(host)
}

// RegisterReportFunc registers the report function that you want to use to
// report errors. Once reportFn is initialzied, it will be used to report
// unexpected errors.
func RegisterReportFunc(fn ReportFunc) {
	Default.RegisterReportFunc(fn)
}

// Render writes a http response to `w`, compliant with the "Problem
// Details for HTTP APIs" RFC:
// https://tools.ietf.org/html/draft-ietf-appsawg-http-problem-00
func Render(ctx context.Context, w http.ResponseWriter, err error) {
	Default.Render(ctx, w, err)
}
