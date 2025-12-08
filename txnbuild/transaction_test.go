package txnbuild

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/price"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestMissingTimebounds(t *testing.T) {
	kp0 := newKeypair0()

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount: &SimpleAccount{AccountID: kp0.Address(), Sequence: 1},
			Operations:    []Operation{&BumpSequence{BumpTo: 0}},
			BaseFee:       MinBaseFee,
		},
	)
	assert.EqualError(t, err, "invalid preconditions: timebounds must be constructed using NewTimebounds(), NewTimeout(), or NewInfiniteTimeout()")
}

func TestTimebounds(t *testing.T) {
	kp0 := newKeypair0()
	tb := NewTimeout(300)

	tp := TransactionParams{
		SourceAccount: &SimpleAccount{AccountID: kp0.Address(), Sequence: 1},
		Operations:    []Operation{&BumpSequence{BumpTo: 0}},
		BaseFee:       MinBaseFee,
	}

	tp.Preconditions.TimeBounds = tb
	tx, err := NewTransaction(tp)
	assert.NoError(t, err)
	assert.Equal(t, tb, tx.preconditions.TimeBounds)
	assert.Equal(t, xdr.TimePoint(tb.MinTime), tx.envelope.TimeBounds().MinTime)
	assert.Equal(t, xdr.TimePoint(tb.MaxTime), tx.envelope.TimeBounds().MaxTime)
}

func TestV2Preconditions(t *testing.T) {
	kp0 := newKeypair0()

	cond := Preconditions{
		TimeBounds:                 NewTimeout(300),
		LedgerBounds:               &LedgerBounds{0, 1},
		MinSequenceNumber:          nil,
		MinSequenceNumberAge:       10,
		MinSequenceNumberLedgerGap: 2,
	}
	assert.True(t, cond.hasV2Conditions())

	tp := TransactionParams{
		SourceAccount: &SimpleAccount{AccountID: kp0.Address(), Sequence: 1},
		Operations:    []Operation{&BumpSequence{BumpTo: 0}},
		BaseFee:       MinBaseFee,
		Preconditions: cond,
	}

	tx, err := NewTransaction(tp)
	assert.NoError(t, err)
	assert.Equal(t, cond, tx.preconditions)

	b64, err := tx.Base64()
	assert.NoError(t, err)

	unpackedTx, err := TransactionFromXDR(b64)
	assert.NoError(t, err)
	if !assert.NotNil(t, unpackedTx.simple) {
		t.FailNow()
	}
	assert.Equal(t, cond, unpackedTx.simple.preconditions)
}

func TestMissingSourceAccount(t *testing.T) {
	_, err := NewTransaction(TransactionParams{})
	assert.EqualError(t, err, "transaction has no source account")
}

func TestIncrementSequenceNum(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), 1)
	inflation := Inflation{}

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&inflation},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), sourceAccount.Sequence)

	_, err = NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&inflation},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), sourceAccount.Sequence)

	_, err = NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: false,
			Operations:           []Operation{&inflation},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), sourceAccount.Sequence)

	_, err = NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: false,
			Operations:           []Operation{&inflation},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), sourceAccount.Sequence)
}

func TestFeeNoOperations(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), 5938436531814403)

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount: &sourceAccount,
			Operations:    []Operation{},
			BaseFee:       MinBaseFee,
			Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.EqualError(t, err, "transaction has no operations")
}

func TestInflation(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(3556091187167235))

	inflation := Inflation{}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&inflation},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAACQAAAAAAAAAB6i5yxQAAAED9zR1l78yiBwd/o44RyE3XP7QT57VmI90qE46TjfncYyqlOaIRWpkh3qouTjV5IRPVGo6+bFWV40H1HE087FgA"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestCreateAccount(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639897))

	createAccount := CreateAccount{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "10",
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAF9eEAAAAAAAAAAAHqLnLFAAAAQB7MjKIwNEOTIjbEeV+QIjaQp/ZpV5qpbkbDaU54gkfdTOFOUxZq66lTS5FOfP5fmPIVD8InQ00Usy2SmzFC/wc="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestPayment(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	payment := Payment{
		Destination: "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
		Amount:      "10",
		Asset:       NativeAsset{},
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&payment},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAQAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAAAAAAAABfXhAAAAAAAAAAAB6i5yxQAAAEDXBkKYzThQi3/XhJqGzfh/EjaAx/4zK3xBT1/JDNtdkk/kxn4qxHVx++xiV72lqZXxiphNwflA8C7mC8Dvim0E"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestPaymentMuxedAccounts(t *testing.T) {
	kp0 := newKeypair0()
	accountID := xdr.MustAddress(kp0.Address())
	mx := xdr.MuxedAccount{
		Type: xdr.CryptoKeyTypeKeyTypeMuxedEd25519,
		Med25519: &xdr.MuxedAccountMed25519{
			Id:      0xcafebabe,
			Ed25519: *accountID.Ed25519,
		},
	}
	sourceAccount := NewSimpleAccount(mx.Address(), int64(9605939170639898))

	payment := Payment{
		Destination:   "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK",
		Amount:        "10",
		Asset:         NativeAsset{},
		SourceAccount: sourceAccount.AccountID,
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&payment},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAQAAAAAAyv66vuDcbeFyXKxmUWK1L6znNbKKIkPkHRJNbLktcKPqLnLFAAAAZAAiII0AAAAbAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAEAAAEAAAAAAMr+ur7g3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAEAAAEAgAAAAAAAAAA/DDS/k60NmXHQTMyQ9wVRHIOKrZc0pKL7DXoD/H/omgAAAAAAAAAABfXhAAAAAAAAAAAB6i5yxQAAAED4Wkvwf/BJV+fqa6Kvi+T/7ZL82pOinN68GlvEi9qK4klH+qITyvN3jRj5Nfz0+VrE2xBJPVc8sS/qN9LlznoC"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestPaymentFailsIfNoAssetSpecified(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	payment := Payment{
		Destination: "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
		Amount:      "10",
	}

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&payment},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	expectedErrMsg := "validation failed for *txnbuild.Payment operation: Field: Asset, Error: asset is undefined"
	require.EqualError(t, err, expectedErrMsg, "An asset is required")
}

func TestBumpSequence(t *testing.T) {
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(9606132444168199))

	bumpSequence := BumpSequence{
		BumpTo: 9606132444168300,
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&bumpSequence},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAGQAIiC6AAAACAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAACwAiILoAAABsAAAAAAAAAAHSh2R+AAAAQJ3Y0klngAqW69ETgBCuo8OQsx4i/6wg6WugDtOfq2hw6MElCQXJJMJRLgo2waDvwNOrWTUU9T3q95Yk0K3PHwo="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestAccountMerge(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484298))

	accountMerge := AccountMerge{
		Destination: "GAS4V4O2B7DW5T7IQRPEEVCRXMDZESKISR7DVIGKZQYYV3OSQ5SH5LVP",
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&accountMerge},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAACwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAACAAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAAAAAAAB6i5yxQAAAECf1HDoBOuPhkKcL9Ll12to6yrRXZg7MmemWf7nca8j0vHDQpti+/OIsT2DOF0YJKEAncQt2CvJ+cefgly8668A"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageData(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(3556091187167235))

	manageData := ManageData{
		Name:  "Fruit preference",
		Value: []byte("Apple"),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&manageData},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAACgAAABBGcnVpdCBwcmVmZXJlbmNlAAAAAQAAAAVBcHBsZQAAAAAAAAAAAAAB6i5yxQAAAEDtRCyQRKKgQ8iLEu7kicHtSzoplfxPtPTMhdRv/sq8UoIBVTxIw+S13Jv+jzs3tyLDLiGCVNXreUNlbfX+980K"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageDataRemoveDataEntry(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484309))

	manageData := ManageData{
		Name: "Fruit preference",
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&manageData},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAAFgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAACgAAABBGcnVpdCBwcmVmZXJlbmNlAAAAAAAAAAAAAAAB6i5yxQAAAEAfK5BWYLX31E3QgEs8Cd40XDAsx6VW27hW8nuyotnS2qOruXdmks89zNroDSYzRTH0rt4qPWnQqsFSio5NFCUA"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestSetOptionsInflationDestination(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484315))

	setOptions := SetOptions{
		InflationDestination: NewInflationDestination(kp1.Address()),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&setOptions},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAAHAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABQAAAAEAAAAAJcrx2g/Hbs/ohF5CVFG7B5JJSJR+OqDKzDGK7dKHZH4AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB6i5yxQAAAEAdES3vQ43R8yzNtsIRY2t2U/ey//NfJb1qZORDkxE6/ZZgx+/wNPxAM3gpEwc2TAotwuqVdT6xga9DSXUaz6MI"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestSetOptionsSetFlags(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484318))

	setOptions := SetOptions{
		SetFlags: []AccountFlag{AuthRequired, AuthRevocable},
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&setOptions},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAAHwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABQAAAAAAAAAAAAAAAQAAAAMAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAeoucsUAAABAn2E6acbadQNs0m2+lc5DpMpPQ/+8Y2l0cUfmSKoHSt5VpB0EZI8lQY9smiOtSd7a3aewrMCJqbY5Iy6a7dFiDg=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestSetOptionsClearFlags(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484319))

	setOptions := SetOptions{
		ClearFlags: []AccountFlag{AuthRequired, AuthRevocable},
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&setOptions},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAAIAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABQAAAAAAAAABAAAAAwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAeoucsUAAABADVzwDfkYL6oxhdJCejMjU4jJ1mhC8Ob2DcMYb/PpotyphljM6IwsXJjAKp4tMwTLBI5fc+x/CU/cdOTpUPZ7Aw=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestSetOptionsMasterWeight(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484320))

	setOptions := SetOptions{
		MasterWeight: NewThreshold(10),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&setOptions},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAAIQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABQAAAAAAAAAAAAAAAAAAAAEAAAAKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAeoucsUAAABAiMR9luF2eXzLBuufIXSBMrNp5VUgCtRRI0+RgAxerFhE4RhXPlq5pcOhsCp+mTQJsVVCxIIq3I0MePGmEoBWAw=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestSetOptionsThresholds(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484322))

	setOptions := SetOptions{
		LowThreshold:    NewThreshold(1),
		MediumThreshold: NewThreshold(2),
		HighThreshold:   NewThreshold(2),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&setOptions},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAAIwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABQAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAQAAAAEAAAACAAAAAQAAAAIAAAAAAAAAAAAAAAAAAAAB6i5yxQAAAEBcEXBW8xLcaMWTrVpTkJXd51ER2boDY+X2hJ3Kb9F/3XK34kFVO5N35E2A7JIlRMRYqu/AgbGAK9Lrr3x+tSEL"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestSetOptionsHomeDomain(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484325))

	setOptions := SetOptions{
		HomeDomain: NewHomeDomain("LovelyLumensLookLuminous.com"),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&setOptions},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAAJgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAHExvdmVseUx1bWVuc0xvb2tMdW1pbm91cy5jb20AAAAAAAAAAAAAAAHqLnLFAAAAQLQuB2c70X8qYUYOY45s+Y8wZ/OkgDVwmUufRno0RPC9bgjsYF0hFaIdW/lHrVBIuyTf59RAgRFSa14I9HN+HgY="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestSetOptionsHomeDomainTooLong(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484323))

	setOptions := SetOptions{
		HomeDomain: NewHomeDomain("LovelyLumensLookLuminousLately.com"),
	}

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&setOptions},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)

	assert.Error(t, err, "A validation error was expected (home domain > 32 chars)")
}

func TestSetOptionsSigner(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484325))

	setOptions := SetOptions{
		Signer: &Signer{Address: kp1.Address(), Weight: Threshold(4)},
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&setOptions},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAAJgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAAQAAAAAAAAAAeoucsUAAABAX4JlCvsDY/ETs+/EoNK0NrO5ZrbwOK+XqR5KnPcqMSw6/xkpJoFp3laqCjcVhdCQfS/hqpdfn/DPKdTHBeDLAQ=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestMultipleOperations(t *testing.T) {
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(9606132444168199))

	inflation := Inflation{}
	bumpSequence := BumpSequence{
		BumpTo: 9606132444168300,
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&inflation, &bumpSequence},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAMgAIiC6AAAACAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAACQAAAAAAAAALACIgugAAAGwAAAAAAAAAAdKHZH4AAABA5n9wINh8OTXZb8yaaYeCpvmjSsvJH80tRAISFXSicFJzFVoTqX3V0of2npBFXaMV4dvoqKHK8XbZFgGX0t7DBQ=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestChangeTrust(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484348))

	changeTrust := ChangeTrust{
		Line:  CreditAsset{"ABCD", kp1.Address()}.MustToChangeTrustAsset(),
		Limit: "10",
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&changeTrust},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAAPQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABgAAAAFBQkNEAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAAAAX14QAAAAAAAAAAAeoucsUAAABA+2EndVXXsBHbRFEQGLsgsvHVm8wCxH9byZ/PP4AhEeAjXSL6IzhGnyRIWIc2SYXRu6GvveVI3yPbzCTvKnVjCg=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestChangeTrustNativeAssetNotAllowed(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484348))

	changeTrust := ChangeTrust{
		Line:  NativeAsset{}.MustToChangeTrustAsset(),
		Limit: "10",
	}

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&changeTrust},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)

	expectedErrMsg := "validation failed for *txnbuild.ChangeTrust operation: Field: Line, Error: native (XLM) asset type is not allowed"
	require.EqualError(t, err, expectedErrMsg, "No trustlines for native assets")
}

