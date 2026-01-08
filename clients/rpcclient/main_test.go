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
