// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package xdr

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"io/ioutil"

	"github.com/klauspost/compress/zstd"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

type Stream struct {
	buf              bytes.Buffer
	compressedReader *countReader
	reader           *countReader
	sha256Hash       hash.Hash

	validateHash bool
	expectedHash [sha256.Size]byte
	xdrDecoder   *BytesDecoder
}

type countReader struct {
	io.ReadCloser
	bytesRead int64
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	c.bytesRead += int64(n)
	return n, err
}

func newCountReader(r io.ReadCloser) *countReader {
	return &countReader{
		r, 0,
	}
}

func NewStream(in io.ReadCloser) *Stream {
	// We write all we read from in to sha256Hash that can be later
	// compared with `expectedHash` using SetExpectedHash and Close.
	sha256Hash := sha256.New()
	teeReader := io.TeeReader(in, sha256Hash)
	return &Stream{
		reader: newCountReader(
			struct {
				io.Reader
				io.Closer
			}{bufio.NewReader(teeReader), in},
		),
		sha256Hash: sha256Hash,
		xdrDecoder: NewBytesDecoder(),
	}
}

func newCompressedXdrStream(in io.ReadCloser, decompressor func(r io.Reader) (io.ReadCloser, error)) (*Stream, error) {
	gzipCountReader := newCountReader(in)
	rdr, err := decompressor(bufio.NewReader(gzipCountReader))
	if err != nil {
		in.Close()
		return nil, err
	}

	stream := NewStream(rdr)
	stream.compressedReader = gzipCountReader
	return stream, nil
}

func NewGzStream(in io.ReadCloser) (*Stream, error) {
	return newCompressedXdrStream(in, func(r io.Reader) (io.ReadCloser, error) {
		return gzip.NewReader(r)
	})
}

type zstdReader struct {
	*zstd.Decoder
}

func (z zstdReader) Close() error {
	z.Decoder.Close()
	return nil
}

func NewZstdStream(in io.ReadCloser) (*Stream, error) {
	return newCompressedXdrStream(in, func(r io.Reader) (io.ReadCloser, error) {
		decoder, err := zstd.NewReader(r)
		return zstdReader{decoder}, err
	})
}

func HashXdr(x interface{}) (Hash, error) {
	var msg bytes.Buffer
	_, err := Marshal(&msg, x)
	if err != nil {
		var zero Hash
		return zero, err
	}
	return sha256.Sum256(msg.Bytes()), nil
}

// SetExpectedHash sets expected hash that will be checked in Close().
// This (obviously) needs to be set before Close() is called.
func (x *Stream) SetExpectedHash(hash [sha256.Size]byte) {
	x.validateHash = true
	x.expectedHash = hash
}

// ExpectedHash returns the expected hash and a boolean indicating if the
// expected hash was set
func (x *Stream) ExpectedHash() ([sha256.Size]byte, bool) {
	return x.expectedHash, x.validateHash
}

// Close closes all internal readers and checks if the expected hash
// (if set by SetExpectedHash) matches the actual hash of the stream.
func (x *Stream) Close() error {
	if x.validateHash {
		// Read all remaining data from reader
		_, err := io.Copy(io.Discard, x.reader)
		if err != nil {
			// close the internal readers to avoid memory leaks
			x.closeReaders()
			return errors.Wrap(err, "Error reading remaining bytes from reader")
		}

		actualHash := x.sha256Hash.Sum([]byte{})

		if !bytes.Equal(x.expectedHash[:], actualHash[:]) {
			// close the internal readers to avoid memory leaks
			x.closeReaders()
			return errors.New("Stream hash does not match expected hash!")
		}
	}

	return x.closeReaders()
}

func (x *Stream) closeReaders() error {
	var err error

	if x.reader != nil {
		if err2 := x.reader.Close(); err2 != nil {
			err = err2
		}
	}

	if x.compressedReader != nil {
		if err2 := x.compressedReader.Close(); err2 != nil {
			err = err2
		}
	}

	return err
}

func (x *Stream) ReadOne(in DecoderFrom) error {
	var nbytes uint32
	err := binary.Read(x.reader, binary.BigEndian, &nbytes)
	if err != nil {
		x.reader.Close()
		if err == io.EOF {
			// Do not wrap io.EOF
			return err
		}
		return errors.Wrap(err, "binary.Read error")
	}
	nbytes &= 0x7fffffff
	x.buf.Reset()
	if nbytes == 0 {
		x.reader.Close()
		return io.EOF
	}
	x.buf.Grow(int(nbytes))
	read, err := x.buf.ReadFrom(io.LimitReader(x.reader, int64(nbytes)))
	if err != nil {
		x.reader.Close()
		return err
	}
	if read != int64(nbytes) {
		x.reader.Close()
		return errors.New("Read wrong number of bytes from XDR")
	}

	readi, err := x.xdrDecoder.DecodeBytes(in, x.buf.Bytes())
	if err != nil {
		x.reader.Close()
		return err
	}
	if int64(readi) != int64(nbytes) {
		return fmt.Errorf("Unmarshalled %d bytes from XDR, expected %d)",
			readi, nbytes)
	}
	return nil
}

// BytesRead returns the number of bytes read in the stream
func (x *Stream) BytesRead() int64 {
	return x.reader.bytesRead
}

// CompressedBytesRead returns the number of compressed bytes read in the stream.
// Returns -1 if underlying reader is not compressed.
func (x *Stream) CompressedBytesRead() int64 {
	if x.compressedReader == nil {
		return -1
	}
	return x.compressedReader.bytesRead
}

// Discard removes n bytes from the stream
func (x *Stream) Discard(n int64) (int64, error) {
	return io.CopyN(ioutil.Discard, x.reader, n)
}

func CreateXdrStream(entries ...BucketEntry) *Stream {
	b := &bytes.Buffer{}
	for _, e := range entries {
		err := MarshalFramed(b, e)
		if err != nil {
			panic(err)
		}
	}

	return NewStream(ioutil.NopCloser(b))
}
