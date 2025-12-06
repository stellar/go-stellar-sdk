package stellarcore

// InfoResponse is the json response returned from stellar-core's /info
// endpoint.
type InfoResponse struct {
	Info struct {
		Build           string     `json:"build"`
		Network         string     `json:"network"`
		ProtocolVersion int        `json:"protocol_version"`
		State           string     `json:"state"`
		Ledger          LedgerInfo `json:"ledger"`
	}
}

// LedgerInfo is the part of the stellar-core's info json response.
// It's returned under `ledger` key
type LedgerInfo struct {
	Age          int    `json:"age"`
	BaseFee      int    `json:"baseFee"`
	BaseReserve  int    `json:"baseReserve"`
	CloseTime    int    `json:"closeTime"`
	Hash         string `json:"hash"`
	MaxTxSetSize int    `json:"maxTxSetSize"`
	Num          int    `json:"num"`
	Version      int    `json:"version"`
}

type SorobanInfoResponse struct {
	// Contract settings
	MaxContractSize          int `json:"max_contract_size"`
	MaxContractDataKeySize   int `json:"max_contract_data_key_size"`
	MaxContractDataEntrySize int `json:"max_contract_data_entry_size"`

	// Compute settings/per-transaction limits
	Tx struct {
		MaxInstructions            int64  `json:"max_instructions"`
		MemoryLimit                uint32 `json:"memory_limit"`
		MaxReadLedgerEntries       uint32 `json:"max_read_ledger_entries"`
		MaxReadBytes               uint32 `json:"max_read_bytes"`
		MaxWriteLedgerEntries      uint32 `json:"max_write_ledger_entries"`
		MaxWriteBytes              uint32 `json:"max_write_bytes"`
		MaxFootprintSize           uint32 `json:"max_footprint_size,omitempty"` // protocol v23+
		MaxContractEventsSizeBytes uint32 `json:"max_contract_events_size_bytes"`
		MaxSizeBytes               uint32 `json:"max_size_bytes"`
	} `json:"tx"`

	// Ledger-wide limits
	Ledger struct {
		MaxInstructions       int64  `json:"max_instructions"`
		MaxReadLedgerEntries  uint32 `json:"max_read_ledger_entries"`
		MaxReadBytes          uint32 `json:"max_read_bytes"`
		MaxWriteLedgerEntries uint32 `json:"max_write_ledger_entries"`
		MaxWriteBytes         uint32 `json:"max_write_bytes"`
		MaxTxSizeBytes        uint32 `json:"max_tx_size_bytes"`
		MaxTxCount            uint32 `json:"max_tx_count"`
	} `json:"ledger"`

	FeeRatePerInstructionsIncrement int64 `json:"fee_rate_per_instructions_increment"`

	// Fees
	FeeReadLedgerEntry       int64 `json:"fee_read_ledger_entry"`
	FeeWriteLedgerEntry      int64 `json:"fee_write_ledger_entry"`
	FeeRead1KB               int64 `json:"fee_read_1kb"`
	FeeWrite1KB              int64 `json:"fee_write_1kb"`
	FeeHistorical1KB         int64 `json:"fee_historical_1kb"`
	FeeContractEventsSize1KB int64 `json:"fee_contract_events_size_1kb"`
	FeeTransactionSize1KB    int64 `json:"fee_transaction_size_1kb"`

	// State archival settings
	StateArchival struct {
		MaxEntryTTL                    uint32 `json:"max_entry_ttl"`
		MinTemporaryTTL                uint32 `json:"min_temporary_ttl"`
		MinPersistentTTL               uint32 `json:"min_persistent_ttl"`
		PersistentRentRateDenominator  int64  `json:"persistent_rent_rate_denominator"`
		TempRentRateDenominator        int64  `json:"temp_rent_rate_denominator"`
		MaxEntriesToArchive            uint32 `json:"max_entries_to_archive"`
		BucketListSizeWindowSampleSize uint32 `json:"bucketlist_size_window_sample_size"`
		EvictionScanSize               uint64 `json:"eviction_scan_size"`
		StartingEvictionScanLevel      uint32 `json:"starting_eviction_scan_level"`
		BucketListSizeSnapshotPeriod   uint32 `json:"bucket_list_size_snapshot_period"`
		AverageBucketListSize          uint64 `json:"average_bucket_list_size"` // Non-configurable
	} `json:"state_archival"`

	MaxDependentTxClusters uint32 `json:"max_dependent_tx_clusters"` // protocol v23+

	// SCP timing config settings
	SCPSettings struct {
		LedgerCloseTimeMS      uint32 `json:"ledger_close_time_ms"`
		NominationTimeoutMS    uint32 `json:"nomination_timeout_ms"`
		NominationTimeoutIncMS uint32 `json:"nomination_timeout_inc_ms"`
		BallotTimeoutMS        uint32 `json:"ballot_timeout_ms"`
		BallotTimeoutIncMS     uint32 `json:"ballot_timeout_inc_ms"`
	} `json:"scp"`
}

// bool indicating whether stellarcore is synced with the network.
func (resp *InfoResponse) IsSynced() bool {
	return resp.Info.State == "Synced!"
}
