package horizonclient

import (
	"context"
	"testing"

	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/support/http/httptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOfferRequestBuildUrl(t *testing.T) {

	er := OfferRequest{ForAccount: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU"}
	endpoint, err := er.BuildURL()

	// It should return valid offers endpoint and no errors
	require.NoError(t, err)
	assert.Equal(t, "accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/offers", endpoint)

	er = OfferRequest{ForAccount: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU", Cursor: "now", Order: OrderDesc}
	endpoint, err = er.BuildURL()

	// It should return valid offers endpoint and no errors
	require.NoError(t, err)
	assert.Equal(t, "accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/offers?cursor=now&order=desc", endpoint)

	er = OfferRequest{OfferID: "12345"}
	endpoint, err = er.BuildURL()

	require.NoError(t, err)
	assert.Equal(t, "offers/12345", endpoint)
}

func TestNextOffersPage(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	offerRequest := OfferRequest{ForAccount: "GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG", Limit: 2}

	hmock.On(
		"GET",
		"https://localhost/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG/offers?limit=2",
	).ReturnString(200, firstOffersPage)

	offers, err := client.Offers(offerRequest)

	if assert.NoError(t, err) {
		assert.Equal(t, len(offers.Embedded.Records), 2)
	}

	assert.Equal(t, int64(2946580), offers.Embedded.Records[0].ID)

	hmock.On(
		"GET",
		"https://horizon-testnet.stellar.org/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG/offers?cursor=2946581&limit=2&order=asc",
	).ReturnString(200, emptyOffersPage)

	nextPage, err := client.NextOffersPage(offers)
	if assert.NoError(t, err) {
		assert.Equal(t, len(nextPage.Embedded.Records), 0)
	}
}

func TestOfferRequestStreamOffers(t *testing.T) {

	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	// offers for account
	orRequest := OfferRequest{ForAccount: "GAQHWQYBBW272OOXNQMMLCA5WY2XAZPODGB7Q3S5OKKIXVESKO55ZQ7C"}
	ctx, cancel := context.WithCancel(context.Background())

	hmock.On(
		"GET",
		"https://localhost/accounts/GAQHWQYBBW272OOXNQMMLCA5WY2XAZPODGB7Q3S5OKKIXVESKO55ZQ7C/offers?cursor=now",
	).ReturnString(200, offerStreamResponse)

	offers := make([]hProtocol.Offer, 1)
	err := client.StreamOffers(ctx, orRequest, func(offer hProtocol.Offer) {
		offers[0] = offer
		cancel()
	})

	if assert.NoError(t, err) {
		assert.Equal(t, offers[0].Amount, "20.4266087")
		assert.Equal(t, offers[0].Seller, "GAQHWQYBBW272OOXNQMMLCA5WY2XAZPODGB7Q3S5OKKIXVESKO55ZQ7C")
	}

	// test error
	orRequest = OfferRequest{ForAccount: "GAQHWQYBBW272OOXNQMMLCA5WY2XAZPODGB7Q3S5OKKIXVESKO55ZQ7C"}
	ctx, cancel = context.WithCancel(context.Background())

	hmock.On(
		"GET",
		"https://localhost/accounts/GAQHWQYBBW272OOXNQMMLCA5WY2XAZPODGB7Q3S5OKKIXVESKO55ZQ7C/offers?cursor=now",
	).ReturnString(500, offerStreamResponse)

	offers = make([]hProtocol.Offer, 1)
	err = client.StreamOffers(ctx, orRequest, func(offer hProtocol.Offer) {
		cancel()
	})

	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "got bad HTTP status code 500")
	}
}

func TestStringOfferID(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	offerRequest := OfferRequest{ForAccount: "GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG", Limit: 1}

	hmock.On(
		"GET",
		"https://localhost/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG/offers?limit=1",
	).ReturnString(200, stringOffersPage)

	offers, err := client.Offers(offerRequest)

	if assert.NoError(t, err) {
		assert.Equal(t, len(offers.Embedded.Records), 1)
	}

	assert.Equal(t, int64(2946580), offers.Embedded.Records[0].ID)
}

var offerStreamResponse = `data: {"_links":{"self":{"href":"https://horizon-testnet.stellar.org/offers/5269100"},"offer_maker":{"href":"https://horizon-testnet.stellar.org/accounts/GAQHWQYBBW272OOXNQMMLCA5WY2XAZPODGB7Q3S5OKKIXVESKO55ZQ7C"}},"id":"5269100","paging_token":"5269100","seller":"GAQHWQYBBW272OOXNQMMLCA5WY2XAZPODGB7Q3S5OKKIXVESKO55ZQ7C","selling":{"asset_type":"credit_alphanum4","asset_code":"DSQ","asset_issuer":"GBDQPTQJDATT7Z7EO4COS4IMYXH44RDLLI6N6WIL5BZABGMUOVMLWMQF"},"buying":{"asset_type":"credit_alphanum4","asset_code":"XCS6","asset_issuer":"GBH2V47NOZRC56QAYCPV5JUBG5NVFJQF5AQTUNFNWNDHSWWTKH2MWR2L"},"amount":"20.4266087","price_r":{"n":24819,"d":10000000},"price":"0.0024819","last_modified_ledger":674449,"last_modified_time":"2019-04-08T11:56:41Z"}
`
var emptyOffersPage = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG/offers?cursor=2946581&limit=2&order=asc"
    },
    "next": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG/offers?cursor=2946583&limit=2&order=asc"
    },
    "prev": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG/offers?cursor=2946582&limit=2&order=desc"
    }
  },
  "_embedded": {
    "records": []
  }
}`

var firstOffersPage = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG/offers?cursor=&limit=2&order=asc"
    },
    "next": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG/offers?cursor=2946581&limit=2&order=asc"
    },
    "prev": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG/offers?cursor=2946580&limit=2&order=desc"
    }
  },
  "_embedded": {
    "records": [
      {
        "_links": {
          "self": {
            "href": "https://horizon-testnet.stellar.org/offers/2946580"
          },
          "offer_maker": {
            "href": "https://horizon-testnet.stellar.org/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG"
          }
        },
        "id": "2946580",
        "paging_token": "2946580",
        "seller": "GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG",
        "selling": {
          "asset_type": "credit_alphanum4",
          "asset_code": "HT",
          "asset_issuer": "GCNSGHUCG5VMGLT5RIYYZSO7VQULQKAJ62QA33DBC5PPBSO57LFWVV6P"
        },
        "buying": {
          "asset_type": "credit_alphanum4",
          "asset_code": "BTC",
          "asset_issuer": "GCNSGHUCG5VMGLT5RIYYZSO7VQULQKAJ62QA33DBC5PPBSO57LFWVV6P"
        },
        "amount": "33.7252478",
        "price_r": {
          "n": 15477,
          "d": 43975000
        },
        "price": "0.0003519",
        "last_modified_ledger": 363492,
        "last_modified_time": "2019-05-16T08:35:22Z"
      },
      {
        "_links": {
          "self": {
            "href": "https://horizon-testnet.stellar.org/offers/2946581"
          },
          "offer_maker": {
            "href": "https://horizon-testnet.stellar.org/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG"
          }
        },
        "id": "2946581",
        "paging_token": "2946581",
        "seller": "GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG",
        "selling": {
          "asset_type": "credit_alphanum4",
          "asset_code": "HT",
          "asset_issuer": "GCNSGHUCG5VMGLT5RIYYZSO7VQULQKAJ62QA33DBC5PPBSO57LFWVV6P"
        },
        "buying": {
          "asset_type": "credit_alphanum4",
          "asset_code": "BTC",
          "asset_issuer": "GCNSGHUCG5VMGLT5RIYYZSO7VQULQKAJ62QA33DBC5PPBSO57LFWVV6P"
        },
        "amount": "20.0242956",
        "price_r": {
          "n": 3157,
          "d": 8795000
        },
        "price": "0.0003590",
        "last_modified_ledger": 363492,
        "last_modified_time": "2019-05-16T08:35:22Z"
      }
    ]
  }
}`

