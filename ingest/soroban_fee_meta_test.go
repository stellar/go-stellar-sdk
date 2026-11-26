package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestSorobanFeeAccessorsAcrossMetaVersions(t *testing.T) {
	ext := xdr.SorobanTransactionMetaExt{V: 1, V1: &xdr.SorobanTransactionMetaExtV1{
		TotalNonRefundableResourceFeeCharged: 11,
		TotalRefundableResourceFeeCharged:    22,
		RentFeeCharged:                       33,
	}}

	v3 := func(meta *xdr.SorobanTransactionMeta) xdr.TransactionMeta {
		return xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{SorobanMeta: meta}}
	}
	v4 := func(meta *xdr.SorobanTransactionMetaV2) xdr.TransactionMeta {
		return xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{SorobanMeta: meta}}
	}

	tests := []struct {
		name string
		meta xdr.TransactionMeta
		ok   bool
	}{
		{"v3 with fee extension", v3(&xdr.SorobanTransactionMeta{Ext: ext}), true},
		{"v4 with fee extension", v4(&xdr.SorobanTransactionMetaV2{Ext: ext}), true},
		{"v3 without fee extension", v3(&xdr.SorobanTransactionMeta{}), false},
		{"v3 without soroban meta", v3(nil), false},
		{"v4 without soroban meta", v4(nil), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := LedgerTransaction{UnsafeMeta: tt.meta}
			for _, accessor := range []struct {
				get  func() (int64, bool)
				want int64
			}{
				{tx.SorobanTotalNonRefundableResourceFeeCharged, 11},
				{tx.SorobanTotalRefundableResourceFeeCharged, 22},
				{tx.SorobanRentFeeCharged, 33},
			} {
				var value int64
				var ok bool
				assert.NotPanics(t, func() { value, ok = accessor.get() })
				assert.Equal(t, tt.ok, ok)
				if tt.ok {
					assert.Equal(t, accessor.want, value)
				}
			}
		})
	}
}
