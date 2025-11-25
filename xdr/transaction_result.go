package xdr

import "github.com/stellar/go-stellar-sdk/support/errors"

// Successful returns true if the transaction succeeded
func (r TransactionResult) Successful() bool {
	return r.Result.Code == TransactionResultCodeTxSuccess ||
		r.Result.Code == TransactionResultCodeTxFeeBumpInnerSuccess
}

// OperationResults returns the operation results for the transaction
func (r TransactionResult) OperationResults() ([]OperationResult, bool) {
	innerResults, ok := r.Result.GetInnerResultPair()
	if ok {
		return innerResults.Result.Result.GetResults()
	}
	return r.Result.GetResults()
}

// Successful returns true if the transaction succeeded
func (r TransactionResultPair) Successful() bool {
	return r.Result.Successful()
}

// OperationResults returns the operation results for the transaction
func (r TransactionResultPair) OperationResults() ([]OperationResult, bool) {
	return r.Result.OperationResults()
}

// InnerHash returns the hash of the inner transaction.
// This function can only be called on fee bump transactions.
func (r TransactionResultPair) InnerHash() Hash {
	return r.Result.InnerHash()
}

// InnerHash returns the hash of the inner transaction.
// This function can only be called on fee bump transactions.
func (r TransactionResult) InnerHash() Hash {
	return r.Result.MustInnerResultPair().TransactionHash
}

// ExtractBalanceID will parse the operation result at `opIndex` within the
// given `txResult`, returning the internal XDR structure for the claimable
// balance ID.
//
// If the specified operation index does not point to a successful
// `CreateClaimableBalance` operation result, this function panics.
func (r TransactionResult) ExtractBalanceID(opIndex int) (*ClaimableBalanceId, error) {
	opResults, ok := r.OperationResults()
	if !ok {
		return nil, errors.New("Failed to retrieve transaction's operation results")
	}

	if opIndex < 0 || opIndex >= len(opResults) {
		return nil, errors.New("Invalid operation index")
	}

	result := opResults[opIndex]
	return result.MustTr().MustCreateClaimableBalanceResult().BalanceId, nil
}

// ExtractBalanceIDHex works like `ExtractBalanceID`, but will return the hex
// encoding of the resulting value.
func (r TransactionResult) ExtractBalanceIDHex(opIndex int) (string, error) {
	balanceId, err := r.ExtractBalanceID(opIndex)
	if err != nil {
		return "", err
	}

	hex, err := MarshalHex(balanceId)
	if err != nil {
		return "", errors.Wrap(err, "Failed to determine balance ID")
	}

	return hex, nil
}
