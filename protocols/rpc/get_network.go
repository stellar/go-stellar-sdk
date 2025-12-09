package protocol

import (
	"github.com/stellar/go-stellar-sdk/protocols/stellarcore"
)

const GetNetworkMethodName = "getNetwork"

type GetNetworkRequest struct{}

type GetNetworkResponse struct {
	FriendbotURL                 string                    `json:"friendbotUrl,omitempty"`
	Passphrase                   string                    `json:"passphrase"`
	ProtocolVersion              int                       `json:"protocolVersion"`
	CoreSupportedProtocolVersion int                       `json:"coreSupportedProtocolVersion"`
	Limits                       stellarcore.NetworkLimits `json:"limits"`
}
