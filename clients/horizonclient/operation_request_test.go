package horizonclient

import (
	"context"
	"testing"

	"github.com/stellar/go-stellar-sdk/protocols/horizon/operations"
	"github.com/stellar/go-stellar-sdk/support/http/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperationRequestBuildUrl(t *testing.T) {
	op := OperationRequest{endpoint: "operations"}
	endpoint, err := op.BuildURL()

	// It should return valid all operations endpoint and no errors
	require.NoError(t, err)
	assert.Equal(t, "operations", endpoint)

	op = OperationRequest{ForAccount: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU", endpoint: "operations"}
	endpoint, err = op.BuildURL()

	// It should return valid account operations endpoint and no errors
	require.NoError(t, err)
	assert.Equal(t, "accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/operations", endpoint)

	op = OperationRequest{ForClaimableBalance: "00000000178826fbfe339e1f5c53417c6fedfe2c05e8bec14303143ec46b38981b09c3f9", endpoint: "operations"}
	endpoint, err = op.BuildURL()

	// It should return valid account transactions endpoint and no errors
	require.NoError(t, err)
	assert.Equal(t, "claimable_balances/00000000178826fbfe339e1f5c53417c6fedfe2c05e8bec14303143ec46b38981b09c3f9/operations", endpoint)

	op = OperationRequest{ForLedger: 123, endpoint: "operations"}
	endpoint, err = op.BuildURL()

	// It should return valid ledger operations endpoint and no errors
	require.NoError(t, err)
	assert.Equal(t, "ledgers/123/operations", endpoint)

	op = OperationRequest{ForLiquidityPool: "123", endpoint: "operations"}
	endpoint, err = op.BuildURL()

	// It should return valid liquidity pool effects operations and no errors
	require.NoError(t, err)
	assert.Equal(t, "liquidity_pools/123/operations", endpoint)

	op = OperationRequest{forOperationID: "123", endpoint: "operations"}
	endpoint, err = op.BuildURL()

	// It should return valid operation operations endpoint and no errors
	require.NoError(t, err)
	assert.Equal(t, "operations/123", endpoint)

	op = OperationRequest{ForTransaction: "123", endpoint: "payments"}
	endpoint, err = op.BuildURL()

	// It should return valid transaction payments endpoint and no errors
	require.NoError(t, err)
	assert.Equal(t, "transactions/123/payments", endpoint)

	op = OperationRequest{ForLedger: 123, forOperationID: "789", endpoint: "operations"}
	_, err = op.BuildURL()

	// error case: too many parameters for building any operation endpoint
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "invalid request: too many parameters")
	}

	op = OperationRequest{Cursor: "123456", Limit: 30, Order: OrderAsc, endpoint: "operations", Join: "transactions"}
	endpoint, err = op.BuildURL()
	// It should return valid all operations endpoint with query params and no errors
	require.NoError(t, err)
	assert.Equal(t, "operations?cursor=123456&join=transactions&limit=30&order=asc", endpoint)

	op = OperationRequest{Cursor: "123456", Limit: 30, Order: OrderAsc, endpoint: "payments", Join: "transactions"}
	endpoint, err = op.BuildURL()
	// It should return valid all operations endpoint with query params and no errors
	require.NoError(t, err)
	assert.Equal(t, "payments?cursor=123456&join=transactions&limit=30&order=asc", endpoint)

	op = OperationRequest{ForAccount: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU", endpoint: "payments", Join: "transactions"}
	endpoint, err = op.BuildURL()
	// It should return valid all operations endpoint with query params and no errors
	require.NoError(t, err)
	assert.Equal(t, "accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/payments?join=transactions", endpoint)

	op = OperationRequest{forOperationID: "1234", endpoint: "payments", Join: "transactions"}
	endpoint, err = op.BuildURL()
	// It should return valid all operations endpoint with query params and no errors
	require.NoError(t, err)
	assert.Equal(t, "operations/1234?join=transactions", endpoint)
}

