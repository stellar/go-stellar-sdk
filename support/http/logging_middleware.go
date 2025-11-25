package http

import (
	stdhttp "net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"

	"github.com/stellar/go-stellar-sdk/support/http/mutil"
	"github.com/stellar/go-stellar-sdk/support/log"
)

// Options allow the middleware logger to accept additional information.
type Options struct {
	ExtraHeaders []string
}

// SetLogger is a middleware that sets a logger on the context.
func SetLoggerMiddleware(l *log.Entry) func(stdhttp.Handler) stdhttp.Handler {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			ctx := r.Context()
			ctx = log.Set(ctx, l)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// LoggingMiddleware is a middleware that logs requests to the logger.
func LoggingMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return LoggingMiddlewareWithOptions(Options{})(next)
}

// LoggingMiddlewareWithOptions is a middleware that logs requests to the logger.
// Requires an Options struct to accept additional information.
func LoggingMiddlewareWithOptions(options Options) func(stdhttp.Handler) stdhttp.Handler {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			mw := mutil.WrapWriter(w)
			ctx := log.PushContext(r.Context(), func(l *log.Entry) *log.Entry {
				return l.WithFields(log.F{
					"req": middleware.GetReqID(r.Context()),
				})
			})
			r = r.WithContext(ctx)

			logStartOfRequest(r, options.ExtraHeaders)

			then := time.Now()
			next.ServeHTTP(mw, r)
			duration := time.Since(then)

			logEndOfRequest(r, duration, mw)
		})
	}
}

var routeRegexp = regexp.MustCompile("{([^:}]*):[^}]*}")

// https://prometheus.io/docs/instrumenting/exposition_formats/
// label_value can be any sequence of UTF-8 characters, but the backslash (\),
// double-quote ("), and line feed (\n) characters have to be escaped as \\,
// \", and \n, respectively.
func sanitizeMetricRoute(routePattern string) string {
	route := routeRegexp.ReplaceAllString(routePattern, "{$1}")
	route = strings.ReplaceAll(route, "\\", "\\\\")
	route = strings.ReplaceAll(route, "\"", "\\\"")
	route = strings.ReplaceAll(route, "\n", "\\n")
	if route == "" {
		// Can be empty when request did not reach the final route (ex. blocked by
		// a middleware). More info: https://github.com/go-chi/chi/issues/270
		return "undefined"
	}
	return route
}

// GetChiRoutePattern returns the chi route pattern from the given request context.
// Author: https://github.com/rliebz
// From: https://github.com/go-chi/chi/issues/270#issuecomment-479184559
// https://github.com/go-chi/chi/blob/master/LICENSE
func GetChiRoutePattern(r *stdhttp.Request) string {
	rctx := chi.RouteContext(r.Context())
	if pattern := rctx.RoutePattern(); pattern != "" {
		// Pattern is already available
		return pattern
	}

	routePath := r.URL.Path
	if r.URL.RawPath != "" {
		routePath = r.URL.RawPath
	}

	tctx := chi.NewRouteContext()
	if !rctx.Routes.Match(tctx, r.Method, routePath) {
		return ""
	}

	// tctx has the updated pattern, since Match mutates it
	return sanitizeMetricRoute(tctx.RoutePattern())
}

// logStartOfRequest emits the logline that reports that an http request is
// beginning processing.
func logStartOfRequest(
	r *stdhttp.Request,
	extraHeaders []string,
) {
	fields := log.F{}
	for _, header := range extraHeaders {
		// Strips "-" characters and lowercases new logrus.Fields keys to be uniform with the other keys in the logger.
		// Simplifies querying extended fields.
		var headerkey = strings.ToLower(strings.ReplaceAll(header, "-", ""))
		fields[headerkey] = r.Header.Get(header)
	}
	fields["subsys"] = "http"
	fields["path"] = r.URL.String()
	fields["method"] = r.Method
	fields["ip"] = r.RemoteAddr
	fields["host"] = r.Host
	fields["useragent"] = r.Header.Get("User-Agent")
	l := log.Ctx(r.Context()).WithFields(fields)

	l.Info("starting request")
}

// logEndOfRequest emits the logline for the end of the request
func logEndOfRequest(
	r *stdhttp.Request,
	duration time.Duration,
	mw mutil.WriterProxy,
) {
	l := log.Ctx(r.Context()).WithFields(log.F{
		"subsys":   "http",
		"path":     r.URL.String(),
		"method":   r.Method,
		"ip":       r.RemoteAddr,
		"status":   mw.Status(),
		"bytes":    mw.BytesWritten(),
		"duration": duration,
	})
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		l = l.WithField("route", routeContext.RoutePattern())
	}
	l.Info("finished request")
}
