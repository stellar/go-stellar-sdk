package xdr_test

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
)

func TestIsAuthorized(t *testing.T) {
	tt := assert.New(t)

	flag := xdr.TrustLineFlags(1)
	tt.True(flag.IsAuthorized())

	flag = xdr.TrustLineFlags(0)
	tt.False(flag.IsAuthorized())

	flag = xdr.TrustLineFlags(2)
	tt.False(flag.IsAuthorized())
}

func TestIsAuthorizedToMaintainLiabilitiesFlag(t *testing.T) {
	tt := assert.New(t)

	flag := xdr.TrustLineFlags(1)
	tt.False(flag.IsAuthorizedToMaintainLiabilitiesFlag())

	flag = xdr.TrustLineFlags(0)
	tt.False(flag.IsAuthorizedToMaintainLiabilitiesFlag())

	flag = xdr.TrustLineFlags(2)
	tt.True(flag.IsAuthorizedToMaintainLiabilitiesFlag())
}

func TestIsClawbackEnabledFlag(t *testing.T) {
	tt := assert.New(t)

	flag := xdr.TrustLineFlags(1)
	tt.False(flag.IsClawbackEnabledFlag())

	flag = xdr.TrustLineFlags(0)
	tt.False(flag.IsClawbackEnabledFlag())

	flag = xdr.TrustLineFlags(4)
	tt.True(flag.IsClawbackEnabledFlag())
}