func TestChangeTrustDeleteTrustline(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484354))

	issuedAsset := CreditAsset{"ABCD", kp1.Address()}
	removeTrust := RemoveTrustlineOp(issuedAsset.MustToChangeTrustAsset())

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&removeTrust},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAAQwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABgAAAAFBQkNEAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAAAAAAAAAAAAAAAAAAAeoucsUAAABAoHdsJCt+XIr73+jSqbEhQ8iqXcqP3LO8C/kWH2dgQj+3hq1FKbthn0BbX/x5umgcE+pyfnTjU0j158qew6tfCw=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestAllowTrust(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484366))

	issuedAsset := CreditAsset{"ABCD", kp1.Address()}
	allowTrust := AllowTrust{
		Trustor:   kp1.Address(),
		Type:      issuedAsset,
		Authorize: true,
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&allowTrust},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAATwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABwAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAAFBQkNEAAAAAQAAAAAAAAAB6i5yxQAAAEBhgUiorWMaRzTGlVThNgiMpVhSYMKsY4cJyL1mrkkpC2qZ7Q9fBtaTGoS27PC6nK9/nBLOVoyyOHgYculoiYQJ"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestAllowTrustNoIssuer(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(40385577484366))

	issuedAsset := CreditAsset{Code: "XYZ"}
	allowTrust := AllowTrust{
		Trustor:   kp1.Address(),
		Type:      issuedAsset,
		Authorize: true,
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&allowTrust},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAACS7AAAATwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABwAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAAFYWVoAAAAAAQAAAAAAAAAB6i5yxQAAAEDvJnLIv/kTm6yraPLQAbTfEcFIutdNRagQ08KjEKeITbro8PkhhBWgQmCzcP7uNAxxUUKATYus3ASmwUoPoFcB"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageSellOfferNewOffer(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761092))

	selling := NativeAsset{}
	buying := CreditAsset{"ABCD", kp0.Address()}
	sellAmount := "100"
	createOffer, err := CreateOfferOp(selling, buying, sellAmount, price.MustParse("0.01"))
	check(err)

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAGQAACVqAAAABQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAwAAAAAAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAA7msoAAAAAAQAAAGQAAAAAAAAAAAAAAAAAAAAB0odkfgAAAEAJl3+AZx/G1ocvk58X/u84LIo+6VdG+1wuK6n2FovWSFVGonVj26xYWlo4kG12AdTSncdF44nc5HAIDCJy6g4L"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageSellOfferDeleteOffer(t *testing.T) {
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761105))

	offerID := int64(2921622)
	deleteOffer, err := DeleteOfferOp(offerID)
	check(err)

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&deleteOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAGQAACVqAAAAEgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAwAAAAAAAAABRkFLRQAAAABBB4BkxJWGYvNgJBoiXUo2tjgWlNmhHMMKdwGN7RSdsQAAAAAAAAAAAAAAAQAAAAEAAAAAACyUlgAAAAAAAAAB0odkfgAAAEAUo0X6chACDJ0UDj39QQTsfBxQui5um8cXZY2noJ1LbPEpliRkG2TeWvD0Bszk8BnQSgZPV/XfgSKwVXN5MskO"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageSellOfferUpdateOffer(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761097))

	selling := NativeAsset{}
	buying := CreditAsset{"ABCD", kp0.Address()}
	sellAmount := "50"
	offerID := int64(2497628)
	updateOffer, err := UpdateOfferOp(selling, buying, sellAmount, price.MustParse("0.02"), offerID)
	check(err)

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&updateOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAGQAACVqAAAACgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAwAAAAAAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAdzWUAAAAAAQAAADIAAAAAACYcXAAAAAAAAAAB0odkfgAAAEAMKloNgv6Hv8x+A92O/8oOUpR6hbxegN4+hkGfTT4d0TqrraLy8gBOtvq718TO4akjc9UbceH6yWjoTmm4egwI"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestCreatePassiveSellOffer(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761100))

	createPassiveOffer := CreatePassiveSellOffer{
		Selling: NativeAsset{},
		Buying:  CreditAsset{"ABCD", kp0.Address()},
		Amount:  "10",
		Price:   xdr.Price{1, 1},
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createPassiveOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAGQAACVqAAAADQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABAAAAAAAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAF9eEAAAAAAQAAAAEAAAAAAAAAAdKHZH4AAABAIFA+zNVC+8dptptusks3Eh8SJ3jk+/6/rPxy7IFg4+gpqUotRma5b7QR/gjbnoAsL1tPU0WSYae2y8sJGhQqCg=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestPathPayment(t *testing.T) {
	kp0 := newKeypair0()
	kp2 := newKeypair2()
	sourceAccount := NewSimpleAccount(kp2.Address(), int64(187316408680450))

	abcdAsset := CreditAsset{"ABCD", kp0.Address()}
	pathPayment := PathPayment{
		SendAsset:   NativeAsset{},
		SendMax:     "10",
		Destination: kp2.Address(),
		DestAsset:   NativeAsset{},
		DestAmount:  "1",
		Path:        []Asset{abcdAsset},
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&pathPayment},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp2,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAGQAAKpdAAAAAwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAgAAAAAAAAAABfXhAAAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAAAAAAAAAJiWgAAAAAEAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAAAAABLhVZmAAAAEBdpC1C/0aBSMtXJrfhl3Vp9rQ1IyWFd2MBeAPNsyAYamEjuqIDqCzzUbd8PiBggIH0eEPZaWsfsAl1qEBER0sO"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestMemoText(t *testing.T) {
	kp2 := newKeypair2()
	sourceAccount := NewSimpleAccount(kp2.Address(), int64(3556099777101824))

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&BumpSequence{BumpTo: 1}},
			Memo:                 MemoText("Twas brillig"),
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp2,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAGQADKJBAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAxUd2FzIGJyaWxsaWcAAAABAAAAAAAAAAsAAAAAAAAAAQAAAAAAAAABLhVZmAAAAECC0/P+zBk5lpH4zIumNt59nFVrPiDGOu8TrJE4r0mXoae8Fmg1yyHQm3Yo5huuPjc/nzwU/R2DKkkQ3C4mWA0N"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestMemoID(t *testing.T) {
	kp2 := newKeypair2()
	sourceAccount := NewSimpleAccount(kp2.Address(), int64(3428320205078528))

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&BumpSequence{BumpTo: 1}},
			Memo:                 MemoID(314159),
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp2,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAGQADC4KAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAABMsvAAAAAQAAAAAAAAALAAAAAAAAAAEAAAAAAAAAAS4VWZgAAABAOT/1f1XoeqY14+wp6rVgwE4fCCPnItc9/85jZN++Fy7lS88e40b3ufQCpzzMCD8AyfHF8BCs/Pn2DiJHxCPQCQ=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestMemoHash(t *testing.T) {
	kp2 := newKeypair2()
	sourceAccount := NewSimpleAccount(kp2.Address(), int64(3428320205078528))

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&BumpSequence{BumpTo: 1}},
			Memo:                 MemoHash([32]byte{0x01}),
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp2,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAGQADC4KAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAwEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAALAAAAAAAAAAEAAAAAAAAAAS4VWZgAAABAIGrmlKahBhdVXl2LZGINCNfUAtxiVawjzqgxzyHV7xpEPTft1besnyiDdLBP1+Tbg+hYQK0N2ncL2XmjQ4pcDQ=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestMemoReturn(t *testing.T) {
	kp2 := newKeypair2()
	sourceAccount := NewSimpleAccount(kp2.Address(), int64(3428320205078528))

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&BumpSequence{BumpTo: 1}},
			Memo:                 MemoReturn([32]byte{0x01}),
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp2,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAGQADC4KAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAABAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAALAAAAAAAAAAEAAAAAAAAAAS4VWZgAAABALixU7p2NPKW1iqJqaHqR3Wsy5q+7nj1EjswOD99/klUSlorvodrZ4DrD/IYGvsKSyV0/Zf9LjEN4s4kVVK4dCg=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageBuyOfferNewOffer(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761092))

	buyOffer := ManageBuyOffer{
		Selling: NativeAsset{},
		Buying:  CreditAsset{"ABCD", kp0.Address()},
		Amount:  "100",
		Price:   price.MustParse("0.01"),
		OfferID: 0,
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&buyOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAGQAACVqAAAABQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAADAAAAAAAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAA7msoAAAAAAQAAAGQAAAAAAAAAAAAAAAAAAAAB0odkfgAAAEB8LqK1uwbwcCQM/hE0rXng2fVCoaMdctQaiS72iJFkq+azWzqYpo1kMa1DUKMvvsJrWPLYjEr9yW8/A3eEE2kF"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageBuyOfferDeleteOffer(t *testing.T) {
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761105))

	buyOffer := ManageBuyOffer{
		Selling: NativeAsset{},
		Buying:  CreditAsset{"ABCD", kp1.Address()},
		Amount:  "0",
		Price:   price.MustParse("0.01"),
		OfferID: int64(2921622),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&buyOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAGQAACVqAAAAEgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAADAAAAAAAAAABQUJDRAAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAAAAAAAAAAAAAQAAAGQAAAAAACyUlgAAAAAAAAAB0odkfgAAAECLZ6PnKZlGBb8S3GFWg6J01d3Zr88/tki8yka2KFzqivMAmY3D/5IMzzJl4U7RdrYEPam9KwCGKR/f647WTwYG"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageBuyOfferUpdateOffer(t *testing.T) {
	kp1 := newKeypair1()
	sourceAccount := NewSimpleAccount(kp1.Address(), int64(41137196761097))

	buyOffer := ManageBuyOffer{
		Selling: NativeAsset{},
		Buying:  CreditAsset{"ABCD", kp1.Address()},
		Amount:  "50",
		Price:   price.MustParse("0.02"),
		OfferID: int64(2921622),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&buyOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAGQAACVqAAAACgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAADAAAAAAAAAABQUJDRAAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAAAdzWUAAAAAAQAAADIAAAAAACyUlgAAAAAAAAAB0odkfgAAAECv7GrE8YDar5M93RmgzslIH2vVAAJlAZoIsmkFNXTJTTb01R9Q+z0Cl5E6KFpm+qiuxHvL2kwhVOoBpkoYQPcB"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestBuildChallengeTx(t *testing.T) {
	kp0 := newKeypair0()

	{
		// 1 minute timebound
		tx, err := BuildChallengeTx(kp0.Seed(), kp0.Address(), "testwebauth.stellar.org", "testanchor.stellar.org", network.TestNetworkPassphrase, time.Minute, nil)
		assert.NoError(t, err)
		txeBase64, err := tx.Base64()
		assert.NoError(t, err)
		var txXDR xdr.TransactionEnvelope
		err = xdr.SafeUnmarshalBase64(txeBase64, &txXDR)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), txXDR.SeqNum(), "sequence number should be 0")
		assert.Equal(t, uint32(200), txXDR.Fee(), "Fee should be 100")
		assert.Equal(t, 2, len(txXDR.Operations()), "number operations should be 2")
		timeDiff := txXDR.TimeBounds().MaxTime - txXDR.TimeBounds().MinTime
		assert.Equal(t, int64(60), int64(timeDiff), "time difference should be 300 seconds")
		op := txXDR.Operations()[0]
		assert.Equal(t, xdr.OperationTypeManageData, op.Body.Type, "operation type should be manage data")
		assert.Equal(t, xdr.String64("testanchor.stellar.org auth"), op.Body.ManageDataOp.DataName, "DataName should be 'testanchor.stellar.org auth'")
		assert.Equal(t, 64, len(*op.Body.ManageDataOp.DataValue), "DataValue should be 64 bytes")
		webAuthOp := txXDR.Operations()[1]
		assert.Equal(t, xdr.OperationTypeManageData, webAuthOp.Body.Type, "operation type should be manage data")
		assert.Equal(t, xdr.String64("web_auth_domain"), webAuthOp.Body.ManageDataOp.DataName, "DataName should be 'web_auth_domain'")
		assert.Equal(t, "testwebauth.stellar.org", string(*webAuthOp.Body.ManageDataOp.DataValue), "DataValue should be 'testwebauth.stellar.org'")
	}

	{
		// 5 minutes timebound
		tx, err := BuildChallengeTx(kp0.Seed(), kp0.Address(), "testwebauth.stellar.org", "testanchor.stellar.org", network.TestNetworkPassphrase, time.Duration(5*time.Minute), nil)
		assert.NoError(t, err)
		txeBase64, err := tx.Base64()
		assert.NoError(t, err)
		var txXDR1 xdr.TransactionEnvelope
		err = xdr.SafeUnmarshalBase64(txeBase64, &txXDR1)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), txXDR1.SeqNum(), "sequence number should be 0")
		assert.Equal(t, uint32(200), txXDR1.Fee(), "Fee should be 100")
		assert.Equal(t, 2, len(txXDR1.Operations()), "number operations should be 2")

		timeDiff := txXDR1.TimeBounds().MaxTime - txXDR1.TimeBounds().MinTime
		assert.Equal(t, int64(300), int64(timeDiff), "time difference should be 300 seconds")
		op1 := txXDR1.Operations()[0]
		assert.Equal(t, xdr.OperationTypeManageData, op1.Body.Type, "operation type should be manage data")
		assert.Equal(t, xdr.String64("testanchor.stellar.org auth"), op1.Body.ManageDataOp.DataName, "DataName should be 'testanchor.stellar.org auth'")
		assert.Equal(t, 64, len(*op1.Body.ManageDataOp.DataValue), "DataValue should be 64 bytes")
		webAuthOp := txXDR1.Operations()[1]
		assert.Equal(t, xdr.OperationTypeManageData, webAuthOp.Body.Type, "operation type should be manage data")
		assert.Equal(t, xdr.String64("web_auth_domain"), webAuthOp.Body.ManageDataOp.DataName, "DataName should be 'web_auth_domain'")
		assert.Equal(t, "testwebauth.stellar.org", string(*webAuthOp.Body.ManageDataOp.DataValue), "DataValue should be 'testwebauth.stellar.org'")
	}

	// transaction with invalid clientAccountID
	{
		_, err := BuildChallengeTx(kp0.Seed(), "test", "testwebauth.stellar.org", "testanchor.stellar.org", network.TestNetworkPassphrase, time.Minute, nil)
		require.EqualError(t, err, "test is not a valid account id or muxed account: invalid address length")
	}

	// transaction with memo
	{
		var memo MemoID = MemoID(1)
		tx, err := BuildChallengeTx(kp0.Seed(), kp0.Address(), "testwebauth.stellar.org", "testanchor.stellar.org", network.TestNetworkPassphrase, time.Minute, &memo)
		assert.NoError(t, err)
		assert.Equal(t, tx.Memo(), &memo)
	}

	// transaction with muxed account
	{
		muxedAccount := "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK"
		tx, err := BuildChallengeTx(kp0.Seed(), muxedAccount, "testwebauth.stellar.org", "testanchor.stellar.org", network.TestNetworkPassphrase, time.Minute, nil)
		assert.NoError(t, err)
		assert.Equal(t, tx.operations[0].GetSourceAccount(), muxedAccount)
	}

	// transaction with memo and muxed account
	{
		var memo MemoID = MemoID(1)
		_, err := BuildChallengeTx(kp0.Seed(), "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK", "testwebauth.stellar.org", "testanchor.stellar.org", network.TestNetworkPassphrase, time.Minute, &memo)
		assert.EqualError(t, err, "memos are not valid for challenge transactions with a muxed client account")
	}

	//transaction with infinite timebound
	_, err := BuildChallengeTx(kp0.Seed(), kp0.Address(), "webauthdomain", "sdf", network.TestNetworkPassphrase, 0, nil)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "provided timebound must be at least 1s (300s is recommended)")
	}
}

