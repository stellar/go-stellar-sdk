package xdr_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// TransactionResponse is the target struct we need to populate.
type TransactionResponse struct {
	Index         uint32
	Result        []byte // xdr.TransactionResultPair
	FeeChanges    []byte // xdr.LedgerEntryChanges
	UnsafeMeta    []byte // xdr.TransactionMeta
	LedgerVersion uint32
	Hash          xdr.Hash
}

// BenchmarkFindTransactionByHash compares full decode vs view-based lookup
// for finding a transaction by hash in a real LedgerCloseMeta and extracting
// its fields into a TransactionResponse.
func BenchmarkFindTransactionByHash(b *testing.B) {
	data, err := os.ReadFile("testdata/ledger_58752000.bin")
	if err != nil {
		b.Fatalf("reading testdata: %v", err)
	}

	var lcm xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshal(data, &lcm); err != nil {
		b.Fatal(err)
	}
	numTx := lcm.CountTransactions()
	b.Logf("Ledger: v%d, %d txs, %d bytes", lcm.V, numTx, len(data))

	// Benchmark at three positions: early, mid (average case), and late.
	positions := []struct {
		name string
		idx  int
	}{
		{"early", min(numTx-1, 10)},
		{"mid", numTx / 2},
		{"late", numTx - 10},
	}

	for _, pos := range positions {
		targetHash := lcm.MustV1().TxProcessing[pos.idx].Result.TransactionHash
		b.Logf("  %s: idx=%d/%d", pos.name, pos.idx, numTx)

		b.Run("full_decode/"+pos.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				resp, err := findByHashFullDecode(data, targetHash)
				if err != nil {
					b.Fatal(err)
				}
				_ = resp
			}
		})

		b.Run("view/"+pos.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				resp, err := findByHashView(data, targetHash)
				if err != nil {
					b.Fatal(err)
				}
				_ = resp
			}
		})

	}
}

// findByHashFullDecode decodes the entire LedgerCloseMeta, iterates transactions
// to find the matching hash, then re-marshals the fields.
func findByHashFullDecode(data []byte, targetHash xdr.Hash) (TransactionResponse, error) {
	var lcm xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshal(data, &lcm); err != nil {
		return TransactionResponse{}, err
	}

	v1 := lcm.MustV1()
	for i, tx := range v1.TxProcessing {
		if tx.Result.TransactionHash != targetHash {
			continue
		}

		result, err := tx.Result.MarshalBinary()
		if err != nil {
			return TransactionResponse{}, err
		}
		feeChanges, err := tx.FeeProcessing.MarshalBinary()
		if err != nil {
			return TransactionResponse{}, err
		}
		meta, err := tx.TxApplyProcessing.MarshalBinary()
		if err != nil {
			return TransactionResponse{}, err
		}

		return TransactionResponse{
			Index:         uint32(i + 1),
			Result:        result,
			FeeChanges:    feeChanges,
			UnsafeMeta:    meta,
			LedgerVersion: uint32(v1.LedgerHeader.Header.LedgerVersion),
			Hash:          targetHash,
		}, nil
	}

	return TransactionResponse{}, fmt.Errorf("transaction not found")
}

// findByHashView uses the view API to scan transaction hashes without decoding,
// then extracts raw bytes for the matching transaction.
func findByHashView(data []byte, targetHash xdr.Hash) (TransactionResponse, error) {
	view := xdr.LedgerCloseMetaView(data)

	v1, err := view.V1()
	if err != nil {
		return TransactionResponse{}, err
	}

	// Ledger version (read once before scanning)
	hdr, err := v1.LedgerHeader()
	if err != nil {
		return TransactionResponse{}, err
	}
	header, err := hdr.Header()
	if err != nil {
		return TransactionResponse{}, err
	}
	ledgerVersion, err := header.LedgerVersion()
	if err != nil {
		return TransactionResponse{}, err
	}
	ledgerVersionVal, err := ledgerVersion.Value()
	if err != nil {
		return TransactionResponse{}, err
	}

	i := -1
	txArr, err := v1.TxProcessing()
	if err != nil {
		return TransactionResponse{}, err
	}
	for txView, iterErr := range txArr.Iter() {
		i++
		if iterErr != nil {
			return TransactionResponse{}, iterErr
		}

		resultView, err := txView.Result()
		if err != nil {
			return TransactionResponse{}, err
		}

		hashView, err := resultView.TransactionHash()
		if err != nil {
			return TransactionResponse{}, err
		}
		hashBytes, err := hashView.Value()
		if err != nil {
			return TransactionResponse{}, err
		}

		if !bytes.Equal(hashBytes, targetHash[:]) {
			continue
		}

		// Found it — extract fields via the view API.
		resultBytes, err := resultView.Raw()
		if err != nil {
			return TransactionResponse{}, err
		}

		feeView, err := txView.FeeProcessing()
		if err != nil {
			return TransactionResponse{}, err
		}
		feeBytes, err := feeView.Raw()
		if err != nil {
			return TransactionResponse{}, err
		}

		metaView, err := txView.TxApplyProcessing()
		if err != nil {
			return TransactionResponse{}, err
		}
		metaBytes, err := metaView.Raw()
		if err != nil {
			return TransactionResponse{}, err
		}

		var hash xdr.Hash
		copy(hash[:], hashBytes)

		return TransactionResponse{
			Index:         uint32(i + 1),
			Result:        resultBytes,
			FeeChanges:    feeBytes,
			UnsafeMeta:    metaBytes,
			LedgerVersion: ledgerVersionVal,
			Hash:          hash,
		}, nil
	}

	return TransactionResponse{}, fmt.Errorf("transaction not found")
}
