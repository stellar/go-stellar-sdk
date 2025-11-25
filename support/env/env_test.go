package env_test

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/support/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// randomEnvName generates a random name for an environment variable. Calls
// t.Fatal if sufficient randomness is unavailable.
func randomEnvName(t *testing.T) string {
	raw := make([]byte, 5)
	_, err := rand.Read(raw)
	require.NoError(t, err)
	return t.Name() + "_" + hex.EncodeToString(raw)
}

// TestString_set tests that env.String will return the value of the
// environment variable when the environment variable is set.
func TestString_set(t *testing.T) {
	envVar := randomEnvName(t)
	err := os.Setenv(envVar, "value")
	require.NoError(t, err)
	defer os.Unsetenv(envVar)

	value := env.String(envVar, "default")
	assert.Equal(t, "value", value)
}

// TestString_set tests that env.String will return the default value given
// when the environment variable is not set.
func TestString_notSet(t *testing.T) {
	envVar := randomEnvName(t)
	value := env.String(envVar, "default")
	assert.Equal(t, "default", value)
}

// TestInt_set tests that env.Int will return the value of the environment
// variable as an int when the environment variable is set.
func TestInt_set(t *testing.T) {
	envVar := randomEnvName(t)
	err := os.Setenv(envVar, "12345")
	require.NoError(t, err)
	defer os.Unsetenv(envVar)

	value := env.Int(envVar, 67890)
	assert.Equal(t, 12345, value)
}

// TestInt_set tests that env.Int will return the default value given when the
// environment variable is not set.
func TestInt_notSet(t *testing.T) {
	envVar := randomEnvName(t)
	value := env.Int(envVar, 67890)
	assert.Equal(t, 67890, value)
}

// TestInt_setInvalid tests that env.Int will panic if the set value cannot be
// parsed as an integer.
func TestInt_setInvalid(t *testing.T) {
	envVar := randomEnvName(t)
	err := os.Setenv(envVar, "1a345")
	require.NoError(t, err)
	defer os.Unsetenv(envVar)

	wantPanic := errors.New(`env var "` + envVar + `" cannot be parsed as int: strconv.Atoi: parsing "1a345": invalid syntax`)
	defer func() {
		r := recover()
		assert.Error(t, wantPanic, r)
	}()
	env.Int(envVar, 67890)
}

// TestDuration_set tests that env.Duration will return the value of the
// environment variable as a time.Duration when the environment variable is
// set to a duration string.
func TestDuration_set(t *testing.T) {
	envVar := randomEnvName(t)
	err := os.Setenv(envVar, "5m30s")
	require.NoError(t, err)
	defer os.Unsetenv(envVar)

	setValue := 5*time.Minute + 30*time.Second
	defaultValue := 2 * time.Minute
	value := env.Duration(envVar, defaultValue)
	assert.Equal(t, setValue, value)
}

// TestDuration_set tests that env.Duration will return the default value given
// when the environment variable is not set.
func TestDuration_notSet(t *testing.T) {
	envVar := randomEnvName(t)
	defaultValue := 5*time.Minute + 30*time.Second
	value := env.Duration(envVar, defaultValue)
	assert.Equal(t, defaultValue, value)
}

// TestDuration_setInvalid tests that env.Duration will panic if the value set
// cannot be parsed as a duration.
func TestDuration_setInvalid(t *testing.T) {
	envVar := randomEnvName(t)
	err := os.Setenv(envVar, "5q30s")
	require.NoError(t, err)
	defer os.Unsetenv(envVar)

	wantPanic := errors.New(`env var "` + envVar + `" cannot be parsed as time.Duration: time: unknown unit q in duration 5q30s`)
	defer func() {
		r := recover()
		assert.Error(t, wantPanic, r)
	}()
	env.Duration(envVar, 5*time.Minute+30*time.Second)
}