func TestHashHex(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639897))

	createAccount := CreateAccount{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "10",
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)

	txeB64, err := tx.Base64()
	assert.NoError(t, err)
	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAF9eEAAAAAAAAAAAHqLnLFAAAAQB7MjKIwNEOTIjbEeV+QIjaQp/ZpV5qpbkbDaU54gkfdTOFOUxZq66lTS5FOfP5fmPIVD8InQ00Usy2SmzFC/wc="
	assert.Equal(t, expected, txeB64, "Base 64 XDR should match")

	hashHex, err := tx.HashHex(network.TestNetworkPassphrase)
	assert.NoError(t, err)
	expected = "1b3905ba8c3c0ecc68ae812f2d77f27c697195e8daf568740fc0f5662f65f759"
	assert.Equal(t, expected, hashHex, "hex encoded hash should match")

	txEnv := tx.ToXDR()
	assert.NoError(t, err)
	assert.NotNil(t, txEnv, "transaction xdr envelope should not be nil")
	sourceAccountFromEnv := txEnv.SourceAccount().ToAccountId()
	assert.Equal(t, sourceAccount.AccountID, sourceAccountFromEnv.Address())
	assert.Equal(t, uint32(100), txEnv.Fee())
	assert.Equal(t, sourceAccount.Sequence, int64(txEnv.SeqNum()))
	assert.Equal(t, xdr.MemoTypeMemoNone, txEnv.Memo().Type)
	assert.Len(t, txEnv.Operations(), 1)
}

func TestTransactionFee(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639897))

	createAccount := CreateAccount{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "10",
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	assert.Equal(t, int64(100), tx.BaseFee(), "Transaction base fee should match")
	assert.Equal(t, int64(100), tx.MaxFee(), "Transaction fee should match")

	tx, err = NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              500,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	assert.Equal(t, int64(500), tx.BaseFee(), "Transaction base fee should match")
	assert.Equal(t, int64(500), tx.MaxFee(), "Transaction fee should match")

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)

	txeB64, err := tx.Base64()
	assert.NoError(t, err)
	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAfQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAF9eEAAAAAAAAAAAHqLnLFAAAAQJ3OvWisOnYNS5R8ZCHrSmbvDrvIYG4+JiAldLYjiXroqvA74r0pQJ4Jw/hZVSGqLZoPIt3RMwYPi3C5xvVLbQU="
	assert.Equal(t, expected, txeB64, "Base 64 XDR should match")
}

func TestPreAuthTransaction(t *testing.T) {
	// Address: GDK3YEHGI3ORGVO7ZEV2XF4SV5JU3BOKHMHPP4QFJ74ZRIIRROZ7ITOJ
	kp0 := newKeypair("SDY4PF6F6OWWERZT6OL2LVNREHUGHKALUI5W4U2JK4GAKPAC2RM43OAU")
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(4353383146192898)) // sequence number is in the future

	createAccount := CreateAccount{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "10",
	}

	// build transaction to be submitted in the future.
	txFuture, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	// save the hash of the future transaction.
	txFutureHash, err := txFuture.Hash(network.TestNetworkPassphrase)
	assert.NoError(t, err)

	// sign transaction without keypairs, the hash of the future transaction on the account
	//  will be used for authorisation.
	txFuture, err = txFuture.Sign(network.TestNetworkPassphrase)
	assert.NoError(t, err)

	txeFutureB64, err := txFuture.Base64()
	assert.NoError(t, err)
	expected := "AAAAAgAAAADVvBDmRt0TVd/JK6uXkq9TTYXKOw738gVP+ZihEYuz9AAAAGQAD3dhAAAAAwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAF9eEAAAAAAAAAAAA="
	assert.Equal(t, expected, txeFutureB64, "Base 64 XDR should match")

	//encode the txFutureHash as a stellar HashTx signer key.
	preAuth, err := strkey.Encode(strkey.VersionByteHashTx, txFutureHash[:])
	assert.NoError(t, err)

	// set sequence number to the current number.
	sourceAccount.Sequence = int64(4353383146192897)

	// add hash of future transaction as signer to account
	setOptions := SetOptions{
		Signer: &Signer{Address: preAuth, Weight: Threshold(2)},
	}

	// build a transaction to add the hash of the future transaction as a signer on the account.
	txNow, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&setOptions},
			BaseFee:              500,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	assert.Equal(t, int64(500), txNow.MaxFee(), "Transaction fee should match")
	txNow, err = txNow.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)

	txeNowB64, err := txNow.Base64()
	assert.NoError(t, err)
	expected = "AAAAAgAAAADVvBDmRt0TVd/JK6uXkq9TTYXKOw738gVP+ZihEYuz9AAAAfQAD3dhAAAAAgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAGPXOumKQj5/MjKKSmjQXe4G4g9nK/mkyzmmROMIZnjtQAAAAIAAAAAAAAAARGLs/QAAABAutrV0Cg03KwfFbzkCGiNxAldLsqQZKRjbsqHZyy2Nu4ouEDHQeIOKLWCLymOp21kKmGGqTYekPXVbGHyujh0DA=="
	assert.Equal(t, expected, txeNowB64, "Base 64 XDR should match")
	// Note: txeFutureB64 can be submitted to the network after txeNowB64 has been applied to the account
}

func TestHashXTransaction(t *testing.T) {
	// 256 bit preimage
	preimage := "this is a preimage for hashx transactions on the stellar network"

	preimageHash := sha256.Sum256([]byte(preimage))

	//encode preimageHash as a stellar HashX signer key
	hashx, err := strkey.Encode(strkey.VersionByteHashX, preimageHash[:])
	assert.NoError(t, err)

	// add hashx as signer to the account
	setOptions := SetOptions{
		Signer: &Signer{Address: hashx, Weight: Threshold(1)},
	}

	// Address: GDK3YEHGI3ORGVO7ZEV2XF4SV5JU3BOKHMHPP4QFJ74ZRIIRROZ7ITOJ
	kp0 := newKeypair("SDY4PF6F6OWWERZT6OL2LVNREHUGHKALUI5W4U2JK4GAKPAC2RM43OAU")
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(4353383146192899))

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&setOptions},
			BaseFee:              500,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)
	assert.Equal(t, int64(500), tx.MaxFee(), "Transaction fee should match")

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)

	txeB64, err := tx.Base64()
	assert.NoError(t, err)

	expected := "AAAAAgAAAADVvBDmRt0TVd/JK6uXkq9TTYXKOw738gVP+ZihEYuz9AAAAfQAD3dhAAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAL7JYG+aCH7iEhT/BWL06rHIhtYklHqyQdwLuk9li6jBQAAAAEAAAAAAAAAARGLs/QAAABAhwcHwm3DsBcqCCy1uzmXo73W7FTxMAes+qHABuHERruvb1ygqwRWA9pjHSUQnoJYCYH4GhY9qrIQYC/MkNeFBw=="
	assert.Equal(t, expected, txeB64, "Base 64 XDR should match")

	// build a transaction to test hashx signer
	payment := Payment{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "10",
		Asset:       NativeAsset{},
	}

	sourceAccount.Sequence = int64(4353383146192902)

	tx, err = NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&payment},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	// sign transaction with the preimage
	tx, err = tx.SignHashX([]byte(preimage))
	assert.NoError(t, err)

	txeB64, err = tx.Base64()
	assert.NoError(t, err)
	expected = "AAAAAgAAAADVvBDmRt0TVd/JK6uXkq9TTYXKOw738gVP+ZihEYuz9AAAAGQAD3dhAAAABwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAQAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAAAAAABfXhAAAAAAAAAAABli6jBQAAAEB0aGlzIGlzIGEgcHJlaW1hZ2UgZm9yIGhhc2h4IHRyYW5zYWN0aW9ucyBvbiB0aGUgc3RlbGxhciBuZXR3b3Jr"
	assert.Equal(t, expected, txeB64, "Base 64 XDR should match")

}

