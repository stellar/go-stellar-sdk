package txnbuild

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/keypair"
)

func TestBaseFeeCanBeZeroOrPositive(t *testing.T) {
	validBaseFees := []int64{0, MinBaseFee}
	for _, bf := range validBaseFees {
		tx, err := NewTransaction(
			TransactionParams{
				SourceAccount: &SimpleAccount{keypair.MustRandom().Address(), 1},
				Operations:    []Operation{&Inflation{}},
				BaseFee:       bf,
				Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
			},
		)
		assert.NoError(t, err)
		assert.Equal(t, bf, tx.baseFee)
	}
}

func TestBaseFeeErrorWhenNegative(t *testing.T) {
	_, err := NewTransaction(
		TransactionParams{
			SourceAccount: &SimpleAccount{keypair.MustRandom().Address(), 1},
			Operations:    []Operation{&Inflation{}},
			BaseFee:       -1,
			Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.EqualError(t, err, "base fee cannot be negative")
}

func TestFeeBumpMinBaseFee(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), 1)

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount: &sourceAccount,
			Operations:    []Operation{&Inflation{}},
			BaseFee:       MinBaseFee,
			Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)
	tx.baseFee -= 2

	_, err = NewFeeBumpTransaction(
		FeeBumpTransactionParams{
			FeeAccount: newKeypair1().Address(),
			BaseFee:    MinBaseFee - 1,
			Inner:      tx,
		},
	)
	assert.EqualError(t, err, "base fee cannot be lower than network minimum of 100")
}

func TestFeeOverflow(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), 1)

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount: &sourceAccount,
			Operations:    []Operation{&Inflation{}, &Inflation{}},
			BaseFee:       math.MaxUint32 / 2,
			Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	_, err = NewTransaction(
		TransactionParams{
			SourceAccount: &sourceAccount,
			Operations:    []Operation{&Inflation{}, &Inflation{}, &Inflation{}},
			BaseFee:       math.MaxUint32 / 2,
			Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.EqualError(t, err, "base fee 2147483647 results in an overflow of max fee")
}

func TestFeeBumpOverflow(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), 1)

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount: &sourceAccount,
			Operations:    []Operation{&Inflation{}},
			BaseFee:       MinBaseFee,
			Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	_, err = NewFeeBumpTransaction(
		FeeBumpTransactionParams{
			FeeAccount: newKeypair1().Address(),
			BaseFee:    math.MaxInt64 / 2,
			Inner:      tx,
		},
	)
	assert.NoError(t, err)

	_, err = NewFeeBumpTransaction(
		FeeBumpTransactionParams{
			FeeAccount: newKeypair1().Address(),
			BaseFee:    math.MaxInt64,
			Inner:      tx,
		},
	)
	assert.EqualError(t, err, "base fee 9223372036854775807 results in an overflow of max fee")
}

func TestFeeBumpFeeGreaterThanOrEqualInner(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), 1)

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount: &sourceAccount,
			Operations:    []Operation{&Inflation{}},
			BaseFee:       2 * MinBaseFee,
			Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	_, err = NewFeeBumpTransaction(
		FeeBumpTransactionParams{
			FeeAccount: newKeypair1().Address(),
			BaseFee:    2 * MinBaseFee,
			Inner:      tx,
		},
	)
	assert.NoError(t, err)

	_, err = NewFeeBumpTransaction(
		FeeBumpTransactionParams{
			FeeAccount: newKeypair1().Address(),
			BaseFee:    2*MinBaseFee - 1,
			Inner:      tx,
		},
	)
	assert.EqualError(t, err, "base fee cannot be lower than provided inner transaction fee")
}
