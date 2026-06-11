package xdr

// Successful reports whether the transaction succeeded, reading only the
// result-code discriminant off the view — the zero-copy twin of
// TransactionResult.Successful. The success-code set lives HERE (next to the
// parsed twin) so the two paths cannot drift.
func (v TransactionResultView) Successful() (bool, error) {
	rr, err := v.Result()
	if err != nil {
		return false, err
	}
	cv, err := rr.Code()
	if err != nil {
		return false, err
	}
	code, err := cv.Value()
	if err != nil {
		return false, err
	}
	return code == TransactionResultCodeTxSuccess ||
		code == TransactionResultCodeTxFeeBumpInnerSuccess, nil
}
