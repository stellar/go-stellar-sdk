package protocol

const GetNetworkMethodName = "getNetwork"

type GetNetworkRequest struct{}

type GetNetworkResponse struct {
	FriendbotURL                 string        `json:"friendbotUrl,omitempty"`
	Passphrase                   string        `json:"passphrase"`
	ProtocolVersion              int           `json:"protocolVersion"`
	CoreSupportedProtocolVersion int           `json:"coreSupportedProtocolVersion"`
	Limits                       NetworkLimits `json:"limits"`
}

// Converts SorobanInfoResponse to have camel case json tags
type NetworkLimits struct {
	// Contract settings
	MaxContractSize          uint32 `json:"maxContractSize,string"`
	MaxContractDataKeySize   uint32 `json:"maxContractDataKeySize,string"`
	MaxContractDataEntrySize uint32 `json:"maxContractDataEntrySize,string"`

	// Compute settings/per-transaction limits
	Tx NetworkLimitsTx `json:"tx"`

	// Ledger-wide limits
	Ledger NetworkLimitsLedger `json:"ledger"`

	FeeRatePerInstructionsIncrement int64 `json:"feeRatePerInstructionsIncrement,string"`
	// Fees
	FeeReadLedgerEntry       int64 `json:"feeReadLedgerEntry,string"`
	FeeWriteLedgerEntry      int64 `json:"feeWriteLedgerEntry,string"`
	FeeRead1KB               int64 `json:"feeRead1KB,string"`
	FeeWrite1KB              int64 `json:"feeWrite1KB,string"`
	FeeHistorical1KB         int64 `json:"feeHistorical1KB,string"`
	FeeContractEventsSize1KB int64 `json:"feeContractEventsSize1KB,string"`
	FeeTransactionSize1KB    int64 `json:"feeTransactionSize1KB,string"`

	// State archival settings
	StateArchival NetworkLimitsStateArchival `json:"stateArchival"`

	MaxDependentTxClusters uint32 `json:"maxDependentTxClusters,string"`

	// SCP timing config settings
	SCPSettings NetworkLimitsSCPSettings `json:"scp"`
}

type NetworkLimitsTx struct {
	MaxInstructions            int64  `json:"maxInstructions,string"`
	MemoryLimit                uint32 `json:"memoryLimit,string"`
	MaxReadLedgerEntries       uint32 `json:"maxReadLedgerEntries,string"`
	MaxReadBytes               uint32 `json:"maxReadBytes,string"`
	MaxWriteLedgerEntries      uint32 `json:"maxWriteLedgerEntries,string"`
	MaxWriteBytes              uint32 `json:"maxWriteBytes,string"`
	MaxFootprintSize           uint32 `json:"maxFootprintSize,string"`
	MaxContractEventsSizeBytes uint32 `json:"maxContractEventsSizeBytes,string"`
	MaxSizeBytes               uint32 `json:"maxSizeBytes,string"`
}

type NetworkLimitsLedger struct {
	MaxInstructions       int64  `json:"maxInstructions,string"`
	MaxReadLedgerEntries  uint32 `json:"maxReadLedgerEntries,string"`
	MaxReadBytes          uint32 `json:"maxReadBytes,string"`
	MaxWriteLedgerEntries uint32 `json:"maxWriteLedgerEntries,string"`
	MaxWriteBytes         uint32 `json:"maxWriteBytes,string"`
	MaxTxSizeBytes        uint32 `json:"maxTxSizeBytes,string"`
	MaxTxCount            uint32 `json:"maxTxCount,string"`
}

type NetworkLimitsStateArchival struct {
	MaxEntryTTL                    uint32 `json:"maxEntryTTL,string"`
	MinTemporaryTTL                uint32 `json:"minTemporaryTTL,string"`
	MinPersistentTTL               uint32 `json:"minPersistentTTL,string"`
	PersistentRentRateDenominator  int64  `json:"persistentRentRateDenominator,string"`
	TempRentRateDenominator        int64  `json:"tempRentRateDenominator,string"`
	MaxEntriesToArchive            uint32 `json:"maxEntriesToArchive,string"`
	BucketListSizeWindowSampleSize uint32 `json:"bucketListSizeWindowSampleSize,string"`
	EvictionScanSize               uint64 `json:"evictionScanSize,string"`
	StartingEvictionScanLevel      uint32 `json:"startingEvictionScanLevel,string"`
	BucketListSizeSnapshotPeriod   uint32 `json:"bucketListSizeSnapshotPeriod,string"`
	AverageBucketListSize          uint64 `json:"averageBucketListSize,string"`
}

type NetworkLimitsSCPSettings struct {
	LedgerCloseTimeMS      uint32 `json:"ledgerCloseTimeMS,string"`
	NominationTimeoutMS    uint32 `json:"nominationTimeoutMS,string"`
	NominationTimeoutIncMS uint32 `json:"nominationTimeoutIncMS,string"`
	BallotTimeoutMS        uint32 `json:"ballotTimeoutMS,string"`
	BallotTimeoutIncMS     uint32 `json:"ballotTimeoutIncMS,string"`
}