func TestFromXDR(t *testing.T) {
	txeB64 := "AAAAACYWIvM98KlTMs0IlQBZ06WkYpZ+gILsQN6ega0++I/sAAAAZAAXeEkAAAABAAAAAAAAAAEAAAAQMkExVjZKNTcwM0c0N1hIWQAAAAEAAAABAAAAACYWIvM98KlTMs0IlQBZ06WkYpZ+gILsQN6ega0++I/sAAAAAQAAAADMSEvcRKXsaUNna++Hy7gWm/CfqTjEA7xoGypfrFGUHAAAAAAAAAACCPHRAAAAAAAAAAABPviP7AAAAEBu6BCKf4WZHPum5+29Nxf6SsJNN8bgjp1+e1uNBaHjRg3rdFZYgUqEqbHxVEs7eze3IeRbjMZxS3zPf/xwJCEI"

	tx, err := TransactionFromXDR(txeB64)
	assert.NoError(t, err)
	newTx, ok := tx.Transaction()
	assert.True(t, ok)
	_, ok = tx.FeeBump()
	assert.False(t, ok)

	assert.Equal(t, "GATBMIXTHXYKSUZSZUEJKACZ2OS2IYUWP2AIF3CA32PIDLJ67CH6Y5UY", newTx.SourceAccount().AccountID, "source accounts should match")
	assert.Equal(t, int64(100), newTx.BaseFee(), "Base fee should match")
	sa := newTx.SourceAccount()
	assert.Equal(t, int64(6606179392290817), sa.Sequence, "Sequence number should match")
	assert.Equal(t, int64(6606179392290817), newTx.SequenceNumber(), "Sequence number should match")
	assert.Equal(t, 1, len(newTx.Operations()), "Operations length should match")
	assert.IsType(t, newTx.Operations()[0], &Payment{}, "Operation types should match")
	paymentOp, ok1 := newTx.Operations()[0].(*Payment)
	assert.Equal(t, true, ok1)
	assert.Equal(t, "GATBMIXTHXYKSUZSZUEJKACZ2OS2IYUWP2AIF3CA32PIDLJ67CH6Y5UY", paymentOp.SourceAccount, "Operation source should match")
	assert.Equal(t, "GDGEQS64ISS6Y2KDM5V67B6LXALJX4E7VE4MIA54NANSUX5MKGKBZM5G", paymentOp.Destination, "Operation destination should match")
	assert.Equal(t, "874.0000000", paymentOp.Amount, "Operation amount should match")

	txeB64 = "AAAAAGigiN2q4qBXAERImNEncpaADylyBRtzdqpEsku6CN0xAAABkAAADXYAAAABAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAABAAAABm5ldyB0eAAAAAAAAgAAAAEAAAAA+Q2efEMLNGF4i+aYfutUXGMSlf8tNevKeS1Jl/oCVGkAAAAGAAAAAVVTRAAAAAAAaKCI3arioFcAREiY0SdyloAPKXIFG3N2qkSyS7oI3TF//////////wAAAAAAAAAKAAAABHRlc3QAAAABAAAABXZhbHVlAAAAAAAAAAAAAAA="

	tx2, err := TransactionFromXDR(txeB64)
	assert.NoError(t, err)
	newTx2, ok := tx2.Transaction()
	assert.True(t, ok)
	_, ok = tx2.FeeBump()
	assert.False(t, ok)

	assert.Equal(t, "GBUKBCG5VLRKAVYAIREJRUJHOKLIADZJOICRW43WVJCLES52BDOTCQZU", newTx2.SourceAccount().AccountID, "source accounts should match")
	assert.Equal(t, int64(200), newTx2.BaseFee(), "Base fee should match")
	assert.Equal(t, int64(14800457302017), newTx2.SourceAccount().Sequence, "Sequence number should match")
	assert.Equal(t, int64(14800457302017), newTx2.SequenceNumber(), "Sequence number should match")

	memo, ok := newTx2.Memo().(MemoText)
	assert.Equal(t, true, ok)
	assert.Equal(t, MemoText("new tx"), memo, "memo should match")
	assert.Equal(t, 2, len(newTx2.Operations()), "Operations length should match")
	assert.IsType(t, newTx2.Operations()[0], &ChangeTrust{}, "Operation types should match")
	assert.IsType(t, newTx2.Operations()[1], &ManageData{}, "Operation types should match")
	op1, ok1 := newTx2.Operations()[0].(*ChangeTrust)
	assert.Equal(t, true, ok1)
	assert.Equal(t, "GD4Q3HT4IMFTIYLYRPTJQ7XLKROGGEUV74WTL26KPEWUTF72AJKGSJS7", op1.SourceAccount, "Operation source should match")
	assetType, err := op1.Line.GetType()
	assert.NoError(t, err)

	assert.Equal(t, AssetTypeCreditAlphanum4, assetType, "Asset type should match")
	assert.Equal(t, "USD", op1.Line.GetCode(), "Asset code should match")
	assert.Equal(t, "GBUKBCG5VLRKAVYAIREJRUJHOKLIADZJOICRW43WVJCLES52BDOTCQZU", op1.Line.GetIssuer(), "Asset issuer should match")
	assert.Equal(t, "922337203685.4775807", op1.Limit, "trustline limit should match")

	op2, ok2 := newTx2.Operations()[1].(*ManageData)
	assert.Equal(t, true, ok2)
	assert.Equal(t, "", op2.SourceAccount, "Operation source should match")
	assert.Equal(t, "test", op2.Name, "Name should match")
	assert.Equal(t, "value", string(op2.Value), "Value should match")

	// Muxed accounts
	txB64WithMuxedAccounts := "AAAAAgAAAQAAAAAAyv66vuDcbeFyXKxmUWK1L6znNbKKIkPkHRJNbLktcKPqLnLFAAAAZAAiII0AAAAbAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAEAAAEAAAAAAMr+ur7g3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAEAAAEAgAAAAAAAAAA/DDS/k60NmXHQTMyQ9wVRHIOKrZc0pKL7DXoD/H/omgAAAAAAAAAABfXhAAAAAAAAAAAB6i5yxQAAAED4Wkvwf/BJV+fqa6Kvi+T/7ZL82pOinN68GlvEi9qK4klH+qITyvN3jRj5Nfz0+VrE2xBJPVc8sS/qN9LlznoC"

	// It provides M-addreses when enabling muxed accounts
	tx3, err := TransactionFromXDR(txB64WithMuxedAccounts)
	assert.NoError(t, err)
	newTx3, ok := tx3.Transaction()
	assert.True(t, ok)
	assert.Equal(t, "MDQNY3PBOJOKYZSRMK2S7LHHGWZIUISD4QORETLMXEWXBI7KFZZMKAAAAAAMV7V2XYGQO", newTx3.sourceAccount.AccountID)
	op3, ok3 := newTx3.Operations()[0].(*Payment)
	assert.True(t, ok3)
	assert.Equal(t, "MDQNY3PBOJOKYZSRMK2S7LHHGWZIUISD4QORETLMXEWXBI7KFZZMKAAAAAAMV7V2XYGQO", op3.SourceAccount)
	assert.Equal(t, "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK", op3.Destination)

}

func TestBuild(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639897))
	createAccount := CreateAccount{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "10",
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	expectedUnsigned := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAF9eEAAAAAAAAAAAA="

	expectedSigned := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAF9eEAAAAAAAAAAAHqLnLFAAAAQB7MjKIwNEOTIjbEeV+QIjaQp/ZpV5qpbkbDaU54gkfdTOFOUxZq66lTS5FOfP5fmPIVD8InQ00Usy2SmzFC/wc="

	txeB64, err := tx.Base64()
	assert.NoError(t, err)
	assert.Equal(t, expectedUnsigned, txeB64, "tx envelope should match")
	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)
	txeB64, err = tx.Base64()
	assert.NoError(t, err)
	assert.Equal(t, expectedSigned, txeB64, "tx envelope should match")
}

func TestFromXDRBuildSignEncode(t *testing.T) {
	expectedUnsigned := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAVuZXd0eAAAAAAAAAEAAAAAAAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAF9eEAAAAAAAAAAAHqLnLFAAAAQAz221zc6QuNPFsmBkLMzd1QPXuNbDabMmdh3EutkV71A7DdAPiFzD0TGgm/loJ9TjOiJGpvaJdDCWDXitAT8Qo="

	expectedSigned := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAVuZXd0eAAAAAAAAAEAAAAAAAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAF9eEAAAAAAAAAAALqLnLFAAAAQAz221zc6QuNPFsmBkLMzd1QPXuNbDabMmdh3EutkV71A7DdAPiFzD0TGgm/loJ9TjOiJGpvaJdDCWDXitAT8QrqLnLFAAAAQAz221zc6QuNPFsmBkLMzd1QPXuNbDabMmdh3EutkV71A7DdAPiFzD0TGgm/loJ9TjOiJGpvaJdDCWDXitAT8Qo="

	kp0 := newKeypair0()

	// test signing transaction  without modification
	tx, err := TransactionFromXDR(expectedUnsigned)
	assert.NoError(t, err)
	newTx, ok := tx.Transaction()
	assert.True(t, ok)
	_, ok = tx.FeeBump()
	assert.False(t, ok)

	//passphrase is needed for signing
	newTx, err = newTx.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)
	txeB64, err := newTx.Base64()
	assert.NoError(t, err)
	assert.Equal(t, expectedSigned, txeB64, "tx envelope should match")

	// test signing transaction  with modification
	expectedSigned2 := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAHAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAVuZXd0eAAAAAAAAAEAAAAAAAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAF9eEAAAAAAAAAAAHqLnLFAAAAQFu7obEnMmrp+1Pnz/8o3IUIOWJ6rVTsJO1dAYapN3/zVjCNW3/JzgewGrKNWjPelF7BNRhk5lx93CFGdHDJ/Ac="
	tx, err = TransactionFromXDR(expectedUnsigned)
	assert.NoError(t, err)
	newTx, ok = tx.Transaction()
	assert.True(t, ok)
	_, ok = tx.FeeBump()
	assert.False(t, ok)

	txeB64, err = newSignedTransaction(
		TransactionParams{
			SourceAccount: &SimpleAccount{
				AccountID: newTx.SourceAccount().AccountID,
				Sequence:  newTx.SourceAccount().Sequence + 1,
			},
			IncrementSequenceNum: false,
			Operations:           newTx.Operations(),
			BaseFee:              newTx.BaseFee(),
			Memo:                 MemoText("newtx"),
			Preconditions:        Preconditions{TimeBounds: newTx.Timebounds()},
		},
		network.TestNetworkPassphrase,
		kp0,
	)
	assert.NoError(t, err)
	assert.Equal(t, expectedSigned2, txeB64, "tx envelope should match")
}

func TestSignWithSecretKey(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	txSource := NewSimpleAccount(kp0.Address(), int64(9605939170639897))
	tx1Source := NewSimpleAccount(kp0.Address(), int64(9605939170639897))
	createAccount := CreateAccount{
		Destination:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:        "10",
		SourceAccount: kp1.Address(),
	}

	expected, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	tx1, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &tx1Source,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	tx1, err = tx1.SignWithKeyString(
		network.TestNetworkPassphrase,
		"SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R", ""+
			"SBMSVD4KKELKGZXHBUQTIROWUAPQASDX7KEJITARP4VMZ6KLUHOGPTYW",
	)
	assert.NoError(t, err)

	actual, err := tx1.Base64()
	assert.NoError(t, err)
	assert.Equal(t, expected, actual, "base64 xdr should match")
}

func TestAddSignatureDecorated(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	txSource := NewSimpleAccount(kp0.Address(), int64(9605939170639897))
	tx1Source := NewSimpleAccount(kp0.Address(), int64(9605939170639897))
	createAccount := CreateAccount{
		Destination:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:        "10",
		SourceAccount: kp1.Address(),
	}

	expected, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	tx1, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &tx1Source,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	// Same if signatures added separately.
	{
		var tx1sigs1 *Transaction
		tx1sigs1, err = tx1.AddSignatureDecorated(
			xdr.DecoratedSignature{
				Hint: kp0.Hint(),
				Signature: func() xdr.Signature {
					var sigBytes []byte
					sigBytes, err = base64.StdEncoding.DecodeString("TVogR6tbrWLnOc1BsP/j+Qrxpja2NWNgeRIwujECYscRdMG7AMtnb3dkCT7sqlbSM0TTzlRh7G+BcVocYBtqBw==")
					if err != nil {
						require.NoError(t, err)
					}
					return xdr.Signature(sigBytes)
				}(),
			},
		)
		assert.NoError(t, err)
		tx1sigs1, err = tx1sigs1.AddSignatureDecorated(
			xdr.DecoratedSignature{
				Hint: kp1.Hint(),
				Signature: func() xdr.Signature {
					var sigBytes []byte
					sigBytes, err = base64.StdEncoding.DecodeString("Iy77JteoW/FbeiuViZpgTyvrHP4BnBOeyVOjrdb5O/MpEMwcSlYXAkCBqPt4tBDil4jIcDDLhm7TsN6aUBkIBg==")
					if err != nil {
						require.NoError(t, err)
					}
					return xdr.Signature(sigBytes)
				}(),
			},
		)
		assert.NoError(t, err)
		var actual string
		actual, err = tx1sigs1.Base64()
		assert.NoError(t, err)
		assert.Equal(t, expected, actual, "base64 xdr should match")
	}

	// Same if signatures added together.
	{
		var tx1sigs2 *Transaction
		tx1sigs2, err = tx1.AddSignatureDecorated(
			xdr.DecoratedSignature{
				Hint: kp0.Hint(),
				Signature: func() xdr.Signature {
					var sigBytes []byte
					sigBytes, err = base64.StdEncoding.DecodeString("TVogR6tbrWLnOc1BsP/j+Qrxpja2NWNgeRIwujECYscRdMG7AMtnb3dkCT7sqlbSM0TTzlRh7G+BcVocYBtqBw==")
					if err != nil {
						require.NoError(t, err)
					}
					return xdr.Signature(sigBytes)
				}(),
			},
			xdr.DecoratedSignature{
				Hint: kp1.Hint(),
				Signature: func() xdr.Signature {
					var sigBytes []byte
					sigBytes, err = base64.StdEncoding.DecodeString("Iy77JteoW/FbeiuViZpgTyvrHP4BnBOeyVOjrdb5O/MpEMwcSlYXAkCBqPt4tBDil4jIcDDLhm7TsN6aUBkIBg==")
					if err != nil {
						require.NoError(t, err)
					}
					return xdr.Signature(sigBytes)
				}(),
			},
		)
		assert.NoError(t, err)
		var actual string
		actual, err = tx1sigs2.Base64()
		assert.NoError(t, err)
		assert.Equal(t, expected, actual, "base64 xdr should match")
	}
}

