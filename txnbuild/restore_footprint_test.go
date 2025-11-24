package txnbuild

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestRestoreAssetBalance(t *testing.T) {
	issuer := newKeypair0()
	sourceAccount := newKeypair1()
	params := AssetBalanceRestorationParams{
		NetworkPassphrase: network.PublicNetworkPassphrase,
		Contract:          "invalid",
		Asset: CreditAsset{
			Code:   "USD",
			Issuer: issuer.Address(),
		},
		SourceAccount: sourceAccount.Address(),
	}
	_, err := NewAssetBalanceRestoration(params)
	require.Error(t, err)

	params.Contract = newKeypair2().Address()
	_, err = NewAssetBalanceRestoration(params)
	require.Error(t, err)

	contractID := xdr.Hash{1}
	params.Contract = strkey.MustEncode(strkey.VersionByteContract, contractID[:])

	op, err := NewAssetBalanceRestoration(params)
	require.NoError(t, err)
	require.NoError(t, op.Validate())
	require.Equal(t, int64(op.Ext.SorobanData.ResourceFee), defaultAssetBalanceRestorationFees.ResourceFee)
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.WriteBytes), defaultAssetBalanceRestorationFees.WriteBytes)
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.DiskReadBytes), defaultAssetBalanceRestorationFees.DiskReadBytes)
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.Instructions), defaultAssetBalanceRestorationFees.Instructions)

	params.Fees = SorobanFees{
		Instructions:  1,
		DiskReadBytes: 2,
		WriteBytes:    3,
		ResourceFee:   4,
	}

	op, err = NewAssetBalanceRestoration(params)
	require.NoError(t, err)
	require.NoError(t, op.Validate())
	require.Equal(t, int64(op.Ext.SorobanData.ResourceFee), int64(4))
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.WriteBytes), uint32(3))
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.DiskReadBytes), uint32(2))
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.Instructions), uint32(1))
}
