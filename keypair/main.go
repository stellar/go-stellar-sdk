package keypair

import (
	"crypto/rand"
	"errors"
	"io"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var (
	// ErrInvalidKey will be returned by operations when the keypair being used
	// could not be decoded.
	ErrInvalidKey = errors.New("invalid key")

	// ErrInvalidSignature is returned when the signature is invalid, either
	// through malformation or if it does not verify the message against the
	// provided public key
	ErrInvalidSignature = errors.New("signature verification failed")

	// ErrCannotSign is returned when attempting to sign a message when
	// the keypair does not have the secret key available
	ErrCannotSign = errors.New("cannot sign")
)

const (
	// DefaultSignerWeight represents the starting weight of the default signer
	// for an account.
	DefaultSignerWeight = 1
)

// KP is the main interface for this package
type KP interface {
	Address() string
	FromAddress() *FromAddress
	Hint() [4]byte
	Verify(input []byte, signature []byte) error
	Sign(input []byte) ([]byte, error)
	SignBase64(input []byte) (string, error)
	SignDecorated(input []byte) (xdr.DecoratedSignature, error)
	SignPayloadDecorated(input []byte) (xdr.DecoratedSignature, error)
}

// Random creates a random full keypair
func Random() (*Full, error) {
	var rawSeed [32]byte

	_, err := io.ReadFull(rand.Reader, rawSeed[:])
	if err != nil {
		return nil, err
	}

	kp, err := FromRawSeed(rawSeed)
	if err != nil {
		return nil, err
	}

	return kp, nil
}

// Master returns the master keypair for a given network passphrase
// Deprecated: Use keypair.Root instead.
func Master(networkPassphrase string) KP {
	return Root(networkPassphrase)
}

// Root returns the root account keypair for a given network passphrase.
func Root(networkPassphrase string) *Full {
	kp, err := FromRawSeed(network.ID(networkPassphrase))
	if err != nil {
		panic(err)
	}

	return kp
}

// Parse constructs a new KP from the provided string, which should be either
// an address, or a seed.  If the provided input is a seed, the resulting KP
// will have signing capabilities.
func Parse(addressOrSeed string) (KP, error) {
	addr, err := ParseAddress(addressOrSeed)
	if err == nil {
		return addr, nil
	}

	if err != strkey.ErrInvalidVersionByte {
		return nil, err
	}

	return ParseFull(addressOrSeed)
}

// ParseAddress constructs a new FromAddress keypair from the provided string,
// which should be an address.
func ParseAddress(address string) (*FromAddress, error) {
	return newFromAddress(address)
}

// ParseFull constructs a new Full keypair from the provided string, which should
// be a seed.
func ParseFull(seed string) (*Full, error) {
	return newFull(seed)
}

// FromRawSeed creates a new keypair from the provided raw ED25519 seed
func FromRawSeed(rawSeed [32]byte) (*Full, error) {
	return newFullFromRawSeed(rawSeed)
}

// MustParse is the panic-on-fail version of Parse
func MustParse(addressOrSeed string) KP {
	kp, err := Parse(addressOrSeed)
	if err != nil {
		panic(err)
	}

	return kp
}

// MustParseAddress is the panic-on-fail version of ParseAddress
func MustParseAddress(address string) *FromAddress {
	kp, err := ParseAddress(address)
	if err != nil {
		panic(err)
	}

	return kp
}

// MustParseFull is the panic-on-fail version of ParseFull
func MustParseFull(seed string) *Full {
	kp, err := ParseFull(seed)
	if err != nil {
		panic(err)
	}

	return kp
}

// MustRandom is the panic-on-fail version of Random.
func MustRandom() *Full {
	kp, err := Random()
	if err != nil {
		panic(err)
	}

	return kp
}
