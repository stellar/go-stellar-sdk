package xdr_test

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
)

func TestNewThreshold(t *testing.T) {
	threshold := xdr.NewThreshold(1, 2, 3, 4)

	assert.Equal(t, byte(1), threshold.MasterKeyWeight())
	assert.Equal(t, byte(2), threshold.ThresholdLow())
	assert.Equal(t, byte(3), threshold.ThresholdMedium())
	assert.Equal(t, byte(4), threshold.ThresholdHigh())
}
