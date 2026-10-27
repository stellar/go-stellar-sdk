package strkey

import (
	"encoding/binary"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

type MuxedContract struct {
	id         uint64
	contractID [32]byte
}

// SetID populates the muxed contract ID.
func (m *MuxedContract) SetID(id uint64) {
	m.id = id
}

// SetContractID populates the muxed contract's underlying C-address.
func (m *MuxedContract) SetContractID(address string) error {
	raw, err := Decode(VersionByteContract, address)
	if err != nil {
		return errors.New("invalid contract address")
	}

	copy(m.contractID[:], raw)

	return nil
}

// ID returns the muxed contract id.
func (m *MuxedContract) ID() uint64 {
	return m.id
}

// ContractID returns the muxed contract's underlying C-address.
func (m *MuxedContract) ContractID() (string, error) {
	return Encode(VersionByteContract, m.contractID[:])
}

// Contract returns the muxed contract's raw 32-byte contract id.
func (m *MuxedContract) Contract() [32]byte {
	return m.contractID
}

// Address returns the muxed contract W-address.
func (m *MuxedContract) Address() (string, error) {
	var raw [40]byte
	copy(raw[:32], m.contractID[:])
	binary.BigEndian.PutUint64(raw[32:], m.id)
	return Encode(VersionByteMuxedContract, raw[:])
}

// DecodeMuxedContract receives a muxed contract W-address and parses it into
// a MuxedContract object containing a contract id and a muxed id.
func DecodeMuxedContract(address string) (*MuxedContract, error) {
	raw, err := Decode(VersionByteMuxedContract, address)
	if err != nil {
		return nil, errors.New("invalid muxed contract")
	}

	var muxed MuxedContract
	copy(muxed.contractID[:], raw[:32])
	muxed.id = binary.BigEndian.Uint64(raw[32:])

	return &muxed, nil
}
