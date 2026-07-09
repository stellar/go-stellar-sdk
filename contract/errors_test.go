package contract

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorKindString(t *testing.T) {
	cases := []struct {
		kind ErrorKind
		want string
	}{
		{KindUnknown, "unknown"},
		{KindSimulationFailed, "simulation_failed"},
		{KindSubmissionFailed, "submission_failed"},
		{KindTimeout, "timeout"},
		{KindNotYetSimulated, "not_yet_simulated"},
		{KindInvalidArgs, "invalid_args"},
		{KindTransactionFailed, "transaction_failed"},
		{KindSourceAccountFailed, "source_account_failed"},
		{ErrorKind(999), "unknown"}, // out-of-range falls through to default
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.kind.String(), "kind %d", int(c.kind))
	}
}

func TestErrorErrorMessage(t *testing.T) {
	cause := errors.New("boom")
	cases := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "details and cause",
			err:  &Error{Kind: KindSimulationFailed, Details: "encode tx", cause: cause},
			want: "contract: simulation_failed: encode tx: boom",
		},
		{
			name: "details only",
			err:  &Error{Kind: KindInvalidArgs, Details: "bad input"},
			want: "contract: invalid_args: bad input",
		},
		{
			name: "cause only",
			err:  &Error{Kind: KindTimeout, cause: cause},
			want: "contract: timeout: boom",
		},
		{
			name: "neither",
			err:  &Error{Kind: KindNotYetSimulated},
			want: "contract: not_yet_simulated",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.err.Error())
		})
	}
}

func TestErrorNilReceiver(t *testing.T) {
	var e *Error
	assert.Equal(t, "<nil>", e.Error())
	assert.Nil(t, e.Unwrap())
	assert.False(t, e.Is(ErrInvalidArgs))
}

func TestErrorUnwrap(t *testing.T) {
	cause := errors.New("rpc down")
	err := &Error{Kind: KindSimulationFailed, cause: cause}

	assert.Equal(t, cause, err.Unwrap())
	assert.Equal(t, cause, errors.Unwrap(err))
	assert.True(t, errors.Is(err, cause), "wrapped cause must be reachable via errors.Is")
}

func TestErrorIs(t *testing.T) {
	t.Run("same kind matches sentinel", func(t *testing.T) {
		err := &Error{Kind: KindInvalidArgs, Details: "x"}
		assert.True(t, errors.Is(err, ErrInvalidArgs))
	})
	t.Run("different kind does not match", func(t *testing.T) {
		err := &Error{Kind: KindTimeout}
		assert.False(t, errors.Is(err, ErrInvalidArgs))
	})
	t.Run("nil target", func(t *testing.T) {
		err := &Error{Kind: KindInvalidArgs}
		assert.False(t, err.Is(nil))
	})
	t.Run("non-*Error target", func(t *testing.T) {
		err := &Error{Kind: KindInvalidArgs}
		assert.False(t, err.Is(errors.New("plain")))
	})
	t.Run("matches through a wrapped cause", func(t *testing.T) {
		cause := errors.New("rpc down")
		err := &Error{Kind: KindSimulationFailed, cause: cause}
		assert.True(t, errors.Is(err, ErrSimulationFailed))
		assert.True(t, errors.Is(err, cause))
	})
	t.Run("same kind, distinct instances are mutually Is-equal", func(t *testing.T) {
		// Documents the intended classifier semantics: Is matches on Kind, so
		// two distinct same-kind errors cannot be told apart via errors.Is.
		a := &Error{Kind: KindInvalidArgs, Details: "first"}
		b := &Error{Kind: KindInvalidArgs, Details: "second"}
		assert.True(t, errors.Is(a, b))
	})
}

func TestInvalidArgsf(t *testing.T) {
	err := invalidArgsf("method %q: %d", "transfer", 7)

	assert.Equal(t, KindInvalidArgs, err.Kind)
	assert.Equal(t, `method "transfer": 7`, err.Details)
	assert.True(t, errors.Is(err, ErrInvalidArgs))

	var ce *Error
	require.True(t, errors.As(error(err), &ce))
	assert.Equal(t, KindInvalidArgs, ce.Kind)
}
