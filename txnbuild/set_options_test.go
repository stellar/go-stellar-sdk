package txnbuild

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleSetFlagsThreeDifferent(t *testing.T) {
	options := SetOptions{}
	options.SetFlags = []AccountFlag{1, 2, 4}

	var xdrOp xdr.SetOptionsOp
	options.handleSetFlags(&xdrOp)

	expected := xdr.Uint32(7)
	assert.Equal(t, expected, *xdrOp.SetFlags, "three different valid flags are ok")
}

func TestHandleSetFlagsThreeSame(t *testing.T) {
	options := SetOptions{}
	options.SetFlags = []AccountFlag{1, 1, 1}

	var xdrOp xdr.SetOptionsOp
	options.handleSetFlags(&xdrOp)

	expected := xdr.Uint32(1)
	assert.Equal(t, expected, *xdrOp.SetFlags, "three of the same valid flags are ok")
}

func TestHandleSetFlagsRedundantFlagsAllowed(t *testing.T) {
	options := SetOptions{}
	options.SetFlags = []AccountFlag{1, 2, 4, 2, 4, 1}

	var xdrOp xdr.SetOptionsOp
	options.handleSetFlags(&xdrOp)

	expected := xdr.Uint32(7)
	assert.Equal(t, expected, *xdrOp.SetFlags, "additional redundant flags are allowed")
}

func TestHandleSetFlagsLessThanThreeAreOK(t *testing.T) {
	options := SetOptions{}
	options.SetFlags = []AccountFlag{1, 2}

	var xdrOp xdr.SetOptionsOp
	options.handleSetFlags(&xdrOp)

	expected := xdr.Uint32(3)
	assert.Equal(t, expected, *xdrOp.SetFlags, "less than three flags are ok")
}

func TestHandleSetFlagsInvalidFlagsAllowed(t *testing.T) {
	options := SetOptions{}
	options.SetFlags = []AccountFlag{3, 3, 3}

	var xdrOp xdr.SetOptionsOp
	options.handleSetFlags(&xdrOp)

	expected := xdr.Uint32(3)
	assert.Equal(t, expected, *xdrOp.SetFlags, "invalid flags are allowed")
}

func TestHandleSetFlagsZeroFlagsAreOK(t *testing.T) {
	options := SetOptions{}
	options.SetFlags = []AccountFlag{0, 2, 0}

	var xdrOp xdr.SetOptionsOp
	options.handleSetFlags(&xdrOp)

	expected := xdr.Uint32(2)
	assert.Equal(t, expected, *xdrOp.SetFlags, "zero flags are ok")
}

func TestHandleClearFlagsThreeDifferent(t *testing.T) {
	options := SetOptions{}
	options.ClearFlags = []AccountFlag{1, 2, 4}

	var xdrOp xdr.SetOptionsOp
	options.handleClearFlags(&xdrOp)

	expected := xdr.Uint32(7)
	assert.Equal(t, expected, *xdrOp.ClearFlags, "three different valid flags are ok")
}

func TestHandleClearFlagsThreeSame(t *testing.T) {
	options := SetOptions{}
	options.ClearFlags = []AccountFlag{1, 1, 1}

	var xdrOp xdr.SetOptionsOp
	options.handleClearFlags(&xdrOp)

	expected := xdr.Uint32(1)
	assert.Equal(t, expected, *xdrOp.ClearFlags, "three of the same valid flags are ok")
}

func TestHandleClearFlagsRedundantFlagsAllowed(t *testing.T) {
	options := SetOptions{}
	options.ClearFlags = []AccountFlag{1, 2, 4, 2, 4, 1}

	var xdrOp xdr.SetOptionsOp
	options.handleClearFlags(&xdrOp)

	expected := xdr.Uint32(7)
	assert.Equal(t, expected, *xdrOp.ClearFlags, "additional redundant flags are allowed")
}

func TestHandleClearFlagsLessThanThreeAreOK(t *testing.T) {
	options := SetOptions{}
	options.ClearFlags = []AccountFlag{1, 2}

	var xdrOp xdr.SetOptionsOp
	options.handleClearFlags(&xdrOp)

	expected := xdr.Uint32(3)
	assert.Equal(t, expected, *xdrOp.ClearFlags, "less than three flags are ok")
}

func TestHandleClearFlagsInvalidFlagsAllowed(t *testing.T) {
	options := SetOptions{}
	options.ClearFlags = []AccountFlag{3, 3, 3}

	var xdrOp xdr.SetOptionsOp
	options.handleClearFlags(&xdrOp)

	expected := xdr.Uint32(3)
	assert.Equal(t, expected, *xdrOp.ClearFlags, "invalid flags are allowed")
}

