// Package http provides easy access to Stellar's best practices for building
// http servers.  The primary method to use is `Serve`, which sets up
// an server that can support http/2 and can gracefully quit after receiving a
// SIGINT signal.
package http

import (
	stdhttp "net/http"
	"net/url"
	"os"
	"time"

	"github.com/stellar/go-stellar-sdk/support/config"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/support/log"
	"gopkg.in/tylerb/graceful.v1"
)

// defaultListenAddr represents the default address and port on which a server
// will listen, provided it is not overridden by setting the `ListenAddr` field
// on a `Config` struct.
const defaultListenAddr = "0.0.0.0:8080"

// defaultShutdownGracePeriod represents the default time in which the running
// process will allow outstanding http requests to complete before aborting
// them.  It will be used when a grace period of 0 is used, which normally
// signifies "no timeout" to our graceful shutdown package.  We choose not to
// provide a "no timeout" mode at present.  Feel free to set the value to a year
// or something if you need a timeout that is effectively "no timeout"; We
// believe that most servers should use a sane timeout and prefer one for the
// default configuration.
const defaultShutdownGracePeriod = 10 * time.Second

const defaultReadTimeout = 5 * time.Second

// SimpleHTTPClientInterface helps mocking http.Client in tests
type SimpleHTTPClientInterface interface {
	PostForm(url string, data url.Values) (*stdhttp.Response, error)
	Get(url string) (*stdhttp.Response, error)
}

// Config represents the configuration of an http server that can be provided to
// `Run`.
type Config struct {
	Handler             stdhttp.Handler
	ListenAddr          string
	TLS                 *config.TLS
	ShutdownGracePeriod time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	TCPKeepAlive        time.Duration
	OnStarting          func()
	OnStopping          func()
	OnStopped           func()
}

// Run starts an http server using the provided config struct.
//
// This method configures the process to listen for termination signals (SIGINT
// and SIGTERM) to trigger a graceful shutdown by way of the graceful package
// (https://github.com/tylerb/graceful).
func Run(conf Config) {
	srv := setup(conf)

	if conf.OnStarting != nil {
		conf.OnStarting()
	}

	var err error
	if conf.TLS != nil {
		err = srv.ListenAndServeTLS(conf.TLS.CertificateFile, conf.TLS.PrivateKeyFile)
	} else {
		err = srv.ListenAndServe()
	}

	if err != nil {
		log.Error(errors.Wrap(err, "failed to start server"))
		os.Exit(1)
	}

	if conf.OnStopped != nil {
		conf.OnStopped()
	}
	os.Exit(0)
}

// setup is a utility function to configure a new graceful server.  Its main
// purpose is to allow us to test the setup process without having to resort to
// a call the `Run`, which takes over the process.
func setup(conf Config) *graceful.Server {
	if conf.Handler == nil {
		panic("Handler must not be nil")
	}

	if conf.ListenAddr == "" {
		conf.ListenAddr = defaultListenAddr
	}

	if conf.ShutdownGracePeriod == time.Duration(0) {
		conf.ShutdownGracePeriod = defaultShutdownGracePeriod
	}

	if conf.ReadTimeout == time.Duration(0) {
		conf.ReadTimeout = defaultReadTimeout
	}

	return &graceful.Server{
		Timeout:      conf.ShutdownGracePeriod,
		TCPKeepAlive: conf.TCPKeepAlive,

		Server: &stdhttp.Server{
			Addr:         conf.ListenAddr,
			Handler:      conf.Handler,
			ReadTimeout:  conf.ReadTimeout,
			WriteTimeout: conf.WriteTimeout,
			IdleTimeout:  conf.IdleTimeout,
		},

		ShutdownInitiated: func() {
			if conf.OnStopping != nil {
				conf.OnStopping()
			}
		},
	}
}