func TestNextOperationsPage(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	operationRequest := OperationRequest{Limit: 2}

	hmock.On(
		"GET",
		"https://localhost/operations?limit=2",
	).ReturnString(200, firstOperationsPage)

	ops, err := client.Operations(operationRequest)

	if assert.NoError(t, err) {
		assert.Equal(t, len(ops.Embedded.Records), 2)
	}

	hmock.On(
		"GET",
		"https://horizon-testnet.stellar.org/operations?cursor=661424967682&limit=2&order=asc",
	).ReturnString(200, emptyOperationsPage)

	nextPage, err := client.NextOperationsPage(ops)
	if assert.NoError(t, err) {
		assert.Equal(t, len(nextPage.Embedded.Records), 0)
	}
}

func TestOperationRequestStreamOperations(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	// All operations
	operationRequest := OperationRequest{}
	ctx, cancel := context.WithCancel(context.Background())

	hmock.On(
		"GET",
		"https://localhost/operations?cursor=now",
	).ReturnString(200, operationStreamResponse)

	operationStream := make([]operations.Operation, 1)
	err := client.StreamOperations(ctx, operationRequest, func(op operations.Operation) {
		operationStream[0] = op
		cancel()
	})

	if assert.NoError(t, err) {
		assert.Equal(t, operationStream[0].GetType(), "create_account")
	}

	// Account payments
	operationRequest = OperationRequest{ForAccount: "GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR"}
	ctx, cancel = context.WithCancel(context.Background())

	hmock.On(
		"GET",
		"https://localhost/accounts/GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR/payments?cursor=now",
	).ReturnString(200, operationStreamResponse)

	err = client.StreamPayments(ctx, operationRequest, func(op operations.Operation) {
		operationStream[0] = op
		cancel()
	})

	if assert.NoError(t, err) {
		payment, ok := operationStream[0].(operations.CreateAccount)
		assert.Equal(t, ok, true)
		assert.Equal(t, payment.Funder, "GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR")
	}

	// test connection error
	operationRequest = OperationRequest{}
	ctx, cancel = context.WithCancel(context.Background())

	hmock.On(
		"GET",
		"https://localhost/operations?cursor=now",
	).ReturnString(500, operationStreamResponse)

	err = client.StreamOperations(ctx, operationRequest, func(op operations.Operation) {
		operationStream[0] = op
		cancel()
	})

	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "got bad HTTP status code 500")
	}
}

func TestManageSellManageBuyOfferOfferID(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	testCases := []struct {
		desc    string
		payload string
	}{
		{
			desc:    "offer_id as a string",
			payload: manageSellBuyOfferOperationsPage,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			hmock.On(
				"GET",
				"https://localhost/operations",
			).ReturnString(200, tc.payload)
			operationRequest := OperationRequest{}
			ops, err := client.Operations(operationRequest)

			if assert.NoError(t, err) {
				assert.Equal(t, len(ops.Embedded.Records), 2)
			}

			mso, ok := ops.Embedded.Records[0].(operations.ManageSellOffer)
			assert.True(t, ok)
			assert.Equal(t, int64(127538671), mso.OfferID)

			mbo, ok := ops.Embedded.Records[1].(operations.ManageBuyOffer)
			assert.True(t, ok)
			assert.Equal(t, int64(127538672), mbo.OfferID)
		})
	}
}

var operationStreamResponse = `data: {"_links":{"self":{"href":"https://horizon-testnet.stellar.org/operations/4934917427201"},"transaction":{"href":"https://horizon-testnet.stellar.org/transactions/1c1449106a54cccd8a2ec2094815ad9db30ae54c69c3309dd08d13fdb8c749de"},"effects":{"href":"https://horizon-testnet.stellar.org/operations/4934917427201/effects"},"succeeds":{"href":"https://horizon-testnet.stellar.org/effects?order=desc\u0026cursor=4934917427201"},"precedes":{"href":"https://horizon-testnet.stellar.org/effects?order=asc\u0026cursor=4934917427201"}},"id":"4934917427201","paging_token":"4934917427201","transaction_successful":true,"source_account":"GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR","type":"create_account","type_i":0,"created_at":"2019-02-27T11:32:39Z","transaction_hash":"1c1449106a54cccd8a2ec2094815ad9db30ae54c69c3309dd08d13fdb8c749de","starting_balance":"10000.0000000","funder":"GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR","account":"GDBLBBDIUULY3HGIKXNK6WVBISY7DCNCDA45EL7NTXWX5R4UZ26HGMGS"}
`

