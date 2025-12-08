// Package xdr contains the generated code for parsing the xdr structures used
// for stellar.
package xdr

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	xdr "github.com/stellar/go-xdr/xdr3"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// CommitHash is the commit hash that was used to generate the xdr in this folder.
// During the process of updating the XDR, the text file below is being updated.
// Then, during compile time, the file content are being embedded into the given string.
//
//go:embed xdr_commit_generated.txt
var CommitHash string

// Keyer represents a type that can be converted into a LedgerKey
type Keyer interface {
	LedgerKey() (LedgerKey, error)
}

var _ = LedgerEntry{}
var _ = LedgerKey{}

var OperationTypeToStringMap = operationTypeMap

var LedgerEntryTypeMap = ledgerEntryTypeMap

func safeUnmarshalString(decoder func(reader io.Reader) io.Reader, options xdr.DecodeOptions, data string, dest interface{}) error {
	count := &countWriter{}
	l := len(data)

	_, err := UnmarshalWithOptions(decoder(io.TeeReader(strings.NewReader(data), count)), dest, options)
	if err != nil {
		return err
	}

	if count.Count != l {
		return fmt.Errorf("input not fully consumed. expected to read: %d, actual: %d", l, count.Count)
	}

	return nil
}

func decodeOptionsWithMaxInputLen(maxInputLen int) xdr.DecodeOptions {
	options := xdr.DefaultDecodeOptions
	options.MaxInputLen = maxInputLen
	return options
}

// SafeUnmarshalBase64 first decodes the provided reader from base64 before
// decoding the xdr into the provided destination. Also ensures that the reader
// is fully consumed.
func SafeUnmarshalBase64(data string, dest interface{}) error {
	decodedLen := base64.StdEncoding.DecodedLen(len(data))
	options := decodeOptionsWithMaxInputLen(decodedLen)
	return safeUnmarshalString(
		func(r io.Reader) io.Reader {
			return base64.NewDecoder(base64.StdEncoding, r)
		},
		options,
		data,
		dest,
	)
}

// SafeUnmarshalHex first decodes the provided reader from hex before
// decoding the xdr into the provided destination. Also ensures that the reader
// is fully consumed.
func SafeUnmarshalHex(data string, dest interface{}) error {
	decodedLen := hex.DecodedLen(len(data))
	options := decodeOptionsWithMaxInputLen(decodedLen)
	return safeUnmarshalString(hex.NewDecoder, options, data, dest)
}

// SafeUnmarshal decodes the provided reader into the destination and verifies
// that provided bytes are all consumed by the unmarshaling process.
func SafeUnmarshal(data []byte, dest interface{}) error {
	r := bytes.NewReader(data)
	n, err := Unmarshal(r, dest)

	if err != nil {
		return err
	}

	if n != len(data) {
		return fmt.Errorf("input not fully consumed. expected to read: %d, actual: %d", len(data), n)
	}

	return nil
}

type DecoderFrom interface {
	decoderFrom
}

// BytesDecoder efficiently manages a byte reader and an
// xdr decoder so that they don't need to be allocated in
// every decoding call.
type BytesDecoder struct {
	decoder *xdr.Decoder
	reader  *bytes.Reader
}

func NewBytesDecoder() *BytesDecoder {
	reader := bytes.NewReader(nil)
	decoder := xdr.NewDecoder(reader)
	return &BytesDecoder{
		decoder: decoder,
		reader:  reader,
	}
}

func (d *BytesDecoder) DecodeBytes(v DecoderFrom, b []byte) (int, error) {
	d.reader.Reset(b)
	return v.DecodeFrom(d.decoder, xdr.DecodeDefaultMaxDepth)
}

func marshalString(encoder func([]byte) string, v interface{}) (string, error) {
	var raw bytes.Buffer

	_, err := Marshal(&raw, v)

	if err != nil {
		return "", err
	}

	return encoder(raw.Bytes()), nil
}

func MarshalBase64(v interface{}) (string, error) {
	return marshalString(base64.StdEncoding.EncodeToString, v)
}

func MarshalHex(v interface{}) (string, error) {
	return marshalString(hex.EncodeToString, v)
}

// EncodingBuffer reuses internal buffers between invocations to minimize allocations.
// For that reason, it is not thread-safe.
// It intentionally only allows EncodeTo method arguments, to guarantee high performance encoding.
type EncodingBuffer struct {
	encoder       *xdr.Encoder
	xdrEncoderBuf bytes.Buffer
	scratchBuf    []byte
}

func growSlice(old []byte, newSize int) []byte {
	oldCap := cap(old)
	if newSize <= oldCap {
		return old[:newSize]
	}
	// the array doesn't fit, lets return a new one with double the capacity
	// to avoid further resizing
	return make([]byte, newSize, 2*newSize)
}

type EncoderTo interface {
	EncodeTo(e *xdr.Encoder) error
}

func NewEncodingBuffer() *EncodingBuffer {
	var ret EncodingBuffer
	ret.encoder = xdr.NewEncoder(&ret.xdrEncoderBuf)
	return &ret
}

