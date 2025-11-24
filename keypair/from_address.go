package keypair

import (
	"crypto/ed25519"
	"encoding"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// FromAddress represents a keypair to which only the address is know.  This KP
// can verify signatures, but cannot sign them.
//
// NOTE: ensure the address provided is a valid strkey encoded stellar address.
// Some operations will panic otherwise. It's recommended that you create these
// structs through the Parse() method.
type FromAddress struct {
	address   string
	publicKey ed25519.PublicKey
}

func newFromAddress(address string) (*FromAddress, error) {
	payload, err := strkey.Decode(strkey.VersionByteAccountID, address)
	if err != nil {
		return nil, err
	}
	pub := ed25519.PublicKey(payload)
	return &FromAddress{
		address:   address,
		publicKey: pub,
	}, nil
}

func newFromAddressWithPublicKey(address string, publicKey ed25519.PublicKey) *FromAddress {
	return &FromAddress{
		address:   address,
		publicKey: publicKey,
	}
}

func (kp *FromAddress) Address() string {
	return kp.address
}

// FromAddress gets the address-only representation, or public key, of this
// keypair, which is itself.
func (kp *FromAddress) FromAddress() *FromAddress {
	return kp
}

func (kp *FromAddress) Hint() (r [4]byte) {
	copy(r[:], kp.publicKey[28:])
	return
}

func (kp *FromAddress) Verify(input []byte, sig []byte) error {
	if len(sig) != 64 {
		return ErrInvalidSignature
	}
	if !ed25519.Verify(kp.publicKey, input, sig) {
		return ErrInvalidSignature
	}
	return nil
}

func (kp *FromAddress) Sign(input []byte) ([]byte, error) {
	return nil, ErrCannotSign
}

func (kp *FromAddress) SignBase64(input []byte) (string, error) {
	return "", ErrCannotSign
}

func (kp *FromAddress) SignDecorated(input []byte) (xdr.DecoratedSignature, error) {
	return xdr.DecoratedSignature{}, ErrCannotSign
}

func (kp *FromAddress) SignPayloadDecorated(input []byte) (xdr.DecoratedSignature, error) {
	return xdr.DecoratedSignature{}, ErrCannotSign
}

func (kp *FromAddress) Equal(a *FromAddress) bool {
	if kp == nil && a == nil {
		return true
	}
	if kp == nil || a == nil {
		return false
	}
	return kp.address == a.address
}

var (
	_ = encoding.TextMarshaler(&FromAddress{})
	_ = encoding.TextUnmarshaler(&FromAddress{})
)

func (kp *FromAddress) UnmarshalText(text []byte) error {
	textKP, err := ParseAddress(string(text))
	if err != nil {
		return err
	}
	*kp = *textKP
	return nil
}

func (kp *FromAddress) MarshalText() ([]byte, error) {
	return []byte(kp.address), nil
}

var (
	_ = encoding.BinaryMarshaler(&FromAddress{})
	_ = encoding.BinaryUnmarshaler(&FromAddress{})
)

func (kp *FromAddress) UnmarshalBinary(b []byte) error {
	accountID := xdr.AccountId{}
	err := xdr.SafeUnmarshal(b, &accountID)
	if err != nil {
		return err
	}
	address := accountID.Address()
	binKP, err := ParseAddress(address)
	if err != nil {
		return err
	}
	*kp = *binKP
	return nil
}

func (kp *FromAddress) MarshalBinary() ([]byte, error) {
	accountID, err := xdr.AddressToAccountId(kp.address)
	if err != nil {
		return nil, err
	}
	return accountID.MarshalBinary()
}