var firstOperationsPage = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/operations?cursor=&limit=2&order=asc"
    },
    "next": {
      "href": "https://horizon-testnet.stellar.org/operations?cursor=661424967682&limit=2&order=asc"
    },
    "prev": {
      "href": "https://horizon-testnet.stellar.org/operations?cursor=661424967681&limit=2&order=desc"
    }
  },
  "_embedded": {
    "records": [
      {	
        "_links": {
          "self": {
            "href": "https://horizon-testnet.stellar.org/operations/661424967681"
          },
          "transaction": {
            "href": "https://horizon-testnet.stellar.org/transactions/749e4f8933221b9942ef38a02856803f379789ec8d971f1f60535db70135673e"
          },
          "effects": {
            "href": "https://horizon-testnet.stellar.org/operations/661424967681/effects"
          },
          "succeeds": {
            "href": "https://horizon-testnet.stellar.org/effects?order=desc&cursor=661424967681"
          },
          "precedes": {
            "href": "https://horizon-testnet.stellar.org/effects?order=asc&cursor=661424967681"
          }
        },
        "id": "661424967681",
        "paging_token": "661424967681",
        "transaction_successful": true,
        "source_account": "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
        "type": "create_account",
        "type_i": 0,
        "created_at": "2019-04-24T09:16:14Z",
        "transaction_hash": "749e4f8933221b9942ef38a02856803f379789ec8d971f1f60535db70135673e",
        "starting_balance": "10000000000.0000000",
        "funder": "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
        "account": "GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR"
      },
      {
        "_links": {
          "self": {
            "href": "https://horizon-testnet.stellar.org/operations/661424967682"
          },
          "transaction": {
            "href": "https://horizon-testnet.stellar.org/transactions/749e4f8933221b9942ef38a02856803f379789ec8d971f1f60535db70135673e"
          },
          "effects": {
            "href": "https://horizon-testnet.stellar.org/operations/661424967682/effects"
          },
          "succeeds": {
            "href": "https://horizon-testnet.stellar.org/effects?order=desc&cursor=661424967682"
          },
          "precedes": {
            "href": "https://horizon-testnet.stellar.org/effects?order=asc&cursor=661424967682"
          }
        },
        "id": "661424967682",
        "paging_token": "661424967682",
        "transaction_successful": true,
        "source_account": "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
        "type": "create_account",
        "type_i": 0,
        "created_at": "2019-04-24T09:16:14Z",
        "transaction_hash": "749e4f8933221b9942ef38a02856803f379789ec8d971f1f60535db70135673e",
        "starting_balance": "10000.0000000",
        "funder": "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
        "account": "GDO34SQXVOSNODK7JCTAXLZUPSAF3JIH4ADQELVIKOQJUWQ3U4BMSCSH"
      }
    ]
  }
}`

var emptyOperationsPage = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/operations?cursor=661424967682&limit=2&order=asc"
    },
    "next": {
      "href": "https://horizon-testnet.stellar.org/operations?cursor=661424967684&limit=2&order=asc"
    },
    "prev": {
      "href": "https://horizon-testnet.stellar.org/operations?cursor=661424967683&limit=2&order=desc"
    }
  },
  "_embedded": {
    "records": []
  }
}`

