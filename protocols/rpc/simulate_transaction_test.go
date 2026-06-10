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

func TestSimulateTransactionRequestAuthV2(t *testing.T) {
	t.Run("defaults to false when omitted", func(t *testing.T) {
		var request SimulateTransactionRequest
		require.NoError(t, json.Unmarshal(
			[]byte(`{ "transaction": "pretend this is XDR" }`), &request))
		require.False(t, request.AuthV2)
	})

	t.Run("unmarshals authV2:true", func(t *testing.T) {
		var request SimulateTransactionRequest
		require.NoError(t, json.Unmarshal(
			[]byte(`{ "transaction": "pretend this is XDR", "authV2": true }`), &request))
		require.True(t, request.AuthV2)
	})

	t.Run("unmarshals authV2:false", func(t *testing.T) {
		var request SimulateTransactionRequest
		require.NoError(t, json.Unmarshal(
			[]byte(`{ "transaction": "pretend this is XDR", "authV2": false }`), &request))
		require.False(t, request.AuthV2)
	})

	t.Run("omitempty drops authV2 when false", func(t *testing.T) {
		data, err := json.Marshal(SimulateTransactionRequest{Transaction: "xdr"})
		require.NoError(t, err)
		require.NotContains(t, string(data), "authV2")
	})

	t.Run("marshals authV2 when true", func(t *testing.T) {
		data, err := json.Marshal(SimulateTransactionRequest{Transaction: "xdr", AuthV2: true})
		require.NoError(t, err)
		require.Contains(t, string(data), `"authV2":true`)
	})

	t.Run("round-trips alongside other fields", func(t *testing.T) {
		original := SimulateTransactionRequest{
			Transaction: "xdr",
			AuthMode:    AuthModeRecord,
			AuthV2:      true,
		}
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded SimulateTransactionRequest
		require.NoError(t, json.Unmarshal(data, &decoded))
		require.Equal(t, original, decoded)
	})
}
