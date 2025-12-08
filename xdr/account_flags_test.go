package xdr_test

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
)

func TestIsAuthRequired(t *testing.T) {
	tt := assert.New(t)

	flag := xdr.AccountFlags(1)
	tt.True(flag.IsAuthRequired())

	flag = xdr.AccountFlags(0)
	tt.False(flag.IsAuthRequired())

	flag = xdr.AccountFlags(2)
	tt.False(flag.IsAuthRequired())

	flag = xdr.AccountFlags(4)
	tt.False(flag.IsAuthRequired())

}

func TestIsAuthRevocable(t *testing.T) {
	tt := assert.New(t)

	flag := xdr.AccountFlags(2)
	tt.True(flag.IsAuthRevocable())

	flag = xdr.AccountFlags(0)
	tt.False(flag.IsAuthRevocable())

	flag = xdr.AccountFlags(1)
	tt.False(flag.IsAuthRevocable())

	flag = xdr.AccountFlags(4)
	tt.False(flag.IsAuthRevocable())

}
func TestIsAuthImmutable(t *testing.T) {
	tt := assert.New(t)

	flag := xdr.AccountFlags(4)
	tt.True(flag.IsAuthImmutable())

	flag = xdr.AccountFlags(0)
	tt.False(flag.IsAuthImmutable())

	flag = xdr.AccountFlags(1)
	tt.False(flag.IsAuthImmutable())

	flag = xdr.AccountFlags(2)
	tt.False(flag.IsAuthImmutable())
}

func TestIsAuthClawbackEnabled(t *testing.T) {
	tt := assert.New(t)

	flag := xdr.AccountFlags(8)
	tt.True(flag.IsAuthClawbackEnabled())

	flag = xdr.AccountFlags(0)
	tt.False(flag.IsAuthClawbackEnabled())

	flag = xdr.AccountFlags(1)
	tt.False(flag.IsAuthClawbackEnabled())

	flag = xdr.AccountFlags(2)
	tt.False(flag.IsAuthClawbackEnabled())
}
