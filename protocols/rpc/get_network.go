package protocol

const GetNetworkMethodName = "getNetwork"

type GetNetworkRequest struct{}

type GetNetworkResponse struct {
	FriendbotURL                 string `json:"friendbotUrl,omitempty"`
	Passphrase                   string `json:"passphrase"`
	ProtocolVersion              int    `json:"protocolVersion"`
	CoreSupportedProtocolVersion int    `json:"coreSupportedProtocolVersion"`
}
