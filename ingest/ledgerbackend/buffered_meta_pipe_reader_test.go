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
	result, err := reader.readLedgerMetaFromPipe()
	require.NoError(t, err)
	assert.Equal(t, uint32(1234), uint32(result.LedgerHeaderHistoryEntry().Header.LedgerSeq))
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

	result1, err := reader.readLedgerMetaFromPipe()
	require.NoError(t, err)
	assert.Equal(t, uint32(100), uint32(result1.LedgerHeaderHistoryEntry().Header.LedgerSeq))

	result2, err := reader.readLedgerMetaFromPipe()
	require.NoError(t, err)
	assert.Equal(t, uint32(200), uint32(result2.LedgerHeaderHistoryEntry().Header.LedgerSeq))
}
