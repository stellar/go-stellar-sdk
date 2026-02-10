package xdr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPoolBasicUsage(t *testing.T) {
	pool := NewPool[LedgerCloseMeta]()

	// Get object
	obj1 := pool.Get()
	require.NotNil(t, obj1)

	// Return to pool
	pool.Put(obj1)

	// Get again - may or may not be same object (GC can clear pool)
	obj2 := pool.Get()
	require.NotNil(t, obj2)
	pool.Put(obj2)
}

func TestPoolWithDecode(t *testing.T) {
	pool := NewPool[ScVal]()

	// Create test data
	original, _ := NewScVal(ScValTypeScvU32, Uint32(12345))
	data, err := original.MarshalBinary()
	require.NoError(t, err)

	// Get from pool, decode, verify, return
	for i := 0; i < 10; i++ {
		obj := pool.Get()
		err := SafeUnmarshal(data, obj)
		require.NoError(t, err)
		require.Equal(t, ScValTypeScvU32, obj.Type)
		require.Equal(t, Uint32(12345), obj.U32)
		pool.Put(obj)
	}
}
