package xdr

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/strkey"
)

// strkeyForTest deterministically encodes a 32-byte payload with the given
// version byte. Avoids importing `keypair` (which would create a cycle).
func strkeyForTest(t *testing.T, v strkey.VersionByte, seed byte) string {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(int(seed) + i)
	}
	s, err := strkey.Encode(v, raw)
	require.NoError(t, err)
	return s
}

func TestScvAddress_Account(t *testing.T) {
	addrStr := strkeyForTest(t, strkey.VersionByteAccountID, 0x40)
	val, err := ScvAddress(addrStr)
	require.NoError(t, err)
	require.Equal(t, ScValTypeScvAddress, val.Type)
	require.Equal(t, ScAddressTypeScAddressTypeAccount, val.Address.Type)
	got, err := val.Address.String()
	require.NoError(t, err)
	require.Equal(t, addrStr, got)
}

func TestScvAddress_Contract(t *testing.T) {
	// 32-byte contract id strkey-encoded
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	cstr, err := strkey.Encode(strkey.VersionByteContract, raw)
	require.NoError(t, err)

	val, err := ScvAddress(cstr)
	require.NoError(t, err)
	require.Equal(t, ScAddressTypeScAddressTypeContract, val.Address.Type)
	got, err := val.Address.String()
	require.NoError(t, err)
	require.Equal(t, cstr, got)
}

func TestScvAddress_Muxed(t *testing.T) {
	// Encode a muxed strkey: 32-byte ed25519 + 8-byte big-endian id.
	pub := make([]byte, 32)
	for i := range pub {
		pub[i] = byte(0x10 + i)
	}
	id := uint64(0xDEADBEEFCAFEBABE)
	payload := make([]byte, 40)
	copy(payload[:32], pub)
	for i := 0; i < 8; i++ {
		payload[32+i] = byte(id >> (56 - 8*i))
	}
	mstr, err := strkey.Encode(strkey.VersionByteMuxedAccount, payload)
	require.NoError(t, err)

	val, err := ScvAddress(mstr)
	require.NoError(t, err)
	require.Equal(t, ScAddressTypeScAddressTypeMuxedAccount, val.Address.Type)
	require.Equal(t, Uint64(id), val.Address.MuxedAccount.Id)
	require.Equal(t, pub, val.Address.MuxedAccount.Ed25519[:])
}

func TestScvAddress_Invalid(t *testing.T) {
	_, err := ScvAddress("not-a-strkey")
	require.Error(t, err)

	// Seed (S...) strkey is valid format but not a supported ScAddress kind.
	seedStrkey := strkeyForTest(t, strkey.VersionByteSeed, 0x70)
	_, err = ScvAddress(seedStrkey)
	require.Error(t, err)
}

func TestScvI128_RoundTripsViaXDR(t *testing.T) {
	cases := []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(-1),
		big.NewInt(1 << 62),
		new(big.Int).Neg(big.NewInt(1 << 62)),
		new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1)), // 2^127 - 1
		new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127)),                // -2^127
	}
	for _, v := range cases {
		val, err := ScvI128(v)
		require.NoError(t, err, v.String())
		require.Equal(t, ScValTypeScvI128, val.Type)

		bin, err := val.MarshalBinary()
		require.NoError(t, err)
		var decoded ScVal
		require.NoError(t, decoded.UnmarshalBinary(bin))
		require.True(t, val.Equals(decoded))

		require.Equal(t, v.String(), decoded.String())
	}
}

func TestScvI128_OutOfRange(t *testing.T) {
	over := new(big.Int).Lsh(big.NewInt(1), 127) // 2^127 (one past max)
	_, err := ScvI128(over)
	require.Error(t, err)

	under := new(big.Int).Sub(new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127)), big.NewInt(1)) // -2^127 - 1
	_, err = ScvI128(under)
	require.Error(t, err)

	_, err = ScvI128(nil)
	require.Error(t, err)
}

func TestScvU128_RoundTripsViaXDR(t *testing.T) {
	cases := []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		new(big.Int).Lsh(big.NewInt(1), 64),
		new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)), // 2^128 - 1
	}
	for _, v := range cases {
		val, err := ScvU128(v)
		require.NoError(t, err, v.String())

		bin, err := val.MarshalBinary()
		require.NoError(t, err)
		var decoded ScVal
		require.NoError(t, decoded.UnmarshalBinary(bin))
		require.True(t, val.Equals(decoded))

		require.Equal(t, v.String(), decoded.String())
	}
}

