package loadtest

import (
	"cmp"
	"errors"
	"io"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const repairPassphrase = network.TestNetworkPassphrase

type testAccount string

func (a testAccount) entry(balance, seq int64) xdr.LedgerEntry {
	return xdr.LedgerEntry{Data: xdr.LedgerEntryData{Type: xdr.LedgerEntryTypeAccount, Account: &xdr.AccountEntry{
		AccountId: xdr.MustAddress(string(a)),
		Balance:   xdr.Int64(balance),
		SeqNum:    xdr.SequenceNumber(seq),
	}}}
}

// update is the [STATE, UPDATED] pair of an account going from (bal0, seq0) to (bal1, seq1).
func (a testAccount) update(bal0, seq0, bal1, seq1 int64) xdr.LedgerEntryChanges {
	pre, post := a.entry(bal0, seq0), a.entry(bal1, seq1)
	return xdr.LedgerEntryChanges{
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryState, State: &pre},
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated, Updated: &post},
	}
}

func (a testAccount) created(balance int64) xdr.LedgerEntryChanges {
	e := a.entry(balance, 0)
	return xdr.LedgerEntryChanges{{Type: xdr.LedgerEntryChangeTypeLedgerEntryCreated, Created: &e}}
}

func (a testAccount) removed(bal, seq int64) xdr.LedgerEntryChanges {
	pre := a.entry(bal, seq)
	key := xdr.LedgerKey{Type: xdr.LedgerEntryTypeAccount, Account: &xdr.LedgerKeyAccount{AccountId: xdr.MustAddress(string(a))}}
	return xdr.LedgerEntryChanges{
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryState, State: &pre},
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved, Removed: &key},
	}
}

func newAccount() testAccount { return testAccount(keypair.MustRandom().Address()) }

type testTx struct {
	source           testAccount
	fee, ops, refund xdr.LedgerEntryChanges
}

func concat(changes ...xdr.LedgerEntryChanges) xdr.LedgerEntryChanges {
	var out xdr.LedgerEntryChanges
	for _, c := range changes {
		out = append(out, c...)
	}
	return out
}

// testRepairLedger is a V2 ledger holding txs, with envelope hashes that pair
// with their results.
func testRepairLedger(t *testing.T, seq uint32, txs ...testTx) []byte {
	var envelopes []xdr.TransactionEnvelope
	var processing []xdr.TransactionResultMetaV1
	for i, tx := range txs {
		env := xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &xdr.TransactionV1Envelope{Tx: xdr.Transaction{
			SourceAccount: xdr.MustMuxedAddress(string(tx.source)),
			SeqNum:        xdr.SequenceNumber(int64(seq)<<8 + int64(i)),
		}}}
		hash, err := network.HashTransactionInEnvelope(env, repairPassphrase)
		require.NoError(t, err)
		envelopes = append(envelopes, env)
		processing = append(processing, xdr.TransactionResultMetaV1{
			Result: xdr.TransactionResultPair{TransactionHash: hash, Result: xdr.TransactionResult{
				Result: xdr.TransactionResultResult{Code: xdr.TransactionResultCodeTxSuccess, Results: &[]xdr.OperationResult{}},
			}},
			FeeProcessing: tx.fee,
			TxApplyProcessing: xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
				Operations: []xdr.OperationMetaV2{{Changes: tx.ops}},
			}},
			PostTxApplyFeeProcessing: tx.refund,
		})
	}
	comps := []xdr.TxSetComponent{{
		Type:                  xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
		TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{Txs: envelopes},
	}}
	lcm := xdr.LedgerCloseMeta{V: 2, V2: &xdr.LedgerCloseMetaV2{
		LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{LedgerSeq: xdr.Uint32(seq)}},
		TxSet: xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{Phases: []xdr.TransactionPhase{
			{V: 0, V0Components: &comps},
			{V: 1, ParallelTxsComponent: &xdr.ParallelTxsComponent{}},
		}}},
		TxProcessing: processing,
	}}
	return marshal(t, lcm)
}

// withUpgrade adds a base fee upgrade with changes to a V2 ledger.
func withUpgrade(t *testing.T, raw []byte, changes xdr.LedgerEntryChanges) []byte {
	var lcm xdr.LedgerCloseMeta
	require.NoError(t, lcm.UnmarshalBinary(raw))
	lcm.V2.UpgradesProcessing = []xdr.UpgradeEntryMeta{{
		Upgrade: xdr.LedgerUpgrade{Type: xdr.LedgerUpgradeTypeLedgerUpgradeBaseFee, NewBaseFee: new(xdr.Uint32)},
		Changes: changes,
	}}
	return marshal(t, lcm)
}

