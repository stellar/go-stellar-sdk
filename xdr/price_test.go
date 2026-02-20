package xdr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func makeAssertBoolNoError(t *testing.T) (func(bool, error), func(bool, error)) {
	t.Helper()
	assertTrueNoError := func(res bool, err error) {
		t.Helper()
		assert.NoError(t, err)
		assert.True(t, res)
	}
	assertFalseNoError := func(res bool, err error) {
		t.Helper()
		assert.NoError(t, err)
		assert.False(t, res)
	}
	return assertTrueNoError, assertFalseNoError
}

func TestPriceInvert(t *testing.T) {
	p := xdr.Price{N: 1, D: 2}
	assert.NoError(t, p.TryInvert())
	assert.Equal(t, xdr.Price{N: 2, D: 1}, p)

	// Using deprecated Price.Invert() method
	p = xdr.Price{N: 1, D: 2}
	p.Invert()
	assert.Equal(t, xdr.Price{N: 2, D: 1}, p)
}

func TestPriceEqual(t *testing.T) {
	assertTrueNoError, assertFalseNoError := makeAssertBoolNoError(t)

	// canonical
	assertTrueNoError(xdr.Price{N: 1, D: 2}.TryEqual(xdr.Price{N: 1, D: 2}))
	assertFalseNoError(xdr.Price{N: 1, D: 2}.TryEqual(xdr.Price{N: 2, D: 3}))

	// not canonical
	assertTrueNoError(xdr.Price{N: 1, D: 2}.TryEqual(xdr.Price{N: 5, D: 10}))
	assertTrueNoError(xdr.Price{N: 5, D: 10}.TryEqual(xdr.Price{N: 1, D: 2}))
	assertTrueNoError(xdr.Price{N: 5, D: 10}.TryEqual(xdr.Price{N: 50, D: 100}))
	assertFalseNoError(xdr.Price{N: 1, D: 3}.TryEqual(xdr.Price{N: 5, D: 10}))
	assertFalseNoError(xdr.Price{N: 5, D: 10}.TryEqual(xdr.Price{N: 1, D: 3}))
	assertFalseNoError(xdr.Price{N: 5, D: 15}.TryEqual(xdr.Price{N: 50, D: 100}))

	// canonical using deprecated Price.Equal() method
	assert.True(t, xdr.Price{N: 1, D: 2}.Equal(xdr.Price{N: 1, D: 2}))
	assert.False(t, xdr.Price{N: 1, D: 2}.Equal(xdr.Price{N: 2, D: 3}))

	// not canonical using deprecated Price.Equal() method
	assert.True(t, xdr.Price{N: 1, D: 2}.Equal(xdr.Price{N: 5, D: 10}))
	assert.True(t, xdr.Price{N: 5, D: 10}.Equal(xdr.Price{N: 1, D: 2}))
	assert.True(t, xdr.Price{N: 5, D: 10}.Equal(xdr.Price{N: 50, D: 100}))
	assert.False(t, xdr.Price{N: 1, D: 3}.Equal(xdr.Price{N: 5, D: 10}))
	assert.False(t, xdr.Price{N: 5, D: 10}.Equal(xdr.Price{N: 1, D: 3}))
	assert.False(t, xdr.Price{N: 5, D: 15}.Equal(xdr.Price{N: 50, D: 100}))
}

func TestPriceCheaper(t *testing.T) {
	assertTrueNoError, assertFalseNoError := makeAssertBoolNoError(t)

	// canonical
	assertTrueNoError(xdr.Price{N: 1, D: 4}.TryCheaper(xdr.Price{N: 1, D: 3}))
	assertFalseNoError(xdr.Price{N: 1, D: 3}.TryCheaper(xdr.Price{N: 1, D: 4}))
	assertFalseNoError(xdr.Price{N: 1, D: 4}.TryCheaper(xdr.Price{N: 1, D: 4}))

	// not canonical
	assertTrueNoError(xdr.Price{N: 10, D: 40}.TryCheaper(xdr.Price{N: 3, D: 9}))
	assertFalseNoError(xdr.Price{N: 3, D: 9}.TryCheaper(xdr.Price{N: 10, D: 40}))
	assertFalseNoError(xdr.Price{N: 10, D: 40}.TryCheaper(xdr.Price{N: 10, D: 40}))

	// canonical using deprecated Price.Cheaper() method
	assert.True(t, xdr.Price{N: 1, D: 4}.Cheaper(xdr.Price{N: 1, D: 3}))
	assert.False(t, xdr.Price{N: 1, D: 3}.Cheaper(xdr.Price{N: 1, D: 4}))
	assert.False(t, xdr.Price{N: 1, D: 4}.Cheaper(xdr.Price{N: 1, D: 4}))

	// not canonical using deprecated Price.Cheaper() method
	assert.True(t, xdr.Price{N: 10, D: 40}.Cheaper(xdr.Price{N: 3, D: 9}))
	assert.False(t, xdr.Price{N: 3, D: 9}.Cheaper(xdr.Price{N: 10, D: 40}))
	assert.False(t, xdr.Price{N: 10, D: 40}.Cheaper(xdr.Price{N: 10, D: 40}))
}

func TestNormalize(t *testing.T) {
	// canonical
	p := xdr.Price{N: 1, D: 4}
	assert.NoError(t, p.TryNormalize())
	assert.Equal(t, xdr.Price{N: 1, D: 4}, p)

	// not canonical
	p = xdr.Price{N: 500, D: 2000}
	assert.NoError(t, p.TryNormalize())
	assert.Equal(t, xdr.Price{N: 1, D: 4}, p)

	// canonical using deprecated Price.Normalize() method
	p = xdr.Price{N: 1, D: 4}
	p.Normalize()
	assert.Equal(t, xdr.Price{N: 1, D: 4}, p)

	// not canonical using deprecated Price.Normalize() method
	p = xdr.Price{N: 500, D: 2000}
	p.Normalize()
	assert.Equal(t, xdr.Price{N: 1, D: 4}, p)
}

func TestInvalidPrices(t *testing.T) {
	negativePrices := []xdr.Price{{N: -1, D: 4}, {N: 1, D: -4}, {N: -1, D: -4}}
	zeroPrices := []xdr.Price{{N: 0, D: 4}, {N: 1, D: 0}, {N: 0, D: 0}}

	errorOnInvalid := func(p xdr.Price) {
		assert.Error(t, p.Validate())
		assert.Error(t, p.TryNormalize())
		assert.Error(t, p.TryInvert())
		_, err := p.TryEqual(xdr.Price{N: 1, D: 4})
		assert.Error(t, err)
		_, err = p.TryCheaper(xdr.Price{N: 1, D: 4})
		assert.Error(t, err)
	}

	for _, p := range negativePrices {
		errorOnInvalid(p)
	}
	for _, p := range zeroPrices {
		errorOnInvalid(p)
	}
}
