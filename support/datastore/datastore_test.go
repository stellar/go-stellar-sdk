package datastore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInvalidStore(t *testing.T) {
	_, err := NewDataStore(context.Background(), DataStoreConfig{Type: "unknown"})
	require.Error(t, err)
}

func TestParsePrefix(t *testing.T) {
	// urlPath is what url.Parse returns for "scheme://bucket<path>".
	ok := map[string]string{
		"":                  "",
		"/":                 "",
		"/a/b":              "a/b",
		"/a/b/":             "a/b",
		"/objects/testnet/": "objects/testnet",
	}
	for in, want := range ok {
		got, err := parsePrefix("bucket"+in, in)
		require.NoError(t, err, "input %q", in)
		require.Equal(t, want, got, "input %q", in)
	}

	for _, in := range []string{"//", "//a", "/a//", "/a//b", "/./a", "/a/./b", "/a/../b", "/a/b/.", "/a/..", "/.", "/.."} {
		_, err := parsePrefix("bucket"+in, in)
		require.Error(t, err, "input %q", in)
	}
}
