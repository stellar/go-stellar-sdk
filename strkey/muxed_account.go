package strkey

import (
	"bytes"
	"fmt"

	xdr "github.com/stellar/go-xdr/xdr3"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

type MuxedAccount struct {
	id      uint64
	ed25519 [32]byte
}

// SetID populates the muxed account ID.
func (m *MuxedAccount) SetID(id uint64) {
	m.id = id
}

// SetAccountID populates the muxed account G-address.
func (m *MuxedAccount) SetAccountID(address string) error {
	raw, err := Decode(VersionByteAccountID, address)
	if err != nil {
		return errors.New("invalid ed25519 public key")
	}
	if len(raw) != 32 {
		return fmt.Errorf("invalid binary length: %d", len(raw))
	}

	copy(m.ed25519[:], raw)

	return nil
}

// ID returns the muxed account id according with the SEP-23 definition for
// multiplexed accounts.
func (m *MuxedAccount) ID() uint64 {
	return m.id
}

// AccountID returns the muxed account G-address according with the SEP-23
// definition for multiplexed accounts.
func (m *MuxedAccount) AccountID() (string, error) {
	return Encode(VersionByteAccountID, m.ed25519[:])
}

// Ed25519 returns the muxed account ed25519 key.
func (m *MuxedAccount) Ed25519() [32]byte {
	return m.ed25519
}

// Address returns the muxed account M-address according with the SEP-23
// definition for multiplexed accounts.
func (m *MuxedAccount) Address() (string, error) {
	if m.ed25519 == [32]byte{} {
		return "", errors.New("muxed account has no ed25519 key")
	}

	b := new(bytes.Buffer)
	_, err := xdr.Marshal(b, m.id)
	if err != nil {
		return "", errors.Wrap(err, "marshaling muxed address id")
	}

	raw := make([]byte, 0, 40)
	raw = append(raw, m.ed25519[:]...)
	raw = append(raw, b.Bytes()...)

	return Encode(VersionByteMuxedAccount, raw)
}

// DecodeMuxedAccount receives a muxed account M-address and parses it into a
// MuxedAccount object containing an ed25519 address and an id.
func DecodeMuxedAccount(address string) (*MuxedAccount, error) {
	raw, err := Decode(VersionByteMuxedAccount, address)
	if err != nil {
		return nil, errors.New("invalid muxed account")
	}
	if len(raw) != 40 {
		return nil, errors.Errorf("invalid binary length: %d", len(raw))
	}

	var muxed MuxedAccount
	copy(muxed.ed25519[:], raw[:32])
	_, err = xdr.Unmarshal(bytes.NewReader(raw[32:]), &muxed.id)
	if err != nil {
		return nil, errors.Wrap(err, "can't marshall binary")
	}

	return &muxed, nil
}