var numberManageSellBuyOfferOperations = `{
	"_links": {
	  "self": {
		"href": "https://horizon-testnet.stellar.org/operations?cursor=661424967682&limit=2&order=asc"
	  },
	  "next": {
		"href": "https://horizon-testnet.stellar.org/operations?cursor=661424967684&limit=2&order=asc"
	  },
	  "prev": {
		"href": "https://horizon-testnet.stellar.org/operations?cursor=661424967683&limit=2&order=desc"
	  }
	},
	"_embedded": {
	  "records": [
		{
			"_links": {
			  "self": {
				"href": "https://horizon-testnet.stellar.org/operations/972702718365697"
			  },
			  "transaction": {
				"href": "https://horizon-testnet.stellar.org/transactions/cfe9eba317025dd0cff111967a3709358153e9ee97472e67c17e42837dd50a52"
			  },
			  "effects": {
				"href": "https://horizon-testnet.stellar.org/operations/972702718365697/effects"
			  },
			  "succeeds": {
				"href": "https://horizon-testnet.stellar.org/effects?order=desc\u0026cursor=972702718365697"
			  },
			  "precedes": {
				"href": "https://horizon-testnet.stellar.org/effects?order=asc\u0026cursor=972702718365697"
			  }
			},
			"id": "972702718365697",
			"paging_token": "972702718365697",
			"transaction_successful": true,
			"source_account": "GBPPEHGF322UNA62WHRHBCUBCVOIT3SLUY7U7XQEEISZ5B2JLZ3AYTDC",
			"type": "manage_offer",
			"type_i": 3,
			"created_at": "2019-11-13T16:46:36Z",
			"transaction_hash": "cfe9eba317025dd0cff111967a3709358153e9ee97472e67c17e42837dd50a52",
			"amount": "1000.0000000",
			"price": "0.1312531",
			"price_r": {
			  "n": 265,
			  "d": 2019
			},
			"buying_asset_type": "credit_alphanum4",
			"buying_asset_code": "BAT",
			"buying_asset_issuer": "GBBJMSXCTLXVOYRL7SJ5ABLJ3GGCUFQXCFIXYUOHZZUDAZJKLXCO32AU",
			"selling_asset_type": "native",
			"offer_id": 127538671
		  },
		  {
			"_links": {
			  "self": {
				"href": "https://horizon-testnet.stellar.org/operations/158041911595009"
			  },
			  "transaction": {
				"href": "https://horizon-testnet.stellar.org/transactions/8a4db87e4749130ba32924943c2f219de497fe2d4f3e074187c5d2159ca2d134"
			  },
			  "effects": {
				"href": "https://horizon-testnet.stellar.org/operations/158041911595009/effects"
			  },
			  "succeeds": {
				"href": "https://horizon-testnet.stellar.org/effects?order=desc&cursor=158041911595009"
			  },
			  "precedes": {
				"href": "https://horizon-testnet.stellar.org/effects?order=asc&cursor=158041911595009"
			  }
			},
			"id": "158041911595009",
			"paging_token": "158041911595009",
			"transaction_successful": true,
			"source_account": "GBBXM7GVMXZMQWDEKSWGEW6GT6XMPBLEVEPLYWIQF3SRS43AIJVU3QES",
			"type": "manage_buy_offer",
			"type_i": 12,
			"created_at": "2019-11-01T17:06:47Z",
			"transaction_hash": "8a4db87e4749130ba32924943c2f219de497fe2d4f3e074187c5d2159ca2d134",
			"amount": "1.0000000",
			"price": "0.5000000",
			"price_r": {
			  "n": 1,
			  "d": 2
			},
			"buying_asset_type": "credit_alphanum12",
			"buying_asset_code": "MosaiRMBA",
			"buying_asset_issuer": "GBBWA24VLGPVMMFMF2OJHW3QHFVSILK2UJSNTORRC6QHK6EPTUADAJFA",
			"selling_asset_type": "native",
			"offer_id": 127538672
		  }
	  ]
	}
  }`

