package protocol

const GetFeeStatsMethodName = "getFeeStats"

type GetFeeStatsRequest struct{}

type FeeDistribution struct {
	Max              uint64 `json:"max,string"`
	Min              uint64 `json:"min,string"`
	Mode             uint64 `json:"mode,string"`
	P10              uint64 `json:"p10,string"`
	P20              uint64 `json:"p20,string"`
	P30              uint64 `json:"p30,string"`
	P40              uint64 `json:"p40,string"`
	P50              uint64 `json:"p50,string"`
	P60              uint64 `json:"p60,string"`
	P70              uint64 `json:"p70,string"`
	P80              uint64 `json:"p80,string"`
	P90              uint64 `json:"p90,string"`
	P95              uint64 `json:"p95,string"`
	P99              uint64 `json:"p99,string"`
	TransactionCount uint32 `json:"transactionCount,string"`
	LedgerCount      uint32 `json:"ledgerCount"`
}

type GetFeeStatsResponse struct {
	SorobanInclusionFee FeeDistribution `json:"sorobanInclusionFee"`
	InclusionFee        FeeDistribution `json:"inclusionFee"`
	LatestLedger        uint32          `json:"latestLedger"`
}
