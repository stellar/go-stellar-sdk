package keypairgentest_test

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/support/keypairgen/keypairgentest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSliceSource_Generate tests that the key returned from the slice source
// is equal to the next key in the slice provided when creating the slice
// source, and each subsequent call returns the key after it and so on.
func TestSliceSource_Generate(t *testing.T) {
	kp1 := keypair.MustRandom()
	kp2 := keypair.MustRandom()
	s := keypairgentest.SliceSource{kp1, kp2}

	gkp1, err := s.Generate()
	require.NoError(t, err)
	assert.Equal(t, gkp1, kp1)

	gkp2, err := s.Generate()
	require.NoError(t, err)
	assert.Equal(t, gkp2, kp2)
}

// TestSliceSource_Generate_noMoreAvailable tests that when Generate is called
// by the slice has been exhausted of values the function panics.
func TestSliceSource_Generate_noMoreAvailable(t *testing.T) {
	kp := keypair.MustRandom()
	s := keypairgentest.SliceSource{kp}

	_, err := s.Generate()
	require.NoError(t, err)

	defer func() {
		r := recover()
		assert.NotNil(t, r)
		assert.EqualError(t, r.(error), "runtime error: index out of range [0] with length 0")
	}()
	_, _ = s.Generate()
}
