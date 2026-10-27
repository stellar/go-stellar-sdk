package ledgerbackend

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func createTestLedgerCloseMeta(seq uint32) xdr.LedgerCloseMeta {
	return xdr.LedgerCloseMeta{
		V: int32(0),
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{
				Header: xdr.LedgerHeader{
					LedgerSeq: xdr.Uint32(seq),
				},
			},
		},
	}
}

func TestReadLedgerMetaFromPipe(t *testing.T) {
	lcm := createTestLedgerCloseMeta(1234)

	var buf bytes.Buffer
	require.NoError(t, xdr.MarshalFramed(&buf, lcm))

	reader := newBufferedLedgerMetaReader(&buf)
	raw, err := reader.readLedgerMetaFromPipe()
	require.NoError(t, err)

	// View-based access — no full decode required to read the sequence.
	seq, err := xdr.LedgerCloseMetaView(raw).LedgerSequence()
	require.NoError(t, err)
	assert.Equal(t, uint32(1234), seq)

	// Round-trip — bytes decode back to the same LedgerCloseMeta. Catches
	// off-by-one or framing bugs that would still leave a view-readable
	// header (e.g., extra trailing bytes consumed by SafeUnmarshal from the
	// next frame).
	var decoded xdr.LedgerCloseMeta
	require.NoError(t, xdr.SafeUnmarshal(raw, &decoded))
	assert.Equal(t, lcm, decoded)
}

func TestReadLedgerMetaFromPipeFrameTooLarge(t *testing.T) {
	var buf bytes.Buffer
	// Write a frame header with length exceeding maxLedgerMetaFrameSize.
	// The high bit marks the last fragment per RFC 5531 record marking.
	frameHeader := uint32(0x80000000) | (maxLedgerMetaFrameSize + 1)
	require.NoError(t, binary.Write(&buf, binary.BigEndian, frameHeader))

	reader := newBufferedLedgerMetaReader(&buf)
	_, err := reader.readLedgerMetaFromPipe()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "frame too large")
}

func TestReadLedgerMetaFromPipeMultipleFrames(t *testing.T) {
	lcm1 := createTestLedgerCloseMeta(100)
	lcm2 := createTestLedgerCloseMeta(200)

	var buf bytes.Buffer
	require.NoError(t, xdr.MarshalFramed(&buf, lcm1))
	require.NoError(t, xdr.MarshalFramed(&buf, lcm2))

	reader := newBufferedLedgerMetaReader(&buf)

	raw1, err := reader.readLedgerMetaFromPipe()
	require.NoError(t, err)
	seq1, err := xdr.LedgerCloseMetaView(raw1).LedgerSequence()
	require.NoError(t, err)
	assert.Equal(t, uint32(100), seq1)

	raw2, err := reader.readLedgerMetaFromPipe()
	require.NoError(t, err)
	seq2, err := xdr.LedgerCloseMetaView(raw2).LedgerSequence()
	require.NoError(t, err)
	assert.Equal(t, uint32(200), seq2)

	// Each frame's raw bytes must round-trip back to its own LedgerCloseMeta —
	// guards against the reader bleeding bytes between frames.
	var decoded1, decoded2 xdr.LedgerCloseMeta
	require.NoError(t, xdr.SafeUnmarshal(raw1, &decoded1))
	require.NoError(t, xdr.SafeUnmarshal(raw2, &decoded2))
	assert.Equal(t, lcm1, decoded1)
	assert.Equal(t, lcm2, decoded2)
}
