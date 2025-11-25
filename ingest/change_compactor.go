package ingest

import (
	"encoding/base64"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ChangeCompactor is a cache of ledger entry changes that squashes all
// changes within a single ledger. By doing this, it decreases number of DB
// queries sent to a DB to update the current state of the ledger.
// It has integrity checks built in so ex. removing an account that was
// previously removed returns an error. In such case verify.StateError is
// returned.
//
// The ChangeCompactor should not be used when ingesting from history archives
// because the history archive snapshots only contain CREATED changes.
// The ChangeCompactor is suited for compacting ledger entry changes derived
// from LedgerCloseMeta payloads because they typically contain a mix of
// CREATED, UPDATED, and REMOVED ledger entry changes and therefore may benefit
// from compaction.
//
// It applies changes to the cache using the following algorithm:
//
//  1. If the change is CREATED it checks if any change connected to given entry
//     is already in the cache. If not, it adds CREATED change. Otherwise, if
//     existing change is:
//     a. CREATED it returns error because we can't add an entry that already
//     exists.
//     b. UPDATED it returns error because we can't add an entry that already
//     exists.
//     c. REMOVED it means that due to previous transitions we want to remove
//     this from a DB what means that it already exists in a DB so we need to
//     update the type of change to UPDATED.
//     d. RESTORED it returns an error as the RESTORED change indicates the
//     entry already exists.
//  2. If the change is UPDATE it checks if any change connected to given entry
//     is already in the cache. If not, it adds UPDATE change. Otherwise, if
//     existing change is:
//     a. CREATED it means that due to previous transitions we want to create
//     this in a DB what means that it doesn't exist in a DB so we need to
//     update the entry but stay with CREATED type.
//     b. UPDATED we simply update it with the new value.
//     c. REMOVED it means that at this point in the ledger the entry is removed
//     so updating it returns an error.
//     d. RESTORED we update it with the new value but keep the change type as
//     RESTORED.
//  3. If the change is REMOVE it checks if any change connected to given entry
//     is already in the cache. If not, it adds REMOVE change. Otherwise, if
//     existing change is:
//     a. CREATED it means that due to previous transitions we want to create
//     this in a DB what means that it doesn't exist in a DB. If it was
//     created and removed in the same ledger it's a noop so we remove entry
//     from the cache.
//     b. UPDATED we simply update it to be a REMOVE change because the UPDATE
//     change means the entry exists in a DB.
//     c. REMOVED it returns error because we can't remove an entry that was
//     already removed.
//     d. RESTORED depending on the change compactor's configuration, we may or
//     may not emit a REMOVE change type for an entry that was restored earlier
//     in the ledger.
//  4. If the change is RESTORED it checks if any change related to the given
//     entry already exists in the cache. If not, it adds the RESTORED change.
//     Otherwise, it returns an error because only archived entries can be
//     restored. If the entry was created, updated or removed in the same
//     ledger, the entry must be active and not archived.
type ChangeCompactor struct {
	// ledger key => Change
	cache          map[string]Change
	encodingBuffer *xdr.EncodingBuffer
	config         ChangeCompactorConfig
}

type ChangeCompactorConfig struct {
	// Determines whether the change compactor emits a REMOVED change when an archived entry
	// is restored and then removed within the same ledger.
	SuppressRemoveAfterRestoreChange bool
}

// NewChangeCompactor returns a new ChangeCompactor.
func NewChangeCompactor(config ChangeCompactorConfig) *ChangeCompactor {
	return &ChangeCompactor{
		cache:          make(map[string]Change),
		encodingBuffer: xdr.NewEncodingBuffer(),
		config:         config,
	}
}

// AddChange adds a change to ChangeCompactor. All changes are stored
// in memory. To get the final, squashed changes call GetChanges.
//
// Please note that the current ledger capacity in pubnet (max 1000 ops/ledger)
// makes ChangeCompactor safe to use in terms of memory usage. If the
// cache takes too much memory, you apply changes returned by GetChanges and
// create a new ChangeCompactor object to continue ingestion.
func (c *ChangeCompactor) AddChange(change Change) error {
	switch change.ChangeType {
	case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
		return c.addCreatedChange(change)
	case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
		return c.addUpdatedChange(change)
	case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
		return c.addRemovedChange(change)
	case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
		return c.addRestoredChange(change)
	default:
		return errors.New("Unknown entry change state")
	}
}

// addCreatedChange adds a change to the cache, but returns an error if create
// change is unexpected.
func (c *ChangeCompactor) addCreatedChange(change Change) error {
	ledgerKey, err := c.getLedgerKey(change.Post)
	if err != nil {
		return err
	}
	ledgerKeyString := string(ledgerKey)

	existingChange, exist := c.cache[ledgerKeyString]
	if !exist {
		c.cache[ledgerKeyString] = change
		return nil
	}

	switch existingChange.ChangeType {
	case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
		fallthrough
	case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
		fallthrough
	case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
		return NewStateError(errors.Errorf(
			"can't create an entry that already exists (ledger key = %s)",
			base64.StdEncoding.EncodeToString(ledgerKey),
		))
	case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
		// If existing type is removed it means that this entry does exist
		// in a DB so we update entry change.
		c.cache[ledgerKeyString] = Change{
			Type:       change.Type,
			Pre:        existingChange.Pre,
			Post:       change.Post,
			ChangeType: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
		}
	default:
		return errors.Errorf("Unknown LedgerEntryChangeType: %d", existingChange.ChangeType)
	}

	return nil
}

func (c *ChangeCompactor) getLedgerKey(ledgerEntry *xdr.LedgerEntry) ([]byte, error) {
	// safe, since we later cast to string (causing a copy)
	key, err := ledgerEntry.LedgerKey()
	if err != nil {
		return nil, errors.Wrap(err, "error getting ledger key for new entry")
	}
	ledgerKey, err := c.encodingBuffer.UnsafeMarshalBinary(key)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling ledger key for new entry")
	}
	return ledgerKey, nil
}

