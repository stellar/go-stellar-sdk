package ingest_test

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Real-ledger equivalence tests: every public view extractor must produce
// output identical to the eager-decode reference path
// (LedgerTransactionReader / GetTransactionEvents / GetDiagnosticEvents) on a
// real pubnet ledger. The synthetic differential tests elsewhere in the
// package cover shapes a modern ledger cannot contain (LCM V0, older meta
// versions, TX_V0 arms, the V3 soroban gate, cursor and error edges); these
// cover the real thing at production size.

const realLedgerPath = "../xdr/testdata/ledger_58752000.bin"

func loadRealLedger(tb testing.TB) []byte {
	tb.Helper()
	data, err := os.ReadFile(realLedgerPath)
	if err != nil {
		tb.Fatalf("testdata not found: %v", err)
	}
	return data
}

// refTransactions decodes the ledger and drains the parsed reference reader.
func refTransactions(tb testing.TB, raw []byte) []ingest.LedgerTransaction {
	tb.Helper()
	var lcm xdr.LedgerCloseMeta
	require.NoError(tb, xdr.SafeUnmarshal(raw, &lcm))
	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(network.PublicNetworkPassphrase, lcm)
	require.NoError(tb, err)
	defer func() { require.NoError(tb, reader.Close()) }()
	var out []ingest.LedgerTransaction
	for {
		tx, err := reader.Read()
		if err == io.EOF {
			return out
		}
		require.NoError(tb, err)
		out = append(out, tx)
	}
}

func TestExtractTxHashes_RealLedgerEquivalence(t *testing.T) {
	raw := loadRealLedger(t)
	refTxs := refTransactions(t, raw)

	hashes, err := realLedgerHashes(raw)
	require.NoError(t, err)
	require.Len(t, hashes, len(refTxs))
	require.NotEmpty(t, hashes, "fixture ledger must carry transactions")
	for i, ref := range refTxs {
		assert.Equal(t, ref.Hash, hashes[i], "tx %d hash", i)
	}
}

func TestExtractLedgerEvents_RealLedgerEquivalence(t *testing.T) {
	raw := loadRealLedger(t)
	refTxs := refTransactions(t, raw)

	got, err := collectLedgerEvents(xdr.ParseLedgerCloseMetaView(raw))
	require.NoError(t, err)
	require.Len(t, got, len(refTxs))

	for i, ref := range refTxs {
		require.Equal(t, [32]byte(ref.Hash), got[i].Hash, "tx %d hash", i)
		refEvents, err := ref.GetTransactionEvents()
		require.NoError(t, err)

		require.Len(t, got[i].TransactionEvents, len(refEvents.TransactionEvents), "tx %d tx-events", i)
		for j, refEv := range refEvents.TransactionEvents {
			refRaw, err := refEv.MarshalBinary()
			require.NoError(t, err)
			assert.Equal(t, refRaw, got[i].TransactionEvents[j], "tx %d tx-event %d", i, j)
		}
		require.Len(t, got[i].OperationEvents, len(refEvents.OperationEvents), "tx %d op groups", i)
		for op, refOpEvents := range refEvents.OperationEvents {
			require.Len(t, got[i].OperationEvents[op], len(refOpEvents), "tx %d op %d events", i, op)
			for j, refEv := range refOpEvents {
				refRaw, err := refEv.MarshalBinary()
				require.NoError(t, err)
				assert.Equal(t, refRaw, got[i].OperationEvents[op][j], "tx %d op %d event %d", i, op, j)
			}
		}
	}
}

func TestLedgerTransactionViewRange_RealLedgerEquivalence(t *testing.T) {
	raw := loadRealLedger(t)
	refTxs := refTransactions(t, raw)

	got, err := ingest.LedgerTransactionViewRange(
		xdr.ParseLedgerCloseMetaView(raw), 0, 0, network.PublicNetworkPassphrase)
	require.NoError(t, err)
	require.Len(t, got, len(refTxs))

	for i, ref := range refTxs {
		v := got[i]
		require.Equal(t, [32]byte(ref.Hash), v.Hash, "tx %d hash", i)
		assert.Equal(t, int32(i+1), v.ApplicationOrder, "tx %d apply order", i)
		assert.Equal(t, ref.Envelope.Type == xdr.EnvelopeTypeEnvelopeTypeTxFeeBump, v.FeeBump, "tx %d fee-bump", i)
		assert.Equal(t, ref.Result.Result.Successful(), v.Successful, "tx %d successful", i)

		refEnv, err := ref.Envelope.MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, refEnv, v.Envelope, "tx %d envelope bytes", i)
		refRes, err := ref.Result.Result.MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, refRes, v.Result, "tx %d result bytes", i)
		refMeta, err := ref.UnsafeMeta.MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, refMeta, v.Meta, "tx %d meta bytes", i)

		refDiag, err := ref.GetDiagnosticEvents()
		require.NoError(t, err)
		require.Len(t, v.DiagnosticEvents, len(refDiag), "tx %d diagnostics", i)
		for j, refEv := range refDiag {
			refRaw, err := refEv.MarshalBinary()
			require.NoError(t, err)
			assert.Equal(t, refRaw, v.DiagnosticEvents[j], "tx %d diagnostic %d", i, j)
		}
	}
}

func TestLedgerTransactionViewByHash_RealLedgerEquivalence(t *testing.T) {
	raw := loadRealLedger(t)
	refTxs := refTransactions(t, raw)
	require.NotEmpty(t, refTxs)

	// First, middle, last — different positions in TxProcessing and the
	// (differently-ordered) TxSet.
	for _, i := range []int{0, len(refTxs) / 2, len(refTxs) - 1} {
		ref := refTxs[i]
		v, found, err := ingest.LedgerTransactionViewByHash(
			xdr.ParseLedgerCloseMetaView(raw), ref.Hash, network.PublicNetworkPassphrase)
		require.NoError(t, err)
		require.True(t, found, "tx %d (%x)", i, ref.Hash)
		assert.Equal(t, int32(i+1), v.ApplicationOrder)
		refEnv, err := ref.Envelope.MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, refEnv, v.Envelope)
	}

	_, found, err := ingest.LedgerTransactionViewByHash(
		xdr.ParseLedgerCloseMetaView(raw), [32]byte{0xde, 0xad}, network.PublicNetworkPassphrase)
	require.NoError(t, err)
	require.False(t, found, "absent hash is a clean miss")
}

// realLedgerHashes lists tx hashes via the streaming extractor (the
// hashes-only extractor is folded into it).
func realLedgerHashes(raw []byte) ([]xdr.Hash, error) {
	evs, err := collectLedgerEvents(xdr.ParseLedgerCloseMetaView(raw))
	if err != nil {
		return nil, err
	}
	out := make([]xdr.Hash, len(evs))
	for i := range evs {
		out[i] = xdr.Hash(evs[i].Hash)
	}
	return out, nil
}
