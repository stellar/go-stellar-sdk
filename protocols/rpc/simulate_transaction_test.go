package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSimulatingNonRootAuth(t *testing.T) {
	var request SimulateTransactionRequest
	requestString := `{ "transaction": "pretend this is XDR" }`

	require.NoError(t, json.Unmarshal([]byte(requestString), &request))
	require.Empty(t, request.AuthMode) // ensure false if omitted

	requestString = `{ "transaction": "pretend this is XDR", "authMode": "record" }`
	require.NoError(t, json.Unmarshal([]byte(requestString), &request))
	require.Equal(t, AuthModeRecord, request.AuthMode)
}

func TestSimulateTransactionResponseMemoryBytes(t *testing.T) {
	var resp SimulateTransactionResponse
	require.NoError(t, json.Unmarshal([]byte(`{ "memoryBytes": 1234, "latestLedger": 1 }`), &resp))
	require.Equal(t, uint64(1234), resp.MemoryBytes)

	out, err := json.Marshal(SimulateTransactionResponse{MemoryBytes: 42})
	require.NoError(t, err)
	require.JSONEq(t, `{"memoryBytes": 42, "latestLedger": 0}`, string(out))

	// omitted when zero
	out, err = json.Marshal(SimulateTransactionResponse{})
	require.NoError(t, err)
	require.JSONEq(t, `{"latestLedger": 0}`, string(out))
}