func TestFeeBumpTransaction_AddSignatureDecorated(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()

	tx, err := NewTransaction(TransactionParams{
		SourceAccount: &SimpleAccount{kp0.Address(), int64(9605939170639897)},
		Operations: []Operation{&CreateAccount{
			Destination:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
			Amount:        "10",
			SourceAccount: kp1.Address(),
		}},
		BaseFee:       MinBaseFee,
		Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
	})
	require.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0, kp1)
	require.NoError(t, err)

	fbtx, err := NewFeeBumpTransaction(FeeBumpTransactionParams{
		Inner:      tx,
		FeeAccount: kp0.Address(),
		BaseFee:    MinBaseFee,
	})
	require.NoError(t, err)
	require.Len(t, fbtx.Signatures(), 0)

	fbtxHash, err := fbtx.Hash(network.TestNetworkPassphrase)
	require.NoError(t, err)

	sig, err := kp0.SignDecorated(fbtxHash[:])
	require.NoError(t, err)
	fbtxWithSig, err := fbtx.AddSignatureDecorated(sig)
	require.NoError(t, err)
	require.Len(t, fbtx.Signatures(), 0)
	require.Len(t, fbtxWithSig.Signatures(), 1)

	sig, err = kp1.SignDecorated(fbtxHash[:])
	require.NoError(t, err)
	fbtxWithTwoSigs, err := fbtxWithSig.AddSignatureDecorated(sig)
	require.NoError(t, err)
	require.Len(t, fbtx.Signatures(), 0)
	require.Len(t, fbtxWithSig.Signatures(), 1)
	require.Len(t, fbtxWithTwoSigs.Signatures(), 2)
}

func TestAddSignatureBase64(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	txSource := NewSimpleAccount(kp0.Address(), int64(9605939170639897))
	tx1Source := NewSimpleAccount(kp0.Address(), int64(9605939170639897))
	createAccount := CreateAccount{
		Destination:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:        "10",
		SourceAccount: kp1.Address(),
	}

	expected, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	tx1, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &tx1Source,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	tx1, err = tx1.AddSignatureBase64(
		network.TestNetworkPassphrase,
		"GDQNY3PBOJOKYZSRMK2S7LHHGWZIUISD4QORETLMXEWXBI7KFZZMKTL3",
		"TVogR6tbrWLnOc1BsP/j+Qrxpja2NWNgeRIwujECYscRdMG7AMtnb3dkCT7sqlbSM0TTzlRh7G+BcVocYBtqBw==",
	)
	assert.NoError(t, err)

	tx1, err = tx1.AddSignatureBase64(
		network.TestNetworkPassphrase,
		"GAS4V4O2B7DW5T7IQRPEEVCRXMDZESKISR7DVIGKZQYYV3OSQ5SH5LVP",
		"Iy77JteoW/FbeiuViZpgTyvrHP4BnBOeyVOjrdb5O/MpEMwcSlYXAkCBqPt4tBDil4jIcDDLhm7TsN6aUBkIBg==",
	)
	assert.NoError(t, err)

	actual, err := tx1.Base64()
	assert.NoError(t, err)
	assert.Equal(t, expected, actual, "base64 xdr should match")
}

func TestTransaction_ClearSignatures(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount: &SimpleAccount{AccountID: kp0.Address(), Sequence: int64(9605939170639898)},
			Operations: []Operation{&CreateAccount{
				Destination:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
				Amount:        "10",
				SourceAccount: kp1.Address(),
			}},
			BaseFee:       MinBaseFee,
			Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	require.NoError(t, err)
	require.Len(t, tx.Signatures(), 0)

	txWithSig, err := tx.Sign(network.TestNetworkPassphrase, kp0)
	require.NoError(t, err)
	require.Len(t, txWithSig.Signatures(), 1)

	txSigCleared, err := txWithSig.ClearSignatures()
	require.NoError(t, err)
	require.Len(t, txSigCleared.Signatures(), 0)

	expected, err := tx.Base64()
	assert.NoError(t, err)
	actual, err := txSigCleared.Base64()
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestFeeBumpTransaction_ClearSignatures(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()

	tx, err := NewTransaction(TransactionParams{
		SourceAccount: &SimpleAccount{AccountID: kp0.Address(), Sequence: int64(9605939170639898)},
		Operations: []Operation{&CreateAccount{
			Destination:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
			Amount:        "10",
			SourceAccount: kp1.Address(),
		}},
		BaseFee:       MinBaseFee,
		Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
	})
	require.NoError(t, err)
	require.Len(t, tx.Signatures(), 0)
	txWithSig, err := tx.Sign(network.TestNetworkPassphrase, kp0)
	require.NoError(t, err)
	require.Len(t, txWithSig.Signatures(), 1)

	fbtx, err := NewFeeBumpTransaction(FeeBumpTransactionParams{
		Inner:      txWithSig,
		FeeAccount: kp0.Address(),
		BaseFee:    MinBaseFee,
	})
	require.NoError(t, err)
	require.Len(t, fbtx.Signatures(), 0)

	fbtxWithSig, err := fbtx.Sign(network.TestNetworkPassphrase, kp0)
	require.NoError(t, err)
	require.Len(t, fbtxWithSig.Signatures(), 1)

	fbtxSigCleared, err := fbtxWithSig.ClearSignatures()
	require.NoError(t, err)
	require.Len(t, fbtxSigCleared.Signatures(), 0)

	expected, err := fbtx.Base64()
	assert.NoError(t, err)
	actual, err := fbtxSigCleared.Base64()
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestReadChallengeTx_validSignedByServerAndClient(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP, clientKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, clientKP.Address(), readClientAccountID)
	assert.NoError(t, err)
}

func TestReadChallengeTx_validSignedByServer(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, clientKP.Address(), readClientAccountID)
	assert.NoError(t, err)
}

func TestReadChallengeTx_invalidNotSignedByServer(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, clientKP.Address(), readClientAccountID)
	assert.EqualError(t, err, "transaction not signed by "+serverKP.Address())
}

func TestReadChallengeTx_invalidCorrupted(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	tx64 = strings.ReplaceAll(tx64, "A", "B")
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Nil(t, readTx)
	assert.Equal(t, "", readClientAccountID)
	assert.EqualError(
		t,
		err,
		"could not parse challenge: unable to unmarshal transaction envelope: "+
			"decoding EnvelopeType: '68174086' is not a valid EnvelopeType enum value",
	)
}

func TestReadChallengeTx_invalidServerAccountIDMismatch(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(newKeypair2().Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, "", readClientAccountID)
	assert.EqualError(t, err, "transaction source account is not equal to server's account")
}

func TestReadChallengeTx_invalidSeqNoNotZero(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), 1234)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, "", readClientAccountID)
	assert.EqualError(t, err, "transaction sequence number must be 0")
}

func TestReadChallengeTx_invalidTimeboundsInfinite(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, "", readClientAccountID)
	assert.EqualError(t, err, "transaction requires non-infinite timebounds")
}

func TestReadChallengeTx_invalidTimeboundsOutsideRange(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimebounds(0, 100)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, "", readClientAccountID)
	assert.Error(t, err)
	assert.Regexp(t, "transaction is not within range of the specified timebounds", err.Error())
}

func TestReadChallengeTx_validTimeboundsWithGracePeriod(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	unixNow := time.Now().UTC().Unix()
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimebounds(unixNow+5*59, unixNow+60*60)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, clientKP.Address(), readClientAccountID)
	assert.NoError(t, err)
}

func TestReadChallengeTx_invalidTimeboundsWithGracePeriod(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	unixNow := time.Now().UTC().Unix()
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimebounds(unixNow+5*61, unixNow+60*60)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, "", readClientAccountID)
	assert.Error(t, err)
	assert.Regexp(t, "transaction is not within range of the specified timebounds", err.Error())
}

func TestReadChallengeTx_invalidOperationWrongType(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := BumpSequence{
		SourceAccount: clientKP.Address(),
		BumpTo:        0,
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, "", readClientAccountID)
	assert.EqualError(t, err, "operation type should be manage_data")
}

func TestReadChallengeTx_invalidOperationNoSourceAccount(t *testing.T) {
	serverKP := newKeypair0()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		Name:  "testanchor.stellar.org auth",
		Value: []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	_, _, _, _, err = ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.EqualError(t, err, "operation should have a source account")
}

func TestReadChallengeTx_invalidDataValueWrongEncodedLength(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 45))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, clientKP.Address(), readClientAccountID)
	assert.EqualError(t, err, "random nonce encoded as base64 should be 64 bytes long")
}

func TestReadChallengeTx_invalidDataValueCorruptBase64(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA?AAAAAAAAAAAAAAAAAAAAAAAAAA"),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, clientKP.Address(), readClientAccountID)
	assert.EqualError(t, err, "failed to decode random nonce provided in manage_data operation: illegal base64 data at input byte 37")
}

func TestReadChallengeTx_invalidDataValueWrongByteLength(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 47))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	assert.NoError(t, err)

	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, clientKP.Address(), readClientAccountID)
	assert.EqualError(t, err, "random nonce before encoding as base64 should be 48 bytes long")
}

func TestReadChallengeTx_acceptsV0AndV1Transactions(t *testing.T) {
	kp0 := newKeypair0()
	tx, err := BuildChallengeTx(
		kp0.Seed(),
		kp0.Address(),
		"testwebauth.stellar.org",
		"testanchor.stellar.org",
		network.TestNetworkPassphrase,
		time.Hour,
		nil,
	)
	assert.NoError(t, err)

	originalHash, err := tx.HashHex(network.TestNetworkPassphrase)
	assert.NoError(t, err)

	v1Challenge, err := marshallBase64(tx.envelope, tx.Signatures())
	assert.NoError(t, err)

	convertToV0(tx)
	v0Challenge, err := marshallBase64(tx.envelope, tx.Signatures())
	assert.NoError(t, err)

	for _, challenge := range []string{v1Challenge, v0Challenge} {
		parsedTx, clientAccountID, _, _, err := ReadChallengeTx(
			challenge,
			kp0.Address(),
			network.TestNetworkPassphrase,
			"testwebauth.stellar.org",
			[]string{"testanchor.stellar.org"},
		)
		assert.NoError(t, err)

		assert.Equal(t, kp0.Address(), clientAccountID)

		hash, err := parsedTx.HashHex(network.TestNetworkPassphrase)
		assert.NoError(t, err)
		assert.Equal(t, originalHash, hash)
	}
}

func TestReadChallengeTx_forbidsFeeBumpTransactions(t *testing.T) {
	kp0 := newKeypair0()
	tx, err := BuildChallengeTx(
		kp0.Seed(),
		kp0.Address(),
		"testwebauth.stellar.org",
		"testanchor.stellar.org",
		network.TestNetworkPassphrase,
		time.Hour,
		nil,
	)
	assert.NoError(t, err)

	kp1 := newKeypair1()
	feeBumpTx, err := NewFeeBumpTransaction(
		FeeBumpTransactionParams{
			Inner:      tx,
			FeeAccount: kp1.Address(),
			BaseFee:    3 * MinBaseFee,
		},
	)
	assert.NoError(t, err)

	feeBumpTx, err = feeBumpTx.Sign(network.TestNetworkPassphrase, kp1)
	assert.NoError(t, err)

	challenge, err := feeBumpTx.Base64()
	assert.NoError(t, err)
	_, _, _, _, err = ReadChallengeTx(
		challenge,
		kp0.Address(),
		network.TestNetworkPassphrase,
		"testwebauth.stellar.org",
		[]string{"testanchor.stellar.org"},
	)
	assert.EqualError(t, err, "challenge cannot be a fee bump transaction")
}

