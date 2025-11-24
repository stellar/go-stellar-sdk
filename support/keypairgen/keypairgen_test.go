package keypairgen_test

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/support/keypairgen"
	"github.com/stellar/go-stellar-sdk/support/keypairgen/keypairgentest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerator_Generate_sourceNotSet(t *testing.T) {
	g := keypairgen.Generator{}
	k1, err := g.Generate()
	require.NoError(t, err)
	t.Log("k1", k1.Address(), k1.Seed())
	k2, err := g.Generate()
	require.NoError(t, err)
	t.Log("k2", k2.Address(), k2.Seed())
	assert.NotEqual(t, k2.Address(), k1.Address())
}

func TestGenerator_Generate_sourceNotSetPtrNil(t *testing.T) {
	g := (*keypairgen.Generator)(nil)
	k1, err := g.Generate()
	require.NoError(t, err)
	t.Log("k1", k1.Address(), k1.Seed())
	k2, err := g.Generate()
	require.NoError(t, err)
	t.Log("k2", k2.Address(), k2.Seed())
	assert.NotEqual(t, k2.Address(), k1.Address())
}

func TestGenerator_Generate_sourceSet(t *testing.T) {
	s := keypairgentest.SliceSource{
		keypair.MustRandom(),
		keypair.MustRandom(),
		keypair.MustRandom(),
	}
	g := keypairgen.Generator{
		Source: &s,
	}
	k1, err := g.Generate()
	require.NoError(t, err)
	t.Log("k1", k1.Address(), k1.Seed())
	k2, err := g.Generate()
	require.NoError(t, err)
	t.Log("k2", k2.Address(), k2.Seed())
	assert.NotEqual(t, k2.Address(), k1.Address())
}
