package xdr

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/strkey"
)

func (e *EncodingBuffer) claimableBalanceCompressEncodeTo(cb ClaimableBalanceId) error {
	if err := e.xdrEncoderBuf.WriteByte(byte(cb.Type)); err != nil {
		return err
	}
	switch cb.Type {
	case ClaimableBalanceIdTypeClaimableBalanceIdTypeV0:
		_, err := e.xdrEncoderBuf.Write(cb.V0[:])
		return err
	default:
		panic("Unknown type")
	}
}

// EncodeToStrkey returns the strkey encoding of the given claimable balance id
// See SEP-23: https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0023.md
func (c ClaimableBalanceId) EncodeToStrkey() (string, error) {

	switch c.Type {
	case ClaimableBalanceIdTypeClaimableBalanceIdTypeV0:
		hash := c.MustV0()
		payload := make([]byte, 0, len(hash)+1)
		payload = append(payload, byte(c.Type))
		payload = append(payload, hash[:]...)
		return strkey.Encode(strkey.VersionByteClaimableBalance, payload)
	default:
		return "", fmt.Errorf("unknown claimable balance id type: %v", c.Type)
	}
}

// MustEncodeToStrkey returns the strkey encoding of the given claimable balance id
// See SEP-23: https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0023.md
func (c ClaimableBalanceId) MustEncodeToStrkey() string {
	address, err := c.EncodeToStrkey()
	if err != nil {
		panic(err)
	}
	return address
}

// DecodeFromStrkey parses a strkey encoded address into the given claimable balance id
// See SEP-23: https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0023.md
func (c *ClaimableBalanceId) DecodeFromStrkey(address string) error {
	payload, err := strkey.Decode(strkey.VersionByteClaimableBalance, address)
	if err != nil {
		return err
	}
	expectedLen := len(Hash{}) + 1
	if len(payload) != expectedLen {
		return fmt.Errorf("invalid payload length, expected %v but got %v", expectedLen, len(payload))
	}
	if ClaimableBalanceIdType(payload[0]) != ClaimableBalanceIdTypeClaimableBalanceIdTypeV0 {
		return fmt.Errorf("invalid claimable balance id type: %v", payload[0])
	}
	c.Type = ClaimableBalanceIdTypeClaimableBalanceIdTypeV0
	c.V0 = &Hash{}
	copy(c.V0[:], payload[1:])
	return nil
}
