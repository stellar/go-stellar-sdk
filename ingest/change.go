package ingest

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Change is a developer friendly representation of LedgerEntryChanges.
// It also provides some helper functions to quickly check if a given
// change has occurred in an entry.
//
// Change represents a modification to a ledger entry, capturing both the before and after states
// of the entry along with the context that explains what caused the change. It is primarily used to
// track changes during transactions and/or operations within a transaction
// and can be helpful in identifying the specific cause of changes to the LedgerEntry state. (https://github.com/stellar/go-stellar-sdk/issues/5535
//
// Behavior:
//
//   - **Created entries**: Pre is nil, and Post is not nil.
//
//   - **Updated entries**: Both Pre and Post are non-nil.
//
//   - **Removed entries**: Pre is not nil, and Post is nil.
//
//     A `Change` can be caused primarily by either a transaction or by an operation within a transaction:
//
//   - **Operations**:
//     Each successful operation can cause multiple ledger entry changes.
//     For example, a path payment operation may affect the source and destination account entries,
//     as well as potentially modify offers and/or liquidity pools.
//
//   - **Transactions**:
//     Some ledger changes, such as those involving fees or account balances, may be caused by
//     the transaction itself and may not be tied to a specific operation within a transaction.
//     For instance, fees for all operations in a transaction are debited from the source account,
//     triggering ledger changes without operation-specific details.
type Change struct {
	// The type of the ledger entry being changed.
	Type xdr.LedgerEntryType

	// The specific type of change, such as Created, Updated, Removed or Restored.
	ChangeType xdr.LedgerEntryChangeType

	// The state of the LedgerEntry before the change. This will be nil if the entry was created.
	Pre *xdr.LedgerEntry

	// The state of the LedgerEntry after the change. This will be nil if the entry was removed.
	Post *xdr.LedgerEntry

	// Specifies why the change occurred, represented as a LedgerEntryChangeReason
	Reason LedgerEntryChangeReason

	// The index of the operation within the transaction that caused the change.
	// This field is relevant only when the Reason is LedgerEntryChangeReasonOperation
	// This field cannot be relied upon when the compactingChangeReader is used.
	OperationIndex uint32

	// The LedgerTransaction responsible for the change.
	// It contains details such as transaction hash, envelope, result pair, and fees.
	// This field is populated only when the Reason is one of:
	// LedgerEntryChangeReasonTransaction, LedgerEntryChangeReasonOperation or LedgerEntryChangeReasonFee
	Transaction *LedgerTransaction

	// The LedgerCloseMeta that precipitated the change.
	// This field is not populated when the Change is obtained from enumerating
	// ledger entries from a history archive snapshot (e.g. via CheckpointChangeReader).
	Ledger *xdr.LedgerCloseMeta

	// Information about the upgrade, if the change occurred as part of an upgrade
	// This field is relevant only when the Reason is LedgerEntryChangeReasonUpgrade
	LedgerUpgrade *xdr.LedgerUpgrade
}

// LedgerEntryChangeReason represents the reason for a ledger entry change.
type LedgerEntryChangeReason uint16

const (
	// LedgerEntryChangeReasonUnknown indicates an unknown or unsupported change reason
	LedgerEntryChangeReasonUnknown LedgerEntryChangeReason = iota

	// LedgerEntryChangeReasonOperation indicates a change caused by an operation in a transaction
	LedgerEntryChangeReasonOperation

	// LedgerEntryChangeReasonTransaction indicates a change caused by the transaction itself
	LedgerEntryChangeReasonTransaction

	// LedgerEntryChangeReasonFee indicates a change related to transaction fees.
	LedgerEntryChangeReasonFee

	// LedgerEntryChangeReasonFeeRefund indicates a change related to transaction fee refunds.
	LedgerEntryChangeReasonFeeRefund

	// LedgerEntryChangeReasonUpgrade indicates a change caused by a ledger upgrade.
	LedgerEntryChangeReasonUpgrade
)

