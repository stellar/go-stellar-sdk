// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package xdr

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXdrStreamHash(t *testing.T) {
	bucketEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:   Int64(200000000),
				},
			},
		},
	}
	stream := CreateXdrStream(bucketEntry)

	// Stream hash should be equal sha256 hash of concatenation of:
	// - uint32 representing the number of bytes of a structure,
	// - xdr-encoded `BucketEntry` above.
	b := &bytes.Buffer{}
	err := MarshalFramed(b, bucketEntry)
	require.NoError(t, err)

	expectedHash := sha256.Sum256(b.Bytes())

	var readBucketEntry BucketEntry
	err = stream.ReadOne(&readBucketEntry)
	require.NoError(t, err)
	assert.Equal(t, bucketEntry, readBucketEntry)

	assert.Equal(t, int(stream.BytesRead()), b.Len())

	assert.Equal(t, io.EOF, stream.ReadOne(&readBucketEntry))
	assert.Equal(t, int(stream.BytesRead()), b.Len())

	assert.NoError(t, stream.ValidateHash(expectedHash))
	assert.NoError(t, stream.Close())
}

func TestXdrStreamDiscard(t *testing.T) {
	firstEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:   Int64(200000000),
				},
			},
		},
	}
	secondEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC23QF2HUE52AMXUFUH3AYJAXXGXXV2VHXYYR6EYXETPKDXZSAW67XO4"),
					Balance:   Int64(100000000),
				},
			},
		},
	}

	fullStream := CreateXdrStream(firstEntry, secondEntry)
	b := &bytes.Buffer{}
	require.NoError(t, MarshalFramed(b, firstEntry))
	require.NoError(t, MarshalFramed(b, secondEntry))
	expectedHash := sha256.Sum256(b.Bytes())

	discardStream := CreateXdrStream(firstEntry, secondEntry)

	var readBucketEntry BucketEntry
	require.NoError(t, fullStream.ReadOne(&readBucketEntry))
	assert.Equal(t, firstEntry, readBucketEntry)

	skipAmount := fullStream.BytesRead()
	bytesRead, err := discardStream.Discard(skipAmount)
	require.NoError(t, err)
	assert.Equal(t, bytesRead, skipAmount)

	require.NoError(t, fullStream.ReadOne(&readBucketEntry))
	assert.Equal(t, secondEntry, readBucketEntry)

	require.NoError(t, discardStream.ReadOne(&readBucketEntry))
	assert.Equal(t, secondEntry, readBucketEntry)

	assert.Equal(t, int(fullStream.BytesRead()), b.Len())
	assert.Equal(t, fullStream.BytesRead(), discardStream.BytesRead())

	assert.Equal(t, io.EOF, fullStream.ReadOne(&readBucketEntry))
	assert.Equal(t, io.EOF, discardStream.ReadOne(&readBucketEntry))

	assert.Equal(t, int(fullStream.BytesRead()), b.Len())
	assert.Equal(t, fullStream.BytesRead(), discardStream.BytesRead())

	assert.NoError(t, discardStream.ValidateHash(expectedHash))
	assert.NoError(t, discardStream.Close())
	assert.NoError(t, fullStream.ValidateHash(expectedHash))
	assert.NoError(t, fullStream.Close())
}

func TestValidateHashMismatch(t *testing.T) {
	bucketEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:   Int64(200000000),
				},
			},
		},
	}
	stream := CreateXdrStream(bucketEntry)

	var readBucketEntry BucketEntry
	require.NoError(t, stream.ReadOne(&readBucketEntry))
	assert.Equal(t, io.EOF, stream.ReadOne(&readBucketEntry))

	var wrongHash [32]byte // zero hash, won't match
	err := stream.ValidateHash(wrongHash)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream hash mismatch")
}

// closedReader is a ReadCloser that returns an error on Read after Close.
type closedReader struct {
	*bytes.Reader
	closed bool
}

func (c *closedReader) Read(p []byte) (int, error) {
	if c.closed {
		return 0, errors.New("read after close")
	}
	return c.Reader.Read(p)
}

func (c *closedReader) Close() error {
	c.closed = true
	return nil
}

func TestValidateHashAfterClose(t *testing.T) {
	bucketEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:   Int64(200000000),
				},
			},
		},
	}

	b := &bytes.Buffer{}
	require.NoError(t, MarshalFramed(b, bucketEntry))
	expectedHash := sha256.Sum256(b.Bytes())

	stream := NewStream(&closedReader{Reader: bytes.NewReader(b.Bytes())})
	var readBucketEntry BucketEntry
	require.NoError(t, stream.ReadOne(&readBucketEntry))

	// Close first, then try ValidateHash — should fail because reader is closed.
	require.NoError(t, stream.Close())
	err := stream.ValidateHash(expectedHash)
	require.Error(t, err)
}

