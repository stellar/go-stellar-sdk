package txnbuild

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
)

func TestManageDataValidateName(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(3556091187167235))

	manageData := ManageData{
		Name:  "This is a very long name for a field that only accepts 64 characters",
		Value: []byte(""),
	}

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: false,
			Operations:           []Operation{&manageData},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	if assert.Error(t, err) {
		expected := "validation failed for *txnbuild.ManageData operation: Field: Name, Error: maximum length is 64 characters"
		assert.Contains(t, err.Error(), expected)
	}
}

func TestManageDataValidateValue(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(3556091187167235))

	manageData := ManageData{
		Name:  "cars",
		Value: []byte("toyota, ford, porsche, lamborghini, hyundai, volkswagen, gmc, kia"),
	}

	_, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: false,
			Operations:           []Operation{&manageData},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
	)
	if assert.Error(t, err) {
		expected := "validation failed for *txnbuild.ManageData operation: Field: Value, Error: maximum length is 64 bytes"
		assert.Contains(t, err.Error(), expected)
	}
}

func TestManageDataRoundTrip(t *testing.T) {
	kp0 := newKeypair0()
	sourceAccount := NewSimpleAccount(kp0.Address(), int64(3556091187167235))

	for _, testCase := range []struct {
		name  string
		value []byte
	}{
		{
			"nil data",
			nil,
		},
		{
			"empty data slice",
			[]byte{},
		},
		{
			"non-empty data slice",
			[]byte{1, 2, 3},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			manageData := ManageData{
				Name:  "key",
				Value: testCase.value,
			}

			tx, err := NewTransaction(
				TransactionParams{
					SourceAccount:        &sourceAccount,
					IncrementSequenceNum: false,
					Operations:           []Operation{&manageData},
					BaseFee:              MinBaseFee,
					Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
				},
			)
			assert.NoError(t, err)

			envelope := tx.ToXDR()
			assert.NoError(t, err)
			assert.Len(t, envelope.Operations(), 1)
			assert.Equal(t, xdr.String64(manageData.Name), envelope.Operations()[0].Body.ManageDataOp.DataName)
			if testCase.value == nil {
				assert.Nil(t, envelope.Operations()[0].Body.ManageDataOp.DataValue)
			} else {
				assert.Len(t, []byte(*envelope.Operations()[0].Body.ManageDataOp.DataValue), len(testCase.value))
				if len(testCase.value) > 0 {
					assert.Equal(t, testCase.value, []byte(*envelope.Operations()[0].Body.ManageDataOp.DataValue))
				}
			}

			txe, err := tx.Base64()
			if err != nil {
				assert.NoError(t, err)
			}

			parsed, err := TransactionFromXDR(txe)
			assert.NoError(t, err)

			tx, _ = parsed.Transaction()

			assert.Len(t, tx.Operations(), 1)
			op := tx.Operations()[0].(*ManageData)
			assert.Equal(t, manageData.Name, op.Name)
			assert.Len(t, op.Value, len(manageData.Value))
			if len(manageData.Value) > 0 {
				assert.Equal(t, manageData.Value, op.Value)
			}
		})
	}
}

func TestManageDataRoundtrip(t *testing.T) {
	manageData := ManageData{
		SourceAccount: "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
		Name:          "foo",
		Value:         []byte("bar"),
	}
	testOperationsMarshalingRoundtrip(t, []Operation{&manageData}, false)

	// with muxed accounts
	manageData = ManageData{
		SourceAccount: "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK",
		Name:          "foo",
		Value:         []byte("bar"),
	}
	testOperationsMarshalingRoundtrip(t, []Operation{&manageData}, true)
}