// String returns a best effort string representation of the change.
// If the Pre or Post xdr is invalid, the field will be omitted from the string.
func (c Change) String() string {
	var pre, post string
	if c.Pre != nil {
		if b64, err := xdr.MarshalBase64(c.Pre); err == nil {
			pre = b64
		}
	}
	if c.Post != nil {
		if b64, err := xdr.MarshalBase64(c.Post); err == nil {
			post = b64
		}
	}
	return fmt.Sprintf(
		"Change{Type: %s, Pre: %s, Post: %s, ChangeType: %s}",
		c.Type.String(),
		pre,
		post,
		c.ChangeType,
	)
}

func (c Change) LedgerKey() (xdr.LedgerKey, error) {
	if c.Pre != nil {
		return c.Pre.LedgerKey()
	}
	return c.Post.LedgerKey()
}

// GetChangesFromLedgerEntryChanges transforms LedgerEntryChanges to []Change.
// Each `update` and `removed` is preceded with `state` and `create` changes
// are alone, without `state`. The transformation we're doing is to move each
// change (state/update, state/removed or create) to an array of pre/post pairs.
// Then:
// - for create, pre is null and post is a new entry,
// - for update, pre is previous state and post is the current state,
// - for removed, pre is previous state and post is null.
// - for restored, pre is null and post is a new/restored entry
//
// stellar-core source:
// https://github.com/stellar/stellar-core/blob/e584b43/src/ledger/LedgerTxn.cpp#L582
func GetChangesFromLedgerEntryChanges(ledgerEntryChanges xdr.LedgerEntryChanges) []Change {
	changes := make([]Change, 0, len(ledgerEntryChanges))
	for i, entryChange := range ledgerEntryChanges {
		switch entryChange.Type {
		case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
			created := entryChange.MustCreated()
			changes = append(changes, Change{
				Type:       created.Data.Type,
				Pre:        nil,
				Post:       &created,
				ChangeType: entryChange.Type,
			})
		case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
			// Update entries always have a previous state entry [state, updated]
			// except for contract entries that are restored and updated within the same
			// transaction, which appear as [restored, updated]
			// For details, see https://github.com/stellar/stellar-protocol/blob/master/core/cap-0062.md
			updated := entryChange.MustUpdated()
			state, ok := ledgerEntryChanges[i-1].GetState()
			if !ok {
				state = ledgerEntryChanges[i-1].MustRestored()
			}
			changes = append(changes, Change{
				Type:       state.Data.Type,
				Pre:        &state,
				Post:       &updated,
				ChangeType: entryChange.Type,
			})
		case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
			// Removed entries always have an associated state entry [state, updated]
			// except for contract entries that are restored and removed within the same
			// transaction, which appear as [restored, removed]
			// For details, see https://github.com/stellar/stellar-protocol/blob/master/core/cap-0062.md
			state, ok := ledgerEntryChanges[i-1].GetState()
			if !ok {
				state = ledgerEntryChanges[i-1].MustRestored()
			}
			changes = append(changes, Change{
				Type:       state.Data.Type,
				Pre:        &state,
				Post:       nil,
				ChangeType: entryChange.Type,
			})
		case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
			restored := entryChange.MustRestored()
			changes = append(changes, Change{
				Type:       restored.Data.Type,
				Pre:        nil,
				Post:       &restored,
				ChangeType: entryChange.Type,
			})
		case xdr.LedgerEntryChangeTypeLedgerEntryState:
			continue
		default:
			panic("Invalid LedgerEntryChangeType")
		}
	}

	sortChanges(changes)
	return changes
}

type sortableChanges struct {
	changes    []Change
	ledgerKeys [][]byte
}

func newSortableChanges(changes []Change) sortableChanges {
	ledgerKeys := make([][]byte, len(changes))
	for i, c := range changes {
		lk, err := c.LedgerKey()
		if err != nil {
			panic(err)
		}
		lkBytes, err := lk.MarshalBinary()
		if err != nil {
			panic(err)
		}
		ledgerKeys[i] = lkBytes
	}
	return sortableChanges{
		changes:    changes,
		ledgerKeys: ledgerKeys,
	}
}

