package maps

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/support/collections/set"
	"github.com/stretchr/testify/require"
)

func TestSanity(t *testing.T) {
	m := map[int]float32{1: 10, 2: 20, 3: 30}
	for k, v := range m {
		require.Contains(t, Keys(m), k)
		require.Contains(t, Values(m), v)
	}

	// compatibility with collections/set.Set
	s := set.Set[float32]{}
	s.Add(1)
	s.Add(2)
	s.Add(3)

	for item := range s {
		require.Contains(t, Keys(s), item)
	}
}
