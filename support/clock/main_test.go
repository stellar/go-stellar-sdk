package clock_test

import (
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/support/clock"
	"github.com/stellar/go-stellar-sdk/support/clock/clocktest"
	"github.com/stretchr/testify/assert"
)

// TestClock_Now_sourceNotSet tests that when the Source field is not set that
// the real time is used when the Clock is asked for the current time.
func TestClock_Now_sourceNotSet(t *testing.T) {
	c := clock.Clock{}
	before := time.Now()
	cNow := c.Now()
	after := time.Now()
	assert.True(t, cNow.After(before))
	assert.True(t, cNow.Before(after))
}

// TestClock_Now_sourceNotSetPtrNil tests that when the identifier is a
// unset/nil pointer to a Clock, it still has default behavior.
func TestClock_Now_sourceNotSetPtrNil(t *testing.T) {
	c := (*clock.Clock)(nil)
	before := time.Now()
	cNow := c.Now()
	after := time.Now()
	assert.True(t, cNow.After(before))
	assert.True(t, cNow.Before(after))
}

// TestClock_Now_sourceSet tests that when the Source field is set that it is
// used when the Clock is asked for the current time.
func TestClock_Now_sourceSet(t *testing.T) {
	timeNow := time.Date(2015, 9, 30, 17, 15, 54, 0, time.UTC)
	c := clock.Clock{
		Source: clocktest.FixedSource(timeNow),
	}
	cNow := c.Now()
	assert.Equal(t, timeNow, cNow)
}
