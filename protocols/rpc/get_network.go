package protocol

import (
	"github.com/stellar/go-stellar-sdk/protocols/stellarcore"
)

const GetNetworkMethodName = "getNetwork"

type GetNetworkRequest struct{}

type GetProtocolVersions struct {
	MinSupportedProtocolVersion  int `json:"minSupportedProtocolVersion"`
	MaxSupportedProtocolVersion  int `json:"maxSupportedProtocolVersion"`
	CoreSupportedProtocolVersion int `json:"coreSupportedProtocolVersion"`
}

type GetNetworkResponse struct {
	FriendbotURL     string                          `json:"friendbotUrl,omitempty"`
	Passphrase       string                          `json:"passphrase"`
	Build            string                          `json:"build"`
	ProtocolVersions GetProtocolVersions             `json:"protocolVersions"`
	Limits           stellarcore.SorobanInfoResponse `json:"limits"`
}
