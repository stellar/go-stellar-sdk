package rpcclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

// jsonRPCRequest represents a JSON-RPC 2.0 request
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      any             `json:"id"`
}

// jsonRPCResponse represents a JSON-RPC 2.0 response
type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
	ID      any    `json:"id"`
}

func TestClient_GetHealth(t *testing.T) {
	expectedResponse := protocol.GetHealthResponse{
		Status:                "healthy",
		LatestLedger:          1000,
		OldestLedger:          100,
		LedgerRetentionWindow: 900,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.GetHealthMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	health, err := client.GetHealth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.Status, health.Status)
	assert.Equal(t, expectedResponse.LatestLedger, health.LatestLedger)
	assert.Equal(t, expectedResponse.OldestLedger, health.OldestLedger)
	assert.Equal(t, expectedResponse.LedgerRetentionWindow, health.LedgerRetentionWindow)
}

func TestClient_GetNetwork(t *testing.T) {
	expectedResponse := protocol.GetNetworkResponse{
		FriendbotURL:    "https://friendbot.stellar.org",
		Passphrase:      "Test SDF Network ; September 2015",
		ProtocolVersion: 21,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.GetNetworkMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	network, err := client.GetNetwork(context.Background())
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.FriendbotURL, network.FriendbotURL)
	assert.Equal(t, expectedResponse.Passphrase, network.Passphrase)
	assert.Equal(t, expectedResponse.ProtocolVersion, network.ProtocolVersion)
}

func TestClient_GetVersionInfo(t *testing.T) {
	expectedResponse := protocol.GetVersionInfoResponse{
		Version:            "1.0.0",
		CommitHash:         "abc123",
		BuildTimestamp:     "2024-01-01T00:00:00Z",
		CaptiveCoreVersion: "v19.10.0",
		ProtocolVersion:    21,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.GetVersionInfoMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	versionInfo, err := client.GetVersionInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.Version, versionInfo.Version)
	assert.Equal(t, expectedResponse.CommitHash, versionInfo.CommitHash)
	assert.Equal(t, expectedResponse.BuildTimestamp, versionInfo.BuildTimestamp)
	assert.Equal(t, expectedResponse.CaptiveCoreVersion, versionInfo.CaptiveCoreVersion)
	assert.Equal(t, expectedResponse.ProtocolVersion, versionInfo.ProtocolVersion)
}

func TestClient_GetLatestLedger(t *testing.T) {
	expectedResponse := protocol.GetLatestLedgerResponse{
		Hash:            "abcd1234",
		ProtocolVersion: 21,
		Sequence:        12345,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.GetLatestLedgerMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	ledger, err := client.GetLatestLedger(context.Background())
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.Hash, ledger.Hash)
	assert.Equal(t, expectedResponse.ProtocolVersion, ledger.ProtocolVersion)
	assert.Equal(t, expectedResponse.Sequence, ledger.Sequence)
}

func TestClient_GetLedgers(t *testing.T) {
	expectedResponse := protocol.GetLedgersResponse{
		Ledgers: []protocol.LedgerInfo{
			{
				Hash:            "abc123",
				Sequence:        1000,
				LedgerCloseTime: 1234567890,
			},
		},
		LatestLedger: 1000,
		OldestLedger: 100,
		Cursor:       "cursor123",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.GetLedgersMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	resp, err := client.GetLedgers(context.Background(), protocol.GetLedgersRequest{
		StartLedger: 1000,
	})
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.LatestLedger, resp.LatestLedger)
	assert.Equal(t, expectedResponse.OldestLedger, resp.OldestLedger)
	assert.Len(t, resp.Ledgers, 1)
	assert.Equal(t, uint32(1000), resp.Ledgers[0].Sequence)
}

func TestClient_GetFeeStats(t *testing.T) {
	expectedResponse := protocol.GetFeeStatsResponse{
		LatestLedger: 1000,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.GetFeeStatsMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	feeStats, err := client.GetFeeStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.LatestLedger, feeStats.LatestLedger)
}

func TestClient_GetLedgerEntries(t *testing.T) {
	expectedResponse := protocol.GetLedgerEntriesResponse{
		Entries:      []protocol.LedgerEntryResult{},
		LatestLedger: 1000,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.GetLedgerEntriesMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	resp, err := client.GetLedgerEntries(context.Background(), protocol.GetLedgerEntriesRequest{
		Keys: []string{"AAA"},
	})
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.LatestLedger, resp.LatestLedger)
	assert.Empty(t, resp.Entries)
}

func TestClient_GetEvents(t *testing.T) {
	expectedResponse := protocol.GetEventsResponse{
		Events:       []protocol.EventInfo{},
		LatestLedger: 1000,
		OldestLedger: 100,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.GetEventsMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	resp, err := client.GetEvents(context.Background(), protocol.GetEventsRequest{
		StartLedger: 500,
		Filters:     []protocol.EventFilter{},
	})
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.LatestLedger, resp.LatestLedger)
	assert.Empty(t, resp.Events)
}

