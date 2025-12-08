package protocol

const GetLatestLedgerMethodName = "getLatestLedger"

type GetLatestLedgerRequest struct{}

type GetLatestLedgerResponse struct {
	// Hash of the latest ledger as a hex-encoded string
	Hash string `json:"id"`
	// Stellar Core protocol version associated with the ledger.
	ProtocolVersion uint32 `json:"protocolVersion"`
	// Sequence number of the latest ledger.
	Sequence uint32 `json:"sequence"`
	// Time the ledger closed at (unix timestamp)
	LedgerCloseTime int64 `json:"closeTime,string"`
	// LedgerHeader of the latest ledger (base64-encoded XDR)
	LedgerHeader string `json:"headerXdr"`
	// LedgerMetadata of the latest ledger (base64-encoded XDR)
	LedgerMetadata string `json:"metadataXdr"`
}
