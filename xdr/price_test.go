package xdr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestPriceInvert(t *testing.T) {
	p := xdr.Price{N: 1, D: 2}
	p.Invert()
	assert.Equal(t, xdr.Price{N: 2, D: 1}, p)
}

func TestPriceEqual(t *testing.T) {
	// canonical
	assert.True(t, xdr.Price{N: 1, D: 2}.Equal(xdr.Price{N: 1, D: 2}))
	assert.False(t, xdr.Price{N: 1, D: 2}.Equal(xdr.Price{N: 2, D: 3}))

	// not canonical
	assert.True(t, xdr.Price{N: 1, D: 2}.Equal(xdr.Price{N: 5, D: 10}))
	assert.True(t, xdr.Price{N: 5, D: 10}.Equal(xdr.Price{N: 1, D: 2}))
	assert.True(t, xdr.Price{N: 5, D: 10}.Equal(xdr.Price{N: 50, D: 100}))
	assert.False(t, xdr.Price{N: 1, D: 3}.Equal(xdr.Price{N: 5, D: 10}))
	assert.False(t, xdr.Price{N: 5, D: 10}.Equal(xdr.Price{N: 1, D: 3}))
	assert.False(t, xdr.Price{N: 5, D: 15}.Equal(xdr.Price{N: 50, D: 100}))
}

func TestPriceCheaper(t *testing.T) {
	// canonical
	assert.True(t, xdr.Price{N: 1, D: 4}.Cheaper(xdr.Price{N: 1, D: 3}))
	assert.False(t, xdr.Price{N: 1, D: 3}.Cheaper(xdr.Price{N: 1, D: 4}))
	assert.False(t, xdr.Price{N: 1, D: 4}.Cheaper(xdr.Price{N: 1, D: 4}))

	// not canonical
	assert.True(t, xdr.Price{N: 10, D: 40}.Cheaper(xdr.Price{N: 3, D: 9}))
	assert.False(t, xdr.Price{N: 3, D: 9}.Cheaper(xdr.Price{N: 10, D: 40}))
	assert.False(t, xdr.Price{N: 10, D: 40}.Cheaper(xdr.Price{N: 10, D: 40}))
}

func TestNormalize(t *testing.T) {
	// canonical
	p := xdr.Price{N: 1, D: 4}
	p.Normalize()
	assert.Equal(t, xdr.Price{N: 1, D: 4}, p)

	// not canonical
	p = xdr.Price{N: 500, D: 2000}
	p.Normalize()
	assert.Equal(t, xdr.Price{N: 1, D: 4}, p)
}