func TestClient_GetTransaction(t *testing.T) {
	expectedResponse := protocol.GetTransactionResponse{
		TransactionDetails: protocol.TransactionDetails{
			Status: protocol.TransactionStatusNotFound,
		},
		LatestLedger: 1000,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.GetTransactionMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	resp, err := client.GetTransaction(context.Background(), protocol.GetTransactionRequest{
		Hash: "abc123",
	})
	require.NoError(t, err)
	assert.Equal(t, protocol.TransactionStatusNotFound, resp.Status)
}

func TestClient_GetTransactions(t *testing.T) {
	expectedResponse := protocol.GetTransactionsResponse{
		Transactions: []protocol.TransactionInfo{},
		LatestLedger: 1000,
		OldestLedger: 100,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.GetTransactionsMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	resp, err := client.GetTransactions(context.Background(), protocol.GetTransactionsRequest{
		StartLedger: 500,
	})
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.LatestLedger, resp.LatestLedger)
	assert.Empty(t, resp.Transactions)
}

func TestClient_SimulateTransaction(t *testing.T) {
	expectedResponse := protocol.SimulateTransactionResponse{
		LatestLedger:   1000,
		MinResourceFee: 100,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.SimulateTransactionMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	resp, err := client.SimulateTransaction(context.Background(), protocol.SimulateTransactionRequest{
		Transaction: "AAAA",
	})
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.LatestLedger, resp.LatestLedger)
	assert.Equal(t, expectedResponse.MinResourceFee, resp.MinResourceFee)
}

func TestClient_SendTransaction(t *testing.T) {
	expectedResponse := protocol.SendTransactionResponse{
		Status:       "PENDING",
		Hash:         "abc123",
		LatestLedger: 1000,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.SendTransactionMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	resp, err := client.SendTransaction(context.Background(), protocol.SendTransactionRequest{
		Transaction: "AAAA",
	})
	require.NoError(t, err)
	assert.Equal(t, expectedResponse.Status, resp.Status)
	assert.Equal(t, expectedResponse.Hash, resp.Hash)
}

func TestClient_LoadAccount_InvalidAddress(t *testing.T) {
	client := NewClient("http://localhost:1234", nil)
	defer client.Close()

	_, err := client.LoadAccount(context.Background(), "invalid-address")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid Stellar account")
}

func TestPollTransaction_Success(t *testing.T) {
	txHash := "abc1"
	expectedResponse := protocol.GetTransactionResponse{
		TransactionDetails: protocol.TransactionDetails{
			Status:          protocol.TransactionStatusSuccess,
			TransactionHash: txHash,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		require.Equal(t, protocol.GetTransactionMethodName, req.Method)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	ctx := context.Background()
	result, err := client.PollTransaction(ctx, txHash)

	require.NoError(t, err)
	assert.Equal(t, protocol.TransactionStatusSuccess, result.Status)
	assert.Equal(t, txHash, result.TransactionHash)
}

func TestPollTransaction_Failed(t *testing.T) {
	txHash := "abc2"
	expectedResponse := protocol.GetTransactionResponse{
		TransactionDetails: protocol.TransactionDetails{
			Status:          protocol.TransactionStatusFailed,
			TransactionHash: txHash,
			ResultXDR:       "some-result-xdr",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  expectedResponse,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	ctx := context.Background()
	result, err := client.PollTransaction(ctx, txHash)

	require.NoError(t, err)
	assert.Equal(t, protocol.TransactionStatusFailed, result.Status)
	assert.Equal(t, "some-result-xdr", result.ResultXDR)
}

func TestPollTransactionWithOptions_PollsUntilSuccess(t *testing.T) {
	txHash := "abc3"
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		count := callCount.Add(1)

		var response protocol.GetTransactionResponse
		if count < 3 {
			// First two calls return NOT_FOUND
			response = protocol.GetTransactionResponse{
				TransactionDetails: protocol.TransactionDetails{
					Status: protocol.TransactionStatusNotFound,
				},
			}
		} else {
			// Third call returns SUCCESS
			response = protocol.GetTransactionResponse{
				TransactionDetails: protocol.TransactionDetails{
					Status:          protocol.TransactionStatusSuccess,
					TransactionHash: txHash,
				},
			}
		}

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  response,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	ctx := context.Background()
	// Use short intervals to speed up the test
	opts := NewPollTransactionOptions().
		WithInitialInterval(10 * time.Millisecond).
		WithMaxInterval(50 * time.Millisecond)
	result, err := client.PollTransactionWithOptions(ctx, txHash, opts)

	require.NoError(t, err)
	assert.Equal(t, protocol.TransactionStatusSuccess, result.Status)
	assert.Equal(t, int32(3), callCount.Load(), "expected 3 calls to GetTransaction")
}

func TestPollTransactionWithOptions_ContextTimeout(t *testing.T) {
	txHash := "abc4"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Always return NOT_FOUND to keep polling
		response := protocol.GetTransactionResponse{
			TransactionDetails: protocol.TransactionDetails{
				Status: protocol.TransactionStatusNotFound,
			},
		}

		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Result:  response,
			ID:      req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	// Use a very short timeout to ensure we hit it
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Use short intervals so we poll a few times before timeout
	opts := NewPollTransactionOptions().
		WithInitialInterval(10 * time.Millisecond).
		WithMaxInterval(20 * time.Millisecond)
	_, err := client.PollTransactionWithOptions(ctx, txHash, opts)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestPollTransactionWithOptions_RPCError(t *testing.T) {
	txHash := "abc5"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Return a JSON-RPC error
		resp := jsonRPCResponse{
			JSONRPC: "2.0",
			Error: map[string]any{
				"code":    -32000,
				"message": "internal server error",
			},
			ID: req.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	ctx := context.Background()
	_, err := client.PollTransactionWithOptions(ctx, txHash, NewPollTransactionOptions())

	require.Error(t, err)
}
