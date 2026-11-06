package strkey

import (
	"bytes"
	"encoding/binary"

	"github.com/stellar/go-stellar-sdk/support/errors"
	xdr "github.com/stellar/go-xdr/xdr3"
)

type SignedPayload struct {
	signer  string
	payload []byte
}

const maxPayloadLen = 64

// NewSignedPayload creates a signed payload from an account ID (G... address)
// and a payload. The payload buffer is copied directly into the structure, so
// it should not be modified after construction.
func NewSignedPayload(signerPublicKey string, payload []byte) (*SignedPayload, error) {
	// CAP-40 rejects empty payloads (SET_OPTIONS_BAD_SIGNER / txMALFORMED), and
	// Decode rejects their strkeys, so refuse to construct one here as well.
	if len(payload) == 0 {
		return nil, errors.New("payload must not be empty")
	}
	if len(payload) > maxPayloadLen {
		return nil, errors.Errorf("payload length %d exceeds max %d",
			len(payload), maxPayloadLen)
	}

	return &SignedPayload{signer: signerPublicKey, payload: payload}, nil
}

// Encode turns a signed payload structure into its StrKey equivalent.
func (sp *SignedPayload) Encode() (string, error) {
	signerBytes, err := Decode(VersionByteAccountID, sp.Signer())
	if err != nil {
		return "", errors.Wrap(err, "failed to decode signed payload signer")
	}

	b := new(bytes.Buffer)
	b.Write(signerBytes)
	xdr.Marshal(b, sp.Payload())

	strkey, err := Encode(VersionByteSignedPayload, b.Bytes())
	if err != nil {
		return "", errors.Wrap(err, "failed to encode signed payload")
	}
	return strkey, nil
}

func (sp *SignedPayload) Signer() string {
	return sp.signer
}

func (sp *SignedPayload) Payload() []byte {
	return sp.payload
}

// DecodeSignedPayload transforms a P... signer into a `SignedPayload` instance.
func DecodeSignedPayload(address string) (*SignedPayload, error) {
	// Decode validates the structure: a 32-byte signer key, a 4-byte declared
	// length of 1-64, and the payload zero-padded to a multiple of four bytes.
	// So the slicing below is safe without further checks.
	raw, err := Decode(VersionByteSignedPayload, address)
	if err != nil {
		return nil, errors.New("invalid signed payload")
	}

	signer, err := Encode(VersionByteAccountID, raw[:32])
	if err != nil {
		return nil, errors.Wrap(err, "invalid signed payload signer")
	}

	declared := binary.BigEndian.Uint32(raw[32:36])
	return NewSignedPayload(signer, raw[36:36+declared])
}
