package xdr

import (
	"math/big"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// String returns a string representation of `p`
func (p Price) String() string {
	return big.NewRat(int64(p.N), int64(p.D)).FloatString(7)
}

// TryString returns a string representation of `p`.
// Returns an error if `p` is invalid.
func (p Price) TryString() (string, error) {
	if err := p.Validate(); err != nil {
		return "", errors.Wrap(err, "invalid price")
	}
	return p.String(), nil
}

// Equal returns whether the price's value is the same,
// taking into account denormalized representation
// (e.g. Price{1, 2}.EqualValue(Price{2,4}) == true )
func (p Price) Equal(q Price) bool {
	// See the Cheaper() method for the reasoning behind this:
	return uint64(p.N)*uint64(q.D) == uint64(q.N)*uint64(p.D)
}

// TryEqual returns whether the price's value is the same,
// taking into account denormalized representation
// (e.g. Price{1, 2}.EqualValue(Price{2,4}) == true )
// Returns an error if either price is invalid.
func (p Price) TryEqual(q Price) (bool, error) {
	if err := p.Validate(); err != nil {
		return false, errors.Wrap(err, "invalid price p")
	}
	if err := q.Validate(); err != nil {
		return false, errors.Wrap(err, "invalid price q")
	}
	return p.Equal(q), nil
}

// Cheaper indicates if the Price's value is lower,
// taking into account denormalized representation
// (e.g. Price{1, 2}.Cheaper(Price{2,4}) == false )
func (p Price) Cheaper(q Price) bool {
	// To avoid float precision issues when naively comparing Price.N/Price.D,
	// we use the cross product instead:
	//
	// Price of p <  Price of q
	//  <==>
	// (p.N / p.D) < (q.N / q.D)
	//  <==>
	// (p.N / p.D) * (p.D * q.D) < (q.N / q.D) * (p.D * q.D)
	//  <==>
	// p.N * q.D < q.N * p.D
	return uint64(p.N)*uint64(q.D) < uint64(q.N)*uint64(p.D)
}

// TryCheaper indicated if the Price's value is lower,
// taking into account denormalized representation.
// (e.g. Price{1, 2}.Cheaper(Price{2,4}) == false )
// Returns an error if either price is invalid
func (p Price) TryCheaper(q Price) (bool, error) {
	if err := p.Validate(); err != nil {
		return false, errors.Wrap(err, "invalid price p")
	}
	if err := q.Validate(); err != nil {
		return false, errors.Wrap(err, "invalid price q")
	}
	return p.Cheaper(q), nil
}

// Normalize sets Price to its rational canonical form
func (p *Price) Normalize() {
	r := big.NewRat(int64(p.N), int64(p.D))
	p.N = Int32(r.Num().Int64())
	p.D = Int32(r.Denom().Int64())
}

// TryNormalize sets the price to its rational canonical form.
// Returns an error if the price is invalid
func (p *Price) TryNormalize() error {
	if err := p.Validate(); err != nil {
		return errors.Wrap(err, "invalid price")
	}
	p.Normalize()
	return nil
}

// Invert inverts Price.
func (p *Price) Invert() {
	if err := p.Validate(); err != nil {
		return
	}
	p.N, p.D = p.D, p.N
}

// TryInvert inverts Price.
// Returns an error if the price is invalid
func (p *Price) TryInvert() error {
	if err := p.Validate(); err != nil {
		return errors.Wrap(err, "invalid price")
	}
	p.Invert()
	return nil
}

// Validate checks if the price is valid and returns an error if not.
func (p Price) Validate() error {
	if p.N == 0 {
		return errors.Errorf("price cannot be 0: %d/%d", p.N, p.D)
	}
	if p.D == 0 {
		return errors.Errorf("price denominator cannot be 0: %d/%d", p.N, p.D)
	}
	if p.N < 0 || p.D < 0 {
		return errors.Errorf("price cannot be negative: %d/%d", p.N, p.D)
	}
	return nil
}
