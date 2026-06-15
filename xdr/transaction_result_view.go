package xdr

// Successful reports whether the transaction succeeded, reading only the
// result-code discriminant off the view — the zero-copy twin of
// TransactionResult.Successful. The success-code set lives HERE (next to the
// parsed twin) so the two paths cannot drift.
func (v TransactionResultView) Successful() (bool, error) {
	// Must* accessors panic with *ViewError on the first malformed field; Try
	// recovers it, collapsing the per-field error ladder to one closure.
	return Try(func() bool {
		code := v.MustResult().MustCode()
		return code == TransactionResultCodeTxSuccess ||
			code == TransactionResultCodeTxFeeBumpInnerSuccess
	})
}
