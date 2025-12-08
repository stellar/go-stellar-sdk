// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

type Hash xdr.Hash

func DecodeHash(s string) (Hash, error) {
	var h Hash
	hs, err := hex.DecodeString(s)
	if err != nil {
		return h, err
	}
	if len(hs) != sha256.Size {
		return h, fmt.Errorf("unexpected hash size: %d", len(hs))
	}
	n := copy(h[:], hs)
	if n != sha256.Size {
		return h, fmt.Errorf("copy() returned unexpected count: %d", n)
	}
	return h, nil
}

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

func MustDecodeHash(s string) Hash {
	h, e := DecodeHash(s)
	if e != nil {
		panic(e)
	}
	return h
}

func EmptyXdrArrayHash() Hash {
	// This is the hash of [0,0,0,0], the value wire-entry you get when you write an
	// XDR variable-length array with zero entries. This appears as a hash in a
	// few contexts we can't easily detect in other ways.
	return MustDecodeHash("df3f619804a92fdb4057192dc43dd748ea778adc52bc498ce80524c014b81119")
}

func (h Hash) IsZero() bool {
	for _, n := range h {
		if n != 0 {
			return false
		}
	}
	return true
}
