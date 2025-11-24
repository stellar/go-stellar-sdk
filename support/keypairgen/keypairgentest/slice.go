package keypairgentest

import (
	"github.com/stellar/go-stellar-sdk/keypair"
)

// SliceSource is a keypairgen.Generator source that has the values returned
// from a slice of keys that are provided at generation one at a time.
type SliceSource []*keypair.Full

// Generate returns the first key in the slice, and then shortens the slice
// removing the returned key, so that each call returns the next key in the
// original source. If called when no keys are available the function will
// panic.
func (s *SliceSource) Generate() (*keypair.Full, error) {
	kp := (*s)[0]
	*s = (*s)[1:]
	return kp, nil
}
