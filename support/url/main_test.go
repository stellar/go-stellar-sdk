package url

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetUrlParam(t *testing.T) {
	testCases := []struct {
		input    string
		key      string
		val      string
		expected string
	}{
		{"http://localhost/a", "x", "1", "http://localhost/a?x=1"},
		{"http://localhost/a?x=1", "y", "2", "http://localhost/a?x=1&y=2"},
		{"http://localhost/a?x=1", "x", "2", "http://localhost/a?x=2"},
	}

	for _, kase := range testCases {
		u, err := Parse(kase.input)
		if assert.NoError(t, err) {
			actual := u.SetParam(kase.key, kase.val).String()
			assert.Equal(t, kase.expected, actual)
		}
	}
}
