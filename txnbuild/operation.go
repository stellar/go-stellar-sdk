package txnbuild

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// Operation represents the operation types of the Stellar network.
type Operation interface {
	BuildXDR() (xdr.Operation, error)
	FromXDR(xdrOp xdr.Operation) error
	Validate() error
	GetSourceAccount() string
}

// SetOpSourceAccount sets the source account ID on an Operation, allowing M-strkeys (as defined in SEP23).
func SetOpSourceAccount(op *xdr.Operation, sourceAccount string) {
	if sourceAccount == "" {
		return
	}
	var opSourceAccountID xdr.MuxedAccount
	opSourceAccountID.SetAddress(sourceAccount)
	op.SourceAccount = &opSourceAccountID
}

// operationFromXDR returns a txnbuild Operation from its corresponding XDR operation
func operationFromXDR(xdrOp xdr.Operation) (Operation, error) {
	var newOp Operation
	switch xdrOp.Body.Type {
	case xdr.OperationTypeCreateAccount:
		newOp = &CreateAccount{}
	case xdr.OperationTypePayment:
		newOp = &Payment{}
	case xdr.OperationTypePathPaymentStrictReceive:
		newOp = &PathPayment{}
	case xdr.OperationTypeManageSellOffer:
		newOp = &ManageSellOffer{}
	case xdr.OperationTypeCreatePassiveSellOffer:
		newOp = &CreatePassiveSellOffer{}
	case xdr.OperationTypeSetOptions:
		newOp = &SetOptions{}
	case xdr.OperationTypeChangeTrust:
		newOp = &ChangeTrust{}
	case xdr.OperationTypeAllowTrust:
		newOp = &AllowTrust{}
	case xdr.OperationTypeAccountMerge:
		newOp = &AccountMerge{}
	case xdr.OperationTypeInflation:
		newOp = &Inflation{}
	case xdr.OperationTypeManageData:
		newOp = &ManageData{}
	case xdr.OperationTypeBumpSequence:
		newOp = &BumpSequence{}
	case xdr.OperationTypeManageBuyOffer:
		newOp = &ManageBuyOffer{}
	case xdr.OperationTypePathPaymentStrictSend:
		newOp = &PathPaymentStrictSend{}
	case xdr.OperationTypeBeginSponsoringFutureReserves:
		newOp = &BeginSponsoringFutureReserves{}
	case xdr.OperationTypeEndSponsoringFutureReserves:
		newOp = &EndSponsoringFutureReserves{}
	case xdr.OperationTypeCreateClaimableBalance:
		newOp = &CreateClaimableBalance{}
	case xdr.OperationTypeClaimClaimableBalance:
		newOp = &ClaimClaimableBalance{}
	case xdr.OperationTypeRevokeSponsorship:
		newOp = &RevokeSponsorship{}
	case xdr.OperationTypeClawback:
		newOp = &Clawback{}
	case xdr.OperationTypeClawbackClaimableBalance:
		newOp = &ClawbackClaimableBalance{}
	case xdr.OperationTypeSetTrustLineFlags:
		newOp = &SetTrustLineFlags{}
	case xdr.OperationTypeLiquidityPoolDeposit:
		newOp = &LiquidityPoolDeposit{}
	case xdr.OperationTypeLiquidityPoolWithdraw:
		newOp = &LiquidityPoolWithdraw{}
	case xdr.OperationTypeInvokeHostFunction:
		newOp = &InvokeHostFunction{}
	case xdr.OperationTypeExtendFootprintTtl:
		newOp = &ExtendFootprintTtl{}
	case xdr.OperationTypeRestoreFootprint:
		newOp = &RestoreFootprint{}
	default:
		return nil, fmt.Errorf("unknown operation type: %d", xdrOp.Body.Type)
	}

	err := newOp.FromXDR(xdrOp)
	return newOp, err
}

func accountFromXDR(account *xdr.MuxedAccount) string {
	if account != nil {
		return account.Address()
	}
	return ""
}

// SorobanOperation represents a smart contract operation on the Stellar network.
type SorobanOperation interface {
	BuildTransactionExt() (xdr.TransactionExt, error)
}
