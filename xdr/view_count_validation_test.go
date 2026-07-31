package xdr

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestArrayCountBufferBound is the regression guard for array count validation:
// a variable-length array view whose wire count word claims far more elements
// than the remaining buffer can hold must be rejected up front by Count(),
// rather than trusting the count for allocation. ScVecView is an unbounded
// var-array of SCVal (4-byte minimum element wire size). A 4-byte buffer holds
// only the count word and zero element bytes, so any positive count must fail.
func TestArrayCountBufferBound(t *testing.T) {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, 1_000_000) // 1e6 elements claimed, 0 bytes available

	_, err := ScVecView(buf).Count()
	require.Error(t, err, "Count() must reject a count whose elements cannot fit the buffer")

	// A legitimately empty vector (count 0) must still succeed.
	zero := make([]byte, 4)
	n, err := ScVecView(zero).Count()
	require.NoError(t, err)
	require.Equal(t, 0, n)
}
