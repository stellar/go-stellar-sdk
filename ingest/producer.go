package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/support/datastore"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// provide testing hooks to inject mocks of these
var datastoreFactory = datastore.NewDataStore

// Generate a default buffered storage config with values
// set to optimize buffered performance to some degree based
// on number of ledgers per file expected in the underlying
// datastore used by an instance of BufferedStorageBackend.
//
// these numbers were derived empirically from benchmarking analysis:
// https://github.com/stellar/go-stellar-sdk/issues/5390
//
// ledgersPerFile - number of ledgers per file from remote datastore schema.
// return - preconfigured instance of BufferedStorageBackendConfig
func DefaultBufferedStorageBackendConfig(ledgersPerFile uint32) ledgerbackend.BufferedStorageBackendConfig {

	config := ledgerbackend.BufferedStorageBackendConfig{
		RetryLimit: 5,
		RetryWait:  30 * time.Second,
	}

	switch {
	case ledgersPerFile < 64:
		config.BufferSize = 100
		config.NumWorkers = 10
		return config
	default:
		config.BufferSize = 10
		config.NumWorkers = 2
		return config
	}
}

type PublisherConfig struct {
	// Registry, optional, include to capture buffered storage backend metrics
	Registry *prometheus.Registry
	// RegistryNamespace, optional, include to emit buffered storage backend
	// under this namespace
	RegistryNamespace string
	// BufferedStorageConfig, required
	BufferedStorageConfig ledgerbackend.BufferedStorageBackendConfig
	//DataStoreConfig, required
	DataStoreConfig datastore.DataStoreConfig
	// Log, optional, if nil uses go default logger
	Log *log.Entry
}

// ApplyLedgerMetadata - creates an internal instance
// of BufferedStorageBackend using provided config and emits
// ledger metadata for the requested range by invoking the provided callback
// once per ledger.
//
// The function is blocking, it will only return when a bounded range
// is completed, the ctx is canceled, or an error occurs.
//
// ledgerRange - the requested range, can be bounded or unbounded.
//
// publisherConfig - PublisherConfig. Provide configuration settings for DataStore
// and BufferedStorageBackend. Use DefaultBufferedStorageBackendConfig() to create
// optimized BufferedStorageBackendConfig.
//
// ctx - the context. Caller uses this to cancel the internal ledger processing,
// when canceled, the function will return asap with that error.
//
// callback - function. Invoked for every LedgerCloseMeta. If callback invocation
// returns an error, the processing will stop and return an error asap.
//
// return - error, function only returns if requested range is bounded or an error occured.
// nil will be returned only if bounded range requested and completed processing with no errors.
// otherwise return will always be an error.
func ApplyLedgerMetadata(ledgerRange ledgerbackend.Range,
	publisherConfig PublisherConfig,
	ctx context.Context,
	callback func(xdr.LedgerCloseMeta) error) error {

	logger := publisherConfig.Log
	if logger == nil {
		logger = log.DefaultLogger
	}

	dataStore, err := datastoreFactory(ctx, publisherConfig.DataStoreConfig)
	if err != nil {
		return fmt.Errorf("failed to create datastore: %w", err)
	}

	schema, err := datastore.LoadSchema(context.Background(), dataStore, publisherConfig.DataStoreConfig)
	if err != nil {
		return fmt.Errorf("failed to retrieve datastore schema: %w", err)
	}

	var ledgerBackend ledgerbackend.LedgerBackend
	ledgerBackend, err = ledgerbackend.NewBufferedStorageBackend(publisherConfig.BufferedStorageConfig, dataStore, schema)
	if err != nil {
		return fmt.Errorf("failed to create buffered storage backend: %w", err)
	}

	if publisherConfig.Registry != nil {
		ledgerBackend = ledgerbackend.WithMetrics(ledgerBackend, publisherConfig.Registry, publisherConfig.RegistryNamespace)
	}

	if ledgerRange.Bounded() && ledgerRange.To() <= ledgerRange.From() {
		return fmt.Errorf("invalid end value for bounded range, must be greater than start")
	}

	if !ledgerRange.Bounded() && ledgerRange.To() > 0 {
		return fmt.Errorf("invalid end value for unbounded range, must be zero")
	}

	from := max(2, ledgerRange.From())
	ledgerBackend.PrepareRange(ctx, ledgerRange)

	for ledgerSeq := from; ledgerSeq <= ledgerRange.To() || !ledgerRange.Bounded(); ledgerSeq++ {
		var ledgerCloseMeta xdr.LedgerCloseMeta

		logger.WithField("sequence", ledgerSeq).Info("Requesting ledger from the backend...")
		startTime := time.Now()
		ledgerCloseMeta, err = ledgerBackend.GetLedger(ctx, ledgerSeq)

		if err != nil {
			return fmt.Errorf("error getting ledger, %w", err)
		}

		log.WithFields(log.F{
			"sequence": ledgerSeq,
			"duration": time.Since(startTime).Seconds(),
		}).Info("Ledger returned from the backend")

		err = callback(ledgerCloseMeta)
		if err != nil {
			return fmt.Errorf("received an error from callback invocation: %w", err)
		}
	}
	return nil
}
