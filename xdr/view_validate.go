package xdr

import (
	"encoding/base64"
	"fmt"
)

// ValidateExact checks that v holds exactly one well-formed value: it is
// ValidateFull, and it also fails if any bytes follow the value. It walks v
// once.
func ValidateExact[T View](v T) error {
	n, err := v.valid(0)
	if err != nil {
		return err
	}
	if n != len(v) {
		return fmt.Errorf("xdr view: %d bytes after the value", len(v)-n)
	}
	return nil
}

// ViewFromBase64 base64-decodes s and returns the bytes as a T, failing
// unless they hold exactly one well-formed T. It checks them with
// ValidateExact, without unmarshaling them into the decoded type.
func ViewFromBase64[T View](s string) (T, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if err = ValidateExact(T(raw)); err != nil {
		return nil, err
	}
	return T(raw), nil
}
