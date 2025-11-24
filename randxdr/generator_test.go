package randxdr

import (
	"bytes"
	"testing"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/stretchr/testify/assert"
)

func TestRandLedgerCloseMeta(t *testing.T) {
	gen := NewGenerator()
	for i := 0; i < 100; i++ {
		// generate random ledgers
		lcm := &xdr.LedgerCloseMeta{}
		shape := &gxdr.LedgerCloseMeta{}
		gen.Next(
			shape,
			[]Preset{
				{IsNestedInnerSet, SetVecLen(0)},
				{IsDeepAuthorizedInvocationTree, SetVecLen(0)},
			},
		)
		// check that the goxdr representation matches the go-xdr representation
		assert.NoError(t, gxdr.Convert(shape, lcm))

		lcmBytes, err := lcm.MarshalBinary()
		assert.NoError(t, err)

		assert.True(t, bytes.Equal(gxdr.Dump(shape), lcmBytes))
	}
}

func TestGeneratorIsDeterministic(t *testing.T) {
	gen := NewGenerator()
	shape := &gxdr.LedgerCloseMeta{}
	gen.Next(
		shape,
		[]Preset{
			{IsNestedInnerSet, SetVecLen(0)},
			{IsDeepAuthorizedInvocationTree, SetVecLen(0)},
		},
	)

	otherGen := NewGenerator()
	otherShape := &gxdr.LedgerCloseMeta{}
	otherGen.Next(
		otherShape,
		[]Preset{
			{IsNestedInnerSet, SetVecLen(0)},
			{IsDeepAuthorizedInvocationTree, SetVecLen(0)},
		},
	)

	assert.True(t, bytes.Equal(gxdr.Dump(shape), gxdr.Dump(otherShape)))
}
