package protocol

const GetVersionInfoMethodName = "getVersionInfo"

type GetVersionInfoRequest struct{}

type GetVersionInfoResponse struct {
	Version            string `json:"version"`
	CommitHash         string `json:"commitHash"`
	BuildTimestamp     string `json:"buildTimestamp"`
	CaptiveCoreVersion string `json:"captiveCoreVersion"`
	ProtocolVersion    uint32 `json:"protocolVersion"`

	// Methods holds the sorted names of the methods the server serves. nil
	// means the server does not report them.
	Methods []string `json:"methods,omitempty"`
	// Limits holds the request limits the server enforces. nil means the
	// server does not report them.
	Limits *VersionInfoLimits `json:"limits,omitempty"`
}

// VersionInfoLimits holds the deployment-configured request limits a server
// reports, so a client can check a request before it sends it. A nil method
// field means the server does not report limits for that method.
type VersionInfoLimits struct {
	// MaxBatchSize is the largest number of calls one JSON-RPC batch may hold.
	// Zero means the server does not report it.
	MaxBatchSize uint `json:"maxBatchSize,omitempty"`

	GetEvents       *PageLimits        `json:"getEvents,omitempty"`
	GetLedgers      *PageLimits        `json:"getLedgers,omitempty"`
	GetTransactions *PageLimits        `json:"getTransactions,omitempty"`
	QueryEvents     *QueryEventsLimits `json:"queryEvents,omitempty"`
}

// PageLimits are the limits of a paginated method.
type PageLimits struct {
	// MaxLimit is the largest pagination limit a request may ask for; pass it
	// to the method's request validator, such as GetLedgersRequest.Validate.
	MaxLimit uint `json:"maxLimit"`
}

// QueryEventsLimits are the queryEvents limits a server enforces. A client
// can check a request against them before it sends it.
type QueryEventsLimits struct {
	// TermBudget caps the distinct index terms of one request; see
	// QueryEventsDefaultTermBudget.
	TermBudget uint32 `json:"termBudget"`
	// MaxFilters caps the filters list; pass it to QueryEventsRequest.Valid.
	MaxFilters MaxFilters `json:"maxFilters"`
	// MaxLimit is the largest limit a request may ask for.
	MaxLimit uint `json:"maxLimit"`
}
