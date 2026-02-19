//go:build xdr_sparse_map

package xdr

import "fmt"

func scValEqualsForXdrSparseMap(s, o ScVal) (bool, error, bool) {
	switch s.Type {
	case ScValTypeScvSparseMap:
		return s.MustSparseMap().Equals(o.MustSparseMap()), nil, true
	default:
		return false, nil, false
	}
}

func scValStringForXdrSparseMap(s ScVal) (string, error, bool) {
	switch s.Type {
	case ScValTypeScvSparseMap:
		sm := s.MustSparseMap()
		if sm == nil {
			return "nil", nil, true
		}
		return fmt.Sprintf("%v", *sm), nil, true
	default:
		return "", nil, false
	}
}
