package xdr

import (
	"fmt"
	"math/big"
)

// String returns a string representation of `p` if `p` is valid,
// and string indicating the price's invalidity otherwise.
// Satisfies the fmt.Stringer interface.
func (p Price) String() string {
	if err := p.Validate(); err != nil {
		return fmt.Sprintf("<invalid price (%d/%d): %v>", p.N, p.D, err)
	}
	return big.NewRat(int64(p.N), int64(p.D)).FloatString(7)
}

// TryEqual returns whether the price's value is the same,
// taking into account denormalized representation
// (e.g. Price{1, 2}.EqualValue(Price{2,4}) == true )
// Returns an error if either price is invalid.
func (p Price) TryEqual(q Price) (bool, error) {
	if err := p.Validate(); err != nil {
		return false, fmt.Errorf("invalid price p: %w", err)
	}
	if err := q.Validate(); err != nil {
		return false, fmt.Errorf("invalid price q: %w", err)
	}
	// See the TryCheaper() method for the reasoning behind this:
	return uint64(p.N)*uint64(q.D) == uint64(q.N)*uint64(p.D), nil
}

// TryCheaper indicates if the Price's value is lower,
// taking into account denormalized representation.
// (e.g. Price{1, 2}.Cheaper(Price{2,4}) == false )
// Returns an error if either price is invalid
func (p Price) TryCheaper(q Price) (bool, error) {
	if err := p.Validate(); err != nil {
		return false, fmt.Errorf("invalid price p: %w", err)
	}
	if err := q.Validate(); err != nil {
		return false, fmt.Errorf("invalid price q: %w", err)
	}
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
	return uint64(p.N)*uint64(q.D) < uint64(q.N)*uint64(p.D), nil
}

// TryNormalize sets the price to its rational canonical form.
// Returns an error if the price is invalid
func (p *Price) TryNormalize() error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("invalid price: %w", err)
	}
	r := big.NewRat(int64(p.N), int64(p.D))
	p.N = Int32(r.Num().Int64())
	p.D = Int32(r.Denom().Int64())
	return nil
}

// TryInvert inverts Price.
// Returns an error if the price is invalid
func (p *Price) TryInvert() error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("invalid price: %w", err)
	}
	p.N, p.D = p.D, p.N
	return nil
}

// Validate checks if the price is valid and returns an error if not.
func (p Price) Validate() error {
	if p.N == 0 {
		return fmt.Errorf("price cannot be 0: %d/%d", p.N, p.D)
	}
	if p.D == 0 {
		return fmt.Errorf("price denominator cannot be 0: %d/%d", p.N, p.D)
	}
	if p.N < 0 || p.D < 0 {
		return fmt.Errorf("price cannot be negative: %d/%d", p.N, p.D)
	}
	return nil
}

// Equal returns whether the price's value is the same,
// taking into account denormalized representation
// (e.g. Price{1, 2}.EqualValue(Price{2,4}) == true ).
// It does not validate the prices and may produce incorrect results for invalid inputs.
//
// Deprecated: Use TryEqual instead, which returns an error for invalid prices.
func (p Price) Equal(q Price) bool {
	return uint64(p.N)*uint64(q.D) == uint64(q.N)*uint64(p.D)
}

// Cheaper indicates if the Price's value is lower,
// taking into account denormalized representation
// (e.g. Price{1, 2}.Cheaper(Price{2,4}) == false ).
// It does not validate the prices and may produce incorrect results for invalid inputs.
//
// Deprecated: Use TryCheaper instead, which returns an error for invalid prices.
func (p Price) Cheaper(q Price) bool {
	return uint64(p.N)*uint64(q.D) < uint64(q.N)*uint64(p.D)
}

// Normalize sets Price to its rational canonical form.
// It panics if the price denominator is zero.
//
// Deprecated: Use TryNormalize instead, which returns an error for invalid prices.
func (p *Price) Normalize() {
	r := big.NewRat(int64(p.N), int64(p.D))
	p.N = Int32(r.Num().Int64())
	p.D = Int32(r.Denom().Int64())
}

// Invert inverts Price.
// It may set a Price with zero denominator if the original price's numerator is zero.
//
// Deprecated: Use TryInvert instead, which returns an error for invalid prices.
func (p *Price) Invert() {
	p.N, p.D = p.D, p.N
}