// UnsafeMarshalBinary marshals the input XDR binary, returning
// a slice pointing to the internal buffer. Handled with care this improveds
// performance since copying is not required.
// Subsequent calls to marshaling methods will overwrite the returned buffer.
func (e *EncodingBuffer) UnsafeMarshalBinary(encodable EncoderTo) ([]byte, error) {
	e.xdrEncoderBuf.Reset()
	if err := encodable.EncodeTo(e.encoder); err != nil {
		return nil, err
	}
	return e.xdrEncoderBuf.Bytes(), nil
}

// UnsafeMarshalBase64 is the base64 version of UnsafeMarshalBinary
func (e *EncodingBuffer) UnsafeMarshalBase64(encodable EncoderTo) ([]byte, error) {
	xdrEncoded, err := e.UnsafeMarshalBinary(encodable)
	if err != nil {
		return nil, err
	}
	neededLen := base64.StdEncoding.EncodedLen(len(xdrEncoded))
	e.scratchBuf = growSlice(e.scratchBuf, neededLen)
	base64.StdEncoding.Encode(e.scratchBuf, xdrEncoded)
	return e.scratchBuf, nil
}

// UnsafeMarshalHex is the hex version of UnsafeMarshalBinary
func (e *EncodingBuffer) UnsafeMarshalHex(encodable EncoderTo) ([]byte, error) {
	xdrEncoded, err := e.UnsafeMarshalBinary(encodable)
	if err != nil {
		return nil, err
	}
	neededLen := hex.EncodedLen(len(xdrEncoded))
	e.scratchBuf = growSlice(e.scratchBuf, neededLen)
	hex.Encode(e.scratchBuf, xdrEncoded)
	return e.scratchBuf, nil
}

func (e *EncodingBuffer) MarshalBinary(encodable EncoderTo) ([]byte, error) {
	xdrEncoded, err := e.UnsafeMarshalBinary(encodable)
	if err != nil {
		return nil, err
	}
	ret := make([]byte, len(xdrEncoded))
	copy(ret, xdrEncoded)
	return ret, nil
}

// LedgerKeyUnsafeMarshalBinaryCompress marshals LedgerKey to []byte but unlike
// MarshalBinary() it removes all unnecessary bytes, exploting the fact
// that XDR is padding data to 4 bytes in union discriminants etc.
// It's primary use is in ingest/io.StateReader that keep LedgerKeys in
// memory so this function decrease memory requirements.
//
// Warning, do not use UnmarshalBinary() on data encoded using this method!
//
// Optimizations:
// - Writes a single byte for union discriminants vs 4 bytes.
// - Removes type and code padding for Asset.
// - Removes padding for AccountIds
func (e *EncodingBuffer) LedgerKeyUnsafeMarshalBinaryCompress(key LedgerKey) ([]byte, error) {
	e.xdrEncoderBuf.Reset()
	err := e.ledgerKeyCompressEncodeTo(key)
	if err != nil {
		return nil, err
	}
	return e.xdrEncoderBuf.Bytes(), nil
}

// GetBinaryCompressedLedgerKeyType gets the key type from the result of LedgerKeyUnsafeMarshalBinaryCompress
func GetBinaryCompressedLedgerKeyType(compressedKey []byte) (LedgerEntryType, error) {
	if len(compressedKey) < 1 {
		return 0, errors.New("empty compressed ledger key")
	}
	result := LedgerEntryType(compressedKey[0])
	if int(result) > len(ledgerEntryTypeMap)-1 {
		return 0, fmt.Errorf("incorrect key type %d", result)
	}
	return result, nil
}

func (e *EncodingBuffer) MarshalBase64(encodable EncoderTo) (string, error) {
	b, err := e.UnsafeMarshalBase64(encodable)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (e *EncodingBuffer) MarshalHex(encodable EncoderTo) (string, error) {
	b, err := e.UnsafeMarshalHex(encodable)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func MarshalFramed(w io.Writer, v interface{}) error {
	var tmp bytes.Buffer
	n, err := Marshal(&tmp, v)
	if err != nil {
		return err
	}
	un := uint32(n)
	if un > 0x7fffffff {
		return fmt.Errorf("Overlong write: %d bytes", n)
	}

	un = un | 0x80000000
	err = binary.Write(w, binary.BigEndian, &un)
	if err != nil {
		return errors.Wrap(err, "error in binary.Write")
	}
	k, err := tmp.WriteTo(w)
	if int64(n) != k {
		return fmt.Errorf("Mismatched write length: %d vs. %d", n, k)
	}
	return err
}

// ReadFrameLength returns a length of a framed XDR object.
func ReadFrameLength(d *xdr.Decoder) (uint32, error) {
	frameLen, n, e := d.DecodeUint()
	if e != nil {
		return 0, errors.Wrap(e, "unmarshaling XDR frame header")
	}
	if n != 4 {
		return 0, errors.New("bad length of XDR frame header")
	}
	if (frameLen & 0x80000000) != 0x80000000 {
		return 0, errors.New("malformed XDR frame header")
	}
	frameLen &= 0x7fffffff
	return frameLen, nil
}

type countWriter struct {
	Count int
}

func (w *countWriter) Write(d []byte) (int, error) {
	l := len(d)
	w.Count += l
	return l, nil
}
