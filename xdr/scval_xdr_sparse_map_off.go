//go:build !xdr_sparse_map

package xdr

func scValEqualsForXdrSparseMap(_, _ ScVal) (bool, error, bool) {
	return false, nil, false
}

func scValStringForXdrSparseMap(_ ScVal) (string, error, bool) {
	return "", nil, false
}
