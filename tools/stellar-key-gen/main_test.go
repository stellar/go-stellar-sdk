package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stretchr/testify/assert"
)

func TestRun_defaultFormat(t *testing.T) {
	args := []string{}
	stdout := strings.Builder{}
	stderr := strings.Builder{}

	exitCode := run(args, &stdout, &stderr)

	t.Logf("exit code: %d", exitCode)
	t.Logf("stdout: %q", stdout.String())
	t.Logf("stderr: %q", stderr.String())

	// Exit code should be zero for success.
	assert.Equal(t, 0, exitCode)

	// Stdout should be public key then secret key new line separated.
	lines := strings.Split(stdout.String(), "\n")
	if assert.Len(t, lines, 3) {
		f, err := keypair.ParseFull(lines[1])
		if assert.NoError(t, err) {
			assert.Equal(t, f.Address(), lines[0])
			assert.Equal(t, f.Seed(), lines[1])
			assert.Equal(t, "", lines[2])
		}
	}

	// Stderr should be empty.
	assert.Equal(t, "", stderr.String())
}

func TestRun_customFormat(t *testing.T) {
	args := []string{
		"-f",
		"{{.SecretKey}},{{.PublicKey}}",
	}
	stdout := strings.Builder{}
	stderr := strings.Builder{}

	exitCode := run(args, &stdout, &stderr)

	t.Logf("exit code: %d", exitCode)
	t.Logf("stdout: %q", stdout.String())
	t.Logf("stderr: %q", stderr.String())

	// Exit code should be zero for success.
	assert.Equal(t, 0, exitCode)

	// Stdout should be secret key then public key comma separated.
	parts := strings.Split(stdout.String(), ",")
	if assert.Len(t, parts, 2) {
		f, err := keypair.ParseFull(parts[0])
		if assert.NoError(t, err) {
			assert.Equal(t, f.Seed(), parts[0])
			assert.Equal(t, f.Address(), parts[1])
		}
	}

	// Stderr should be empty.
	assert.Equal(t, "", stderr.String())
}

func TestRun_invalidFormat(t *testing.T) {
	args := []string{
		"-f",
		"{{.FooBar}}",
	}
	stdout := strings.Builder{}
	stderr := strings.Builder{}

	exitCode := run(args, &stdout, &stderr)

	t.Logf("exit code: %d", exitCode)
	t.Logf("stdout: %q", stdout.String())
	t.Logf("stderr: %q", stderr.String())

	// Exit code should be one for failure.
	assert.Equal(t, 1, exitCode)

	// Stdout should be empty.
	assert.Equal(t, "", stdout.String())

	// Stderr should contain the error.
	assert.Contains(t, stderr.String(), "can't evaluate field FooBar")
}

func TestRun_random(t *testing.T) {
	args := []string{
		"-f",
		"{{.SecretKey}}",
	}
	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			stdout := strings.Builder{}
			stderr := strings.Builder{}

			exitCode := run(args, &stdout, &stderr)

			// Exit code should be zero for success.
			assert.Equal(t, 0, exitCode)

			// Stdout will contain the secret, which should not have be seen before.
			key := stdout.String()
			if seen[key] {
				t.Error(key, "seen before")
			} else {
				t.Log(key, "not seen before")
			}

			// Stderr should be empty.
			assert.Equal(t, "", stderr.String())
		})
	}
}
