package ingest

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
)

// ErrNotFound is returned when the requested ledger is not found
var ErrNotFound = errors.New("ledger not found")

// StateError is a fatal error indicating that the Change stream
// produced a result which violates fundamental invariants (e.g. an account
// transferred more XLM than the account held in its balance).
type StateError struct {
	error
}

// NewStateError creates a new StateError.
func NewStateError(err error) StateError {
	return StateError{err}
}
