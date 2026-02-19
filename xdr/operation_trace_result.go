package xdr

import "fmt"

// MapOperationResultTr returns a string representation of the result code for the operation.
// Each operation type corresponds to a specific result struct (e.g., CreateAccountResult, PaymentResult, ...)
// This function extracts and returns the result code's string value.
func (o OperationResultTr) MapOperationResultTr() (string, error) {
	var operationTraceDescription string
	operationType := o.Type

	switch operationType {
	case OperationTypeCreateAccount:
		operationTraceDescription = o.CreateAccountResult.Code.String()
	case OperationTypePayment:
		operationTraceDescription = o.PaymentResult.Code.String()
	case OperationTypePathPaymentStrictReceive:
		operationTraceDescription = o.PathPaymentStrictReceiveResult.Code.String()
	case OperationTypePathPaymentStrictSend:
		operationTraceDescription = o.PathPaymentStrictSendResult.Code.String()
	case OperationTypeManageBuyOffer:
		operationTraceDescription = o.ManageBuyOfferResult.Code.String()
	case OperationTypeManageSellOffer:
		operationTraceDescription = o.ManageSellOfferResult.Code.String()
	case OperationTypeCreatePassiveSellOffer:
		operationTraceDescription = o.CreatePassiveSellOfferResult.Code.String()
	case OperationTypeSetOptions:
		operationTraceDescription = o.SetOptionsResult.Code.String()
	case OperationTypeChangeTrust:
		operationTraceDescription = o.ChangeTrustResult.Code.String()
	case OperationTypeAllowTrust:
		operationTraceDescription = o.AllowTrustResult.Code.String()
	case OperationTypeAccountMerge:
		operationTraceDescription = o.AccountMergeResult.Code.String()
	case OperationTypeInflation:
		operationTraceDescription = o.InflationResult.Code.String()
	case OperationTypeManageData:
		operationTraceDescription = o.ManageDataResult.Code.String()
	case OperationTypeBumpSequence:
		operationTraceDescription = o.BumpSeqResult.Code.String()
	case OperationTypeCreateClaimableBalance:
		operationTraceDescription = o.CreateClaimableBalanceResult.Code.String()
	case OperationTypeClaimClaimableBalance:
		operationTraceDescription = o.ClaimClaimableBalanceResult.Code.String()
	case OperationTypeBeginSponsoringFutureReserves:
		operationTraceDescription = o.BeginSponsoringFutureReservesResult.Code.String()
	case OperationTypeEndSponsoringFutureReserves:
		operationTraceDescription = o.EndSponsoringFutureReservesResult.Code.String()
	case OperationTypeRevokeSponsorship:
		operationTraceDescription = o.RevokeSponsorshipResult.Code.String()
	case OperationTypeClawback:
		operationTraceDescription = o.ClawbackResult.Code.String()
	case OperationTypeClawbackClaimableBalance:
		operationTraceDescription = o.ClawbackClaimableBalanceResult.Code.String()
	case OperationTypeSetTrustLineFlags:
		operationTraceDescription = o.SetTrustLineFlagsResult.Code.String()
	case OperationTypeLiquidityPoolDeposit:
		operationTraceDescription = o.LiquidityPoolDepositResult.Code.String()
	case OperationTypeLiquidityPoolWithdraw:
		operationTraceDescription = o.LiquidityPoolWithdrawResult.Code.String()
	case OperationTypeInvokeHostFunction:
		operationTraceDescription = o.InvokeHostFunctionResult.Code.String()
	case OperationTypeExtendFootprintTtl:
		operationTraceDescription = o.ExtendFootprintTtlResult.Code.String()
	case OperationTypeRestoreFootprint:
		operationTraceDescription = o.RestoreFootprintResult.Code.String()
	default:
		if desc, err, ok := mapOperationResultTrForXdrHelloWorld(o); ok {
			return desc, err
		}
		return "", fmt.Errorf("unknown operation type: %s", operationType.String())
	}
	return operationTraceDescription, nil
}
