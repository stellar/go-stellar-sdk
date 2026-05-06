package xdr_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// DBEvent mirrors the data stored per event row in stellar-rpc's DB.
type DBEvent struct {
	ContractID []byte    // 32 bytes or nil
	EventType  int32     // ContractEventType enum
	EventXDR   []byte    // full DiagnosticEvent XDR (for event_data column)
	Topics     [4][]byte // topics 1-4 as individual ScVal XDR (for filtering)
	TxHash     [32]byte
	TxSuccess  bool
}

// BenchmarkExtractAllEvents extracts all contract events from a ledger,
// mirroring the stellar-rpc ingestion pattern.
func BenchmarkExtractAllEvents(b *testing.B) {
	data, err := os.ReadFile("testdata/ledger_58752000.bin")
	if err != nil {
		b.Fatalf("testdata not found: %v", err)
	}

	var lcm xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshal(data, &lcm); err != nil {
		b.Fatal(err)
	}
	numTx := lcm.CountTransactions()

	// Count events for logging
	totalEvents := 0
	v1 := lcm.MustV1()
	for i := 0; i < numTx; i++ {
		meta := v1.TxProcessing[i].TxApplyProcessing
		if meta.V == 3 {
			v3 := meta.MustV3()
			if v3.SorobanMeta != nil {
				totalEvents += len(v3.SorobanMeta.Events)
			}
		}
	}
	b.Logf("Ledger: v%d, %d txs, %d events, %d bytes", lcm.V, numTx, totalEvents, len(data))

	b.Run("full_decode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			events, err := extractAllEventsFullDecode(data)
			if err != nil {
				b.Fatal(err)
			}
			_ = events
		}
	})

	b.Run("view", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			events, err := extractAllEventsView(data)
			if err != nil {
				b.Fatal(err)
			}
			_ = events
		}
	})
}

// BenchmarkExtractEventsByTxHash finds a transaction by hash, then extracts
// all its events.
func BenchmarkExtractEventsByTxHash(b *testing.B) {
	data, err := os.ReadFile("testdata/ledger_58752000.bin")
	if err != nil {
		b.Fatalf("testdata not found: %v", err)
	}

	var lcm xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshal(data, &lcm); err != nil {
		b.Fatal(err)
	}

	// Find a transaction with events
	v1 := lcm.MustV1()
	var targetHash xdr.Hash
	targetEvents := 0
	for i := range v1.TxProcessing {
		meta := v1.TxProcessing[i].TxApplyProcessing
		if meta.V == 3 && meta.MustV3().SorobanMeta != nil && len(meta.MustV3().SorobanMeta.Events) > 0 {
			targetHash = v1.TxProcessing[i].Result.TransactionHash
			targetEvents = len(meta.MustV3().SorobanMeta.Events)
			break
		}
	}
	if targetEvents == 0 {
		b.Skip("no transactions with events in test data")
	}
	b.Logf("Target tx hash=%x..., %d events", targetHash[:4], targetEvents)

	b.Run("full_decode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			events, err := extractEventsByHashFullDecode(data, targetHash)
			if err != nil {
				b.Fatal(err)
			}
			_ = events
		}
	})

	b.Run("view", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			events, err := extractEventsByHashView(data, targetHash)
			if err != nil {
				b.Fatal(err)
			}
			_ = events
		}
	})
}

// --- Full decode implementations ---

