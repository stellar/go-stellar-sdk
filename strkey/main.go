package strkey

import (
	"encoding/base32"
	"encoding/binary"
	"fmt"

	"github.com/stellar/go-stellar-sdk/strkey/internal/crc16"
	"github.com/stellar/go-stellar-sdk/support/errors"
)

// ErrInvalidVersionByte is returned when the version byte from a provided
// strkey-encoded string is not one of the valid values.
var ErrInvalidVersionByte = errors.New("invalid version byte")

// VersionByte represents one of the possible prefix values for a StrKey base
// string--the string the when encoded using base32 yields a final StrKey.
type VersionByte byte

const (
	//VersionByteAccountID is the version byte used for encoded stellar addresses
	VersionByteAccountID VersionByte = 6 << 3 // Base32-encodes to 'G...'

	//VersionByteSeed is the version byte used for encoded stellar seed
	VersionByteSeed = 18 << 3 // Base32-encodes to 'S...'

	//VersionByteMuxedAccount is the version byte used for encoded stellar multiplexed addresses
	VersionByteMuxedAccount = 12 << 3 // Base32-encodes to 'M...'

	//VersionByteHashTx is the version byte used for encoded stellar hashTx
	//signer keys.
	VersionByteHashTx = 19 << 3 // Base32-encodes to 'T...'

	//VersionByteHashX is the version byte used for encoded stellar hashX
	//signer keys.
	VersionByteHashX = 23 << 3 // Base32-encodes to 'X...'

	//VersionByteSignedPayload is the version byte used for encoding "signed
	//payload" (CAP-40) signer keys.
	VersionByteSignedPayload = 15 << 3 // Base-32 encodes to 'P'

	//VersionByteContract is the version byte used for encoded stellar contracts
	VersionByteContract = 2 << 3 // Base-32 encodes to 'C'

	//VersionByteLiquidityPool is the version byte used for encoded stellar liquidity pools
	VersionByteLiquidityPool = 11 << 3

	//VersionByteClaimableBalance is the version byte used for encoded stellar claimable balances
	VersionByteClaimableBalance = 1 << 3
)

// maxPayloadSize is the maximum length of the payload for all versions. The
// largest strkey is a signed payload: 32-byte public key + 4-byte payload
// length + 64-byte payload
const maxPayloadSize = 100

// maxRawSize is the maximum length of a strkey in its raw form not encoded.
const maxRawSize = 1 + /* version byte */ maxPayloadSize + 2 /* checksum */

// maxEncodedSize is the maximum length of a strkey when base32 encoded.
const maxEncodedSize = (maxRawSize*8 + 4) / 5 // (8n+4)/5 is the EncodedLen for no padding

// encoding to use when encoding and decoding a strkey to and from strings.
var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// DecodeAny decodes the provided StrKey into a raw value, checking the checksum
// and if the version byte is one of allowed values.
func DecodeAny(src string) (VersionByte, []byte, error) {
	raw, err := decodeString(src)
	if err != nil {
		return 0, nil, err
	}

	// decode into components
	version := VersionByte(raw[0])
	vp := raw[0 : len(raw)-2]
	payload := raw[1 : len(raw)-2]
	checksum := raw[len(raw)-2:]

	// ensure version byte is allowed
	if err := checkValidVersionByte(version); err != nil {
		return 0, nil, err
	}

	// ensure checksum is valid
	if err := crc16.Validate(vp, binary.LittleEndian.Uint16(checksum)); err != nil {
		return 0, nil, err
	}

	// if we made it through the gaunlet, return the decoded value
	return version, payload, nil
}

// Decode decodes the provided StrKey into a raw value, checking the checksum
// and ensuring the expected VersionByte (the version parameter) is the value
// actually encoded into the provided src string.
func Decode(expected VersionByte, src string) ([]byte, error) {
	if err := checkValidVersionByte(expected); err != nil {
		return nil, err
	}

	raw, err := decodeString(src)
	if err != nil {
		return nil, err
	}

	// check length
	if len(raw) < 3 {
		return nil, errors.New("decoded string is too short")
	}

	// decode into components
	version := VersionByte(raw[0])
	vp := raw[0 : len(raw)-2]
	payload := raw[1 : len(raw)-2]
	checksum := raw[len(raw)-2:]

	// ensure version byte is expected
	if version != expected {
		return nil, ErrInvalidVersionByte
	}

	// ensure checksum is valid
	if err := crc16.Validate(vp, binary.LittleEndian.Uint16(checksum)); err != nil {
		return nil, err
	}

	// if we made it through the gauntlet, return the decoded value
	return payload, nil
}

// MustDecode is like Decode, but panics on error
func MustDecode(expected VersionByte, src string) []byte {
	d, err := Decode(expected, src)
	if err != nil {
		panic(err)
	}
	return d
}

// Encode encodes the provided data to a StrKey, using the provided version
// byte.
func Encode(version VersionByte, src []byte) (string, error) {
	if err := checkValidVersionByte(version); err != nil {
		return "", err
	}

	payloadSize := len(src)

	// check src does not exceed maximum payload size
	if payloadSize > maxPayloadSize {
		return "", fmt.Errorf("data exceeds maximum payload size for strkey")
	}

	// pack
	//  1 byte version
	//  src bytes
	//  2 byte crc16
	rawArr := [maxRawSize]byte{}
	rawSize := 1 + payloadSize + 2
	raw := rawArr[:rawSize]
	raw[0] = byte(version)
	copy(raw[1:], src)
	crc := crc16.Checksum(raw[:1+payloadSize])
	binary.LittleEndian.PutUint16(raw[1+payloadSize:], crc)

	// base32 encode
	encArr := [maxEncodedSize]byte{}
	encSize := encoding.EncodedLen(rawSize)
	enc := encArr[:encSize]
	encoding.Encode(enc, raw)

	return string(enc), nil
}

