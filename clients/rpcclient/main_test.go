package rpcclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
