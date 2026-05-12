package loadtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePreBenchmarkCheckpoint(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    uint32
		wantErr bool
	}{
		{
			name:   "bare line",
			output: "Published final checkpoint before benchmark: ledger 1234\n",
			want:   1234,
		},
		{
			name: "embedded in multi-line log output",
			output: strings.Join([]string{
				"2026-05-05T12:34:56 [INFO] starting apply-load",
				"2026-05-05T12:35:00 [INFO] Published final checkpoint before benchmark: ledger 42",
				"2026-05-05T12:35:01 [INFO] benchmark phase begins",
			}, "\n"),
			want: 42,
		},
		{
			name:    "non-numeric ledger",
			output:  "Published final checkpoint before benchmark: ledger abc",
			wantErr: true,
		},
		{
			name:    "empty input",
			output:  "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePreBenchmarkCheckpoint(tt.output)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseConfig_DefaultCfg pins the parsed shape of the default config
// shipped with this package. Updating default-apply-load.cfg without
// updating the expected values here will fail this test.
func TestParseConfig_DefaultCfg(t *testing.T) {
	got, err := parseConfig("testdata/default-apply-load.cfg")
	require.NoError(t, err)
	assert.Equal(t, applyLoadConfig{
		NetworkPassphrase:    "load test network",
		MetadataOutputStream: "meta.xdr",
		HistoryArchiveName:   "local",
	}, got)
}

func TestParseConfig_Errors(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		wantErr  string // substring; empty means expect any non-nil error
	}{
		{
			name: "missing NETWORK_PASSPHRASE",
			contents: `
METADATA_OUTPUT_STREAM = "metadata.xdr"
[HISTORY.local]
get   = "cp history/{0} {1}"
`,
			wantErr: "NETWORK_PASSPHRASE",
		},
		{
			name: "missing METADATA_OUTPUT_STREAM",
			contents: `
NETWORK_PASSPHRASE = "x"
[HISTORY.local]
get   = "cp history/{0} {1}"
`,
			wantErr: "METADATA_OUTPUT_STREAM",
		},
		{
			name: "missing HISTORY section",
			contents: `
NETWORK_PASSPHRASE = "x"
METADATA_OUTPUT_STREAM = "metadata.xdr"
`,
			wantErr: "HISTORY",
		},
		{
			name: "multiple history archives rejected",
			contents: `
NETWORK_PASSPHRASE = "x"
METADATA_OUTPUT_STREAM = "metadata.xdr"
[HISTORY.local]
get = "cp history/{0} {1}"
[HISTORY.backup]
get = "cp history/{0} {1}"
`,
			wantErr: "exactly one history archive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "apply-load.cfg")
			require.NoError(t, os.WriteFile(path, []byte(tt.contents), 0o644))

			_, err := parseConfig(path)
			require.Error(t, err)
			if tt.wantErr != "" {
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseConfig_FileNotFound(t *testing.T) {
	_, err := parseConfig(filepath.Join(t.TempDir(), "does-not-exist.cfg"))
	require.Error(t, err)
}
