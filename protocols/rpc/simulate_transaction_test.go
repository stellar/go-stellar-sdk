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