// MustEncode is like Encode, but panics on error
func MustEncode(version VersionByte, src []byte) string {
	e, err := Encode(version, src)
	if err != nil {
		panic(err)
	}
	return e
}

// Version extracts and returns the version byte from the provided source
// string.
func Version(src string) (VersionByte, error) {
	raw, err := decodeString(src)
	if err != nil {
		return VersionByte(0), err
	}

	return VersionByte(raw[0]), nil
}

// checkValidVersionByte returns an error if the provided value
// is not one of the defined valid version byte constants.
func checkValidVersionByte(version VersionByte) error {
	switch version {
	case VersionByteAccountID, VersionByteMuxedAccount, VersionByteSeed,
		VersionByteHashTx, VersionByteHashX, VersionByteSignedPayload,
		VersionByteContract, VersionByteLiquidityPool, VersionByteClaimableBalance:
		return nil
	default:
		return ErrInvalidVersionByte
	}
}

var decodingTable = initDecodingTable()

func initDecodingTable() [256]byte {
	var localDecodingTable [256]byte
	for i := range localDecodingTable {
		localDecodingTable[i] = 0xff
	}
	for i, ch := range []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567") {
		localDecodingTable[ch] = byte(i)
	}
	return localDecodingTable
}

// decodeString decodes a base32 string into the raw bytes, and ensures it could
// potentially be strkey encoded (i.e. it has both a version byte and a
// checksum, neither of which are explicitly checked by this func)
func decodeString(src string) ([]byte, error) {
	// operations on strings are expensive since it involves unicode parsing
	// so, we use bytes from the beginning
	srcBytes := []byte(src)
	// The minimal binary decoded length is 3 bytes (version byte and 2-byte CRC) which,
	// in unpadded base32 (since each character provides 5 bits) corresponds to ceiling(8*3/5) = 5
	if len(srcBytes) < 5 {
		return nil, errors.Errorf("strkey is %d bytes long; minimum valid length is 5", len(srcBytes))
	}
	// SEP23 enforces strkeys to be in canonical base32 representation.
	// Go's decoder doesn't help us there, so we need to do it ourselves.
	// 1. Make sure there is no full unused leftover byte at the end
	//   (i.e. there shouldn't be 5 or more leftover bits)
	leftoverBits := (len(srcBytes) * 5) % 8
	if leftoverBits >= 5 {
		return nil, errors.New("non-canonical strkey; unused leftover character")
	}
	// 2. In the last byte of the strkey there may be leftover bits (4 at most, otherwise it would be a full byte,
	//    which we have for checked above). If there are any leftover bits, they should be set to 0
	if leftoverBits > 0 {
		lastChar := srcBytes[len(srcBytes)-1]
		decodedLastChar := decodingTable[lastChar]
		if decodedLastChar == 0xff {
			// The last character from the input wasn't in the expected input alphabet.
			// Let's output an error matching the errors from the base32 decoder invocation below
			return nil, errors.Wrap(base32.CorruptInputError(len(srcBytes)), "base32 decode failed")
		}
		leftoverBitsMask := byte(0x0f) >> (4 - leftoverBits)
		if decodedLastChar&leftoverBitsMask != 0 {
			return nil, errors.New("non-canonical strkey; unused bits should be set to 0")
		}
	}
	n, err := base32.StdEncoding.WithPadding(base32.NoPadding).Decode(srcBytes, srcBytes)
	if err != nil {
		return nil, errors.Wrap(err, "base32 decode failed")
	}

	return srcBytes[:n], nil
}

// IsValidEd25519PublicKey validates a stellar public key
func IsValidEd25519PublicKey(i interface{}) bool {
	enc, ok := i.(string)

	if !ok {
		return false
	}

	_, err := Decode(VersionByteAccountID, enc)

	return err == nil
}

func IsValidContractAddress(i interface{}) bool {
	enc, ok := i.(string)
	if !ok {
		return false
	}
	_, err := Decode(VersionByteContract, enc)
	return err == nil
}

func IsValidClaimableBalance(i interface{}) bool {
	enc, ok := i.(string)
	if !ok {
		return false
	}
	_, err := Decode(VersionByteClaimableBalance, enc)
	return err == nil
}

func IsValidLiquidityPool(i interface{}) bool {
	enc, ok := i.(string)
	if !ok {
		return false
	}
	_, err := Decode(VersionByteLiquidityPool, enc)
	return err == nil
}

// IsValidMuxedAccountEd25519PublicKey validates a Stellar SEP-23 muxed address.
func IsValidMuxedAccountEd25519PublicKey(s string) bool {
	_, err := Decode(VersionByteMuxedAccount, s)
	return err == nil
}

// IsValidEd25519SecretSeed validates a stellar secret key
func IsValidEd25519SecretSeed(i interface{}) bool {
	enc, ok := i.(string)

	if !ok {
		return false
	}

	_, err := Decode(VersionByteSeed, enc)

	return err == nil
}
