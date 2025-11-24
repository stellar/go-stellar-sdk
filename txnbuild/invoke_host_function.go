package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/amount"
	"github.com/stellar/go-stellar-sdk/ingest/sac"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type InvokeHostFunction struct {
	HostFunction  xdr.HostFunction
	Auth          []xdr.SorobanAuthorizationEntry
	SourceAccount string
	Ext           xdr.TransactionExt
}

type SorobanFees struct {
	Instructions  uint32
	DiskReadBytes uint32
	WriteBytes    uint32
	ResourceFee   int64
}

var defaultPaymentToContractFees = SorobanFees{
	Instructions:  400_000,
	DiskReadBytes: 1_000,
	WriteBytes:    1_000,
	ResourceFee:   5_000_000,
}

// PaymentToContractParams configures the payment returned by NewPaymentToContract
type PaymentToContractParams struct {
	// NetworkPassphrase is the passphrase for the Stellar network
	NetworkPassphrase string
	// Destination is the contract recipient of the payment
	Destination string
	// Amount is the amount being transferred
	Amount string
	// Asset is the asset being transferred
	Asset Asset
	// SourceAccount is the source account of the payment, it must be a Stellar account in strkey encoded`VersionByteAccountID` format, i.e. a 'G' account.
	SourceAccount string
	// Fees configures the fee values for the
	// soroban transaction. If this field is omitted
	// default fee values will be used
	Fees *SorobanFees
}

