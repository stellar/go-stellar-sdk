package xdr

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/strkey"
)

func (n NodeId) GetAddress() (string, error) {
	switch n.Type {
	case PublicKeyTypePublicKeyTypeEd25519:
		ed, ok := n.GetEd25519()
		if !ok {
			return "", fmt.Errorf("could not get NodeID.Ed25519")
		}
		raw := make([]byte, 32)
		copy(raw, ed[:])
		encodedAddress, err := strkey.Encode(strkey.VersionByteAccountID, raw)
		if err != nil {
			return "", err
		}
		return encodedAddress, nil
	default:
		return "", fmt.Errorf("unknown NodeId.PublicKeyType")
	}
}