func TestReadChallengeTx_allowsMuxedAccountsForClientAccountId(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	aid := xdr.MustAddress(kp1.Address())
	muxedAccount := xdr.MuxedAccount{
		Type: xdr.CryptoKeyTypeKeyTypeMuxedEd25519,
		Med25519: &xdr.MuxedAccountMed25519{
			Id:      0xcafebabe,
			Ed25519: *aid.Ed25519,
		},
	}
	tx, err := BuildChallengeTx(
		kp0.Seed(),
		muxedAccount.Address(),
		"testwebauth.stellar.org",
		"testanchor.stellar.org",
		network.TestNetworkPassphrase,
		time.Hour,
		nil,
	)
	assert.NoError(t, err)

	challenge, err := marshallBase64(tx.ToXDR(), tx.Signatures())
	assert.NoError(t, err)

	tx, _, _, _, err = ReadChallengeTx(
		challenge,
		kp0.Address(),
		network.TestNetworkPassphrase,
		"testwebauth.stellar.org",
		[]string{"testanchor.stellar.org"},
	)
	assert.NoError(t, err)
	assert.Equal(t, tx.envelope.Operations()[0].SourceAccount, &muxedAccount)
}

func TestReadChallengeTransaction_forbidsMuxedTxSourceAccount(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	homeDomain := "testanchor.stellar.org"
	webAuthDomain := "testwebauth.stellar.org"

	aid := xdr.MustAddress(kp0.Address())
	muxedAccount := xdr.MuxedAccount{
		Type: xdr.CryptoKeyTypeKeyTypeMuxedEd25519,
		Med25519: &xdr.MuxedAccountMed25519{
			Id:      0xcafebabe,
			Ed25519: *aid.Ed25519,
		},
	}
	serverAccount := SimpleAccount{
		AccountID: muxedAccount.Address(),
		Sequence:  0,
	}
	randomNonce, _ := generateRandomNonce(48)
	randomNonceToString := base64.StdEncoding.EncodeToString(randomNonce)
	currentTime := time.Now().UTC()
	maxTime := currentTime.Add(300)

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &serverAccount,
			IncrementSequenceNum: false,
			Operations: []Operation{
				&ManageData{
					SourceAccount: kp1.Address(),
					Name:          homeDomain + " auth",
					Value:         []byte(randomNonceToString),
				},
				&ManageData{
					SourceAccount: serverAccount.GetAccountID(),
					Name:          "web_auth_domain",
					Value:         []byte(webAuthDomain),
				},
			},
			BaseFee: MinBaseFee,
			Memo:    MemoID(1),
			Preconditions: Preconditions{
				TimeBounds: NewTimebounds(currentTime.Unix(), maxTime.Unix()),
			},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)

	challenge, err := marshallBase64(tx.ToXDR(), tx.Signatures())
	assert.NoError(t, err)

	_, _, _, _, err = ReadChallengeTx(
		challenge,
		kp0.Address(),
		network.TestNetworkPassphrase,
		webAuthDomain,
		[]string{homeDomain},
	)
	assert.EqualError(t, err, "invalid source account: only valid Ed25519 accounts are allowed in challenge transactions")
}

func TestReadChallengeTransaction_forbidsMemoWithMuxedClientAccount(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	homeDomain := "testanchor.stellar.org"
	webAuthDomain := "testwebauth.stellar.org"

	serverAccount := SimpleAccount{
		AccountID: kp0.Address(),
		Sequence:  0,
	}
	aid := xdr.MustAddress(kp1.Address())
	muxedAccount := xdr.MuxedAccount{
		Type: xdr.CryptoKeyTypeKeyTypeMuxedEd25519,
		Med25519: &xdr.MuxedAccountMed25519{
			Id:      0xcafebabe,
			Ed25519: *aid.Ed25519,
		},
	}
	randomNonce, _ := generateRandomNonce(48)
	randomNonceToString := base64.StdEncoding.EncodeToString(randomNonce)
	currentTime := time.Now().UTC()
	maxTime := currentTime.Add(time.Second * 300)

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &serverAccount,
			IncrementSequenceNum: false,
			Operations: []Operation{
				&ManageData{
					SourceAccount: muxedAccount.Address(),
					Name:          homeDomain + " auth",
					Value:         []byte(randomNonceToString),
				},
				&ManageData{
					SourceAccount: serverAccount.GetAccountID(),
					Name:          "web_auth_domain",
					Value:         []byte(webAuthDomain),
				},
			},
			BaseFee: MinBaseFee,
			Memo:    MemoID(1),
			Preconditions: Preconditions{
				TimeBounds: NewTimebounds(currentTime.Unix(), maxTime.Unix()),
			},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)

	challenge, err := marshallBase64(tx.ToXDR(), tx.Signatures())
	assert.NoError(t, err)

	_, _, _, _, err = ReadChallengeTx(
		challenge,
		kp0.Address(),
		network.TestNetworkPassphrase,
		webAuthDomain,
		[]string{homeDomain},
	)
	assert.EqualError(t, err, "memos are not valid for challenge transactions with a muxed client account")
}

func TestReadChallengeTransaction_forbidsNonIdMemo(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	homeDomain := "testanchor.stellar.org"
	webAuthDomain := "testwebauth.stellar.org"

	serverAccount := SimpleAccount{
		AccountID: kp0.Address(),
		Sequence:  0,
	}
	randomNonce, _ := generateRandomNonce(48)
	randomNonceToString := base64.StdEncoding.EncodeToString(randomNonce)
	currentTime := time.Now().UTC()
	maxTime := currentTime.Add(300)

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &serverAccount,
			IncrementSequenceNum: false,
			Operations: []Operation{
				&ManageData{
					SourceAccount: kp1.Address(),
					Name:          homeDomain + " auth",
					Value:         []byte(randomNonceToString),
				},
				&ManageData{
					SourceAccount: serverAccount.GetAccountID(),
					Name:          "web_auth_domain",
					Value:         []byte(webAuthDomain),
				},
			},
			BaseFee: MinBaseFee,
			Memo:    MemoText("test"),
			Preconditions: Preconditions{
				TimeBounds: NewTimebounds(currentTime.Unix(), maxTime.Unix()),
			},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)

	challenge, err := marshallBase64(tx.ToXDR(), tx.Signatures())
	assert.NoError(t, err)

	_, _, _, _, err = ReadChallengeTx(
		challenge,
		kp0.Address(),
		network.TestNetworkPassphrase,
		webAuthDomain,
		[]string{homeDomain},
	)
	assert.EqualError(t, err, "invalid memo, only ID memos are permitted")
}

func TestReadChallengeTx_doesVerifyHomeDomainFailure(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	_, _, _, _, err = ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"willfail"})
	assert.EqualError(t, err, "operation key does not match any homeDomains passed (key=\"testanchor.stellar.org auth\", homeDomains=[willfail])")
}

func TestReadChallengeTx_doesVerifyHomeDomainSuccess(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	_, _, _, _, err = ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, nil, err)
}

func TestReadChallengeTx_allowsAdditionalManageDataOpsWithSourceAccountSetToServerAccount(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	op2 := ManageData{
		SourceAccount: txSource.AccountID,
		Name:          "testanchor.stellar.org auth",
		Value:         []byte("a value"),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &op2, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, clientKP.Address(), readClientAccountID)
	assert.NoError(t, err)
}

func TestReadChallengeTx_disallowsAdditionalManageDataOpsWithoutSourceAccountSetToServerAccount(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	op2 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "a key",
		Value:         []byte("a value"),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &op2, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	_, _, _, _, err = ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.EqualError(t, err, "subsequent operations are unrecognized")
}

func TestReadChallengeTx_disallowsAdditionalOpsOfOtherTypes(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	op2 := BumpSequence{
		SourceAccount: txSource.AccountID,
		BumpTo:        0,
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &op2, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	readTx, readClientAccountID, _, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.Equal(t, tx, readTx)
	assert.Equal(t, clientKP.Address(), readClientAccountID)
	assert.EqualError(t, err, "operation type should be manage_data")
}

func TestReadChallengeTx_matchesHomeDomain(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	_, _, matchedHomeDomain, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	require.NoError(t, err)
	assert.Equal(t, matchedHomeDomain, "testanchor.stellar.org")
}

func TestReadChallengeTx_doesNotMatchHomeDomain(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	_, _, matchedHomeDomain, _, err := ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"not", "going", "to", "match"})
	assert.Equal(t, matchedHomeDomain, "")
	assert.EqualError(t, err, "operation key does not match any homeDomains passed (key=\"testanchor.stellar.org auth\", homeDomains=[not going to match])")
}

func TestReadChallengeTx_validWhenWebAuthDomainMissing(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	_, _, _, _, err = ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.NoError(t, err)
}

func TestReadChallengeTx_invalidWebAuthDomainSourceAccount(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	_, _, _, _, err = ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.EqualError(t, err, `web auth domain operation must have server source account`)
}

func TestReadChallengeTx_invalidWebAuthDomain(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.example.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)
	_, _, _, _, err = ReadChallengeTx(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.EqualError(t, err, `web auth domain operation value is "testwebauth.example.org" but expect "testwebauth.stellar.org"`)
}

func TestVerifyChallengeTxThreshold_invalidServer(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}

	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		clientKP,
	)
	assert.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}
	signersFound, err := VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.Empty(t, signersFound)
	assert.EqualError(t, err, "transaction not signed by "+serverKP.Address())
}

func TestVerifyChallengeTxThreshold_validServerAndClientKeyMeetingThreshold(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}
	wantSigners := []string{
		clientKP.Address(),
	}

	signersFound, err := VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.ElementsMatch(t, wantSigners, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxThreshold_validServerAndMultipleClientKeyMeetingThreshold(t *testing.T) {
	serverKP := newKeypair0()
	clientKP1 := newKeypair1()
	clientKP2 := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP1.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP1, clientKP2,
	)
	assert.NoError(t, err)

	threshold := Threshold(3)
	signerSummary := map[string]int32{
		clientKP1.Address(): 1,
		clientKP2.Address(): 2,
	}
	wantSigners := []string{
		clientKP1.Address(),
		clientKP2.Address(),
	}

	signersFound, err := VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.ElementsMatch(t, wantSigners, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxThreshold_validServerAndMultipleClientKeyMeetingThresholdSomeUnused(t *testing.T) {
	serverKP := newKeypair0()
	clientKP1 := newKeypair1()
	clientKP2 := newKeypair2()
	clientKP3 := keypair.MustRandom()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP1.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	threshold := Threshold(3)
	signerSummary := SignerSummary{
		clientKP1.Address(): 1,
		clientKP2.Address(): 2,
		clientKP3.Address(): 2,
	}
	wantSigners := []string{
		clientKP1.Address(),
		clientKP2.Address(),
	}

	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP1, clientKP2,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.ElementsMatch(t, wantSigners, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxThreshold_validServerAndMultipleClientKeyMeetingThresholdSomeUnusedIgnorePreauthTxHashAndXHash(t *testing.T) {
	serverKP := newKeypair0()
	clientKP1 := newKeypair1()
	clientKP2 := newKeypair2()
	clientKP3 := keypair.MustRandom()
	preauthTxHash := "TAQCSRX2RIDJNHFIFHWD63X7D7D6TRT5Y2S6E3TEMXTG5W3OECHZ2OG4"
	xHash := "XDRPF6NZRR7EEVO7ESIWUDXHAOMM2QSKIQQBJK6I2FB7YKDZES5UCLWD"
	unknownSignerType := "?ARPF6NZRR7EEVO7ESIWUDXHAOMM2QSKIQQBJK6I2FB7YKDZES5UCLWD"
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP1.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	threshold := Threshold(3)
	signerSummary := SignerSummary{
		clientKP1.Address(): 1,
		clientKP2.Address(): 2,
		clientKP3.Address(): 2,
		preauthTxHash:       10,
		xHash:               10,
		unknownSignerType:   10,
	}
	wantSigners := []string{
		clientKP1.Address(),
		clientKP2.Address(),
	}

	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP1, clientKP2,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.ElementsMatch(t, wantSigners, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxThreshold_invalidServerAndMultipleClientKeyNotMeetingThreshold(t *testing.T) {
	serverKP := newKeypair0()
	clientKP1 := newKeypair1()
	clientKP2 := newKeypair2()
	clientKP3 := keypair.MustRandom()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP1.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	threshold := Threshold(10)
	signerSummary := SignerSummary{
		clientKP1.Address(): 1,
		clientKP2.Address(): 2,
		clientKP3.Address(): 2,
	}

	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP1, clientKP2,
	)
	assert.NoError(t, err)

	_, err = VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.EqualError(t, err, "signers with weight 3 do not meet threshold 10")
}

func TestVerifyChallengeTxThreshold_invalidClientKeyUnrecognized(t *testing.T) {
	serverKP := newKeypair0()
	clientKP1 := newKeypair1()
	clientKP2 := newKeypair2()
	clientKP3 := keypair.MustRandom()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP1.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	threshold := Threshold(10)
	signerSummary := map[string]int32{
		clientKP1.Address(): 1,
		clientKP2.Address(): 2,
	}

	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP1, clientKP2, clientKP3,
	)
	assert.NoError(t, err)

	_, err = VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.EqualError(t, err, "transaction has unrecognized signatures")
}

func TestVerifyChallengeTxThreshold_invalidNoSigners(t *testing.T) {
	serverKP := newKeypair0()
	clientKP1 := newKeypair1()
	clientKP2 := newKeypair2()
	clientKP3 := keypair.MustRandom()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP1.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	threshold := Threshold(10)
	signerSummary := SignerSummary{}

	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP1, clientKP2, clientKP3,
	)
	assert.NoError(t, err)

	_, err = VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.EqualError(t, err, "no verifiable signers provided, at least one G... address must be provided")
}

func TestVerifyChallengeTxThreshold_weightsAddToMoreThan8Bits(t *testing.T) {
	serverKP := newKeypair0()
	clientKP1 := newKeypair1()
	clientKP2 := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP1.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP1, clientKP2,
	)
	assert.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP1.Address(): 255,
		clientKP2.Address(): 1,
	}
	wantSigners := []string{
		clientKP1.Address(),
		clientKP2.Address(),
	}

	signersFound, err := VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.ElementsMatch(t, wantSigners, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxThreshold_matchesHomeDomain(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	_, err = tx.Base64()
	require.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}

	tx, err = tx.Sign(network.TestNetworkPassphrase, clientKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)

	_, err = VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	require.NoError(t, err)
}

func TestVerifyChallengeTxThreshold_doesNotMatchHomeDomain(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}

	_, err = VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"not", "going", "to", "match"}, threshold, signerSummary)
	assert.EqualError(t, err, "operation key does not match any homeDomains passed (key=\"testanchor.stellar.org auth\", homeDomains=[not going to match])")
}

