// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package xdr

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"

	"github.com/klauspost/compress/zstd"
)

const DefaultMaxXDRStreamRecordSize = 64 * 1024 * 1024 // 64 MB

var ErrRecordTooLarge = errors.New("xdr record too large")

type Stream struct {
	buf              bytes.Buffer
	compressedReader *countReader
	reader           *countReader
	sha256Hash       hash.Hash
	maxRecordSize    uint32
	xdrDecoder       *BytesDecoder

	// boundaryOffset and boundaryState are BytesRead() and the hash state at
	// the start of the last ReadOne, which is the last record boundary.
	// ResumeFrom uses them to continue a failed stream from that record.
	boundaryOffset int64
	boundaryState  []byte

	// closed and closeErr make Close idempotent. A bool rather than a
	// sync.Once because callers copy a Stream by value to replace a failed
	// one, and go vet rejects copying a sync.Once.
	closed   bool
	closeErr error
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
	// used with ValidateHash to verify stream integrity.
	// The tee sits above the bufio.Reader so the hash covers exactly the
	// bytes delivered to the decoder (BytesRead()), not bufio's read-ahead.
	sha256Hash := sha256.New()
	teeReader := io.TeeReader(bufio.NewReader(in), sha256Hash)
	return &Stream{
		reader: newCountReader(
			struct {
				io.Reader
				io.Closer
			}{teeReader, in},
		),
		sha256Hash:    sha256Hash,
		maxRecordSize: DefaultMaxXDRStreamRecordSize,
		xdrDecoder:    NewBytesDecoder(),
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

// SetMaxRecordSize sets the maximum allowed size for a single XDR record.
//
// This method may be called before reading begins or between calls to
// ReadOne. The new limit only applies to records read after the call; it
// does not retroactively affect or re-validate records that have already
// been read from the stream.
//
// If size is 0, the default (DefaultMaxXDRStreamRecordSize) is used.
// This method is not safe for concurrent use with other operations on the
// same Stream; if a Stream is accessed from multiple goroutines, configure
// the maximum record size once before starting to read.
func (x *Stream) SetMaxRecordSize(size uint32) {
	if size == 0 {
		x.maxRecordSize = DefaultMaxXDRStreamRecordSize
	} else {
		x.maxRecordSize = size
	}
}

// ValidateHash drains any remaining bytes from the stream, then checks that the
// stream's SHA-256 hash matches the given expected hash.
//
// It must be called after reading all records, and before Close(), to ensure that the
// hash covers the complete stream. If called after Close(), the underlying reader will
// already be closed and the internal io.Copy used to drain remaining bytes will fail
// with an error from the closed reader rather than successfully validating the hash.
func (x *Stream) ValidateHash(expected [sha256.Size]byte) error {
	// Drain remaining bytes so the hash covers the entire stream.
	// After a full read (ReadOne returned EOF), this is near-zero bytes.
	if _, err := io.Copy(io.Discard, x.reader); err != nil {
		return fmt.Errorf("reading remaining stream bytes for hash validation: %w", err)
	}
	actualHash := x.sha256Hash.Sum(nil)
	if !bytes.Equal(expected[:], actualHash) {
		return fmt.Errorf("stream hash mismatch: expected %x, got %x", expected, actualHash)
	}
	return nil
}

// Close closes all internal readers and releases resources. Only the first
// call closes anything. Later calls return the first call's error.
func (x *Stream) Close() error {
	if !x.closed {
		x.closed = true
		x.closeErr = x.closeReaders()
	}
	return x.closeErr
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
	x.boundaryOffset = x.reader.bytesRead
	x.boundaryState, _ = x.sha256Hash.(encoding.BinaryAppender).AppendBinary(x.boundaryState[:0])
	nbytes, err := ReadFrameLength(x.reader)
	if err != nil {
		x.Close()
		if errors.Is(err, io.EOF) {
			// Do not wrap io.EOF
			return io.EOF
		}
		return fmt.Errorf("reading frame length: %w", err)
	}
	x.buf.Reset()
	if nbytes == 0 {
		x.Close()
		return io.EOF
	}
	if nbytes > x.maxRecordSize {
		x.Close()
		return fmt.Errorf("%w: %d bytes (max %d)", ErrRecordTooLarge, nbytes, x.maxRecordSize)
	}
	x.buf.Grow(int(nbytes))
	read, err := x.buf.ReadFrom(io.LimitReader(x.reader, int64(nbytes)))
	if err != nil {
		x.Close()
		return err
	}
	if read != int64(nbytes) {
		x.Close()
		return errors.New("Read wrong number of bytes from XDR")
	}

	readi, err := x.xdrDecoder.DecodeBytes(in, x.buf.Bytes())
	if err != nil {
		x.Close()
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

// ResumeFrom positions x, a new stream over the same content as prev, at the
// start of the record on which prev failed, and continues prev's hash from
// there. ValidateHash on x then covers every byte that prev delivered to the
// decoder followed by every byte x delivers.
//
// The skipped bytes pass through x's hash first and are then replaced by
// prev's hash state, so the final digest is the hash of prev's bytes before
// the boundary followed by x's bytes after it.
func (x *Stream) ResumeFrom(prev *Stream) error {
	if _, err := x.Discard(prev.boundaryOffset); err != nil {
		return err
	}
	if prev.boundaryState == nil {
		// prev never ran ReadOne, so it delivered nothing. x's fresh hash is
		// already the right state.
		return nil
	}
	return x.sha256Hash.(encoding.BinaryUnmarshaler).UnmarshalBinary(prev.boundaryState)
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
