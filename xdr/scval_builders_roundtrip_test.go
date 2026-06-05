package xdr

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/strkey"
)

// TestScvBuilders_Roundtrip asserts every Scv* builder produces an ScVal that
// round-trips through MarshalBinary/UnmarshalBinary unchanged. The 128-bit
// variants are covered separately in scval_builders_test.go.
func TestScvBuilders_Roundtrip(t *testing.T) {
	accountStrkey := strkeyForTest(t, strkey.VersionByteAccountID, 0x40)
	contractRaw := make([]byte, 32)
	for i := range contractRaw {
		contractRaw[i] = byte(i + 1)
	}
	contractStrkey, err := strkey.Encode(strkey.VersionByteContract, contractRaw)
	require.NoError(t, err)

	mustScv := func(v ScVal, err error) ScVal {
		t.Helper()
		require.NoError(t, err)
		return v
	}

	cases := []struct {
		name string
		val  ScVal
	}{
		{"address_account", mustScv(ScvAddress(accountStrkey))},
		{"address_contract", mustScv(ScvAddress(contractStrkey))},
		{"symbol_empty", mustScv(ScvSymbol(""))},
		{"symbol_typical", mustScv(ScvSymbol("transfer"))},
		{"symbol_max_len", mustScv(ScvSymbol("ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"))},
		{"string_empty", ScvString("")},
		{"string_unicode", ScvString("héllo, 世界")},
		{"bool_true", ScvBool(true)},
		{"bool_false", ScvBool(false)},
		{"bytes_empty", ScvBytes(nil)},
		{"bytes_nonempty", ScvBytes([]byte{0xde, 0xad, 0xbe, 0xef})},
		{"vec_empty", ScvVec()},
		{"vec_nested", ScvVec(ScvBool(true), ScvU128FromUint64(7), ScvString("x"))},
		{"map_empty", mustScv(ScvMap(map[string]ScVal{}))},
		{"map_mixed", mustScv(ScvMap(map[string]ScVal{
			"flag":   ScvBool(false),
			"amount": ScvI128FromInt64(-42),
			"label":  ScvString("ok"),
		}))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bin, err := tc.val.MarshalBinary()
			require.NoError(t, err)

			var decoded ScVal
			require.NoError(t, decoded.UnmarshalBinary(bin))
			require.True(t, tc.val.Equals(decoded), "decoded ScVal differs from original")

			// Re-encoding the decoded value must yield byte-identical output.
			rebin, err := decoded.MarshalBinary()
			require.NoError(t, err)
			require.Equal(t, bin, rebin)
		})
	}
}

// TestScvBuilders_TypeDiscriminants pins each builder's resulting ScVal.Type
// so a future XDR regeneration that renames or reorders the enum surfaces here
// rather than at a distant call site.
func TestScvBuilders_TypeDiscriminants(t *testing.T) {
	addrStr := strkeyForTest(t, strkey.VersionByteAccountID, 0x12)
	addrVal, err := ScvAddress(addrStr)
	require.NoError(t, err)

	sym, err := ScvSymbol("ok")
	require.NoError(t, err)

	i128, err := ScvI128(big.NewInt(1))
	require.NoError(t, err)

	u128, err := ScvU128(big.NewInt(1))
	require.NoError(t, err)

	m, err := ScvMap(map[string]ScVal{"k": ScvBool(true)})
	require.NoError(t, err)

	cases := []struct {
		name string
		got  ScValType
		want ScValType
	}{
		{"address", addrVal.Type, ScValTypeScvAddress},
		{"symbol", sym.Type, ScValTypeScvSymbol},
		{"string", ScvString("x").Type, ScValTypeScvString},
		{"bool", ScvBool(true).Type, ScValTypeScvBool},
		{"bytes", ScvBytes(nil).Type, ScValTypeScvBytes},
		{"vec", ScvVec().Type, ScValTypeScvVec},
		{"map", m.Type, ScValTypeScvMap},
		{"i128", i128.Type, ScValTypeScvI128},
		{"u128", u128.Type, ScValTypeScvU128},
		{"i128_from_int64", ScvI128FromInt64(0).Type, ScValTypeScvI128},
		{"u128_from_uint64", ScvU128FromUint64(0).Type, ScValTypeScvU128},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.got)
		})
	}
}