var manageSellBuyOfferOperationsPage = `{
	"_links": {
	  "self": {
		"href": "https://horizon-testnet.stellar.org/operations?cursor=661424967682&limit=2&order=asc"
	  },
	  "next": {
		"href": "https://horizon-testnet.stellar.org/operations?cursor=661424967684&limit=2&order=asc"
	  },
	  "prev": {
		"href": "https://horizon-testnet.stellar.org/operations?cursor=661424967683&limit=2&order=desc"
	  }
	},
	"_embedded": {
	  "records": [
		{
			"_links": {
			  "self": {
				"href": "https://horizon-testnet.stellar.org/operations/972702718365697"
			  },
			  "transaction": {
				"href": "https://horizon-testnet.stellar.org/transactions/cfe9eba317025dd0cff111967a3709358153e9ee97472e67c17e42837dd50a52"
			  },
			  "effects": {
				"href": "https://horizon-testnet.stellar.org/operations/972702718365697/effects"
			  },
			  "succeeds": {
				"href": "https://horizon-testnet.stellar.org/effects?order=desc\u0026cursor=972702718365697"
			  },
			  "precedes": {
				"href": "https://horizon-testnet.stellar.org/effects?order=asc\u0026cursor=972702718365697"
			  }
			},
			"id": "972702718365697",
			"paging_token": "972702718365697",
			"transaction_successful": true,
			"source_account": "GBPPEHGF322UNA62WHRHBCUBCVOIT3SLUY7U7XQEEISZ5B2JLZ3AYTDC",
			"type": "manage_offer",
			"type_i": 3,
			"created_at": "2019-11-13T16:46:36Z",
			"transaction_hash": "cfe9eba317025dd0cff111967a3709358153e9ee97472e67c17e42837dd50a52",
			"amount": "1000.0000000",
			"price": "0.1312531",
			"price_r": {
			  "n": 265,
			  "d": 2019
			},
			"buying_asset_type": "credit_alphanum4",
			"buying_asset_code": "BAT",
			"buying_asset_issuer": "GBBJMSXCTLXVOYRL7SJ5ABLJ3GGCUFQXCFIXYUOHZZUDAZJKLXCO32AU",
			"selling_asset_type": "native",
			"offer_id": "127538671"
		  },
		  {
			"_links": {
			  "self": {
				"href": "https://horizon-testnet.stellar.org/operations/158041911595009"
			  },
			  "transaction": {
				"href": "https://horizon-testnet.stellar.org/transactions/8a4db87e4749130ba32924943c2f219de497fe2d4f3e074187c5d2159ca2d134"
			  },
			  "effects": {
				"href": "https://horizon-testnet.stellar.org/operations/158041911595009/effects"
			  },
			  "succeeds": {
				"href": "https://horizon-testnet.stellar.org/effects?order=desc&cursor=158041911595009"
			  },
			  "precedes": {
				"href": "https://horizon-testnet.stellar.org/effects?order=asc&cursor=158041911595009"
			  }
			},
			"id": "158041911595009",
			"paging_token": "158041911595009",
			"transaction_successful": true,
			"source_account": "GBBXM7GVMXZMQWDEKSWGEW6GT6XMPBLEVEPLYWIQF3SRS43AIJVU3QES",
			"type": "manage_buy_offer",
			"type_i": 12,
			"created_at": "2019-11-01T17:06:47Z",
			"transaction_hash": "8a4db87e4749130ba32924943c2f219de497fe2d4f3e074187c5d2159ca2d134",
			"amount": "1.0000000",
			"price": "0.5000000",
			"price_r": {
			  "n": 1,
			  "d": 2
			},
			"buying_asset_type": "credit_alphanum12",
			"buying_asset_code": "MosaiRMBA",
			"buying_asset_issuer": "GBBWA24VLGPVMMFMF2OJHW3QHFVSILK2UJSNTORRC6QHK6EPTUADAJFA",
			"selling_asset_type": "native",
			"offer_id": "127538672"
		  }
	  ]
	}
  }`
