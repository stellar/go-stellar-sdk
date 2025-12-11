package protocol

const GetVersionInfoMethodName = "getVersionInfo"

type GetVersionInfoRequest struct{}

type GetVersionInfoResponse struct {
	Version            string `json:"version"`
	CommitHash         string `json:"commitHash"`
	BuildTimestamp     string `json:"buildTimestamp"`
	CaptiveCoreVersion string `json:"captiveCoreVersion"`
	ProtocolVersion    uint32 `json:"protocolVersion"`
}
