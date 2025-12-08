package xdr

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/stellar/go-stellar-sdk/strkey"
)

func MustMuxedAddress(address string) MuxedAccount {
	muxed := MuxedAccount{}
	err := muxed.SetAddress(address)
	if err != nil {
		panic(err)
	}
	return muxed
}

func MustMuxedAddressPtr(address string) *MuxedAccount {
	muxed := MustMuxedAddress(address)
	return &muxed
}

func MuxedAccountFromAccountId(gAddress string, id uint64) (MuxedAccount, error) {
	accountId, err := AddressToAccountId(gAddress)
	if err != nil {
		return MuxedAccount{}, err
	}

	return NewMuxedAccount(
		CryptoKeyTypeKeyTypeMuxedEd25519,
		MuxedAccountMed25519{
			Id:      Uint64(id),
			Ed25519: accountId.MustEd25519(),
		},
	)
}

// SetEd25519Address modifies the receiver, setting it's value to the MuxedAccount form
// of the provided G-address. Unlike SetAddress(), it only supports G-addresses.
func (m *MuxedAccount) SetEd25519Address(address string) error {
	if m == nil {
		return nil
	}

	switch len(address) {
	case 56:
		raw, err := strkey.Decode(strkey.VersionByteAccountID, address)
		if err != nil {
			return err
		}
		if len(raw) != 32 {
			return fmt.Errorf("invalid binary length: %d", len(raw))
		}
		var ui Uint256
		copy(ui[:], raw)
		*m, err = NewMuxedAccount(CryptoKeyTypeKeyTypeEd25519, ui)
		return err
	default:
		return errors.New("invalid address length")
	}

}

// SetAddress modifies the receiver, setting it's value to the MuxedAccount form
// of the provided strkey G-address or M-address, as described in SEP23.
func (m *MuxedAccount) SetAddress(address string) error {
	if m == nil {
		return nil
	}

	switch len(address) {
	case 56:
		return m.SetEd25519Address(address)
	case 69:
		raw, err := strkey.Decode(strkey.VersionByteMuxedAccount, address)
		if err != nil {
			return err
		}
		if len(raw) != 40 {
			return fmt.Errorf("invalid binary length: %d", len(raw))
		}
		var muxed MuxedAccountMed25519
		copy(muxed.Ed25519[:], raw[:32])
		if err = muxed.Id.UnmarshalBinary(raw[32:]); err != nil {
			return err
		}
		*m, err = NewMuxedAccount(CryptoKeyTypeKeyTypeMuxedEd25519, muxed)
		return err
	default:
		return errors.New("invalid address length")
	}

}

// AddressToMuxedAccount returns an MuxedAccount for a given address string
// or SEP23 M-address.
// If the address is not valid the error returned will not be nil
func AddressToMuxedAccount(address string) (MuxedAccount, error) {
	result := MuxedAccount{}
	err := result.SetAddress(address)

	return result, err
}

// Address returns the strkey-encoded form of this MuxedAccount. In particular, it will
// return an M- strkey representation for CryptoKeyTypeKeyTypeMuxedEd25519 variants of the account
// (according to SEP23). This method will panic if the MuxedAccount is backed by a public key of an
// unknown type.
func (m *MuxedAccount) Address() string {
	address, err := m.GetAddress()
	if err != nil {
		panic(err)
	}
	return address
}

// GetAddress returns the strkey-encoded form of this MuxedAccount. In particular, it will
// return an M-strkey representation for CryptoKeyTypeKeyTypeMuxedEd25519 variants of the account
// (according to SEP23). In addition it will return an error if the MuxedAccount is backed by a
// public key of an unknown type.
func (m *MuxedAccount) GetAddress() (string, error) {
	if m == nil {
		return "", nil
	}

	raw := make([]byte, 0, 40)
	switch m.Type {
	case CryptoKeyTypeKeyTypeEd25519:
		ed, ok := m.GetEd25519()
		if !ok {
			return "", errors.New("could not get Ed25519")
		}
		raw = append(raw, ed[:]...)
		return strkey.Encode(strkey.VersionByteAccountID, raw)
	case CryptoKeyTypeKeyTypeMuxedEd25519:
		ed, ok := m.GetMed25519()
		if !ok {
			return "", errors.New("could not get Med25519")
		}
		idBytes, err := ed.Id.MarshalBinary()
		if err != nil {
			return "", errors.Wrap(err, "could not marshal ID")
		}
		raw = append(raw, ed.Ed25519[:]...)
		raw = append(raw, idBytes...)
		return strkey.Encode(strkey.VersionByteMuxedAccount, raw)
	default:
		return "", fmt.Errorf("Unknown muxed account type: %v", m.Type)
	}
}

// GetId retrieves the underlying memo ID if this is a fully muxed account. It
// will return an error if the muxed account does not have a memo ID (i.e it's
// of the key type Ed25519).
func (m *MuxedAccount) GetId() (uint64, error) {
	if m == nil {
		return 0, nil
	}

	switch m.Type {
	case CryptoKeyTypeKeyTypeEd25519:
		return 0, errors.New("muxed account has no ID")

	case CryptoKeyTypeKeyTypeMuxedEd25519:
		ed, ok := m.GetMed25519()
		if !ok {
			return 0, errors.New("could not get Med25519")
		}
		return uint64(ed.Id), nil

	default:
		return 0, fmt.Errorf("Unknown muxed account type: %v", m.Type)
	}
}

// ToAccountId transforms a MuxedAccount to an AccountId, dropping the
// memo Id if necessary
func (m MuxedAccount) ToAccountId() AccountId {
	result := AccountId{Type: PublicKeyTypePublicKeyTypeEd25519}
	switch m.Type {
	case CryptoKeyTypeKeyTypeEd25519:
		ed := m.MustEd25519()
		result.Ed25519 = &ed
	case CryptoKeyTypeKeyTypeMuxedEd25519:
		ed := m.MustMed25519().Ed25519
		result.Ed25519 = &ed
	default:
		panic(fmt.Errorf("Unknown muxed account type: %v", m.Type))
	}
	return result
}