func TestScvU128_OutOfRange(t *testing.T) {
	_, err := ScvU128(big.NewInt(-1))
	require.Error(t, err)

	over := new(big.Int).Lsh(big.NewInt(1), 128)
	_, err = ScvU128(over)
	require.Error(t, err)

	_, err = ScvU128(nil)
	require.Error(t, err)
}

func TestScvI128FromInt64(t *testing.T) {
	for _, v := range []int64{0, 1, -1, 1 << 62, -(1 << 62)} {
		val := ScvI128FromInt64(v)
		require.Equal(t, big.NewInt(v).String(), val.String())
	}
}

func TestScvU128FromUint64(t *testing.T) {
	for _, v := range []uint64{0, 1, 1 << 63, ^uint64(0)} {
		val := ScvU128FromUint64(v)
		require.Equal(t, new(big.Int).SetUint64(v).String(), val.String())
	}
}

func TestScvSymbol_Valid(t *testing.T) {
	for _, s := range []string{"", "transfer", "A_b9", "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"} {
		val, err := ScvSymbol(s)
		require.NoError(t, err, s)
		require.Equal(t, ScValTypeScvSymbol, val.Type)
		require.Equal(t, s, string(*val.Sym))
	}
}

func TestScvSymbol_Invalid(t *testing.T) {
	// Too long
	_, err := ScvSymbol("123456789012345678901234567890123") // 33 chars
	require.Error(t, err)

	// Bad char
	_, err = ScvSymbol("with space")
	require.Error(t, err)
	_, err = ScvSymbol("with-dash")
	require.Error(t, err)
	_, err = ScvSymbol("ünicode")
	require.Error(t, err)
}

func TestScvString(t *testing.T) {
	val := ScvString("hello world")
	require.Equal(t, ScValTypeScvString, val.Type)
	require.Equal(t, "hello world", string(*val.Str))

	// Strings allow anything, including unicode.
	val2 := ScvString("héllo")
	require.Equal(t, "héllo", string(*val2.Str))
}

func TestScvBool(t *testing.T) {
	tv := ScvBool(true)
	require.Equal(t, ScValTypeScvBool, tv.Type)
	require.True(t, *tv.B)

	fv := ScvBool(false)
	require.False(t, *fv.B)
}

func TestScvBytes(t *testing.T) {
	b := []byte{0xde, 0xad, 0xbe, 0xef}
	val := ScvBytes(b)
	require.Equal(t, ScValTypeScvBytes, val.Type)
	require.Equal(t, b, []byte(*val.Bytes))
}

func TestScvVec(t *testing.T) {
	val := ScvVec(ScvBool(true), ScvU128FromUint64(7))
	require.Equal(t, ScValTypeScvVec, val.Type)
	require.NotNil(t, val.Vec)
	require.Len(t, **val.Vec, 2)

	// Empty vec is well-formed.
	empty := ScvVec()
	require.NotNil(t, empty.Vec)
	require.Len(t, **empty.Vec, 0)
}

func TestScvMap_DeterministicOrdering(t *testing.T) {
	in := map[string]ScVal{
		"zeta":  ScvBool(true),
		"alpha": ScvU128FromUint64(1),
		"mu":    ScvString("middle"),
	}
	val, err := ScvMap(in)
	require.NoError(t, err)
	require.Equal(t, ScValTypeScvMap, val.Type)

	entries := **val.Map
	require.Len(t, entries, 3)
	require.Equal(t, "alpha", string(*entries[0].Key.Sym))
	require.Equal(t, "mu", string(*entries[1].Key.Sym))
	require.Equal(t, "zeta", string(*entries[2].Key.Sym))

	// Building twice yields byte-identical encoding.
	val2, err := ScvMap(in)
	require.NoError(t, err)
	a, err := val.MarshalBinary()
	require.NoError(t, err)
	b, err := val2.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, a, b)
}

func TestScvMap_InvalidKey(t *testing.T) {
	_, err := ScvMap(map[string]ScVal{"bad key": ScvBool(true)})
	require.Error(t, err)
}