func (s sortableChanges) Len() int {
	return len(s.changes)
}

func (s sortableChanges) Less(i, j int) bool {
	return bytes.Compare(s.ledgerKeys[i], s.ledgerKeys[j]) < 0
}

func (s sortableChanges) Swap(i, j int) {
	s.changes[i], s.changes[j] = s.changes[j], s.changes[i]
	s.ledgerKeys[i], s.ledgerKeys[j] = s.ledgerKeys[j], s.ledgerKeys[i]
}

// sortChanges is applied on a list of changes to ensure that LedgerEntryChanges
// from Tx Meta are ingested in a deterministic order.
// The changes are sorted by ledger key. It is unexpected for there to be
// multiple changes with the same ledger key in a LedgerEntryChanges group,
// but if that is the case, we fall back to the original ordering of the changes
// by using a stable sorting algorithm.
func sortChanges(changes []Change) {
	sort.Stable(newSortableChanges(changes))
}

// getLiquidityPool gets the most recent state of the LiquidityPool that exists or existed.
func (c Change) getLiquidityPool() (*xdr.LiquidityPoolEntry, error) {
	var entry *xdr.LiquidityPoolEntry
	if c.Pre != nil {
		entry = c.Pre.Data.LiquidityPool
	}
	if c.Post != nil {
		entry = c.Post.Data.LiquidityPool
	}
	if entry == nil {
		return &xdr.LiquidityPoolEntry{}, errors.New("this change does not include a liquidity pool")
	}
	return entry, nil
}

// GetLiquidityPoolType returns the liquidity pool type.
func (c Change) GetLiquidityPoolType() (xdr.LiquidityPoolType, error) {
	lp, err := c.getLiquidityPool()
	if err != nil {
		return xdr.LiquidityPoolType(0), err
	}
	return lp.Body.Type, nil
}

// AccountChangedExceptSigners returns true if account has changed WITHOUT
// checking the signers (except master key weight!). In other words, if the only
// change is connected to signers, this function will return false.
func (c Change) AccountChangedExceptSigners() (bool, error) {
	if c.Type != xdr.LedgerEntryTypeAccount {
		panic("This should not be called on changes other than Account changes")
	}

	// New account
	if c.Pre == nil {
		return true, nil
	}

	// Account merged
	// c.Pre != nil at this point.
	if c.Post == nil {
		return true, nil
	}

	// c.Pre != nil && c.Post != nil at this point.
	if c.Pre.LastModifiedLedgerSeq != c.Post.LastModifiedLedgerSeq {
		return true, nil
	}

	// Don't use short assignment statement (:=) to ensure variables below
	// are not pointers (if `xdr` package changes in the future)!
	var preAccountEntry, postAccountEntry xdr.AccountEntry
	preAccountEntry = c.Pre.Data.MustAccount()
	postAccountEntry = c.Post.Data.MustAccount()

	// preAccountEntry and postAccountEntry are copies so it's fine to
	// modify them here, EXCEPT pointers inside them!
	if preAccountEntry.Ext.V == 0 {
		preAccountEntry.Ext.V = 1
		preAccountEntry.Ext.V1 = &xdr.AccountEntryExtensionV1{
			Liabilities: xdr.Liabilities{
				Buying:  0,
				Selling: 0,
			},
		}
	}

	preAccountEntry.Signers = nil

	if postAccountEntry.Ext.V == 0 {
		postAccountEntry.Ext.V = 1
		postAccountEntry.Ext.V1 = &xdr.AccountEntryExtensionV1{
			Liabilities: xdr.Liabilities{
				Buying:  0,
				Selling: 0,
			},
		}
	}

	postAccountEntry.Signers = nil

	preBinary, err := preAccountEntry.MarshalBinary()
	if err != nil {
		return false, errors.Wrap(err, "Error running preAccountEntry.MarshalBinary")
	}

	postBinary, err := postAccountEntry.MarshalBinary()
	if err != nil {
		return false, errors.Wrap(err, "Error running postAccountEntry.MarshalBinary")
	}

	return !bytes.Equal(preBinary, postBinary), nil
}