// maxTTL returns the ttl entry with the highest LiveUntilLedgerSeq
func maxTTL(a, b xdr.TtlEntry) xdr.TtlEntry {
	if a.LiveUntilLedgerSeq > b.LiveUntilLedgerSeq {
		return a
	}
	return b
}

// addUpdatedChange adds a change to the cache, but returns an error if update
// change is unexpected.
func (c *ChangeCompactor) addUpdatedChange(change Change) error {
	ledgerKey, err := c.getLedgerKey(change.Post)
	if err != nil {
		return err
	}
	ledgerKeyString := string(ledgerKey)

	existingChange, exist := c.cache[ledgerKeyString]
	if !exist {
		c.cache[ledgerKeyString] = change
		return nil
	}

	switch existingChange.ChangeType {
	case xdr.LedgerEntryChangeTypeLedgerEntryCreated,
		xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
		xdr.LedgerEntryChangeTypeLedgerEntryRestored:
		post := change.Post
		if change.Type == xdr.LedgerEntryTypeTtl {
			// CAP-63 introduces special update semantics for TTL entries, see
			// https://github.com/stellar/stellar-protocol/blob/master/core/cap-0063.md#ttl-ledger-change-semantics
			*post.Data.Ttl = maxTTL(*existingChange.Post.Data.Ttl, *post.Data.Ttl)
		}
		c.cache[ledgerKeyString] = Change{
			Type:       change.Type,
			Pre:        existingChange.Pre,
			Post:       post,
			ChangeType: existingChange.ChangeType, //keep the existing change type
		}
	case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
		return NewStateError(errors.Errorf(
			"can't update an entry that was previously removed (ledger key = %s)",
			base64.StdEncoding.EncodeToString(ledgerKey),
		))
	default:
		return errors.Errorf("Unknown LedgerEntryChangeType: %d", existingChange.ChangeType)
	}

	return nil
}

// addRemovedChange adds a change to the cache, but returns an error if remove
// change is unexpected.
func (c *ChangeCompactor) addRemovedChange(change Change) error {
	ledgerKey, err := c.getLedgerKey(change.Pre)
	if err != nil {
		return err
	}
	ledgerKeyString := string(ledgerKey)

	existingChange, exist := c.cache[ledgerKeyString]
	if !exist {
		c.cache[ledgerKeyString] = change
		return nil
	}

	switch existingChange.ChangeType {
	case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
		// If existing type is created it means that this will be no op.
		// Entry was created and is now removed in a single ledger.
		delete(c.cache, ledgerKeyString)
	case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
		c.cache[ledgerKeyString] = Change{
			Type:       change.Type,
			Pre:        existingChange.Pre,
			Post:       nil,
			ChangeType: change.ChangeType,
		}
	case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
		return NewStateError(errors.Errorf(
			"can't remove an entry that was previously removed (ledger key = %s)",
			base64.StdEncoding.EncodeToString(ledgerKey),
		))
	case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
		if c.config.SuppressRemoveAfterRestoreChange {
			// Entry was restored and removed in the same ledger; deleting it is effectively a noop.
			delete(c.cache, ledgerKeyString)
		} else {
			c.cache[ledgerKeyString] = Change{
				Type:       change.Type,
				Pre:        change.Pre,
				Post:       nil,
				ChangeType: change.ChangeType,
			}
		}
	default:
		return errors.Errorf("Unknown LedgerEntryChangeType: %d", existingChange.ChangeType)
	}

	return nil
}

// addRestoredChange adds a change to the cache, but returns an error if the restore
// change is unexpected.
func (c *ChangeCompactor) addRestoredChange(change Change) error {
	ledgerKey, err := c.getLedgerKey(change.Post)
	if err != nil {
		return err
	}
	ledgerKeyString := string(ledgerKey)

	if _, exist := c.cache[ledgerKeyString]; exist {
		return NewStateError(errors.Errorf(
			"can't restore an entry that is already active (ledger key = %s)",
			base64.StdEncoding.EncodeToString(ledgerKey),
		))
	}
	c.cache[ledgerKeyString] = change
	return nil
}

// GetChanges returns a slice of Changes in the cache. The order of changes is
// random but each change is connected to a separate entry.
func (c *ChangeCompactor) GetChanges() []Change {
	changes := make([]Change, 0, len(c.cache))

	for _, entryChange := range c.cache {
		changes = append(changes, entryChange)
	}

	return changes
}

// Size returns number of ledger entries in the cache.
func (c *ChangeCompactor) Size() int {
	return len(c.cache)
}