func TestValidateHashDrainsUnreadBytes(t *testing.T) {
	firstEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:   Int64(200000000),
				},
			},
		},
	}
	secondEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC23QF2HUE52AMXUFUH3AYJAXXGXXV2VHXYYR6EYXETPKDXZSAW67XO4"),
					Balance:   Int64(100000000),
				},
			},
		},
	}

	// Compute expected hash over the full stream (both entries).
	b := &bytes.Buffer{}
	require.NoError(t, MarshalFramed(b, firstEntry))
	require.NoError(t, MarshalFramed(b, secondEntry))
	expectedHash := sha256.Sum256(b.Bytes())

	// Only read the first entry, leaving the second unread.
	stream := CreateXdrStream(firstEntry, secondEntry)
	var readBucketEntry BucketEntry
	require.NoError(t, stream.ReadOne(&readBucketEntry))
	assert.Equal(t, firstEntry, readBucketEntry)

	// ValidateHash should drain the remaining bytes and still match.
	assert.NoError(t, stream.ValidateHash(expectedHash))
	assert.NoError(t, stream.Close())
}

func TestReadOneRecordTooLarge(t *testing.T) {
	// Write a 4-byte header claiming 256 MB, with no actual payload.
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], 256*1024*1024|xdrFrameLastFragment)
	stream := NewStream(io.NopCloser(bytes.NewReader(header[:])))

	var entry BucketEntry
	err := stream.ReadOne(&entry)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRecordTooLarge))
}

func TestReadOneCustomMaxRecordSize(t *testing.T) {
	// Write a 4-byte header claiming 2048 bytes.
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], 2048|xdrFrameLastFragment)
	stream := NewStream(io.NopCloser(bytes.NewReader(header[:])))
	stream.SetMaxRecordSize(1024)

	var entry BucketEntry
	err := stream.ReadOne(&entry)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRecordTooLarge))
}

func TestReadOneMaxBoundary(t *testing.T) {
	// Header at exactly DefaultMaxXDRStreamRecordSize + 1 -> rejected.
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], (DefaultMaxXDRStreamRecordSize+1)|xdrFrameLastFragment)
	stream := NewStream(io.NopCloser(bytes.NewReader(header[:])))

	var entry BucketEntry
	err := stream.ReadOne(&entry)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRecordTooLarge))

	// Header at exactly DefaultMaxXDRStreamRecordSize -> accepted (will fail reading data, not size check).
	binary.BigEndian.PutUint32(header[:], DefaultMaxXDRStreamRecordSize|xdrFrameLastFragment)
	stream = NewStream(io.NopCloser(bytes.NewReader(header[:])))

	err = stream.ReadOne(&entry)
	require.Error(t, err)
	// Should NOT be ErrRecordTooLarge — the size is accepted, but reading the payload fails.
	assert.False(t, errors.Is(err, ErrRecordTooLarge))
}

func TestSetMaxRecordSizeZero(t *testing.T) {
	// SetMaxRecordSize(0) should use the default.
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], (DefaultMaxXDRStreamRecordSize+1)|xdrFrameLastFragment)
	stream := NewStream(io.NopCloser(bytes.NewReader(header[:])))
	stream.SetMaxRecordSize(0) // should reset to default

	var entry BucketEntry
	err := stream.ReadOne(&entry)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRecordTooLarge))
}

func TestXdrStreamResumeFrom(t *testing.T) {
	entries := []BucketEntry{
		{
			Type: BucketEntryTypeLiveentry,
			LiveEntry: &LedgerEntry{
				Data: LedgerEntryData{
					Type: LedgerEntryTypeAccount,
					Account: &AccountEntry{
						AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
						Balance:   Int64(200000000),
					},
				},
			},
		},
		{
			Type: BucketEntryTypeLiveentry,
			LiveEntry: &LedgerEntry{
				Data: LedgerEntryData{
					Type: LedgerEntryTypeAccount,
					Account: &AccountEntry{
						AccountId: MustAddress("GC23QF2HUE52AMXUFUH3AYJAXXGXXV2VHXYYR6EYXETPKDXZSAW67XO4"),
						Balance:   Int64(100000000),
					},
				},
			},
		},
		{
			Type: BucketEntryTypeDeadentry,
			DeadEntry: &LedgerKey{
				Type:    LedgerEntryTypeAccount,
				Account: &LedgerKeyAccount{AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML")},
			},
		},
	}

	b := &bytes.Buffer{}
	for _, e := range entries {
		require.NoError(t, MarshalFramed(b, e))
	}
	expectedHash := sha256.Sum256(b.Bytes())

	readOne := func(stream *Stream) BucketEntry {
		var entry BucketEntry
		require.NoError(t, stream.ReadOne(&entry))
		return entry
	}

	// Read two records from the first stream. Its last record boundary is
	// then the start of the second record.
	first := NewStream(io.NopCloser(bytes.NewReader(b.Bytes())))
	assert.Equal(t, entries[0], readOne(first))
	assert.Equal(t, entries[1], readOne(first))
	require.NoError(t, first.Close())

	// The resumed stream starts at the second record and finishes the
	// first stream's hash.
	resumed := NewStream(io.NopCloser(bytes.NewReader(b.Bytes())))
	require.NoError(t, resumed.ResumeFrom(first))

	assert.Equal(t, entries[1], readOne(resumed))
	assert.Equal(t, entries[2], readOne(resumed))
	var entry BucketEntry
	assert.Equal(t, io.EOF, resumed.ReadOne(&entry))

	assert.Equal(t, int64(b.Len()), resumed.BytesRead())
	assert.NoError(t, resumed.ValidateHash(expectedHash))
	assert.NoError(t, resumed.Close())
}

func TestXdrStreamResumeFromUnreadStream(t *testing.T) {
	bucketEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:   Int64(200000000),
				},
			},
		},
	}
	b := &bytes.Buffer{}
	require.NoError(t, MarshalFramed(b, bucketEntry))
	expectedHash := sha256.Sum256(b.Bytes())

	// A stream that never ran ReadOne has no record boundary. Resuming from
	// it is the same as starting fresh.
	first := NewStream(io.NopCloser(bytes.NewReader(nil)))
	resumed := NewStream(io.NopCloser(bytes.NewReader(b.Bytes())))
	require.NoError(t, resumed.ResumeFrom(first))

	var entry BucketEntry
	require.NoError(t, resumed.ReadOne(&entry))
	assert.Equal(t, bucketEntry, entry)
	assert.Equal(t, io.EOF, resumed.ReadOne(&entry))
	assert.NoError(t, resumed.ValidateHash(expectedHash))
	assert.NoError(t, resumed.Close())
}

