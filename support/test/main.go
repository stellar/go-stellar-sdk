// Package test contains simple test helpers that should not
// have any service-specific dependencies.
// think constants, custom matchers, generic helpers etc.
package test

import (
	"bytes"
	"context"

	"github.com/sirupsen/logrus"
	"github.com/stellar/go-stellar-sdk/support/log"
)

// ContextWithLogBuffer returns a context and a buffer into which the new, bound
// logger will write into.  This method allows you to inspect what data was
// logged more easily in your tests.
func ContextWithLogBuffer() (context.Context, *bytes.Buffer) {
	output := new(bytes.Buffer)
	l := log.New()
	l.SetOutput(output)
	l.DisableColors()
	l.SetLevel(logrus.DebugLevel)

	ctx := log.Set(context.Background(), l)
	return ctx, output
}