func TestHandleClearFlagsZeroFlagsAreOK(t *testing.T) {
	options := SetOptions{}
	options.ClearFlags = []AccountFlag{0, 2, 0}

	var xdrOp xdr.SetOptionsOp
	options.handleClearFlags(&xdrOp)

	expected := xdr.Uint32(2)
	assert.Equal(t, expected, *xdrOp.ClearFlags, "zero flags are ok")
}

func TestEmptyHomeDomainOK(t *testing.T) {
	options := SetOptions{
		HomeDomain: NewHomeDomain(""),
	}
	op, err := options.BuildXDR()
	require.NoError(t, err)

	assert.Equal(t, "", string(*op.Body.MustSetOptionsOp().HomeDomain), "empty string home domain is set")
}

// TestSetOptionsBuildXDRReuse checks that a second build of the same SetOptions omits fields cleared after the first build.
func TestSetOptionsBuildXDRReuse(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	options := SetOptions{
		InflationDestination: NewInflationDestination(kp0.Address()),
		SetFlags:             []AccountFlag{AuthRequired},
		ClearFlags:           []AccountFlag{AuthRevocable},
		MasterWeight:         NewThreshold(0),
		LowThreshold:         NewThreshold(1),
		MediumThreshold:      NewThreshold(2),
		HighThreshold:        NewThreshold(3),
		HomeDomain:           NewHomeDomain("first.example"),
		Signer:               &Signer{Address: kp1.Address(), Weight: 1},
	}
	_, err := options.BuildXDR()
	require.NoError(t, err)

	options.InflationDestination = nil
	options.SetFlags = nil
	options.ClearFlags = nil
	options.MasterWeight = nil
	options.LowThreshold = nil
	options.MediumThreshold = nil
	options.HighThreshold = nil
	options.HomeDomain = NewHomeDomain("second.example")
	options.Signer = nil
	got, err := options.BuildXDR()
	require.NoError(t, err)

	fresh := SetOptions{HomeDomain: NewHomeDomain("second.example")}
	want, err := fresh.BuildXDR()
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestSetOptionsFromXDRReuse checks that FromXDR into a populated SetOptions replaces every field with the value from the XDR.
func TestSetOptionsFromXDRReuse(t *testing.T) {
	source := SetOptions{
		SetFlags:   []AccountFlag{AuthRevocable},
		ClearFlags: []AccountFlag{AuthRequired},
		HomeDomain: NewHomeDomain("second.example"),
	}
	op, err := source.BuildXDR()
	require.NoError(t, err)

	reused := SetOptions{
		InflationDestination: NewInflationDestination(newKeypair0().Address()),
		SetFlags:             []AccountFlag{AuthRequired},
		ClearFlags:           []AccountFlag{AuthRevocable},
		MasterWeight:         NewThreshold(0),
		LowThreshold:         NewThreshold(1),
		MediumThreshold:      NewThreshold(2),
		HighThreshold:        NewThreshold(3),
		HomeDomain:           NewHomeDomain("first.example"),
		Signer:               &Signer{Address: newKeypair1().Address(), Weight: 1},
		SourceAccount:        newKeypair2().Address(),
	}
	require.NoError(t, reused.FromXDR(op))

	assert.Equal(t, source, reused)
}

// TestSetOptionsRoundtrip tests that a SetOptions with every field set survives a transaction XDR round trip.
func TestSetOptionsRoundtrip(t *testing.T) {
	options := SetOptions{
		InflationDestination: NewInflationDestination(newKeypair0().Address()),
		SetFlags:             []AccountFlag{AuthRequired, AuthClawbackEnabled},
		ClearFlags:           []AccountFlag{AuthRevocable, AuthImmutable},
		MasterWeight:         NewThreshold(0),
		LowThreshold:         NewThreshold(1),
		MediumThreshold:      NewThreshold(2),
		HighThreshold:        NewThreshold(3),
		HomeDomain:           NewHomeDomain("stellar.org"),
		Signer:               &Signer{Address: newKeypair1().Address(), Weight: 4},
		SourceAccount:        "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
	}
	testOperationsMarshalingRoundtrip(t, []Operation{&options}, false)

	options.SourceAccount = "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK"
	testOperationsMarshalingRoundtrip(t, []Operation{&options}, true)
}
