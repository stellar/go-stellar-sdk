package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTransactionRequestIsValid(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request GetTransactionRequest
		wantErr string // empty means valid
	}{
		{
			name:    "no bounds",
			request: GetTransactionRequest{Hash: "abc"},
		},
		{
			name:    "only minLedger",
			request: GetTransactionRequest{Hash: "abc", MinLedger: 100},
		},
		{
			name:    "only maxLedger",
			request: GetTransactionRequest{Hash: "abc", MaxLedger: 100},
		},
		{
			name:    "equal bounds",
			request: GetTransactionRequest{Hash: "abc", MinLedger: 100, MaxLedger: 100},
		},
		{
			name:    "min below max",
			request: GetTransactionRequest{Hash: "abc", MinLedger: 100, MaxLedger: 200},
		},
		{
			name:    "min above max",
			request: GetTransactionRequest{Hash: "abc", MinLedger: 200, MaxLedger: 100},
			wantErr: "minLedger (200) must not exceed maxLedger (100)",
		},
		{
			name:    "invalid format",
			request: GetTransactionRequest{Hash: "abc", Format: "hex"},
			wantErr: "got 'hex': expected base64, json for optional 'xdrFormat'",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.request.IsValid()
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

func TestGetTransactionRequestJSON(t *testing.T) {
	t.Run("bounds omitted", func(t *testing.T) {
		raw, err := json.Marshal(GetTransactionRequest{Hash: "abc"})
		require.NoError(t, err)

		var fields map[string]any
		require.NoError(t, json.Unmarshal(raw, &fields))
		assert.NotContains(t, fields, "minLedger")
		assert.NotContains(t, fields, "maxLedger")
	})

	t.Run("bounds present", func(t *testing.T) {
		raw, err := json.Marshal(GetTransactionRequest{Hash: "abc", MinLedger: 100, MaxLedger: 200})
		require.NoError(t, err)

		var fields map[string]any
		require.NoError(t, json.Unmarshal(raw, &fields))
		assert.Equal(t, float64(100), fields["minLedger"])
		assert.Equal(t, float64(200), fields["maxLedger"])
	})

	t.Run("round trip", func(t *testing.T) {
		request := GetTransactionRequest{Hash: "abc", Format: FormatJSON, MinLedger: 100, MaxLedger: 200}
		raw, err := json.Marshal(request)
		require.NoError(t, err)

		var decoded GetTransactionRequest
		require.NoError(t, json.Unmarshal(raw, &decoded))
		assert.Equal(t, request, decoded)
	})
}
