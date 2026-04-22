package xdr_test

import (
	"os"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// TxSummary captures multiple fields per transaction — tests multi-field access patterns.
type TxSummary struct {
	Hash      [32]byte
	ResultRaw []byte
	FeeRaw    []byte
	MetaRaw   []byte
}

// BenchmarkExtractAllTransactions extracts all transactions from a ledger,
// accessing multiple fields per TransactionResultMeta struct.
func BenchmarkExtractAllTransactions(b *testing.B) {
	data, err := os.ReadFile("testdata/ledger_58752000.bin")
	if err != nil {
		b.Fatalf("testdata not found: %v", err)
	}

	b.Run("full_decode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			txs, err := extractAllTxFullDecode(data)
			if err != nil {
				b.Fatal(err)
			}
			_ = txs
		}
	})

	b.Run("view", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			txs, err := extractAllTxView(data)
			if err != nil {
				b.Fatal(err)
			}
			_ = txs
		}
	})
}

func extractAllTxFullDecode(data []byte) ([]TxSummary, error) {
	var lcm xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshal(data, &lcm); err != nil {
		return nil, err
	}

	v1 := lcm.MustV1()
	results := make([]TxSummary, 0, len(v1.TxProcessing))

	for _, tx := range v1.TxProcessing {
		resultRaw, err := tx.Result.MarshalBinary()
		if err != nil {
			return nil, err
		}
		feeRaw, err := tx.FeeProcessing.MarshalBinary()
		if err != nil {
			return nil, err
		}
		metaRaw, err := tx.TxApplyProcessing.MarshalBinary()
		if err != nil {
			return nil, err
		}

		results = append(results, TxSummary{
			Hash:      tx.Result.TransactionHash,
			ResultRaw: resultRaw,
			FeeRaw:    feeRaw,
			MetaRaw:   metaRaw,
		})
	}

	return results, nil
}

func extractAllTxView(data []byte) ([]TxSummary, error) {
	view := xdr.LedgerCloseMetaView(data)
	v1, err := view.V1()
	if err != nil {
		return nil, err
	}

	txArr, err := v1.TxProcessing()
	if err != nil {
		return nil, err
	}

	txCount, err := txArr.Count()
	if err != nil {
		return nil, err
	}
	results := make([]TxSummary, 0, txCount)
	for tx, iterErr := range txArr.Iter() {
		if iterErr != nil {
			return nil, iterErr
		}

		resultView, err := tx.Result()
		if err != nil {
			return nil, err
		}
		hashView, err := resultView.TransactionHash()
		if err != nil {
			return nil, err
		}
		hashBytes, err := hashView.Value()
		if err != nil {
			return nil, err
		}
		resultRaw, err := resultView.Raw()
		if err != nil {
			return nil, err
		}

		feeView, err := tx.FeeProcessing()
		if err != nil {
			return nil, err
		}
		feeRaw, err := feeView.Raw()
		if err != nil {
			return nil, err
		}

		metaView, err := tx.TxApplyProcessing()
		if err != nil {
			return nil, err
		}
		metaRaw, err := metaView.Raw()
		if err != nil {
			return nil, err
		}

		var hash [32]byte
		copy(hash[:], hashBytes)
		results = append(results, TxSummary{
			Hash:      hash,
			ResultRaw: resultRaw,
			FeeRaw:    feeRaw,
			MetaRaw:   metaRaw,
		})
	}

	return results, nil
}

// BenchmarkExtractAllHashes extracts just the transaction hash from every
// transaction in the ledger. Minimal per-tx work — measures iteration overhead.
func BenchmarkExtractAllHashes(b *testing.B) {
	data, err := os.ReadFile("testdata/ledger_58752000.bin")
	if err != nil {
		b.Fatalf("testdata not found: %v", err)
	}

	var lcm xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshal(data, &lcm); err != nil {
		b.Fatal(err)
	}
	numTx := lcm.CountTransactions()
	b.Logf("Ledger: %d txs, %d bytes", numTx, len(data))

	b.Run("full_decode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hashes, err := extractAllHashesFullDecode(data)
			if err != nil {
				b.Fatal(err)
			}
			_ = hashes
		}
	})

	b.Run("view", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			hashes, err := extractAllHashesView(data)
			if err != nil {
				b.Fatal(err)
			}
			_ = hashes
		}
	})
}

func extractAllHashesFullDecode(data []byte) ([][32]byte, error) {
	var lcm xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshal(data, &lcm); err != nil {
		return nil, err
	}

	v1 := lcm.MustV1()
	hashes := make([][32]byte, len(v1.TxProcessing))
	for i, tx := range v1.TxProcessing {
		hashes[i] = tx.Result.TransactionHash
	}
	return hashes, nil
}

func extractAllHashesView(data []byte) ([][32]byte, error) {
	view := xdr.LedgerCloseMetaView(data)
	v1, err := view.V1()
	if err != nil {
		return nil, err
	}

	txArr, err := v1.TxProcessing()
	if err != nil {
		return nil, err
	}

	txCount, err := txArr.Count()
	if err != nil {
		return nil, err
	}
	hashes := make([][32]byte, 0, txCount)

	for tx, iterErr := range txArr.Iter() {
		if iterErr != nil {
			return nil, iterErr
		}

		resultView, err := tx.Result()
		if err != nil {
			return nil, err
		}
		hashView, err := resultView.TransactionHash()
		if err != nil {
			return nil, err
		}
		hashBytes, err := hashView.Value()
		if err != nil {
			return nil, err
		}

		var hash [32]byte
		copy(hash[:], hashBytes)
		hashes = append(hashes, hash)
	}

	return hashes, nil
}
