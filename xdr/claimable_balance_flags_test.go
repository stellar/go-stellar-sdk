package xdr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestIsClawbackEnabled(t *testing.T) {
	tt := assert.New(t)

	flag := xdr.ClaimableBalanceFlags(1)
	tt.True(flag.IsClawbackEnabled())

	flag = xdr.ClaimableBalanceFlags(0)
	tt.False(flag.IsClawbackEnabled())

	flag = xdr.ClaimableBalanceFlags(2)
	tt.False(flag.IsClawbackEnabled())

	flag = xdr.ClaimableBalanceFlags(4)
	tt.False(flag.IsClawbackEnabled())

}
