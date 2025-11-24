package keypair_test

import (
	"crypto/rand"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stretchr/testify/require"
)

func BenchmarkFromAddress_ParseAddress(b *testing.B) {
	// Secret key for setting up the components.
	sk := keypair.MustRandom()
	address := sk.Address()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = keypair.ParseAddress(address)
	}
}

func BenchmarkFromAddress_FromAddress(b *testing.B) {
	// Secret key for setting up the components.
	sk := keypair.MustRandom()

	// Public key that'll be used during the benchmark.
	pk := sk.FromAddress()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pk.FromAddress()
	}
}

func BenchmarkFromAddress_Hint(b *testing.B) {
	// Secret key for setting up the components.
	sk := keypair.MustRandom()

	// Public key that'll be used during the benchmark.
	pk := sk.FromAddress()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pk.Hint()
	}
}

func BenchmarkFromAddress_Verify(b *testing.B) {
	// Secret key for setting up the components.
	sk := keypair.MustRandom()

	// Random input for creating a valid signature.
	input := [32]byte{}
	_, err := rand.Read(input[:])
	require.NoError(b, err)

	// Valid signature to use for verification.
	sig, err := sk.Sign(input[:])
	require.NoError(b, err)

	// Public key that'll be used during the benchmark.
	pk := sk.FromAddress()

	// Double check that the function succeeds without error when run with these
	// inputs to ensure the benchmark is a fair benchmark.
	err = pk.Verify(input[:], sig)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pk.Verify(input[:], sig)
	}
}

func BenchmarkFull_ParseFull(b *testing.B) {
	// Secret key for setting up the components.
	sk := keypair.MustRandom()
	address := sk.Address()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = keypair.ParseFull(address)
	}
}

func BenchmarkFull_FromRawSeed(b *testing.B) {
	rawSeed := [32]byte{}
	_, err := rand.Read(rawSeed[:])
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = keypair.FromRawSeed(rawSeed)
	}
}

func BenchmarkFull_FromAddress(b *testing.B) {
	// Secret key for setting up the components.
	sk := keypair.MustRandom()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sk.FromAddress()
	}
}

func BenchmarkFull_Hint(b *testing.B) {
	// Secret key for setting up the components.
	sk := keypair.MustRandom()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sk.Hint()
	}
}

func BenchmarkFull_Verify(b *testing.B) {
	// Secret key for setting up the components.
	sk := keypair.MustRandom()

	// Random input for creating a valid signature.
	input := [32]byte{}
	_, err := rand.Read(input[:])
	require.NoError(b, err)

	// Valid signature to use for verification.
	sig, err := sk.Sign(input[:])
	require.NoError(b, err)

	// Double check that the function succeeds without error when run with these
	// inputs to ensure the benchmark is a fair benchmark.
	err = sk.Verify(input[:], sig)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sk.Verify(input[:], sig)
	}
}

func BenchmarkFull_Sign(b *testing.B) {
	// Secret key for setting up the components.
	sk := keypair.MustRandom()

	// Random input for creating a valid signature.
	input := [32]byte{}
	_, err := rand.Read(input[:])
	require.NoError(b, err)

	// Double check that the function succeeds without error when run with these
	// inputs to ensure the benchmark is a fair benchmark.
	_, err = sk.Sign(input[:])
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sk.Sign(input[:])
	}
}
