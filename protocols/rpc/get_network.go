package protocol

const GetNetworkMethodName = "getNetwork"

type GetNetworkRequest struct{}

type GetNetworkLimitsResponse struct {
	MaxContractSize          int    `json:"maxContractSize"`
	MaxContractDataKeySize   int    `json:"maxContractDataKeySize"`
	MaxContractDataEntrySize int    `json:"maxContractDataEntrySize"`
	MaxTxInstructions        int64  `json:"maxTxInstructions"`
	MaxTxReadLedgerEntries   uint32 `json:"maxTxReadLedgerEntries"`
	MaxTxReadBytes           uint32 `json:"maxTxReadBytes"`
	MaxTxWriteLedgerEntries  uint32 `json:"maxTxWriteLedgerEntries"`
	MaxTxWriteBytes          uint32 `json:"maxTxWriteBytes"`
}

type GetNetworkResponse struct {
	FriendbotURL    string                   `json:"friendbotUrl,omitempty"`
	Passphrase      string                   `json:"passphrase"`
	ProtocolVersion int                      `json:"protocolVersion"`
	Build           string                   `json:"build"`
	Limits          GetNetworkLimitsResponse `json:"limits"`
}