func TestXdrStreamResumeFromShortDownload(t *testing.T) {
	bucketEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:   Int64(200000000),
				},
			},
		},
	}
	b := &bytes.Buffer{}
	require.NoError(t, MarshalFramed(b, bucketEntry))
	require.NoError(t, MarshalFramed(b, bucketEntry))

	first := NewStream(io.NopCloser(bytes.NewReader(b.Bytes())))
	var entry BucketEntry
	require.NoError(t, first.ReadOne(&entry))
	require.NoError(t, first.ReadOne(&entry))

	// The new download is shorter than the boundary, so the resume fails.
	resumed := NewStream(io.NopCloser(bytes.NewReader(b.Bytes()[:4])))
	require.Error(t, resumed.ResumeFrom(first))
}

func TestXdrStreamHashCoversOnlyBytesRead(t *testing.T) {
	bucketEntry := BucketEntry{
		Type: BucketEntryTypeLiveentry,
		LiveEntry: &LedgerEntry{
			Data: LedgerEntryData{
				Type: LedgerEntryTypeAccount,
				Account: &AccountEntry{
					AccountId: MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:   Int64(200000000),
				},
			},
		},
	}
	b := &bytes.Buffer{}
	require.NoError(t, MarshalFramed(b, bucketEntry))
	firstFrameLen := b.Len()
	require.NoError(t, MarshalFramed(b, bucketEntry))

	// After one record the hash covers that record only, not the bytes the
	// buffered reader has already pulled from the source.
	stream := NewStream(io.NopCloser(bytes.NewReader(b.Bytes())))
	var entry BucketEntry
	require.NoError(t, stream.ReadOne(&entry))
	assert.Equal(t, int64(firstFrameLen), stream.BytesRead())
	expected := sha256.Sum256(b.Bytes()[:firstFrameLen])
	assert.Equal(t, expected[:], stream.sha256Hash.Sum(nil))
}

// countingCloser counts Close calls and returns closeErr from each one.
type countingCloser struct {
	io.Reader
	closes   int
	closeErr error
}

func (c *countingCloser) Close() error {
	c.closes++
	return c.closeErr
}

func TestXdrStreamCloseIsIdempotent(t *testing.T) {
	closer := &countingCloser{Reader: bytes.NewReader(nil)}
	stream := NewStream(closer)

	require.NoError(t, stream.Close())
	require.NoError(t, stream.Close())
	assert.Equal(t, 1, closer.closes)
}

func TestXdrStreamCloseReturnsFirstError(t *testing.T) {
	closer := &countingCloser{Reader: bytes.NewReader(nil), closeErr: errors.New("close failed")}
	stream := NewStream(closer)

	require.EqualError(t, stream.Close(), "close failed")
	closer.closeErr = nil
	require.EqualError(t, stream.Close(), "close failed")
	assert.Equal(t, 1, closer.closes)
}

func TestXdrStreamCloseAfterReadOneError(t *testing.T) {
	// ReadOne closes the reader when the frame is short. A later Close must
	// not close it a second time.
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], 64|xdrFrameLastFragment)
	closer := &countingCloser{Reader: bytes.NewReader(header[:])}
	stream := NewStream(closer)

	var entry BucketEntry
	require.Error(t, stream.ReadOne(&entry))
	require.NoError(t, stream.Close())
	assert.Equal(t, 1, closer.closes)
}
