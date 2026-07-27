package xdr

// scanSeq2 adapts a scanner's Next/Cur/Err methods to the two-variable range
// shape the pre-Scan tests were written against (in-band error yielded once
// at the point iteration stops).
import "iter"

func scanSeq2[E any](next func() bool, cur func() E, errFn func() error) iter.Seq2[E, error] {
	return func(yield func(E, error) bool) {
		for next() {
			if !yield(cur(), nil) {
				return
			}
		}
		if err := errFn(); err != nil {
			var zero E
			yield(zero, err)
		}
	}
}
