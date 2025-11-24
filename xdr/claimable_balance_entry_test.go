package xdr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestClaimableBalanceEntry_Flags(t *testing.T) {
	entry := xdr.ClaimableBalanceEntry{
		Ext: xdr.ClaimableBalanceEntryExt{
			V: 0,
		},
	}

	assert.Equal(t, xdr.ClaimableBalanceFlags(0), entry.Flags())

	entry = xdr.ClaimableBalanceEntry{
		Ext: xdr.ClaimableBalanceEntryExt{
			V: 1,
			V1: &xdr.ClaimableBalanceEntryExtensionV1{
				Flags: xdr.Uint32(xdr.ClaimableBalanceFlagsClaimableBalanceClawbackEnabledFlag),
			},
		},
	}

	assert.Equal(t, xdr.ClaimableBalanceFlagsClaimableBalanceClawbackEnabledFlag, entry.Flags())
}