func TestVerifyChallengeTxThreshold_doesVerifyHomeDomainFailure(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "will fail auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}

	_, err = VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.EqualError(t, err, "operation key does not match any homeDomains passed (key=\"will fail auth\", homeDomains=[testanchor.stellar.org])")
}

func TestVerifyChallengeTxThreshold_doesVerifyHomeDomainSuccess(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}
	wantSigners := []string{
		clientKP.Address(),
	}

	signers, err := VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.Equal(t, nil, err)
	assert.Equal(t, signers, wantSigners)
}

func TestVerifyChallengeTxThreshold_allowsAdditionalManageDataOpsWithSourceAccountSetToServerAccount(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	op2 := ManageData{
		SourceAccount: txSource.AccountID,
		Name:          "a key",
		Value:         []byte("a value"),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &op2, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}
	wantSigners := []string{
		clientKP.Address(),
	}

	signersFound, err := VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.ElementsMatch(t, wantSigners, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxThreshold_disallowsAdditionalManageDataOpsWithoutSourceAccountSetToServerAccount(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	op2 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "a key",
		Value:         []byte("a value"),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &op2, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}

	signersFound, err := VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.Empty(t, signersFound)
	assert.EqualError(t, err, "subsequent operations are unrecognized")
}

func TestVerifyChallengeTxThreshold_disallowsAdditionalOpsOfOtherTypes(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	op2 := BumpSequence{
		SourceAccount: txSource.AccountID,
		BumpTo:        0,
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &op2, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}

	signersFound, err := VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.Empty(t, signersFound)
	assert.EqualError(t, err, "operation type should be manage_data")
}

func TestVerifyChallengeTxThreshold_validWhenWebAuthDomainMissing(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}
	wantSigners := []string{
		clientKP.Address(),
	}

	signersFound, err := VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.ElementsMatch(t, wantSigners, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxThreshold_invalidWebAuthDomainSourceAccount(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}

	_, err = VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.EqualError(t, err, `web auth domain operation must have server source account`)
}

func TestVerifyChallengeTxThreshold_invalidWebAuthDomain(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.example.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	threshold := Threshold(1)
	signerSummary := SignerSummary{
		clientKP.Address(): 1,
	}

	_, err = VerifyChallengeTxThreshold(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, threshold, signerSummary)
	assert.EqualError(t, err, `web auth domain operation value is "testwebauth.example.org" but expect "testwebauth.stellar.org"`)
}

func TestVerifyChallengeTxSigners_invalidServer(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		clientKP,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address())
	assert.Empty(t, signersFound)
	assert.EqualError(t, err, "transaction not signed by "+serverKP.Address())
}

func TestVerifyChallengeTxSigners_validServerAndClientMasterKey(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address())
	assert.Equal(t, []string{clientKP.Address()}, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxSigners_invalidServerAndNoClient(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address())
	assert.Empty(t, signersFound)
	assert.EqualError(t, err, "transaction not signed by "+clientKP.Address())
}

func TestVerifyChallengeTxSigners_invalidServerAndUnrecognizedClient(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	unrecognizedKP := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, unrecognizedKP,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address())
	assert.Empty(t, signersFound)
	assert.EqualError(t, err, "transaction not signed by "+clientKP.Address())
}

func TestVerifyChallengeTxSigners_validServerAndMultipleClientSigners(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	clientKP2 := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP, clientKP2,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address(), clientKP2.Address())
	assert.Equal(t, []string{clientKP.Address(), clientKP2.Address()}, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxSigners_validServerAndMultipleClientSignersReverseOrder(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	clientKP2 := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP2, clientKP,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address(), clientKP2.Address())
	assert.Equal(t, []string{clientKP.Address(), clientKP2.Address()}, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxSigners_validServerAndClientSignersNotMasterKey(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	clientKP2 := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP2,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP2.Address())
	assert.Equal(t, []string{clientKP2.Address()}, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxSigners_validServerAndClientSignersIgnoresServerSigner(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	clientKP2 := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP2,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, serverKP.Address(), clientKP2.Address())
	assert.Equal(t, []string{clientKP2.Address()}, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxSigners_invalidServerNoClientSignersIgnoresServerSigner(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	clientKP2 := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, serverKP.Address(), clientKP2.Address())
	assert.Empty(t, signersFound)
	assert.EqualError(t, err, "transaction not signed by "+clientKP2.Address())
}

func TestVerifyChallengeTxSigners_validServerAndClientSignersIgnoresDuplicateSigner(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	clientKP2 := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP2,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP2.Address(), clientKP2.Address())
	assert.Equal(t, []string{clientKP2.Address()}, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxSigners_validIgnorePreauthTxHashAndXHash(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	clientKP2 := newKeypair2()
	preauthTxHash := "TAQCSRX2RIDJNHFIFHWD63X7D7D6TRT5Y2S6E3TEMXTG5W3OECHZ2OG4"
	xHash := "XDRPF6NZRR7EEVO7ESIWUDXHAOMM2QSKIQQBJK6I2FB7YKDZES5UCLWD"
	unknownSignerType := "?ARPF6NZRR7EEVO7ESIWUDXHAOMM2QSKIQQBJK6I2FB7YKDZES5UCLWD"
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP2,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP2.Address(), preauthTxHash, xHash, unknownSignerType)
	assert.Equal(t, []string{clientKP2.Address()}, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxSigners_invalidServerAndClientSignersIgnoresDuplicateSignerInError(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	clientKP2 := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP2,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address(), clientKP.Address())
	assert.Empty(t, signersFound)
	assert.EqualError(t, err, "transaction not signed by "+clientKP.Address())
}

func TestVerifyChallengeTxSigners_invalidServerAndClientSignersFailsDuplicateSignatures(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	clientKP2 := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP2, clientKP2,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP2.Address())
	assert.Equal(t, []string{clientKP2.Address()}, signersFound)
	assert.EqualError(t, err, "transaction has unrecognized signatures")
}

func TestVerifyChallengeTxSigners_invalidServerAndClientSignersFailsSignerSeed(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	clientKP2 := newKeypair2()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP2,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP2.Seed())
	assert.Empty(t, signersFound)
	assert.EqualError(t, err, "no verifiable signers provided, at least one G... address must be provided")
}

func TestVerifyChallengeTxSigners_invalidNoSigners(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	_, err = VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"})
	assert.EqualError(t, err, "no verifiable signers provided, at least one G... address must be provided")
}

func TestVerifyChallengeTxSigners_doesVerifyHomeDomainFailure(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	_, err = VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"validation failed"}, clientKP.Address())
	assert.EqualError(t, err, "operation key does not match any homeDomains passed (key=\"testanchor.stellar.org auth\", homeDomains=[validation failed])")
}

func TestVerifyChallengeTxSigners_matchesHomeDomain(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	_, err = tx.Base64()
	require.NoError(t, err)

	signers := []string{clientKP.Address()}
	tx, err = tx.Sign(network.TestNetworkPassphrase, clientKP)
	require.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)

	_, err = VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, signers...)
	require.NoError(t, err)
}

func TestVerifyChallengeTxSigners_doesNotMatchHomeDomain(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(300)},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, serverKP)
	assert.NoError(t, err)
	_, err = tx.Base64()
	require.NoError(t, err)

	signers := []string{clientKP.Address()}
	tx, err = tx.Sign(network.TestNetworkPassphrase, clientKP)
	require.NoError(t, err)
	tx64, err := tx.Base64()
	require.NoError(t, err)

	_, err = VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"not", "going", "to", "match"}, signers...)
	assert.EqualError(t, err, "operation key does not match any homeDomains passed (key=\"testanchor.stellar.org auth\", homeDomains=[not going to match])")
}

func TestVerifyChallengeTxSigners_doesVerifyHomeDomainSuccess(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	_, err = VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address())
	assert.Equal(t, nil, err)
}

func TestVerifyChallengeTxSigners_allowsAdditionalManageDataOpsWithSourceAccountSetToServerAccount(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	op2 := ManageData{
		SourceAccount: txSource.AccountID,
		Name:          "a key",
		Value:         []byte("a value"),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &op2, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address())
	assert.Equal(t, []string{clientKP.Address()}, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxSigners_disallowsAdditionalManageDataOpsWithoutSourceAccountSetToServerAccount(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	op2 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "a key",
		Value:         []byte("a value"),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &op2, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address())
	assert.Empty(t, signersFound)
	assert.EqualError(t, err, "subsequent operations are unrecognized")
}

func TestVerifyChallengeTxSigners_disallowsAdditionalOpsOfOtherTypes(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)

	op1 := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	op2 := BumpSequence{
		SourceAccount: txSource.AccountID,
		BumpTo:        0,
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &op2, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address())
	assert.Empty(t, signersFound)
	assert.EqualError(t, err, "operation type should be manage_data")
}

func TestVerifyChallengeTxSigners_validWhenWebAuthDomainMissing(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	wantSigners := []string{clientKP.Address()}

	signersFound, err := VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address())
	assert.ElementsMatch(t, wantSigners, signersFound)
	assert.NoError(t, err)
}

func TestVerifyChallengeTxSigners_invalidWebAuthDomainSourceAccount(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.stellar.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	_, err = VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address())
	assert.EqualError(t, err, `web auth domain operation must have server source account`)
}

func TestVerifyChallengeTxSigners_invalidWebAuthDomain(t *testing.T) {
	serverKP := newKeypair0()
	clientKP := newKeypair1()
	txSource := NewSimpleAccount(serverKP.Address(), -1)
	op := ManageData{
		SourceAccount: clientKP.Address(),
		Name:          "testanchor.stellar.org auth",
		Value:         []byte(base64.StdEncoding.EncodeToString(make([]byte, 48))),
	}
	webAuthDomainOp := ManageData{
		SourceAccount: serverKP.Address(),
		Name:          "web_auth_domain",
		Value:         []byte("testwebauth.example.org"),
	}
	tx64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op, &webAuthDomainOp},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
		network.TestNetworkPassphrase,
		serverKP, clientKP,
	)
	assert.NoError(t, err)

	_, err = VerifyChallengeTxSigners(tx64, serverKP.Address(), network.TestNetworkPassphrase, "testwebauth.stellar.org", []string{"testanchor.stellar.org"}, clientKP.Address())
	assert.EqualError(t, err, `web auth domain operation value is "testwebauth.example.org" but expect "testwebauth.stellar.org"`)
}