// replayState reads each ledger through the change reader and compactor, as
// Horizon does, and returns the resulting state: ledger key to post image,
// with "" for a removed entry.
func replayState(t *testing.T, ledgers ...[]byte) (map[string]string, error) {
	state := map[string]string{}
	for _, raw := range ledgers {
		var lcm xdr.LedgerCloseMeta
		require.NoError(t, lcm.UnmarshalBinary(raw))
		reader, err := ingest.NewLedgerChangeReaderFromLedgerCloseMeta(repairPassphrase, lcm)
		require.NoError(t, err)
		changes := ingest.NewCompactingChangeReader(reader, ingest.ChangeCompactorConfig{})
		for {
			c, err := changes.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			entry := c.Post
			if entry == nil {
				entry = c.Pre
			}
			key, err := entry.LedgerKey()
			require.NoError(t, err)
			k := string(marshal(t, key))
			state[k] = ""
			if c.Post != nil {
				state[k] = string(marshal(t, c.Post))
			}
		}
	}
	return state, nil
}

func TestRepairChangeOrder(t *testing.T) {
	x, y := newAccount(), newAccount()
	for _, c := range []struct {
		name    string
		ledgers [][]byte
	}{
		{
			name: "fee charged to an account an earlier ledger created",
			ledgers: [][]byte{
				testRepairLedger(t, 10, testTx{source: y, fee: y.update(1000, 1, 990, 1), ops: concat(x.created(500), y.update(990, 1, 490, 1))}),
				testRepairLedger(t, 11, testTx{source: x, fee: x.update(500, 0, 490, 0), ops: x.update(490, 0, 490, 1)}),
			},
		},
		{
			name: "refund to an account a later ledger removes",
			ledgers: [][]byte{
				testRepairLedger(t, 10, testTx{
					source: x, fee: x.update(1000, 0, 900, 0), ops: x.update(900, 0, 900, 1), refund: x.update(900, 1, 950, 1),
				}),
				testRepairLedger(t, 11, testTx{
					source: y, fee: y.update(100, 1, 90, 1), ops: concat(x.removed(950, 1), y.update(90, 1, 1040, 1)),
				}),
			},
		},
		{
			name: "refund read after a later ledger's changes",
			ledgers: [][]byte{
				testRepairLedger(t, 10, testTx{
					source: x, fee: x.update(1000, 0, 900, 0), ops: x.update(900, 0, 900, 1), refund: x.update(900, 1, 950, 1),
				}),
				testRepairLedger(t, 11, testTx{source: x, fee: x.update(950, 1, 850, 1), ops: x.update(850, 1, 850, 2)}),
			},
		},
		{
			name: "upgrade to an account only earlier ledgers change",
			ledgers: [][]byte{
				testRepairLedger(t, 10, testTx{source: x, fee: x.update(1000, 0, 900, 0)}),
				withUpgrade(t, testRepairLedger(t, 11), x.update(900, 0, 900, 0)),
				testRepairLedger(t, 12, testTx{source: y, fee: y.update(1000, 0, 900, 0)}),
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			want, err := replayState(t, c.ledgers...)
			require.NoError(t, err)
			merged, _, err := MergeLedgerBytes(c.ledgers, MergeOptions{HeaderFrom: len(c.ledgers) - 1})
			require.NoError(t, err)
			got, err := replayState(t, merged)
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}

	t.Run("rejects an upgrade to an account a later ledger changes", func(t *testing.T) {
		ledgers := [][]byte{
			withUpgrade(t, testRepairLedger(t, 10), x.update(1000, 0, 1000, 0)),
			testRepairLedger(t, 11, testTx{source: x, fee: x.update(1000, 0, 900, 0)}),
		}
		_, _, err := MergeLedgerBytes(ledgers, MergeOptions{HeaderFrom: 1})
		require.ErrorContains(t, err, "upgrades an account")
	})
}

func TestApplyEdits(t *testing.T) {
	src := rand.NewChaCha8([32]byte{})
	r := rand.New(src) //nolint:gosec // deterministic test data
	bytesOf := func(n int) []byte {
		b := make([]byte, n)
		_, _ = src.Read(b)
		return b
	}
	for range 5000 {
		n := r.IntN(64)
		root := append(make([]byte, 0, n+r.IntN(16)), bytesOf(n)...)
		var edits []edit
		for at := r.IntN(4); at <= n; {
			if r.IntN(2) == 0 {
				edits = append(edits, edit{at, at, bytesOf(r.IntN(8))})
			}
			if at == n {
				break
			}
			end := at + 1 + r.IntN(min(6, n-at))
			if r.IntN(2) == 0 {
				edits = append(edits, edit{at, end, bytesOf(r.IntN(8))})
			}
			at = end + r.IntN(4)
		}
		want := spliced(root, edits)
		r.Shuffle(len(edits), func(i, j int) { edits[i], edits[j] = edits[j], edits[i] })
		require.Equal(t, want, applyEdits(root, edits))
	}
}

// spliced is applyEdits out of place: root with each edit's span replaced, an
// insertion going before an edit at the same offset.
func spliced(root []byte, edits []edit) []byte {
	edits = slices.Clone(edits)
	slices.SortFunc(edits, func(a, b edit) int { return cmp.Or(a.start-b.start, a.end-b.end) })
	out := []byte{}
	at := 0
	for _, e := range edits {
		out = append(append(out, root[at:e.start]...), e.repl...)
		at = e.end
	}
	return append(out, root[at:]...)
}
