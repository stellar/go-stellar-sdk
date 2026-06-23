package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	SimulateTransactionMethodName        = "simulateTransaction"
	DefaultInstructionLeeway      uint64 = 0

	AuthModeEnforce            = "enforce"
	AuthModeRecord             = "record"
	AuthModeRecordAllowNonroot = "record_allow_nonroot"
)

type SimulateTransactionRequest struct {
	Transaction    string          `json:"transaction"`
	ResourceConfig *ResourceConfig `json:"resourceConfig,omitempty"`
	AuthMode       string          `json:"authMode,omitempty"`
	Format         string          `json:"xdrFormat,omitempty"`

	// UseUpgradedAuth opts simulation into recording AddressV2 ("upgraded")
	// authorization credentials instead of the legacy Address credentials. It is
	// best-effort: it only affects the recording auth modes and is silently
	// ignored on protocol versions whose host cannot emit AddressV2.
	//
	// Deprecated: this flag is transitional. Once the RPC returns AddressV2
	// credentials by default it becomes a no-op, so do not rely on omitting it
	// to keep receiving the legacy Address format.
	UseUpgradedAuth bool `json:"useUpgradedAuth,omitempty"`
}

type ResourceConfig struct {
	InstructionLeeway uint64 `json:"instructionLeeway"`
}

func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		InstructionLeeway: DefaultInstructionLeeway,
	}
}

// SimulateHostFunctionResult contains the simulation result of each HostFunction within the single
// InvokeHostFunctionOp allowed in a Transaction
type SimulateHostFunctionResult struct {
	AuthXDR  *[]string         `json:"auth,omitempty"`
	AuthJSON []json.RawMessage `json:"authJson,omitempty"`

	ReturnValueXDR  *string         `json:"xdr,omitempty"`
	ReturnValueJSON json.RawMessage `json:"returnValueJson,omitempty"`
}

type RestorePreamble struct {
	// TransactionDataXDR is an xdr.SorobanTransactionData in base64
	TransactionDataXDR  string          `json:"transactionData,omitempty"`
	TransactionDataJSON json.RawMessage `json:"transactionDataJson,omitempty"`

	MinResourceFee int64 `json:"minResourceFee,string"`
}
type LedgerEntryChangeType int //nolint:recvcheck

const (
	LedgerEntryChangeTypeCreated LedgerEntryChangeType = iota + 1
	LedgerEntryChangeTypeUpdated
	LedgerEntryChangeTypeDeleted
)

var (
	LedgerEntryChangeTypeName = map[LedgerEntryChangeType]string{ //nolint:gochecknoglobals
		LedgerEntryChangeTypeCreated: "created",
		LedgerEntryChangeTypeUpdated: "updated",
		LedgerEntryChangeTypeDeleted: "deleted",
	}
	LedgerEntryChangeTypeValue = map[string]LedgerEntryChangeType{ //nolint:gochecknoglobals
		"created": LedgerEntryChangeTypeCreated,
		"updated": LedgerEntryChangeTypeUpdated,
		"deleted": LedgerEntryChangeTypeDeleted,
	}
)

func (l LedgerEntryChangeType) String() string {
	return LedgerEntryChangeTypeName[l]
}

func (l LedgerEntryChangeType) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

func (l *LedgerEntryChangeType) Parse(s string) error {
	s = strings.TrimSpace(strings.ToLower(s))
	value, ok := LedgerEntryChangeTypeValue[s]
	if !ok {
		return fmt.Errorf("%q is not a valid ledger entry change type", s)
	}
	*l = value
	return nil
}

func (l *LedgerEntryChangeType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return l.Parse(s)
}

// LedgerEntryChange designates a change in a ledger entry. Before and After cannot be omitted at the same time.
// If Before is omitted, it constitutes a creation, if After is omitted, it constitutes a deletion.
type LedgerEntryChange struct {
	Type LedgerEntryChangeType `json:"type"`

	KeyXDR  string          `json:"key,omitempty"` // LedgerEntryKey in base64
	KeyJSON json.RawMessage `json:"keyJson,omitempty"`

	BeforeXDR  *string         `json:"before"` // LedgerEntry XDR in base64
	BeforeJSON json.RawMessage `json:"beforeJson,omitempty"`

	AfterXDR  *string         `json:"after"` // LedgerEntry XDR in base64
	AfterJSON json.RawMessage `json:"afterJson,omitempty"`
}

type SimulateTransactionResponse struct {
	Error string `json:"error,omitempty"`

	TransactionDataXDR  string          `json:"transactionData,omitempty"` // SorobanTransactionData XDR in base64
	TransactionDataJSON json.RawMessage `json:"transactionDataJson,omitempty"`

	EventsXDR  []string          `json:"events,omitempty"` // DiagnosticEvent XDR in base64
	EventsJSON []json.RawMessage `json:"eventsJson,omitempty"`

	MinResourceFee int64 `json:"minResourceFee,string,omitempty"`
	// an array of the individual host function call results
	Results []SimulateHostFunctionResult `json:"results,omitempty"`
	// If present, it indicates that a prior RestoreFootprint is required
	RestorePreamble *RestorePreamble `json:"restorePreamble,omitempty"`
	// If present, it indicates how the state (ledger entries) will change as a result of the transaction execution.
	StateChanges []LedgerEntryChange `json:"stateChanges,omitempty"`
	LatestLedger uint32              `json:"latestLedger"`
}
