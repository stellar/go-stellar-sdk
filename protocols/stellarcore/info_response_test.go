package stellarcore

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfoResponse_IsSynced(t *testing.T) {
	cases := []struct {
		Name     string
		JSON     string
		Expected bool
	}{
		{
			Name: "synced",
			JSON: `{
				"info": {
						"UNSAFE_QUORUM": "UNSAFE QUORUM ALLOWED",
						"build": "v0.6.4",
						"ledger": {
								"age": 0,
								"closeTime": 1512512956,
								"hash": "a0035988ef68f225df4fb37b4639b8648c2d77dc4b3b1b0f5cd3bfa385fb4cc3",
								"num": 5787995
						},
						"network": "Test SDF Network ; September 2015",
						"numPeers": 6,
						"protocol_version": 8,
						"quorum": {
								"5787994": {
										"agree": 3,
										"disagree": 0,
										"fail_at": 2,
										"hash": "273af2",
										"missing": 0,
										"phase": "EXTERNALIZE"
								}
						},
						"state": "Synced!"
				}
			}`,
			Expected: true,
		},
		{
			Name: "joining scp",
			JSON: `{
				"info": {
						"UNSAFE_QUORUM": "UNSAFE QUORUM ALLOWED",
						"build": "v0.6.4",
						"ledger": {
								"age": 17,
								"closeTime": 1512520421,
								"hash": "263c1e575422e960cb1b51a38feac8f54947d1cd6ba8c7f1da5302b063ad7045",
								"num": 5789919
						},
						"network": "Test SDF Network ; September 2015",
						"numPeers": 0,
						"protocol_version": 8,
						"state": "Joining SCP"
				}
			}`,
			Expected: false,
		},
	}

	for _, kase := range cases {
		t.Run(kase.Name, func(t *testing.T) {
			var resp InfoResponse
			err := json.Unmarshal([]byte(kase.JSON), &resp)
			require.NoError(t, err)

			assert.True(t, kase.Expected == resp.IsSynced(), "sync state is unexpected")
		})
	}
}
