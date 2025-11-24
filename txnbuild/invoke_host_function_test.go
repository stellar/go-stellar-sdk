package txnbuild

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/stretchr/testify/assert"
)

func TestPaymentToContract(t *testing.T) {
	issuer := newKeypair0()
	sourceAccount := newKeypair1()
	params := PaymentToContractParams{
		NetworkPassphrase: network.PublicNetworkPassphrase,
		Destination:       "invalid",
		Amount:            "10",
		Asset: CreditAsset{
			Code:   "USD",
			Issuer: issuer.Address(),
		},
		SourceAccount: sourceAccount.Address(),
	}
	_, err := NewPaymentToContract(params)
	require.Error(t, err)

	params.Destination = newKeypair2().Address()
	_, err = NewPaymentToContract(params)
	require.Error(t, err)

	contractID := xdr.Hash{1}
	params.Destination = strkey.MustEncode(strkey.VersionByteContract, contractID[:])

	op, err := NewPaymentToContract(params)
	require.NoError(t, err)
	require.NoError(t, op.Validate())
	require.Equal(t, int64(op.Ext.SorobanData.ResourceFee), defaultPaymentToContractFees.ResourceFee)
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.WriteBytes), defaultPaymentToContractFees.WriteBytes)
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.DiskReadBytes), defaultPaymentToContractFees.DiskReadBytes)
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.Instructions), defaultPaymentToContractFees.Instructions)

	params.Fees = &SorobanFees{
		Instructions:  1,
		DiskReadBytes: 2,
		WriteBytes:    3,
		ResourceFee:   4,
	}

	op, err = NewPaymentToContract(params)
	require.NoError(t, err)
	require.NoError(t, op.Validate())
	require.Equal(t, int64(op.Ext.SorobanData.ResourceFee), int64(4))
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.WriteBytes), uint32(3))
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.DiskReadBytes), uint32(2))
	require.Equal(t, uint32(op.Ext.SorobanData.Resources.Instructions), uint32(1))
}

func TestCreateInvokeHostFunctionValid(t *testing.T) {
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761100))

	invokeHostFunctionOp := InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type:           xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{},
		},
		Auth:          []xdr.SorobanAuthorizationEntry{},
		SourceAccount: sourceAccount.AccountID,
	}

	assert.NoError(t, invokeHostFunctionOp.Validate())
}

func TestCreateInvokeHostFunctionInvalid(t *testing.T) {
	invokeHostFunctionOp := InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type:           xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{},
		},
		Auth:          []xdr.SorobanAuthorizationEntry{},
		SourceAccount: "invalid account value",
	}

	assert.Error(t, invokeHostFunctionOp.Validate())
}

func TestInvokeHostFunctionRoundTrip(t *testing.T) {
	val := xdr.Int32(4)
	wasmId := xdr.Hash{1, 2, 3, 4}
	i64 := xdr.Int64(45)
	accountId := xdr.MustAddress("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	var invokeHostFunctionOp = &InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: xdr.ScAddress{
					Type:       xdr.ScAddressTypeScAddressTypeContract,
					ContractId: &xdr.ContractId{0x1, 0x2},
				},
				FunctionName: "foo",
				Args: xdr.ScVec{
					xdr.ScVal{
						Type: xdr.ScValTypeScvI32,
						I32:  &val,
					},
				},
			},
		},
		Auth: []xdr.SorobanAuthorizationEntry{
			{
				Credentials: xdr.SorobanCredentials{
					Type: xdr.SorobanCredentialsTypeSorobanCredentialsAddress,
					Address: &xdr.SorobanAddressCredentials{
						Address: xdr.ScAddress{
							Type:      xdr.ScAddressTypeScAddressTypeAccount,
							AccountId: &accountId,
						},
						Nonce:                     0,
						SignatureExpirationLedger: 0,
						Signature: xdr.ScVal{
							Type: xdr.ScValTypeScvI64,
							I64:  &i64,
						},
					},
				},
				RootInvocation: xdr.SorobanAuthorizedInvocation{
					Function: xdr.SorobanAuthorizedFunction{
						Type: xdr.SorobanAuthorizedFunctionTypeSorobanAuthorizedFunctionTypeContractFn,
						ContractFn: &xdr.InvokeContractArgs{
							ContractAddress: xdr.ScAddress{
								Type:       xdr.ScAddressTypeScAddressTypeContract,
								ContractId: &xdr.ContractId{0x1, 0x2},
							},
							FunctionName: "foo",
							Args: xdr.ScVec{
								xdr.ScVal{
									Type: xdr.ScValTypeScvI32,
									I32:  &val,
								},
							},
						},
					},
					SubInvocations: nil,
				},
			},
		},
		SourceAccount: "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
		Ext: xdr.TransactionExt{
			V: 1,
			SorobanData: &xdr.SorobanTransactionData{
				Resources: xdr.SorobanResources{
					Footprint: xdr.LedgerFootprint{
						ReadOnly: []xdr.LedgerKey{
							{
								Type: xdr.LedgerEntryTypeContractData,
								ContractData: &xdr.LedgerKeyContractData{
									Contract: xdr.ScAddress{
										Type:       xdr.ScAddressTypeScAddressTypeContract,
										ContractId: &xdr.ContractId{1, 2, 3},
									},
									Key: xdr.ScVal{
										Type: xdr.ScValTypeScvContractInstance,
										Instance: &xdr.ScContractInstance{
											Executable: xdr.ContractExecutable{
												Type:     xdr.ContractExecutableTypeContractExecutableWasm,
												WasmHash: &wasmId,
											},
										},
									},
								},
							},
						},
						ReadWrite: []xdr.LedgerKey{
							{
								Type: xdr.LedgerEntryTypeContractData,
								ContractData: &xdr.LedgerKeyContractData{
									Contract: xdr.ScAddress{
										Type:       xdr.ScAddressTypeScAddressTypeContract,
										ContractId: &xdr.ContractId{1, 2, 3},
									},
									Key: xdr.ScVal{
										Type: xdr.ScValTypeScvI64,
										I64:  &i64,
									},
								},
							},
						},
					},
					Instructions:  0,
					DiskReadBytes: 0,
					WriteBytes:    0,
				},
				ResourceFee: 1,
				Ext: xdr.SorobanTransactionDataExt{
					V: 0,
				},
			},
		},
	}
	testOperationsMarshalingRoundtrip(t, []Operation{invokeHostFunctionOp}, false)

	// with muxed accounts
	invokeHostFunctionOp.SourceAccount = "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK"
	testOperationsMarshalingRoundtrip(t, []Operation{invokeHostFunctionOp}, true)
}
