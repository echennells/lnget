package cli

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockPhraseReader is a test implementation of PhraseReader that
// returns preconfigured values for each method.
type mockPhraseReader struct {
	terminal    bool
	line        string
	lineErr     error
	password    []byte
	passwordErr error
	envVars     map[string]string
}

// IsTerminal reports whether the mock is configured as a terminal.
func (m *mockPhraseReader) IsTerminal() bool {
	return m.terminal
}

// ReadLine returns the preconfigured line or error.
func (m *mockPhraseReader) ReadLine() (string, error) {
	return m.line, m.lineErr
}

// ReadPassword returns the preconfigured password bytes or error.
func (m *mockPhraseReader) ReadPassword() ([]byte, error) {
	return m.password, m.passwordErr
}

// Getenv returns the value from the mock's env var map.
func (m *mockPhraseReader) Getenv(key string) string {
	return m.envVars[key]
}

// TestResolvePairingPhrase tests the phrase resolution priority and
// error handling across all input sources.
func TestResolvePairingPhrase(t *testing.T) {
	t.Parallel()

	devNull, err := os.Open(os.DevNull)
	require.NoError(t, err)

	t.Cleanup(func() { _ = devNull.Close() })

	tests := []struct {
		name       string
		args       []string
		fromStdin  bool
		reader     *mockPhraseReader
		wantPhrase string
		wantErr    string
	}{
		{
			name:       "positional argument",
			args:       []string{"alpha bravo charlie"},
			reader:     &mockPhraseReader{},
			wantPhrase: "alpha bravo charlie",
		},
		{
			name:       "positional argument preserves internal spaces",
			args:       []string{"  alpha bravo charlie  "},
			reader:     &mockPhraseReader{},
			wantPhrase: "  alpha bravo charlie  ",
		},
		{
			name:      "stdin piped input",
			fromStdin: true,
			reader: &mockPhraseReader{
				line: "delta echo foxtrot",
			},
			wantPhrase: "delta echo foxtrot",
		},
		{
			name:      "stdin piped trims whitespace",
			fromStdin: true,
			reader: &mockPhraseReader{
				line: "  delta echo foxtrot  ",
			},
			wantPhrase: "delta echo foxtrot",
		},
		{
			name:      "stdin piped read error",
			fromStdin: true,
			reader: &mockPhraseReader{
				lineErr: fmt.Errorf("%s", "broken pipe"),
			},
			wantErr: "broken pipe",
		},
		{
			name:      "stdin terminal secure input",
			fromStdin: true,
			reader: &mockPhraseReader{
				terminal: true,
				password: []byte("golf hotel india"),
			},
			wantPhrase: "golf hotel india",
		},
		{
			name:      "stdin terminal trims whitespace",
			fromStdin: true,
			reader: &mockPhraseReader{
				terminal: true,
				password: []byte("  golf hotel india  "),
			},
			wantPhrase: "golf hotel india",
		},
		{
			name:      "stdin terminal read error",
			fromStdin: true,
			reader: &mockPhraseReader{
				terminal:    true,
				passwordErr: fmt.Errorf("%s", "terminal reset"),
			},
			wantErr: "failed to read pairing phrase",
		},
		{
			name: "env var fallback",
			reader: &mockPhraseReader{
				envVars: map[string]string{
					"LNGET_LN_LNC_PAIRING_PHRASE": "juliet kilo lima",
				},
			},
			wantPhrase: "juliet kilo lima",
		},
		{
			name: "positional takes priority over env var",
			args: []string{"from args"},
			reader: &mockPhraseReader{
				envVars: map[string]string{
					"LNGET_LN_LNC_PAIRING_PHRASE": "from env",
				},
			},
			wantPhrase: "from args",
		},
		{
			name:      "stdin takes priority over env var",
			fromStdin: true,
			reader: &mockPhraseReader{
				line: "from stdin",
				envVars: map[string]string{
					"LNGET_LN_LNC_PAIRING_PHRASE": "from env",
				},
			},
			wantPhrase: "from stdin",
		},
		{
			name:    "no source provided",
			reader:  &mockPhraseReader{},
			wantErr: "pairing phrase required",
		},
		{
			name: "empty env var treated as missing",
			reader: &mockPhraseReader{
				envVars: map[string]string{
					"LNGET_LN_LNC_PAIRING_PHRASE": "",
				},
			},
			wantErr: "pairing phrase required",
		},
		{
			name:      "stdin piped empty line",
			fromStdin: true,
			reader:    &mockPhraseReader{},
			wantErr:   "pairing phrase required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolvePairingPhrase(
				tc.args, tc.fromStdin, tc.reader,
				devNull,
			)

			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantPhrase, got)
		})
	}
}