func extractAllEventsFullDecode(data []byte) ([]DBEvent, error) {
	var lcm xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshal(data, &lcm); err != nil {
		return nil, err
	}

	v1 := lcm.MustV1()
	var results []DBEvent

	for _, tx := range v1.TxProcessing {
		meta := tx.TxApplyProcessing
		if meta.V != 3 {
			continue
		}
		v3 := meta.MustV3()
		if v3.SorobanMeta == nil {
			continue
		}

		txHash := tx.Result.TransactionHash
		// Determine success from result code
		txSuccess := tx.Result.Result.Result.Code == xdr.TransactionResultCodeTxSuccess

		for _, event := range v3.SorobanMeta.Events {
			dbEvent := DBEvent{
				EventType: int32(event.Type),
				TxHash:    txHash,
				TxSuccess: txSuccess,
			}

			if event.ContractId != nil {
				dbEvent.ContractID = event.ContractId[:]
			}

			// Full DiagnosticEvent XDR
			diagEvent := xdr.DiagnosticEvent{
				InSuccessfulContractCall: txSuccess,
				Event:                    event,
			}
			eventXDR, err := diagEvent.MarshalBinary()
			if err != nil {
				return nil, err
			}
			dbEvent.EventXDR = eventXDR

			// Topics 1-4
			if event.Body.V == 0 && event.Body.V0 != nil {
				for j := 0; j < len(event.Body.V0.Topics) && j < 4; j++ {
					topicXDR, err := event.Body.V0.Topics[j].MarshalBinary()
					if err != nil {
						return nil, err
					}
					dbEvent.Topics[j] = topicXDR
				}
			}

			results = append(results, dbEvent)
		}
	}

	return results, nil
}

func extractEventsByHashFullDecode(data []byte, targetHash xdr.Hash) ([]DBEvent, error) {
	var lcm xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshal(data, &lcm); err != nil {
		return nil, err
	}

	v1 := lcm.MustV1()
	for _, tx := range v1.TxProcessing {
		if tx.Result.TransactionHash != targetHash {
			continue
		}

		meta := tx.TxApplyProcessing
		if meta.V != 3 {
			return nil, nil
		}
		v3 := meta.MustV3()
		if v3.SorobanMeta == nil {
			return nil, nil
		}

		txSuccess := tx.Result.Result.Result.Code == xdr.TransactionResultCodeTxSuccess
		var results []DBEvent

		for _, event := range v3.SorobanMeta.Events {
			dbEvent := DBEvent{
				EventType: int32(event.Type),
				TxHash:    targetHash,
				TxSuccess: txSuccess,
			}

			if event.ContractId != nil {
				dbEvent.ContractID = event.ContractId[:]
			}

			diagEvent := xdr.DiagnosticEvent{
				InSuccessfulContractCall: txSuccess,
				Event:                    event,
			}
			eventXDR, err := diagEvent.MarshalBinary()
			if err != nil {
				return nil, err
			}
			dbEvent.EventXDR = eventXDR

			if event.Body.V == 0 && event.Body.V0 != nil {
				for j := 0; j < len(event.Body.V0.Topics) && j < 4; j++ {
					topicXDR, err := event.Body.V0.Topics[j].MarshalBinary()
					if err != nil {
						return nil, err
					}
					dbEvent.Topics[j] = topicXDR
				}
			}

			results = append(results, dbEvent)
		}
		return results, nil
	}

	return nil, nil // tx not found
}

// --- View implementations ---

func extractAllEventsView(data []byte) ([]DBEvent, error) {
	view := xdr.LedgerCloseMetaView(data)
	v1, err := view.V1()
	if err != nil {
		return nil, err
	}

	var results []DBEvent

	txArr, err := v1.TxProcessing()
	if err != nil {
		return nil, err
	}
	for tx, iterErr := range txArr.Iter() {
		if iterErr != nil {
			return nil, iterErr
		}

		events, err := extractEventsFromTxView(data, &tx)
		if err != nil {
			return nil, err
		}
		results = append(results, events...)
	}

	return results, nil
}

func extractEventsByHashView(data []byte, targetHash xdr.Hash) ([]DBEvent, error) {
	view := xdr.LedgerCloseMetaView(data)
	v1, err := view.V1()
	if err != nil {
		return nil, err
	}

	txArr, err := v1.TxProcessing()
	if err != nil {
		return nil, err
	}
	for tx, iterErr := range txArr.Iter() {
		if iterErr != nil {
			return nil, iterErr
		}

		// Check hash
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
		if !bytes.Equal(hashBytes, targetHash[:]) {
			continue
		}

		// Found — extract events
		return extractEventsFromTxView(data, &tx)
	}

	return nil, nil // not found
}

