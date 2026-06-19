package protocol

const GetHealthMethodName = "getHealth"

type GetHealthRequest struct{}

type GetHealthResponse struct {
	Status       string `json:"status"`
	LatestLedger uint32 `json:"latestLedger"`
	// LatestLedgerCloseTime is the time the latest ledger closed at, as a unix timestamp in seconds.
	LatestLedgerCloseTime int64  `json:"latestLedgerCloseTime"`
	OldestLedger          uint32 `json:"oldestLedger"`
	LedgerRetentionWindow uint32 `json:"ledgerRetentionWindow"`
}
