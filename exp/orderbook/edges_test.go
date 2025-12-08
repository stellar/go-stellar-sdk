package orderbook

import (
	"runtime"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveOffersMemoryUsage(t *testing.T) {
	edges := edgeSet{}
	for i := 0; i < 2000; i++ {
		edges = edges.addOffer(1, xdr.OfferEntry{
			SellerId: xdr.MustAddress("GCZFUQEPMLGUE2NB5RR7C3I2LTTLOEBM7GYD7PDKI4SU5HHWTDB553WD"),
			OfferId:  xdr.Int64(i),
		})
	}

	var afterAdded, afterRemoved, afterReallocate runtime.MemStats
	runtime.ReadMemStats(&afterAdded)

	t.Logf("after added: %d\n", afterAdded.HeapInuse)

	// Remove all offers except one
	for i := 0; i < 2000-1; i++ {
		var removed bool
		edges, removed = edges.removeOffer(1, xdr.Int64(i))
		require.True(t, removed)

	}

	runtime.ReadMemStats(&afterRemoved)
	t.Logf("after removed: %d\n", afterRemoved.HeapInuse)

	require.True(t, edges[0].reallocate)
	edges.reallocate()
	runtime.GC()

	runtime.ReadMemStats(&afterReallocate)
	t.Logf("after reallocate: %d\n", afterReallocate.HeapInuse)

	assert.Less(t, afterReallocate.HeapInuse, afterAdded.HeapInuse)
	assert.Less(t, afterReallocate.HeapInuse, afterRemoved.HeapInuse)
}