var stringOffersPage = `{
  "_embedded": {
    "records": [
      {
        "_links": {
          "self": {
            "href": "https://horizon-testnet.stellar.org/offers/2946580"
          },
          "offer_maker": {
            "href": "https://horizon-testnet.stellar.org/accounts/GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG"
          }
        },
        "id": "2946580",
        "paging_token": "2946580",
        "seller": "GBZ5OD56VRTRQKMNADD6VUZUG3FCILMAMYQY5ZSC3AW3GBXNEPIK76IG",
        "selling": {
          "asset_type": "credit_alphanum4",
          "asset_code": "HT",
          "asset_issuer": "GCNSGHUCG5VMGLT5RIYYZSO7VQULQKAJ62QA33DBC5PPBSO57LFWVV6P"
        },
        "buying": {
          "asset_type": "credit_alphanum4",
          "asset_code": "BTC",
          "asset_issuer": "GCNSGHUCG5VMGLT5RIYYZSO7VQULQKAJ62QA33DBC5PPBSO57LFWVV6P"
        },
        "amount": "33.7252478",
        "price_r": {
          "n": 15477,
          "d": 43975000
        },
        "price": "0.0003519",
        "last_modified_ledger": 363492,
        "last_modified_time": "2019-05-16T08:35:22Z"
      }
    ]
  }
}`
