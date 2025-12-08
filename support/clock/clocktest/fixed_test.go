package clocktest_test

import (
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/support/clock"
	"github.com/stellar/go-stellar-sdk/support/clock/clocktest"
	"github.com/stretchr/testify/assert"
)

// TestNewFixed tests that the time returned from the fixed clocks Now()
// function is equal to the time given when creating the clock.
func TestFixedSource_Now(t *testing.T) {
	timeNow := time.Date(2015, 9, 30, 17, 15, 54, 0, time.UTC)
	c := clock.Clock{Source: clocktest.FixedSource(timeNow)}
	assert.Equal(t, timeNow, c.Now())
}

// TestNewFixed_compose tests that FixedSource can be used easily to change
// time during a test.
func TestFixedSource_compose(t *testing.T) {
	timeNow := time.Date(2015, 9, 30, 17, 15, 54, 0, time.UTC)
	c := clock.Clock{Source: clocktest.FixedSource(timeNow)}
	assert.Equal(t, timeNow, c.Now())
	c.Source = clocktest.FixedSource(timeNow.AddDate(0, 0, 1))
	assert.Equal(t, timeNow.AddDate(0, 0, 1), c.Now())
}
