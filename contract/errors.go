package contract

import (
	"fmt"
)

// ErrorKind classifies an *Error so callers can branch on the lifecycle stage
// or invariant that produced it without inspecting message strings.
type ErrorKind int

const (
	// KindUnknown is the zero value and indicates the kind was not set.
	KindUnknown ErrorKind = iota
	// KindSimulationFailed: the RPC simulateTransaction call returned an error.
	KindSimulationFailed
	// KindSubmissionFailed: the RPC sendTransaction call or the on-chain
	// inclusion path returned a failure status.
	KindSubmissionFailed
	// KindTimeout: a poll loop (simulation, send, get-transaction) exceeded
	// its deadline.
	KindTimeout
	// KindNotYetSimulated: the caller invoked an operation that requires
	// simulation results before simulation has been run.
	KindNotYetSimulated
	// KindInvalidArgs: caller-supplied arguments failed validation before any
	// network call was made.
	KindInvalidArgs
	// KindTransactionFailed: the transaction was included in a ledger with a
	// FAILED status. Unlike KindSubmissionFailed, submission succeeded but
	// on-chain execution returned an error.
	KindTransactionFailed
	// KindSourceAccountFailed: the source account could not be prepared before
	// simulation — LoadAccount failed (an RPC transport error or the account
	// was not found on-chain), or its sequence number could not be
	// incremented. Distinct from KindSimulationFailed, which covers only the
	// simulateTransaction call, and from KindInvalidArgs, which is reserved for
	// purely local validation that makes no network call.
	KindSourceAccountFailed
)

// String returns a stable, lower-case identifier for the kind, suitable for
// log fields and error messages.
func (k ErrorKind) String() string {
	switch k {
	case KindSimulationFailed:
		return "simulation_failed"
	case KindSubmissionFailed:
		return "submission_failed"
	case KindTimeout:
		return "timeout"
	case KindNotYetSimulated:
		return "not_yet_simulated"
	case KindInvalidArgs:
		return "invalid_args"
	case KindTransactionFailed:
		return "transaction_failed"
	case KindSourceAccountFailed:
		return "source_account_failed"
	case KindUnknown:
		fallthrough
	default:
		return "unknown"
	}
}

// Error is the canonical error type returned by the contract package. It
// carries a machine-readable Kind plus an optional human-readable Details
// string and underlying cause. Callers should match on Kind via errors.Is
// against one of the package sentinels, and unwrap with errors.As / errors.Unwrap
// to retrieve typed causes.
type Error struct {
	Kind    ErrorKind
	Details string
	cause   error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch {
	case e.Details != "" && e.cause != nil:
		return fmt.Sprintf("contract: %s: %s: %v", e.Kind, e.Details, e.cause)
	case e.Details != "":
		return fmt.Sprintf("contract: %s: %s", e.Kind, e.Details)
	case e.cause != nil:
		return fmt.Sprintf("contract: %s: %v", e.Kind, e.cause)
	default:
		return fmt.Sprintf("contract: %s", e.Kind)
	}
}

// Unwrap exposes the underlying cause for errors.As / errors.Unwrap.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is reports whether the target matches this error. Two *Error values match
// when they share the same Kind, which lets the package-level sentinels act
// as classifiers via errors.Is.
func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Kind == t.Kind
}

// Sentinels for errors.Is. Each is a zero-Details *Error pinned to a Kind;
// errors built inside the package are matched against these constants. Match
// on a sentinel rather than inspecting Details strings.
var (
	ErrInvalidArgs         = &Error{Kind: KindInvalidArgs}
	ErrSourceAccountFailed = &Error{Kind: KindSourceAccountFailed}
	ErrSimulationFailed    = &Error{Kind: KindSimulationFailed}
	ErrNotYetSimulated     = &Error{Kind: KindNotYetSimulated}
)

// Compile-time interface checks.
var (
	_ error = (*Error)(nil)
)

func invalidArgsf(format string, args ...any) *Error {
	return &Error{Kind: KindInvalidArgs, Details: fmt.Sprintf(format, args...)}
}
