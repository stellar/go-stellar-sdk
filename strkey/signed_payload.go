package strkey

import (
	"bytes"

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
	raw, err := Decode(VersionByteSignedPayload, address)
	if err != nil {
		return nil, errors.New("invalid signed payload")
	}

	const signerLen = 32
	rawSigner, raw := raw[:signerLen], raw[signerLen:]
	signer, err := Encode(VersionByteAccountID, rawSigner)
	if err != nil {
		return nil, errors.Wrap(err, "invalid signed payload signer")
	}

	payload := []byte{}
	reader := bytes.NewBuffer(raw)
	readBytes, err := xdr.Unmarshal(reader, &payload)
	if err != nil {
		return nil, errors.Wrap(err, "invalid signed payload")
	}

	if len(raw) != readBytes || reader.Len() > 0 {
		return nil, errors.New("invalid signed payload padding")
	}

	return NewSignedPayload(signer, payload)
}