// NewPaymentToContract constructs an invoke host operation to send a payment from a
// an account to a destination smart contract. Note the account sending the payment
// must be the source account of the operation because the returned invoke host operation
// will use the source account as the auth credentials.
func NewPaymentToContract(params PaymentToContractParams) (InvokeHostFunction, error) {
	asset, err := params.Asset.ToXDR()
	if err != nil {
		return InvokeHostFunction{}, err
	}

	var assetContractID xdr.ContractId
	assetContractID, err = asset.ContractID(params.NetworkPassphrase)
	if err != nil {
		return InvokeHostFunction{}, err
	}

	sourceAccount, err := xdr.AddressToAccountId(params.SourceAccount)
	if err != nil {
		return InvokeHostFunction{}, err
	}

	decoded, err := strkey.Decode(strkey.VersionByteContract, params.Destination)
	if err != nil {
		return InvokeHostFunction{}, err
	}
	var destinationContractID xdr.ContractId
	copy(destinationContractID[:], decoded)

	parsedAmount, err := amount.Parse(params.Amount)
	if err != nil {
		return InvokeHostFunction{}, err
	}

	transferArgs := xdr.ScVec{
		xdr.ScVal{
			Type: xdr.ScValTypeScvAddress,
			Address: &xdr.ScAddress{
				Type:      xdr.ScAddressTypeScAddressTypeAccount,
				AccountId: &sourceAccount,
			},
		},
		xdr.ScVal{
			Type: xdr.ScValTypeScvAddress,
			Address: &xdr.ScAddress{
				Type:       xdr.ScAddressTypeScAddressTypeContract,
				ContractId: &destinationContractID,
			},
		},
		xdr.ScVal{
			Type: xdr.ScValTypeScvI128,
			I128: &xdr.Int128Parts{
				Hi: 0,
				Lo: xdr.Uint64(parsedAmount),
			},
		},
	}

	resources := defaultPaymentToContractFees
	if params.Fees != nil {
		resources = *params.Fees
	}

	assetContractInstance := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract: xdr.ScAddress{
				Type:       xdr.ScAddressTypeScAddressTypeContract,
				ContractId: &assetContractID,
			},
			Key: xdr.ScVal{
				Type: xdr.ScValTypeScvLedgerKeyContractInstance,
			},
			Durability: xdr.ContractDataDurabilityPersistent,
		},
	}

	footprint := xdr.LedgerFootprint{
		ReadOnly: []xdr.LedgerKey{
			assetContractInstance,
		},
		ReadWrite: []xdr.LedgerKey{
			sac.ContractBalanceLedgerKey(assetContractID, destinationContractID),
		},
	}

	if asset.IsNative() {
		footprint.ReadWrite = append(footprint.ReadWrite,
			xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.LedgerKeyAccount{
					AccountId: sourceAccount,
				},
			},
		)
	} else {
		issuer, err := asset.GetIssuerAccountId()
		if err != nil {
			return InvokeHostFunction{}, err
		}
		footprint.ReadOnly = append(footprint.ReadOnly,
			xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.LedgerKeyAccount{
					AccountId: issuer,
				},
			},
		)
		if !sourceAccount.Equals(issuer) {
			footprint.ReadWrite = append(footprint.ReadWrite,
				xdr.LedgerKey{
					Type: xdr.LedgerEntryTypeTrustline,
					TrustLine: &xdr.LedgerKeyTrustLine{
						AccountId: sourceAccount,
						Asset: xdr.TrustLineAsset{
							Type:       asset.Type,
							AlphaNum4:  asset.AlphaNum4,
							AlphaNum12: asset.AlphaNum12,
						},
					},
				},
			)
		}
	}

	return InvokeHostFunction{
		HostFunction: xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: xdr.ScAddress{
					Type:       xdr.ScAddressTypeScAddressTypeContract,
					ContractId: &assetContractID,
				},
				FunctionName: "transfer",
				Args:         transferArgs,
			},
		},
		SourceAccount: params.SourceAccount,
		Auth: []xdr.SorobanAuthorizationEntry{
			{
				Credentials: xdr.SorobanCredentials{
					Type: xdr.SorobanCredentialsTypeSorobanCredentialsSourceAccount,
				},
				RootInvocation: xdr.SorobanAuthorizedInvocation{
					Function: xdr.SorobanAuthorizedFunction{
						Type: xdr.SorobanAuthorizedFunctionTypeSorobanAuthorizedFunctionTypeContractFn,
						ContractFn: &xdr.InvokeContractArgs{
							ContractAddress: xdr.ScAddress{
								Type:       xdr.ScAddressTypeScAddressTypeContract,
								ContractId: &assetContractID,
							},
							FunctionName: "transfer",
							Args:         transferArgs,
						},
					},
				},
			},
		},
		Ext: xdr.TransactionExt{
			V: 1,
			SorobanData: &xdr.SorobanTransactionData{
				Resources: xdr.SorobanResources{
					Footprint:     footprint,
					Instructions:  xdr.Uint32(resources.Instructions),
					DiskReadBytes: xdr.Uint32(resources.DiskReadBytes),
					WriteBytes:    xdr.Uint32(resources.WriteBytes),
				},
				ResourceFee: xdr.Int64(resources.ResourceFee),
			},
		},
	}, nil
}

func (f *InvokeHostFunction) BuildXDR() (xdr.Operation, error) {

	opType := xdr.OperationTypeInvokeHostFunction
	xdrOp := xdr.InvokeHostFunctionOp{
		HostFunction: f.HostFunction,
		Auth:         f.Auth,
	}

	body, err := xdr.NewOperationBody(opType, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR Operation")
	}

	op := xdr.Operation{Body: body}

	SetOpSourceAccount(&op, f.SourceAccount)
	return op, nil
}

func (f *InvokeHostFunction) FromXDR(xdrOp xdr.Operation) error {
	result, ok := xdrOp.Body.GetInvokeHostFunctionOp()
	if !ok {
		return errors.New("error parsing invoke host function operation from xdr")
	}

	f.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	f.HostFunction = result.HostFunction
	f.Auth = result.Auth

	return nil
}

func (f *InvokeHostFunction) Validate() error {
	if f.SourceAccount != "" {
		_, err := xdr.AddressToMuxedAccount(f.SourceAccount)
		if err != nil {
			return NewValidationError("SourceAccount", err.Error())
		}
	}
	return nil
}

func (f *InvokeHostFunction) GetSourceAccount() string {
	return f.SourceAccount
}

func (f *InvokeHostFunction) BuildTransactionExt() (xdr.TransactionExt, error) {
	return f.Ext, nil
}
