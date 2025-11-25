package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/ingest/sac"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type RestoreFootprint struct {
	SourceAccount string
	Ext           xdr.TransactionExt
}

var defaultAssetBalanceRestorationFees = SorobanFees{
	Instructions:  0,
	DiskReadBytes: 500,
	WriteBytes:    500,
	ResourceFee:   4_000_000,
}

// AssetBalanceRestorationParams configures the restore footprint operation returned by
// NewAssetBalanceRestoration
type AssetBalanceRestorationParams struct {
	// NetworkPassphrase is the passphrase for the Stellar network
	NetworkPassphrase string
	// Contract is the contract which holds the asset balance
	Contract string
	// Asset is the asset which is held in the balance
	Asset Asset
	// SourceAccount is the source account for the restoration operation
	SourceAccount string
	// Fees configures the fee values for the
	// soroban transaction. If this field is omitted
	// default fee values will be used
	Fees SorobanFees
}

// NewAssetBalanceRestoration constructs a restore footprint operation which restores an
// asset balance for a smart contract
func NewAssetBalanceRestoration(params AssetBalanceRestorationParams) (RestoreFootprint, error) {
	asset, err := params.Asset.ToXDR()
	if err != nil {
		return RestoreFootprint{}, err
	}

	var assetContractID xdr.Hash
	assetContractID, err = asset.ContractID(params.NetworkPassphrase)
	if err != nil {
		return RestoreFootprint{}, err
	}

	decoded, err := strkey.Decode(strkey.VersionByteContract, params.Contract)
	if err != nil {
		return RestoreFootprint{}, err
	}
	var contractID xdr.Hash
	copy(contractID[:], decoded)

	resources := params.Fees
	if resources.ResourceFee == 0 {
		resources = defaultAssetBalanceRestorationFees
	}

	return RestoreFootprint{
		SourceAccount: params.SourceAccount,
		Ext: xdr.TransactionExt{
			V: 1,
			SorobanData: &xdr.SorobanTransactionData{
				Resources: xdr.SorobanResources{
					Footprint: xdr.LedgerFootprint{
						ReadWrite: []xdr.LedgerKey{
							sac.ContractBalanceLedgerKey(assetContractID, contractID),
						},
					},
					Instructions:  xdr.Uint32(resources.Instructions),
					DiskReadBytes: xdr.Uint32(resources.DiskReadBytes),
					WriteBytes:    xdr.Uint32(resources.WriteBytes),
				},
				ResourceFee: xdr.Int64(resources.ResourceFee),
			},
		},
	}, nil
}

func (f *RestoreFootprint) BuildXDR() (xdr.Operation, error) {
	xdrOp := xdr.RestoreFootprintOp{
		Ext: xdr.ExtensionPoint{
			V: 0,
		},
	}

	body, err := xdr.NewOperationBody(xdr.OperationTypeRestoreFootprint, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR Operation")
	}

	op := xdr.Operation{Body: body}

	SetOpSourceAccount(&op, f.SourceAccount)
	return op, nil
}

func (f *RestoreFootprint) FromXDR(xdrOp xdr.Operation) error {
	_, ok := xdrOp.Body.GetRestoreFootprintOp()
	if !ok {
		return errors.New("error parsing invoke host function operation from xdr")
	}
	f.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	return nil
}

func (f *RestoreFootprint) Validate() error {
	if f.SourceAccount != "" {
		_, err := xdr.AddressToMuxedAccount(f.SourceAccount)
		if err != nil {
			return NewValidationError("SourceAccount", err.Error())
		}
	}
	return nil
}

func (f *RestoreFootprint) GetSourceAccount() string {
	return f.SourceAccount
}

func (f *RestoreFootprint) BuildTransactionExt() (xdr.TransactionExt, error) {
	return f.Ext, nil
}