func TestVerifyTxSignatureUnsignedTx(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	txSource := NewSimpleAccount(kp0.Address(), int64(9605939170639897))
	createAccount := CreateAccount{
		Destination:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:        "10",
		SourceAccount: kp1.Address(),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewTimeout(1000)},
		},
	)
	assert.NoError(t, err)

	err = verifyTxSignature(tx, network.TestNetworkPassphrase, kp0.Address())
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "transaction not signed by GDQNY3PBOJOKYZSRMK2S7LHHGWZIUISD4QORETLMXEWXBI7KFZZMKTL3")
	}
}

func TestVerifyTxSignatureSingle(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	txSource := NewSimpleAccount(kp0.Address(), int64(9605939170639897))
	createAccount := CreateAccount{
		Destination:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:        "10",
		SourceAccount: kp1.Address(),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)
	err = verifyTxSignature(tx, network.TestNetworkPassphrase, kp0.Address())
	assert.NoError(t, err)
}

func TestVerifyTxSignatureMultiple(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	txSource := NewSimpleAccount(kp0.Address(), int64(9605939170639897))
	createAccount := CreateAccount{
		Destination:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:        "10",
		SourceAccount: kp1.Address(),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	// verify tx with multiple signature
	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0, kp1)
	assert.NoError(t, err)
	err = verifyTxSignature(tx, network.TestNetworkPassphrase, kp0.Address())
	assert.NoError(t, err)
	err = verifyTxSignature(tx, network.TestNetworkPassphrase, kp1.Address())
	assert.NoError(t, err)
}
func TestVerifyTxSignatureInvalid(t *testing.T) {
	kp0 := newKeypair0()
	kp1 := newKeypair1()
	txSource := NewSimpleAccount(kp0.Address(), int64(9605939170639897))
	createAccount := CreateAccount{
		Destination:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:        "10",
		SourceAccount: kp1.Address(),
	}
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &txSource,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	// verify invalid signer
	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0, kp1)
	assert.NoError(t, err)
	err = verifyTxSignature(tx, network.TestNetworkPassphrase, "GATBMIXTHXYKSUZSZUEJKACZ2OS2IYUWP2AIF3CA32PIDLJ67CH6Y5UY")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "transaction not signed by GATBMIXTHXYKSUZSZUEJKACZ2OS2IYUWP2AIF3CA32PIDLJ67CH6Y5UY")
	}
}

func TestClaimableBalanceIds(t *testing.T) {
	aKeys := keypair.MustParseFull("SC4REDCJNPFAYW4SMH44KNGO5JRDQ72G4HE6GILRBSICI3M2IUOC7AAL")
	A := "MDUJNO4HVE4YCQHV7LINPWVDQJFSAPHHUNSTT64YRBCCRZ5UYUXAWAAAAAAAAAAE2IUOE"
	B := "GCACCFMIWJAHUUASSE2WC7V6VVDLYRLSJYZ3DJEXCG523FSHTNII6KOG"
	aMuxedAccount := NewSimpleAccount(A, int64(5894915628204034))

	// Create the operation and submit it in a transaction.
	claimableBalanceEntry := CreateClaimableBalance{
		Destinations: []Claimant{
			NewClaimant(B, &UnconditionalPredicate),
		},
		Asset:  NativeAsset{},
		Amount: "420",
	}

	// Build, sign, and submit the transaction
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &aMuxedAccount,
			IncrementSequenceNum: true,
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			Operations:           []Operation{&claimableBalanceEntry},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, aKeys)
	assert.NoError(t, err)
	calculatedBalanceId, err := tx.ClaimableBalanceID(0)
	assert.NoError(t, err)

	var txResult xdr.TransactionResult
	resultXdr := "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAAOAAAAAAAAAABw2JZZYIt4n/WXKcnDow3mbTBMPrOnldetgvGUlpTSEQAAAAA="
	err = xdr.SafeUnmarshalBase64(resultXdr, &txResult)
	assert.NoError(t, err)

	results, ok := txResult.OperationResults()
	assert.True(t, ok)

	// We look at the first result since our first (and only) operation in the
	// transaction was the CreateClaimableBalanceOp.
	operationResult := results[0].MustTr().CreateClaimableBalanceResult
	actualBalanceId, err := xdr.MarshalHex(operationResult.BalanceId)
	assert.NoError(t, err)

	assert.Equal(t, actualBalanceId, calculatedBalanceId)
}

func TestTransaction_marshalUnmarshalText(t *testing.T) {
	k := keypair.MustRandom()

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &SimpleAccount{AccountID: k.Address(), Sequence: 1},
			IncrementSequenceNum: false,
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			Operations:           []Operation{&BumpSequence{BumpTo: 2}},
		},
	)
	assert.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, k)
	assert.NoError(t, err)

	b64, err := tx.Base64()
	require.NoError(t, err)
	t.Log("tx base64:", b64)

	marshaled, err := tx.MarshalText()
	require.NoError(t, err)
	t.Log("tx marshaled text:", string(marshaled))
	assert.Equal(t, b64, string(marshaled))

	tx2 := &Transaction{}
	err = tx2.UnmarshalText(marshaled)
	require.NoError(t, err)
	assert.Equal(t, tx, tx2)

	err = (&FeeBumpTransaction{}).UnmarshalText(marshaled)
	assert.EqualError(t, err, "transaction envelope unmarshaled into Transaction is not a transaction")
}

func TestFeeBumpTransaction_marshalUnmarshalText(t *testing.T) {
	k := keypair.MustRandom()

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &SimpleAccount{AccountID: k.Address(), Sequence: 1},
			IncrementSequenceNum: false,
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			Operations:           []Operation{&BumpSequence{BumpTo: 2}},
		},
	)
	require.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, k)
	require.NoError(t, err)

	fbtx, err := NewFeeBumpTransaction(FeeBumpTransactionParams{
		Inner:      tx,
		FeeAccount: k.Address(),
		BaseFee:    MinBaseFee,
	})
	require.NoError(t, err)

	b64, err := fbtx.Base64()
	require.NoError(t, err)
	t.Log("tx base64:", b64)

	marshaled, err := fbtx.MarshalText()
	require.NoError(t, err)
	t.Log("tx marshaled text:", string(marshaled))
	assert.Equal(t, b64, string(marshaled))

	fbtx2 := &FeeBumpTransaction{}
	err = fbtx2.UnmarshalText(marshaled)
	require.NoError(t, err)
	assert.Equal(t, fbtx, fbtx2)

	err = (&Transaction{}).UnmarshalText(marshaled)
	assert.EqualError(t, err, "transaction envelope unmarshaled into FeeBumpTransaction is not a fee bump transaction")
}

func TestNewGenericTransactionWithTransaction(t *testing.T) {
	k := keypair.MustRandom()
	txIn, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &SimpleAccount{AccountID: k.Address(), Sequence: 1},
			IncrementSequenceNum: false,
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			Operations:           []Operation{&BumpSequence{BumpTo: 2}},
		},
	)
	require.NoError(t, err)

	gtx := NewGenericTransactionWithTransaction(txIn)
	txOut, ok := gtx.Transaction()
	require.True(t, ok)
	require.Equal(t, txIn, txOut)
}

func TestNewGenericTransactionWithFeeBumpTransaction(t *testing.T) {
	k := keypair.MustRandom()
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &SimpleAccount{AccountID: k.Address(), Sequence: 1},
			IncrementSequenceNum: false,
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			Operations:           []Operation{&BumpSequence{BumpTo: 2}},
		},
	)
	require.NoError(t, err)
	fbtxIn, err := NewFeeBumpTransaction(FeeBumpTransactionParams{
		Inner:      tx,
		FeeAccount: k.Address(),
		BaseFee:    MinBaseFee,
	})
	require.NoError(t, err)

	gtx := NewGenericTransactionWithFeeBumpTransaction(fbtxIn)
	fbtxOut, ok := gtx.FeeBump()
	require.True(t, ok)
	require.Equal(t, fbtxIn, fbtxOut)
}

func TestGenericTransaction_marshalUnmarshalText(t *testing.T) {
	k := keypair.MustRandom()

	// GenericTransaction containing nothing.
	gtx := &GenericTransaction{}
	_, err := gtx.MarshalText()
	assert.EqualError(t, err, "unable to marshal empty GenericTransaction")

	// GenericTransaction containing a Transaction.
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &SimpleAccount{AccountID: k.Address(), Sequence: 1},
			IncrementSequenceNum: false,
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			Operations:           []Operation{&BumpSequence{BumpTo: 2}},
		},
	)
	require.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, k)
	require.NoError(t, err)
	txb64, err := tx.Base64()
	require.NoError(t, err)
	t.Log("tx base64:", txb64)

	gtx = tx.ToGenericTransaction()
	marshaled, err := gtx.MarshalText()
	require.NoError(t, err)
	t.Log("tx marshaled text:", string(marshaled))
	assert.Equal(t, txb64, string(marshaled))
	gtx2 := &GenericTransaction{}
	err = gtx2.UnmarshalText(marshaled)
	require.NoError(t, err)
	assert.Equal(t, gtx, gtx2)

	// GenericTransaction containing a FeeBumpTransaction.
	fbtx, err := NewFeeBumpTransaction(FeeBumpTransactionParams{
		Inner:      tx,
		FeeAccount: k.Address(),
		BaseFee:    MinBaseFee,
	})
	require.NoError(t, err)
	fbtxb64, err := fbtx.Base64()
	require.NoError(t, err)
	t.Log("fbtx base64:", fbtxb64)

	fbgtx := fbtx.ToGenericTransaction()
	marshaled, err = fbgtx.MarshalText()
	require.NoError(t, err)
	t.Log("fbtx marshaled text:", string(marshaled))
	assert.Equal(t, fbtxb64, string(marshaled))
	fbgtx2 := &GenericTransaction{}
	err = fbgtx2.UnmarshalText(marshaled)
	require.NoError(t, err)
	assert.Equal(t, fbgtx, fbgtx2)
}

func TestGenericTransaction_marshalBinary(t *testing.T) {
	k := keypair.MustRandom()

	// GenericTransaction containing nothing.
	gtx := &GenericTransaction{}
	_, err := gtx.MarshalBinary()
	assert.EqualError(t, err, "unable to marshal empty GenericTransaction")

	// GenericTransaction containing a Transaction.
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &SimpleAccount{AccountID: k.Address(), Sequence: 1},
			IncrementSequenceNum: false,
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			Operations:           []Operation{&BumpSequence{BumpTo: 2}},
		},
	)
	require.NoError(t, err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, k)
	require.NoError(t, err)
	txb, err := tx.MarshalBinary()
	require.NoError(t, err)

	gtx = tx.ToGenericTransaction()
	marshaled, err := gtx.MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, txb, marshaled)

	// GenericTransaction containing a FeeBumpTransaction.
	fbtx, err := NewFeeBumpTransaction(FeeBumpTransactionParams{
		Inner:      tx,
		FeeAccount: k.Address(),
		BaseFee:    MinBaseFee,
	})
	require.NoError(t, err)
	fbtxb, err := fbtx.MarshalBinary()
	require.NoError(t, err)

	fbgtx := fbtx.ToGenericTransaction()
	marshaled, err = fbgtx.MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, fbtxb, marshaled)
}

func TestGenericTransaction_HashHex(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639897))

	createAccount := CreateAccount{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "10",
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)

	gtx := tx.ToGenericTransaction()

	expected := "1b3905ba8c3c0ecc68ae812f2d77f27c697195e8daf568740fc0f5662f65f759"
	hashHex, err := tx.HashHex(network.TestNetworkPassphrase)
	require.NoError(t, err)
	assert.Equal(t, expected, hashHex)
	hashHex, err = gtx.HashHex(network.TestNetworkPassphrase)
	require.NoError(t, err)
	assert.Equal(t, expected, hashHex)

	fbtx, err := NewFeeBumpTransaction(FeeBumpTransactionParams{
		Inner:      tx,
		FeeAccount: kp0.Address(),
		BaseFee:    MinBaseFee,
	})
	require.NoError(t, err)

	gtx = fbtx.ToGenericTransaction()

	expected = "9a194faa93e4b6efcd8da1c4b25797c666d596f3262d7db43b794b6f2db8d767"
	hashHex, err = fbtx.HashHex(network.TestNetworkPassphrase)
	require.NoError(t, err)
	assert.Equal(t, expected, hashHex)
	hashHex, err = gtx.HashHex(network.TestNetworkPassphrase)
	require.NoError(t, err)
	assert.Equal(t, expected, hashHex)
}