// extractEventsFromTxView extracts all contract events from a single
// TransactionResultMeta view.
func extractEventsFromTxView(ledgerData []byte, tx *xdr.TransactionResultMetaView) ([]DBEvent, error) {
	// Get tx hash and success flag so the view path does the same work as
	// the full-decode comparison (which reads both from the transaction result).
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
	var txHash [32]byte
	copy(txHash[:], hashBytes)

	txResult, err := resultView.Result()
	if err != nil {
		return nil, err
	}
	resultResult, err := txResult.Result()
	if err != nil {
		return nil, err
	}
	codeView, err := resultResult.Code()
	if err != nil {
		return nil, err
	}
	code, err := codeView.Value()
	if err != nil {
		return nil, err
	}
	txSuccess := code == xdr.TransactionResultCodeTxSuccess

	// Navigate to TxApplyProcessing → TransactionMeta
	metaView, err := tx.TxApplyProcessing()
	if err != nil {
		return nil, err
	}
	metaV, err := metaView.V()
	if err != nil {
		return nil, err
	}
	metaVVal, err := metaV.Value()
	if err != nil {
		return nil, err
	}
	if metaVVal != 3 {
		return nil, nil
	}

	v3, err := metaView.V3()
	if err != nil {
		return nil, err
	}

	// Check SorobanMeta optional
	sorobanOpt, err := v3.SorobanMeta()
	if err != nil {
		return nil, err
	}
	sorobanMeta, present, err := sorobanOpt.Unwrap()
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}

	var results []DBEvent

	eventsArr, err := sorobanMeta.Events()
	if err != nil {
		return nil, err
	}
	for event, eventErr := range eventsArr.Iter() {
		if eventErr != nil {
			return nil, eventErr
		}

		dbEvent := DBEvent{TxHash: txHash, TxSuccess: txSuccess}

		// Event type
		evType, err := event.Type()
		if err != nil {
			return nil, err
		}
		evTypeVal, err := evType.Value()
		if err != nil {
			return nil, err
		}
		dbEvent.EventType = int32(evTypeVal)

		// Contract ID (optional)
		cidOpt, err := event.ContractId()
		if err != nil {
			return nil, err
		}
		cidView, present, err := cidOpt.Unwrap()
		if err != nil {
			return nil, err
		}
		if present {
			dbEvent.ContractID, err = cidView.Value()
			if err != nil {
				return nil, err
			}
		}

		// Full event XDR — synthesize a DiagnosticEvent by prepending the
		// bool discriminant to the ContractEvent bytes.
		// DiagnosticEvent = { bool inSuccessfulContractCall; ContractEvent event; }
		// XDR: 4 bytes (bool) + ContractEvent bytes.
		eventRaw, err := event.Raw()
		if err != nil {
			return nil, err
		}
		diagXDR := make([]byte, 4+len(eventRaw))
		if txSuccess {
			diagXDR[3] = 1 // big-endian bool true
		}
		copy(diagXDR[4:], eventRaw)
		dbEvent.EventXDR = diagXDR

		// Topics 1-4 as individual XDR blobs
		body, err := event.Body()
		if err != nil {
			return nil, err
		}
		bodyV, err := body.V()
		if err != nil {
			return nil, err
		}
		bodyVVal, err := bodyV.Value()
		if err != nil {
			return nil, err
		}
		if bodyVVal == 0 {
			v0, err := body.V0()
			if err != nil {
				return nil, err
			}
			topicsArr, err := v0.Topics()
			if err != nil {
				return nil, err
			}
			topicCount, err := topicsArr.Count()
			if err != nil {
				return nil, err
			}
			for j := 0; j < topicCount && j < 4; j++ {
				topic, err := topicsArr.At(j)
				if err != nil {
					return nil, err
				}
				topicRaw, err := topic.Raw()
				if err != nil {
					return nil, err
				}
				dbEvent.Topics[j] = topicRaw
			}
		}

		results = append(results, dbEvent)
	}

	return results, nil
}
