package protocol

const GetHealthMethodName = "getHealth"

type GetHealthRequest struct{}

type GetHealthResponse struct {
	Status       string `json:"status"`
	LatestLedger uint32 `json:"latestLedger"`
	// LatestLedgerCloseTime is the time the latest ledger closed at, as a unix timestamp in seconds.
	LatestLedgerCloseTime int64  `json:"latestLedgerCloseTime,string"`
	OldestLedger          uint32 `json:"oldestLedger"`
	// OldestLedgerCloseTime is the time the oldest ledger closed at, as a unix timestamp in seconds.
	OldestLedgerCloseTime int64  `json:"oldestLedgerCloseTime,string"`
	LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
}
