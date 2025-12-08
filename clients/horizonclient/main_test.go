package horizonclient

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/protocols/horizon/effects"
	"github.com/stellar/go-stellar-sdk/protocols/horizon/operations"
	"github.com/stellar/go-stellar-sdk/support/clock"
	"github.com/stellar/go-stellar-sdk/support/clock/clocktest"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/support/http/httptest"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/stretchr/testify/assert"
)

func TestFixHTTP(t *testing.T) {
	client := &Client{
		HorizonURL: "https://localhost/",
	}
	// No HTTP client is provided
	assert.Nil(t, client.HTTP, "client HTTP is nil")
	client.Root()
	// When a request is made, default HTTP client is set
	assert.IsType(t, client.HTTP, &http.Client{})
}

func TestCheckMemoRequired(t *testing.T) {
	tt := assert.New(t)

	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	kp := keypair.MustParseFull("SA26PHIKZM6CXDGR472SSGUQQRYXM6S437ZNHZGRM6QA4FOPLLLFRGDX")
	sourceAccount := txnbuild.NewSimpleAccount(kp.Address(), int64(0))

	paymentMemoRequired := txnbuild.Payment{
		Destination: "GAYHAAKPAQLMGIJYMIWPDWCGUCQ5LAWY4Q7Q3IKSP57O7GUPD3NEOSEA",
		Amount:      "10",
		Asset:       txnbuild.NativeAsset{},
	}

	paymentNoMemo := txnbuild.Payment{
		Destination: "GDWIRURRED6SQSZVQVVMK46PE2MOZEKHV6ZU54JG3NPVRDIF4XCXYYW4",
		Amount:      "10",
		Asset:       txnbuild.NativeAsset{},
	}

	asset := txnbuild.CreditAsset{Code: "ABCD", Issuer: kp.Address()}
	pathPaymentStrictSend := txnbuild.PathPaymentStrictSend{
		SendAsset:   asset,
		SendAmount:  "10",
		Destination: "GDYM6SBBGDF6ZDRM2SKGVIWM257Q4V63V3IYNDQQWPKNV4QDERS4YTLX",
		DestAsset:   txnbuild.NativeAsset{},
		DestMin:     "1",
		Path:        []txnbuild.Asset{asset},
	}

	pathPaymentStrictReceive := txnbuild.PathPaymentStrictReceive{
		SendAsset:   asset,
		SendMax:     "10",
		Destination: "GD2JTIDP2JJKNIDXW4L6AU2RYFXZIUH3YFIS43PJT2467AP46CWBHSCN",
		DestAsset:   txnbuild.NativeAsset{},
		DestAmount:  "1",
		Path:        []txnbuild.Asset{asset},
	}

	accountMerge := txnbuild.AccountMerge{
		Destination: "GBVZZ5XPHECNGA5SENAJP4C6ZJ7FGZ55ZZUCTFTHREZM73LKUGCQDRHR",
	}

	testCases := []struct {
		desc         string
		destination  string
		expected     string
		operations   []txnbuild.Operation
		mockNotFound bool
	}{
		{
			desc:        "payment operation",
			destination: "GAYHAAKPAQLMGIJYMIWPDWCGUCQ5LAWY4Q7Q3IKSP57O7GUPD3NEOSEA",
			expected:    "operation[0]: destination account requires a memo in the transaction",
			operations: []txnbuild.Operation{
				&paymentMemoRequired,
				&pathPaymentStrictReceive,
				&pathPaymentStrictSend,
				&accountMerge,
			},
		},
		{
			desc:        "strict receive operation",
			destination: "GD2JTIDP2JJKNIDXW4L6AU2RYFXZIUH3YFIS43PJT2467AP46CWBHSCN",
			expected:    "operation[1]: destination account requires a memo in the transaction",
			operations: []txnbuild.Operation{
				&paymentNoMemo,
				&pathPaymentStrictReceive,
			},
			mockNotFound: true,
		},
		{
			desc:        "strict send operation",
			destination: "GDYM6SBBGDF6ZDRM2SKGVIWM257Q4V63V3IYNDQQWPKNV4QDERS4YTLX",
			expected:    "operation[1]: destination account requires a memo in the transaction",
			operations: []txnbuild.Operation{
				&paymentNoMemo,
				&pathPaymentStrictSend,
			},
			mockNotFound: true,
		},
		{
			desc:        "merge account operation",
			destination: "GBVZZ5XPHECNGA5SENAJP4C6ZJ7FGZ55ZZUCTFTHREZM73LKUGCQDRHR",
			expected:    "operation[1]: destination account requires a memo in the transaction",
			operations: []txnbuild.Operation{
				&paymentNoMemo,
				&accountMerge,
			},
			mockNotFound: true,
		},
		{
			desc: "two operations with same destination",
			operations: []txnbuild.Operation{
				&paymentNoMemo,
				&paymentNoMemo,
			},
			mockNotFound: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tx, err := txnbuild.NewTransaction(
				txnbuild.TransactionParams{
					SourceAccount:        &sourceAccount,
					IncrementSequenceNum: true,
					Operations:           tc.operations,
					BaseFee:              txnbuild.MinBaseFee,
					Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimebounds(0, 10)},
				},
			)
			tt.NoError(err)

			if len(tc.destination) > 0 {
				hmock.On(
					"GET",
					fmt.Sprintf("https://localhost/accounts/%s/data/config.memo_required", tc.destination),
				).ReturnJSON(200, memoRequiredResponse)
			}

			if tc.mockNotFound {
				hmock.On(
					"GET",
					"https://localhost/accounts/GDWIRURRED6SQSZVQVVMK46PE2MOZEKHV6ZU54JG3NPVRDIF4XCXYYW4/data/config.memo_required",
				).ReturnString(404, notFoundResponse)
			}

			err = client.checkMemoRequired(tx)

			if len(tc.expected) > 0 {
				tt.Error(err)
				tt.Contains(err.Error(), tc.expected)
				tt.Equal(ErrAccountRequiresMemo, errors.Cause(err))
			} else {
				tt.NoError(err)
			}
		})
	}
}

func TestAccounts(t *testing.T) {
	tt := assert.New(t)
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	accountRequest := AccountsRequest{}
	_, err := client.Accounts(accountRequest)
	if tt.Error(err) {
		tt.Contains(err.Error(), "invalid request: no parameters")
	}

	accountRequest = AccountsRequest{
		Signer: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
		Asset:  "COP:GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
	}
	_, err = client.Accounts(accountRequest)
	if tt.Error(err) {
		tt.Contains(err.Error(), "invalid request: too many parameters")
	}

	var accounts hProtocol.AccountsPage

	hmock.On(
		"GET",
		"https://localhost/accounts?signer=GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP",
	).ReturnString(200, accountsResponse)

	accountRequest = AccountsRequest{
		Signer: "GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP",
	}
	accounts, err = client.Accounts(accountRequest)
	tt.NoError(err)
	tt.Len(accounts.Embedded.Records, 1)

	hmock.On(
		"GET",
		"https://localhost/accounts?asset=COP%3AGAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP",
	).ReturnString(200, accountsResponse)

	accountRequest = AccountsRequest{
		Asset: "COP:GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP",
	}
	accounts, err = client.Accounts(accountRequest)
	tt.NoError(err)
	tt.Len(accounts.Embedded.Records, 1)

	hmock.On(
		"GET",
		"https://localhost/accounts?signer=GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP&cursor=GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H&limit=200&order=desc",
	).ReturnString(200, accountsResponse)

	accountRequest = AccountsRequest{
		Signer: "GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP",
		Order:  "desc",
		Cursor: "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H",
		Limit:  200,
	}
	accounts, err = client.Accounts(accountRequest)
	tt.NoError(err)
	tt.Len(accounts.Embedded.Records, 1)

	// connection error
	hmock.On(
		"GET",
		"https://localhost/accounts?signer=GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP",
	).ReturnError("http.Client error")

	accountRequest = AccountsRequest{
		Signer: "GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP",
	}
	accounts, err = client.Accounts(accountRequest)
	if tt.Error(err) {
		tt.Contains(err.Error(), "http.Client error")
		_, ok := err.(*Error)
		tt.Equal(ok, false)
	}
}

func TestAccountDetail(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	// no parameters
	accountRequest := AccountRequest{}
	hmock.On(
		"GET",
		"https://localhost/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
	).ReturnString(200, accountResponse)

	_, err := client.AccountDetail(accountRequest)
	// error case: no account id
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no account ID provided")
	}

	// wrong parameters
	accountRequest = AccountRequest{DataKey: "test"}
	hmock.On(
		"GET",
		"https://localhost/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
	).ReturnString(200, accountResponse)

	_, err = client.AccountDetail(accountRequest)
	// error case: no account id
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no account ID provided")
	}

	accountRequest = AccountRequest{AccountID: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU"}

	// happy path
	hmock.On(
		"GET",
		"https://localhost/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
	).ReturnString(200, accountResponse)

	account, err := client.AccountDetail(accountRequest)

	if assert.NoError(t, err) {
		assert.Equal(t, account.ID, "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU")
		assert.Equal(t, account.Signers[0].Key, "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU")
		assert.Equal(t, account.Signers[0].Type, "ed25519_public_key")
		assert.Equal(t, account.Data["test"], "dGVzdA==")
		balance, balanceErr := account.GetNativeBalance()
		assert.Nil(t, balanceErr)
		assert.Equal(t, balance, "9999.9999900")
		assert.NotNil(t, account.LastModifiedTime)
		assert.Equal(t, "2019-03-05 13:23:50 +0000 UTC", account.LastModifiedTime.String())
		assert.Equal(t, uint32(103307), account.LastModifiedLedger)
	}

	// failure response
	hmock.On(
		"GET",
		"https://localhost/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
	).ReturnString(404, notFoundResponse)

	account, err = client.AccountDetail(accountRequest)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "horizon error")
		horizonError, ok := err.(*Error)
		assert.Equal(t, ok, true)
		assert.Equal(t, horizonError.Problem.Title, "Resource Missing")
	}

	// connection error
	hmock.On(
		"GET",
		"https://localhost/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
	).ReturnError("http.Client error")

	_, err = client.AccountDetail(accountRequest)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "http.Client error")
		_, ok := err.(*Error)
		assert.Equal(t, ok, false)
	}
}

func TestAccountData(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	// no parameters
	accountRequest := AccountRequest{}
	hmock.On(
		"GET",
		"https://localhost/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/data/test",
	).ReturnString(200, accountResponse)

	_, err := client.AccountData(accountRequest)
	// error case: few parameters
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "too few parameters")
	}

	// wrong parameters
	accountRequest = AccountRequest{AccountID: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU"}
	hmock.On(
		"GET",
		"https://localhost/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/data/test",
	).ReturnString(200, accountResponse)

	_, err = client.AccountData(accountRequest)
	// error case: few parameters
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "too few parameters")
	}

	accountRequest = AccountRequest{AccountID: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU", DataKey: "test"}

	// happy path
	hmock.On(
		"GET",
		"https://localhost/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/data/test",
	).ReturnString(200, accountData)

	data, err := client.AccountData(accountRequest)
	if assert.NoError(t, err) {
		assert.Equal(t, data.Value, "dGVzdA==")
	}

}

func TestEffectsRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	effectRequest := EffectRequest{}

	// all effects
	hmock.On(
		"GET",
		"https://localhost/effects",
	).ReturnString(200, effectsResponse)

	effs, err := client.Effects(effectRequest)
	if assert.NoError(t, err) {
		assert.IsType(t, effs, effects.EffectsPage{})
		links := effs.Links
		assert.Equal(t, links.Self.Href, "https://horizon-testnet.stellar.org/operations/43989725060534273/effects?cursor=&limit=10&order=asc")

		assert.Equal(t, links.Next.Href, "https://horizon-testnet.stellar.org/operations/43989725060534273/effects?cursor=43989725060534273-3&limit=10&order=asc")

		assert.Equal(t, links.Prev.Href, "https://horizon-testnet.stellar.org/operations/43989725060534273/effects?cursor=43989725060534273-1&limit=10&order=desc")

		adEffect := effs.Embedded.Records[0]
		acEffect := effs.Embedded.Records[1]
		arEffect := effs.Embedded.Records[2]
		assert.IsType(t, adEffect, effects.AccountDebited{})
		assert.IsType(t, acEffect, effects.AccountCredited{})
		// account_removed effect does not have a struct. Defaults to effects.Base
		assert.IsType(t, arEffect, effects.Base{})

		c, ok := acEffect.(effects.AccountCredited)
		assert.Equal(t, ok, true)
		assert.Equal(t, c.ID, "0043989725060534273-0000000002")
		assert.Equal(t, c.Amount, "9999.9999900")
		assert.Equal(t, c.Account, "GBO7LQUWCC7M237TU2PAXVPOLLYNHYCYYFCLVMX3RBJCML4WA742X3UB")
		assert.Equal(t, c.Asset.Type, "native")
	}

	effectRequest = EffectRequest{ForAccount: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU"}
	hmock.On(
		"GET",
		"https://localhost/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/effects",
	).ReturnString(200, effectsResponse)

	effs, err = client.Effects(effectRequest)
	if assert.NoError(t, err) {
		assert.IsType(t, effs, effects.EffectsPage{})
	}

	// too many parameters
	effectRequest = EffectRequest{ForAccount: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU", ForLedger: "123"}
	hmock.On(
		"GET",
		"https://localhost/effects",
	).ReturnString(200, effectsResponse)

	_, err = client.Effects(effectRequest)
	// error case
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "too many parameters")
	}
}

func TestAssetsRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	assetRequest := AssetRequest{}

	// all assets
	hmock.On(
		"GET",
		"https://localhost/assets",
	).ReturnString(200, assetsResponse)

	assets, err := client.Assets(assetRequest)
	if assert.NoError(t, err) {
		assert.IsType(t, assets, hProtocol.AssetsPage{})
		record := assets.Embedded.Records[0]
		assert.Equal(t, record.Asset.Code, "ABC")
		assert.Equal(t, record.Asset.Issuer, "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU")
		assert.Equal(t, record.PT, "1")
		assert.Equal(t, record.Flags.AuthRevocable, false)
		assert.Equal(t, record.Flags.AuthRequired, true)
		assert.Equal(t, record.Flags.AuthImmutable, false)
	}

}

func TestFeeStats(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	// happy path
	hmock.On(
		"GET",
		"https://localhost/fee_stats",
	).ReturnString(200, feesResponse)

	fees, err := client.FeeStats()

	if assert.NoError(t, err) {
		assert.Equal(t, uint32(22606298), fees.LastLedger)
		assert.Equal(t, int64(100), fees.LastLedgerBaseFee)
		assert.Equal(t, 0.97, fees.LedgerCapacityUsage)
		assert.Equal(t, int64(130), fees.MaxFee.Min)
		assert.Equal(t, int64(8000), fees.MaxFee.Max)
		assert.Equal(t, int64(250), fees.MaxFee.Mode)
		assert.Equal(t, int64(150), fees.MaxFee.P10)
		assert.Equal(t, int64(200), fees.MaxFee.P20)
		assert.Equal(t, int64(300), fees.MaxFee.P30)
		assert.Equal(t, int64(400), fees.MaxFee.P40)
		assert.Equal(t, int64(500), fees.MaxFee.P50)
		assert.Equal(t, int64(1000), fees.MaxFee.P60)
		assert.Equal(t, int64(2000), fees.MaxFee.P70)
		assert.Equal(t, int64(3000), fees.MaxFee.P80)
		assert.Equal(t, int64(4000), fees.MaxFee.P90)
		assert.Equal(t, int64(5000), fees.MaxFee.P95)
		assert.Equal(t, int64(8000), fees.MaxFee.P99)

		assert.Equal(t, int64(100), fees.FeeCharged.Min)
		assert.Equal(t, int64(100), fees.FeeCharged.Max)
		assert.Equal(t, int64(100), fees.FeeCharged.Mode)
		assert.Equal(t, int64(100), fees.FeeCharged.P10)
		assert.Equal(t, int64(100), fees.FeeCharged.P20)
		assert.Equal(t, int64(100), fees.FeeCharged.P30)
		assert.Equal(t, int64(100), fees.FeeCharged.P40)
		assert.Equal(t, int64(100), fees.FeeCharged.P50)
		assert.Equal(t, int64(100), fees.FeeCharged.P60)
		assert.Equal(t, int64(100), fees.FeeCharged.P70)
		assert.Equal(t, int64(100), fees.FeeCharged.P80)
		assert.Equal(t, int64(100), fees.FeeCharged.P90)
		assert.Equal(t, int64(100), fees.FeeCharged.P95)
		assert.Equal(t, int64(100), fees.FeeCharged.P99)
	}
}

func TestOfferRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	offersRequest := OfferRequest{ForAccount: "GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F"}

	// account offers
	hmock.On(
		"GET",
		"https://localhost/accounts/GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F/offers",
	).ReturnString(200, offersResponse)

	offers, err := client.Offers(offersRequest)
	if assert.NoError(t, err) {
		assert.IsType(t, offers, hProtocol.OffersPage{})
		record := offers.Embedded.Records[0]
		assert.Equal(t, record.ID, int64(432323))
		assert.Equal(t, record.Seller, "GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F")
		assert.Equal(t, record.PT, "432323")
		assert.Equal(t, record.Selling.Code, "ABC")
		assert.Equal(t, record.Amount, "1999979.8700000")
		assert.Equal(t, record.LastModifiedLedger, int32(103307))
	}

	hmock.On(
		"GET",
		"https://localhost/offers?seller=GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
	).ReturnString(200, offersResponse)

	offersRequest = OfferRequest{
		Seller: "GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
	}

	offers, err = client.Offers(offersRequest)
	if assert.NoError(t, err) {
		assert.Len(t, offers.Embedded.Records, 1)
	}

	hmock.On(
		"GET",
		"https://localhost/offers?buying=COP%3AGDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
	).ReturnString(200, offersResponse)

	offersRequest = OfferRequest{
		Buying: "COP:GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
	}

	offers, err = client.Offers(offersRequest)
	if assert.NoError(t, err) {
		assert.Len(t, offers.Embedded.Records, 1)
	}

	hmock.On(
		"GET",
		"https://localhost/offers?selling=EUR%3AGDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
	).ReturnString(200, offersResponse)

	offersRequest = OfferRequest{
		Selling: "EUR:GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
	}

	offers, err = client.Offers(offersRequest)
	if assert.NoError(t, err) {
		assert.Len(t, offers.Embedded.Records, 1)
	}

	hmock.On(
		"GET",
		"https://localhost/offers?selling=EUR%3AGDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
	).ReturnString(200, offersResponse)

	offersRequest = OfferRequest{
		Selling: "EUR:GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
	}

	offers, err = client.Offers(offersRequest)
	if assert.NoError(t, err) {
		assert.Len(t, offers.Embedded.Records, 1)
	}

	hmock.On(
		"GET",
		"https://localhost/offers?buying=EUR%3AGDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F&seller=GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F&selling=EUR%3AGDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F&cursor=30&limit=20&order=desc",
	).ReturnString(200, offersResponse)

	offersRequest = OfferRequest{
		Seller:  "GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
		Buying:  "EUR:GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
		Selling: "EUR:GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
		Order:   "desc",
		Limit:   20,
		Cursor:  "30",
	}

	offers, err = client.Offers(offersRequest)
	if assert.NoError(t, err) {
		assert.Len(t, offers.Embedded.Records, 1)
	}
}
func TestOfferDetailsRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	// account offers
	hmock.On(
		"GET",
		"https://localhost/offers/5635",
	).ReturnString(200, offerResponse)

	record, err := client.OfferDetails("5635")
	if assert.NoError(t, err) {
		assert.IsType(t, record, hProtocol.Offer{})
		assert.Equal(t, record.ID, int64(5635))
		assert.Equal(t, record.Seller, "GD6UOZ3FGFI5L2X6F52YPJ6ICSW375BNBZIQC4PCLSEOO6SMX7CUS5MB")
		assert.Equal(t, record.PT, "5635")
		assert.Equal(t, record.Selling.Type, "native")
		assert.Equal(t, record.Buying.Code, "AstroDollar")
		assert.Equal(t, record.Buying.Issuer, "GDA2EHKPDEWZTAL6B66FO77HMOZL3RHZTIJO7KJJK5RQYSDUXEYMPJYY")
		assert.Equal(t, record.Amount, "100.0000000")
		assert.Equal(t, record.LastModifiedLedger, int32(356183))
	}

	_, err = client.OfferDetails("S6ES")
	assert.Error(t, err)
	assert.EqualError(t, err, "invalid offer ID provided")

	_, err = client.OfferDetails("")
	assert.Error(t, err)
	assert.EqualError(t, err, "no offer ID provided")
}

func TestOperationsRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	operationRequest := OperationRequest{Join: "transactions"}

	// all operations
	hmock.On(
		"GET",
		"https://localhost/operations?join=transactions",
	).ReturnString(200, multipleOpsResponse)

	ops, err := client.Operations(operationRequest)
	if assert.NoError(t, err) {
		assert.IsType(t, ops, operations.OperationsPage{})
		links := ops.Links
		assert.Equal(t, links.Self.Href, "https://horizon.stellar.org/transactions/b63307ef92bb253df13361a72095156d19fc0713798bc2e6c3bd9ee63cc3ca53/operations?cursor=&limit=10&order=asc")

		assert.Equal(t, links.Next.Href, "https://horizon.stellar.org/transactions/b63307ef92bb253df13361a72095156d19fc0713798bc2e6c3bd9ee63cc3ca53/operations?cursor=98447788659970049&limit=10&order=asc")

		assert.Equal(t, links.Prev.Href, "https://horizon.stellar.org/transactions/b63307ef92bb253df13361a72095156d19fc0713798bc2e6c3bd9ee63cc3ca53/operations?cursor=98447788659970049&limit=10&order=desc")

		paymentOp := ops.Embedded.Records[0]
		mangageOfferOp := ops.Embedded.Records[1]
		createAccountOp := ops.Embedded.Records[2]
		assert.IsType(t, paymentOp, operations.Payment{})
		assert.IsType(t, mangageOfferOp, operations.ManageSellOffer{})
		assert.IsType(t, createAccountOp, operations.CreateAccount{})

		c, ok := createAccountOp.(operations.CreateAccount)
		assert.Equal(t, ok, true)
		assert.Equal(t, c.ID, "98455906148208641")
		assert.Equal(t, c.StartingBalance, "2.0000000")
		assert.Equal(t, c.TransactionHash, "ade3c60f1b581e8744596673d95bffbdb8f68f199e0e2f7d63b7c3af9fd8d868")
	}

	// all payments
	hmock.On(
		"GET",
		"https://localhost/payments?join=transactions",
	).ReturnString(200, paymentsResponse)

	ops, err = client.Payments(operationRequest)
	if assert.NoError(t, err) {
		assert.IsType(t, ops, operations.OperationsPage{})
		links := ops.Links
		assert.Equal(t, links.Self.Href, "https://horizon-testnet.stellar.org/payments?cursor=&limit=2&order=desc")

		assert.Equal(t, links.Next.Href, "https://horizon-testnet.stellar.org/payments?cursor=2024660468248577&limit=2&order=desc")

		assert.Equal(t, links.Prev.Href, "https://horizon-testnet.stellar.org/payments?cursor=2024660468256769&limit=2&order=asc")

		createAccountOp := ops.Embedded.Records[0]
		paymentOp := ops.Embedded.Records[1]

		assert.IsType(t, paymentOp, operations.Payment{})
		assert.IsType(t, createAccountOp, operations.CreateAccount{})

		p, ok := paymentOp.(operations.Payment)
		assert.Equal(t, ok, true)
		assert.Equal(t, p.ID, "2024660468248577")
		assert.Equal(t, p.Amount, "177.0000000")
		assert.Equal(t, p.TransactionHash, "87d7a29539e7902b14a6c720094856f74a77128ab332d8629432c5a176a9fe7b")
	}

	// operations for account
	operationRequest = OperationRequest{ForAccount: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU"}
	hmock.On(
		"GET",
		"https://localhost/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/operations",
	).ReturnString(200, multipleOpsResponse)

	ops, err = client.Operations(operationRequest)
	if assert.NoError(t, err) {
		assert.IsType(t, ops, operations.OperationsPage{})
	}

	// too many parameters
	operationRequest = OperationRequest{ForAccount: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU", ForLedger: 123}
	hmock.On(
		"GET",
		"https://localhost/operations",
	).ReturnString(200, multipleOpsResponse)

	_, err = client.Operations(operationRequest)
	// error case
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "too many parameters")
	}

	// operation detail
	opID := "1103965508866049"
	hmock.On(
		"GET",
		"https://localhost/operations/1103965508866049",
	).ReturnString(200, opsResponse)

	record, err := client.OperationDetail(opID)
	if assert.NoError(t, err) {
		assert.Equal(t, record.GetType(), "change_trust")
		c, ok := record.(operations.ChangeTrust)
		assert.Equal(t, ok, true)
		assert.Equal(t, c.ID, "1103965508866049")
		assert.Equal(t, c.TransactionSuccessful, true)
		assert.Equal(t, c.TransactionHash, "93c2755ec61c8b01ac11daa4d8d7a012f56be172bdfcaf77a6efd683319ca96d")
		assert.Equal(t, c.Asset.Code, "UAHd")
		assert.Equal(t, c.Asset.Issuer, "GDDETPGV4OJVNBTB6GQICCPGH5DZRYYB7XQCSAZO2ZQH6HO7SWXHKKJN")
		assert.Equal(t, c.Limit, "922337203685.4775807")
		assert.Equal(t, c.Trustee, "GDDETPGV4OJVNBTB6GQICCPGH5DZRYYB7XQCSAZO2ZQH6HO7SWXHKKJN")
		assert.Equal(t, c.Trustor, "GBMVGXJXJ7ZBHIWMXHKR6IVPDTYKHJPXC2DHZDPJBEZWZYAC7NKII7IB")
		assert.Equal(t, c.Links.Self.Href, "https://horizon-testnet.stellar.org/operations/1103965508866049")
		assert.Equal(t, c.Links.Effects.Href, "https://horizon-testnet.stellar.org/operations/1103965508866049/effects")
		assert.Equal(t, c.Links.Transaction.Href, "https://horizon-testnet.stellar.org/transactions/93c2755ec61c8b01ac11daa4d8d7a012f56be172bdfcaf77a6efd683319ca96d")

		assert.Equal(t, c.Links.Succeeds.Href, "https://horizon-testnet.stellar.org/effects?order=desc\u0026cursor=1103965508866049")

		assert.Equal(t, c.Links.Precedes.Href, "https://horizon-testnet.stellar.org/effects?order=asc\u0026cursor=1103965508866049")
	}

}

func TestSubmitTransactionXDRRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	txXdr := `AAAAABB90WssODNIgi6BHveqzxTRmIpvAFRyVNM+Hm2GVuCcAAAAZAAABD0AAuV/AAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAyTBGxOgfSApppsTnb/YRr6gOR8WT0LZNrhLh4y3FCgoAAAAXSHboAAAAAAAAAAABhlbgnAAAAEAivKe977CQCxMOKTuj+cWTFqc2OOJU8qGr9afrgu2zDmQaX5Q0cNshc3PiBwe0qw/+D/qJk5QqM5dYeSUGeDQP`

	// failure response
	hmock.
		On("POST", "https://localhost/transactions").
		ReturnString(400, transactionFailure)

	_, err := client.SubmitTransactionXDR(txXdr)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "horizon error")
		horizonError, ok := errors.Cause(err).(*Error)
		assert.Equal(t, ok, true)
		assert.Equal(t, horizonError.Problem.Title, "Transaction Failed")
	}

	// connection error
	hmock.
		On("POST", "https://localhost/transactions").
		ReturnError("http.Client error")

	_, err = client.SubmitTransactionXDR(txXdr)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "http.Client error")
		_, ok := err.(*Error)
		assert.Equal(t, ok, false)
	}

	// successful tx
	hmock.On(
		"POST",
		"https://localhost/transactions",
	).Return(func(request *http.Request) (*http.Response, error) {
		val := request.FormValue("tx")
		assert.Equal(t, val, txXdr)
		return httpmock.NewStringResponse(http.StatusOK, txSuccess), nil
	})

	resp, err := client.SubmitTransactionXDR(txXdr)
	if assert.NoError(t, err) {
		assert.IsType(t, resp, hProtocol.Transaction{})
		assert.Equal(t, resp.Links.Transaction.Href, "https://horizon-testnet.stellar.org/transactions/bcc7a97264dca0a51a63f7ea971b5e7458e334489673078bb2a34eb0cce910ca")
		assert.Equal(t, resp.Hash, "bcc7a97264dca0a51a63f7ea971b5e7458e334489673078bb2a34eb0cce910ca")
		assert.Equal(t, resp.Ledger, int32(354811))
		assert.Equal(t, resp.EnvelopeXdr, txXdr)
		assert.Equal(t, resp.ResultXdr, "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA=")
		assert.Equal(t, resp.ResultMetaXdr, `AAAAAQAAAAIAAAADAAVp+wAAAAAAAAAAEH3Rayw4M0iCLoEe96rPFNGYim8AVHJU0z4ebYZW4JwACBP/TuycHAAABD0AAuV+AAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAVp+wAAAAAAAAAAEH3Rayw4M0iCLoEe96rPFNGYim8AVHJU0z4ebYZW4JwACBP/TuycHAAABD0AAuV/AAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAwAAAAMABWn7AAAAAAAAAAAQfdFrLDgzSIIugR73qs8U0ZiKbwBUclTTPh5thlbgnAAIE/9O7JwcAAAEPQAC5X8AAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEABWn7AAAAAAAAAAAQfdFrLDgzSIIugR73qs8U0ZiKbwBUclTTPh5thlbgnAAIE+gGdbQcAAAEPQAC5X8AAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAABWn7AAAAAAAAAADJMEbE6B9ICmmmxOdv9hGvqA5HxZPQtk2uEuHjLcUKCgAAABdIdugAAAVp+wAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==`)
	}
}

func TestSubmitTransactionRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	kp := keypair.MustParseFull("SA26PHIKZM6CXDGR472SSGUQQRYXM6S437ZNHZGRM6QA4FOPLLLFRGDX")
	sourceAccount := txnbuild.NewSimpleAccount(kp.Address(), int64(0))

	payment := txnbuild.Payment{
		Destination: kp.Address(),
		Amount:      "10",
		Asset:       txnbuild.NativeAsset{},
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&payment},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimebounds(0, 10)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp)
	assert.NoError(t, err)

	txXdr := "AAAAAgAAAAAFNPMlEPLB6oWPI/Zl1sBEXxwv93ChUnv7KQK9KxrTtgAAAGQAAAAAAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAKAAAAAAAAAAEAAAAAAAAAAQAAAAAFNPMlEPLB6oWPI/Zl1sBEXxwv93ChUnv7KQK9KxrTtgAAAAAAAAAABfXhAAAAAAAAAAABKxrTtgAAAECmVMsI0W6JmfJNeLzgH+PseZA2AgYGZl8zaHgkOvhZw65Hj9OaCdw6yssG55qu7X2sauJAwfxaoTL4gwbmH94H"
	// successful tx with config.memo_required not found
	hmock.On(
		"POST",
		"https://localhost/transactions",
	).Return(func(request *http.Request) (*http.Response, error) {
		val := request.FormValue("tx")
		assert.Equal(t, val, txXdr)
		return httpmock.NewStringResponse(http.StatusOK, txSuccess), nil
	})

	hmock.On(
		"GET",
		"https://localhost/accounts/GACTJ4ZFCDZMD2UFR4R7MZOWYBCF6HBP65YKCUT37MUQFPJLDLJ3N5D2/data/config.memo_required",
	).ReturnString(404, notFoundResponse)

	_, err = client.SubmitTransaction(tx)
	assert.NoError(t, err)

	// verify submit parses correctly when result_meta_xdr absent when skip_meta=true
	hmock.On(
		"POST",
		"https://localhost/transactions",
	).Return(func(request *http.Request) (*http.Response, error) {
		val := request.FormValue("tx")
		assert.Equal(t, val, txXdr)
		return httpmock.NewStringResponse(http.StatusOK, strings.Replace(txDetailResponse, "<result_meta_xdr>", "", 1)), nil
	})

	hmock.On(
		"GET",
		"https://localhost/accounts/GACTJ4ZFCDZMD2UFR4R7MZOWYBCF6HBP65YKCUT37MUQFPJLDLJ3N5D2/data/config.memo_required",
	).ReturnString(404, notFoundResponse)

	theTx, err := client.SubmitTransaction(tx)
	assert.NoError(t, err)
	assert.Empty(t, theTx.ResultMetaXdr)

	// memo required - does not submit transaction
	hmock.On(
		"GET",
		"https://localhost/accounts/GACTJ4ZFCDZMD2UFR4R7MZOWYBCF6HBP65YKCUT37MUQFPJLDLJ3N5D2/data/config.memo_required",
	).ReturnJSON(200, memoRequiredResponse)

	_, err = client.SubmitTransaction(tx)
	assert.Error(t, err)
	assert.Equal(t, ErrAccountRequiresMemo, errors.Cause(err))
}

func TestSubmitTransactionRequestMuxedAccounts(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	kp := keypair.MustParseFull("SA26PHIKZM6CXDGR472SSGUQQRYXM6S437ZNHZGRM6QA4FOPLLLFRGDX")
	accountID := xdr.MustAddress(kp.Address())
	mx := xdr.MuxedAccount{
		Type: xdr.CryptoKeyTypeKeyTypeMuxedEd25519,
		Med25519: &xdr.MuxedAccountMed25519{
			Id:      0xcafebabe,
			Ed25519: *accountID.Ed25519,
		},
	}
	sourceAccount := txnbuild.NewSimpleAccount(mx.Address(), int64(0))

	payment := txnbuild.Payment{
		Destination: kp.Address(),
		Amount:      "10",
		Asset:       txnbuild.NativeAsset{},
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&payment},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimebounds(0, 10)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp)
	assert.NoError(t, err)

	txXdr := "AAAAAgAAAQAAAAAAyv66vgU08yUQ8sHqhY8j9mXWwERfHC/3cKFSe/spAr0rGtO2AAAAZAAAAAAAAAABAAAAAQAAAAAAAAAAAAAAAAAAAAoAAAAAAAAAAQAAAAAAAAABAAAAAAU08yUQ8sHqhY8j9mXWwERfHC/3cKFSe/spAr0rGtO2AAAAAAAAAAAF9eEAAAAAAAAAAAErGtO2AAAAQJvQkE9UVo/mfFBl/8ZPTzSUyVO4nvW0BYfnbowoBPEdRfLOLQz28v6sBKQc2b86NUfVHN5TQVo3+jH4nK9wVgk="
	// successful tx with config.memo_required not found
	hmock.On(
		"POST",
		"https://localhost/transactions",
	).Return(func(request *http.Request) (*http.Response, error) {
		val := request.FormValue("tx")
		assert.Equal(t, val, txXdr)
		return httpmock.NewStringResponse(http.StatusOK, txSuccess), nil
	})

	hmock.On(
		"GET",
		"https://localhost/accounts/GACTJ4ZFCDZMD2UFR4R7MZOWYBCF6HBP65YKCUT37MUQFPJLDLJ3N5D2/data/config.memo_required",
	).ReturnString(404, notFoundResponse)

	_, err = client.SubmitTransaction(tx)
	assert.NoError(t, err)

	// memo required - does not submit transaction
	hmock.On(
		"GET",
		"https://localhost/accounts/GACTJ4ZFCDZMD2UFR4R7MZOWYBCF6HBP65YKCUT37MUQFPJLDLJ3N5D2/data/config.memo_required",
	).ReturnJSON(200, memoRequiredResponse)

	_, err = client.SubmitTransaction(tx)
	assert.Error(t, err)
	assert.Equal(t, ErrAccountRequiresMemo, errors.Cause(err))
}

func TestSubmitFeeBumpTransaction(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	kp := keypair.MustParseFull("SA26PHIKZM6CXDGR472SSGUQQRYXM6S437ZNHZGRM6QA4FOPLLLFRGDX")
	sourceAccount := txnbuild.NewSimpleAccount(kp.Address(), int64(0))

	payment := txnbuild.Payment{
		Destination: kp.Address(),
		Amount:      "10",
		Asset:       txnbuild.NativeAsset{},
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&payment},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimebounds(0, 10)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp)
	assert.NoError(t, err)

	feeBumpKP := keypair.MustParseFull("SA5ZEFDVFZ52GRU7YUGR6EDPBNRU2WLA6IQFQ7S2IH2DG3VFV3DOMV2Q")
	feeBumpTx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      tx,
		FeeAccount: feeBumpKP.Address(),
		BaseFee:    txnbuild.MinBaseFee * 2,
	})
	feeBumpTx, err = feeBumpTx.Sign(network.TestNetworkPassphrase, feeBumpKP)
	feeBumpTxB64, err := feeBumpTx.Base64()
	assert.NoError(t, err)

	// successful tx with config.memo_required not found
	hmock.On(
		"POST",
		"https://localhost/transactions",
	).Return(func(request *http.Request) (*http.Response, error) {
		val := request.FormValue("tx")
		assert.Equal(t, val, feeBumpTxB64)
		return httpmock.NewStringResponse(http.StatusOK, txSuccess), nil
	})

	hmock.On(
		"GET",
		"https://localhost/accounts/GACTJ4ZFCDZMD2UFR4R7MZOWYBCF6HBP65YKCUT37MUQFPJLDLJ3N5D2/data/config.memo_required",
	).ReturnString(404, notFoundResponse)

	_, err = client.SubmitFeeBumpTransaction(feeBumpTx)
	assert.NoError(t, err)

	// memo required - does not submit transaction
	hmock.On(
		"GET",
		"https://localhost/accounts/GACTJ4ZFCDZMD2UFR4R7MZOWYBCF6HBP65YKCUT37MUQFPJLDLJ3N5D2/data/config.memo_required",
	).ReturnJSON(200, memoRequiredResponse)

	_, err = client.SubmitFeeBumpTransaction(feeBumpTx)
	assert.Error(t, err)
	assert.Equal(t, ErrAccountRequiresMemo, errors.Cause(err))
}

func TestSubmitTransactionWithOptionsRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	kp := keypair.MustParseFull("SA26PHIKZM6CXDGR472SSGUQQRYXM6S437ZNHZGRM6QA4FOPLLLFRGDX")
	sourceAccount := txnbuild.NewSimpleAccount(kp.Address(), int64(0))

	payment := txnbuild.Payment{
		Destination: kp.Address(),
		Amount:      "10",
		Asset:       txnbuild.NativeAsset{},
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&payment},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimebounds(0, 10)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp)
	assert.NoError(t, err)

	hmock.
		On("POST", "https://localhost/transactions").
		ReturnString(400, transactionFailure)

	_, err = client.SubmitTransactionWithOptions(tx, SubmitTxOpts{SkipMemoRequiredCheck: true})

	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "horizon error")
		horizonError, ok := errors.Cause(err).(*Error)
		assert.Equal(t, ok, true)
		assert.Equal(t, horizonError.Problem.Title, "Transaction Failed")
	}

	// connection error
	hmock.
		On("POST", "https://localhost/transactions").
		ReturnError("http.Client error")

	_, err = client.SubmitTransactionWithOptions(tx, SubmitTxOpts{SkipMemoRequiredCheck: true})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "http.Client error")
		_, ok := err.(*Error)
		assert.Equal(t, ok, false)
	}

	txXdr := "AAAAAgAAAAAFNPMlEPLB6oWPI/Zl1sBEXxwv93ChUnv7KQK9KxrTtgAAAGQAAAAAAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAKAAAAAAAAAAEAAAAAAAAAAQAAAAAFNPMlEPLB6oWPI/Zl1sBEXxwv93ChUnv7KQK9KxrTtgAAAAAAAAAABfXhAAAAAAAAAAABKxrTtgAAAECmVMsI0W6JmfJNeLzgH+PseZA2AgYGZl8zaHgkOvhZw65Hj9OaCdw6yssG55qu7X2sauJAwfxaoTL4gwbmH94H"
	// successful tx
	hmock.On(
		"POST",
		"https://localhost/transactions",
	).Return(func(request *http.Request) (*http.Response, error) {
		val := request.FormValue("tx")
		assert.Equal(t, val, txXdr)
		return httpmock.NewStringResponse(http.StatusOK, txSuccess), nil
	})

	_, err = client.SubmitTransactionWithOptions(tx, SubmitTxOpts{SkipMemoRequiredCheck: true})
	assert.NoError(t, err)

	// successful tx with config.memo_required not found
	hmock.On(
		"POST",
		"https://localhost/transactions",
	).Return(func(request *http.Request) (*http.Response, error) {
		val := request.FormValue("tx")
		assert.Equal(t, val, txXdr)
		return httpmock.NewStringResponse(http.StatusOK, txSuccess), nil
	})

	hmock.On(
		"GET",
		"https://localhost/accounts/GACTJ4ZFCDZMD2UFR4R7MZOWYBCF6HBP65YKCUT37MUQFPJLDLJ3N5D2/data/config.memo_required",
	).ReturnString(404, notFoundResponse)

	_, err = client.SubmitTransactionWithOptions(tx, SubmitTxOpts{SkipMemoRequiredCheck: false})
	assert.NoError(t, err)

	// memo required - does not submit transaction
	hmock.On(
		"GET",
		"https://localhost/accounts/GACTJ4ZFCDZMD2UFR4R7MZOWYBCF6HBP65YKCUT37MUQFPJLDLJ3N5D2/data/config.memo_required",
	).ReturnJSON(200, memoRequiredResponse)

	_, err = client.SubmitTransactionWithOptions(tx, SubmitTxOpts{SkipMemoRequiredCheck: false})
	assert.Error(t, err)
	assert.Equal(t, ErrAccountRequiresMemo, errors.Cause(err))

	// skips memo check if tx includes a memo
	txXdr = "AAAAAgAAAAAFNPMlEPLB6oWPI/Zl1sBEXxwv93ChUnv7KQK9KxrTtgAAAGQAAAAAAAAAAgAAAAEAAAAAAAAAAAAAAAAAAAAKAAAAAQAAAApIZWxsb1dvcmxkAAAAAAABAAAAAAAAAAEAAAAABTTzJRDyweqFjyP2ZdbARF8cL/dwoVJ7+ykCvSsa07YAAAAAAAAAAAX14QAAAAAAAAAAASsa07YAAABA7rDHZ+HcBIQbWByMZL3aT231WuwjOhxvb0c1i3vPzArUCE+HdCIJXq6Mk/xdhJj6QEEJrg15uAxke3P3k2vWCw=="
	hmock.On(
		"POST",
		"https://localhost/transactions",
	).Return(func(request *http.Request) (*http.Response, error) {
		val := request.FormValue("tx")
		assert.Equal(t, val, txXdr)
		return httpmock.NewStringResponse(http.StatusOK, txSuccess), nil
	})

	tx, err = txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&payment},
			BaseFee:              txnbuild.MinBaseFee,
			Memo:                 txnbuild.MemoText("HelloWorld"),
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimebounds(0, 10)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp)
	assert.NoError(t, err)

	_, err = client.SubmitTransactionWithOptions(tx, SubmitTxOpts{SkipMemoRequiredCheck: false})
	assert.NoError(t, err)
}

func TestSubmitFeeBumpTransactionWithOptions(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	kp := keypair.MustParseFull("SA26PHIKZM6CXDGR472SSGUQQRYXM6S437ZNHZGRM6QA4FOPLLLFRGDX")
	sourceAccount := txnbuild.NewSimpleAccount(kp.Address(), int64(0))

	payment := txnbuild.Payment{
		Destination: kp.Address(),
		Amount:      "10",
		Asset:       txnbuild.NativeAsset{},
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&payment},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimebounds(0, 10)},
		},
	)
	assert.NoError(t, err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp)
	assert.NoError(t, err)

	feeBumpKP := keypair.MustParseFull("SA5ZEFDVFZ52GRU7YUGR6EDPBNRU2WLA6IQFQ7S2IH2DG3VFV3DOMV2Q")
	feeBumpTx, err := txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      tx,
		FeeAccount: feeBumpKP.Address(),
		BaseFee:    txnbuild.MinBaseFee * 2,
	})
	feeBumpTx, err = feeBumpTx.Sign(network.TestNetworkPassphrase, feeBumpKP)
	feeBumpTxB64, err := feeBumpTx.Base64()
	assert.NoError(t, err)

	hmock.
		On("POST", "https://localhost/transactions").
		ReturnString(400, transactionFailure)

	_, err = client.SubmitFeeBumpTransactionWithOptions(feeBumpTx, SubmitTxOpts{SkipMemoRequiredCheck: true})

	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "horizon error")
		horizonError, ok := errors.Cause(err).(*Error)
		assert.Equal(t, ok, true)
		assert.Equal(t, horizonError.Problem.Title, "Transaction Failed")
	}

	// connection error
	hmock.
		On("POST", "https://localhost/transactions").
		ReturnError("http.Client error")

	_, err = client.SubmitFeeBumpTransactionWithOptions(feeBumpTx, SubmitTxOpts{SkipMemoRequiredCheck: true})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "http.Client error")
		_, ok := err.(*Error)
		assert.Equal(t, ok, false)
	}

	// successful tx
	hmock.On(
		"POST",
		"https://localhost/transactions",
	).Return(func(request *http.Request) (*http.Response, error) {
		val := request.FormValue("tx")
		assert.Equal(t, val, feeBumpTxB64)
		return httpmock.NewStringResponse(http.StatusOK, txSuccess), nil
	})

	_, err = client.SubmitFeeBumpTransactionWithOptions(feeBumpTx, SubmitTxOpts{SkipMemoRequiredCheck: true})
	assert.NoError(t, err)

	// successful tx with config.memo_required not found
	hmock.On(
		"POST",
		"https://localhost/transactions",
	).Return(func(request *http.Request) (*http.Response, error) {
		val := request.FormValue("tx")
		assert.Equal(t, val, feeBumpTxB64)
		return httpmock.NewStringResponse(http.StatusOK, txSuccess), nil
	})

	hmock.On(
		"GET",
		"https://localhost/accounts/GACTJ4ZFCDZMD2UFR4R7MZOWYBCF6HBP65YKCUT37MUQFPJLDLJ3N5D2/data/config.memo_required",
	).ReturnString(404, notFoundResponse)

	_, err = client.SubmitFeeBumpTransactionWithOptions(feeBumpTx, SubmitTxOpts{SkipMemoRequiredCheck: false})
	assert.NoError(t, err)

	// memo required - does not submit transaction
	hmock.On(
		"GET",
		"https://localhost/accounts/GACTJ4ZFCDZMD2UFR4R7MZOWYBCF6HBP65YKCUT37MUQFPJLDLJ3N5D2/data/config.memo_required",
	).ReturnJSON(200, memoRequiredResponse)

	_, err = client.SubmitFeeBumpTransactionWithOptions(feeBumpTx, SubmitTxOpts{SkipMemoRequiredCheck: false})
	assert.Error(t, err)
	assert.Equal(t, ErrAccountRequiresMemo, errors.Cause(err))

	tx, err = txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&payment},
			BaseFee:              txnbuild.MinBaseFee,
			Memo:                 txnbuild.MemoText("HelloWorld"),
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimebounds(0, 10)},
		},
	)
	assert.NoError(t, err)

	feeBumpKP = keypair.MustParseFull("SA5ZEFDVFZ52GRU7YUGR6EDPBNRU2WLA6IQFQ7S2IH2DG3VFV3DOMV2Q")
	feeBumpTx, err = txnbuild.NewFeeBumpTransaction(txnbuild.FeeBumpTransactionParams{
		Inner:      tx,
		FeeAccount: feeBumpKP.Address(),
		BaseFee:    txnbuild.MinBaseFee * 2,
	})
	feeBumpTx, err = feeBumpTx.Sign(network.TestNetworkPassphrase, feeBumpKP)
	feeBumpTxB64, err = feeBumpTx.Base64()
	assert.NoError(t, err)

	// skips memo check if tx includes a memo
	hmock.On(
		"POST",
		"https://localhost/transactions",
	).Return(func(request *http.Request) (*http.Response, error) {
		val := request.FormValue("tx")
		assert.Equal(t, val, feeBumpTxB64)
		return httpmock.NewStringResponse(http.StatusOK, txSuccess), nil
	})

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp)
	assert.NoError(t, err)

	_, err = client.SubmitFeeBumpTransactionWithOptions(feeBumpTx, SubmitTxOpts{SkipMemoRequiredCheck: false})
	assert.NoError(t, err)
}

func TestTransactionsRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	transactionRequest := TransactionRequest{}

	// all transactions
	hmock.On(
		"GET",
		"https://localhost/transactions",
	).ReturnString(200, txPageResponse)

	txs, err := client.Transactions(transactionRequest)
	if assert.NoError(t, err) {
		assert.IsType(t, txs, hProtocol.TransactionsPage{})
		links := txs.Links
		assert.Equal(t, links.Self.Href, "https://horizon-testnet.stellar.org/transactions?cursor=&limit=10&order=desc")

		assert.Equal(t, links.Next.Href, "https://horizon-testnet.stellar.org/transactions?cursor=1881762611335168&limit=10&order=desc")

		assert.Equal(t, links.Prev.Href, "https://horizon-testnet.stellar.org/transactions?cursor=1881771201286144&limit=10&order=asc")

		tx := txs.Embedded.Records[0]
		assert.IsType(t, tx, hProtocol.Transaction{})
		assert.Equal(t, tx.ID, "3274f131af56ecb6d8668acf6eb0b31b5f8faeca785cbce0a911a5a81308a599")
		assert.Equal(t, tx.Ledger, int32(438134))
		assert.Equal(t, tx.FeeCharged, int64(100))
		assert.Equal(t, tx.MaxFee, int64(100))
		assert.Equal(t, tx.Hash, "3274f131af56ecb6d8668acf6eb0b31b5f8faeca785cbce0a911a5a81308a599")
	}

	transactionRequest = TransactionRequest{ForAccount: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU"}
	hmock.On(
		"GET",
		"https://localhost/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/transactions",
	).ReturnString(200, txPageResponse)

	txs, err = client.Transactions(transactionRequest)
	if assert.NoError(t, err) {
		assert.IsType(t, txs, hProtocol.TransactionsPage{})
	}

	// too many parameters
	transactionRequest = TransactionRequest{ForAccount: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU", ForLedger: 123}
	hmock.On(
		"GET",
		"https://localhost/transactions",
	).ReturnString(200, txPageResponse)

	_, err = client.Transactions(transactionRequest)
	// error case
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "too many parameters")
	}

	// transaction detail
	txHash := "5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c"
	hmock.On(
		"GET",
		"https://localhost/transactions/5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c",
	).ReturnString(200, strings.Replace(txDetailResponse, "<result_meta_xdr>", "result_meta_xdr: AAAAAQAAAAIAAAADAAavdgAAAAAAAAAAtoYrQZHbnPLAFsF4YB88J5VSg0/piQNHm0SL9l0HW1EAAAAXSHbnnAAGr3UAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAavdgAAAAAAAAAAtoYrQZHbnPLAFsF4YB88J5VSg0/piQNHm0SL9l0HW1EAAAAXSHbnnAAGr3UAAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMABq9zAAAAAAAAAADMSEvcRKXsaUNna++Hy7gWm/CfqTjEA7xoGypfrFGUHAAAAAUQ/z+cAABeBgAASuQAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEABq92AAAAAAAAAADMSEvcRKXsaUNna++Hy7gWm/CfqTjEA7xoGypfrFGUHAAAAAcXjracAABeBgAASuQAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMABq92AAAAAAAAAAC2hitBkduc8sAWwXhgHzwnlVKDT+mJA0ebRIv2XQdbUQAAABdIduecAAavdQAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEABq92AAAAAAAAAAC2hitBkduc8sAWwXhgHzwnlVKDT+mJA0ebRIv2XQdbUQAAABVB53CcAAavdQAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==", 1))

	record, err := client.TransactionDetail(txHash)
	if assert.NoError(t, err) {
		assert.Equal(t, record.ID, "5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c")
		assert.Equal(t, record.Successful, true)
		assert.Equal(t, record.Hash, "5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c")
		assert.Equal(t, record.Memo, "2A1V6J5703G47XHY")
	}

	// transaction detail when skip meta enabled and result_meta_xdr is absent
	hmock.On(
		"GET",
		"https://localhost/transactions/5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c",
	).ReturnString(200, strings.Replace(txDetailResponse, "<result_meta_xdr>", "", 1))

	record, err = client.TransactionDetail(txHash)
	if assert.NoError(t, err) {
		assert.Empty(t, record.ResultMetaXdr)
	}
}

func TestOrderBookRequest(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	orderBookRequest := OrderBookRequest{BuyingAssetType: AssetTypeNative, SellingAssetCode: "USD", SellingAssetType: AssetType4, SellingAssetIssuer: "GBVOL67TMUQBGL4TZYNMY3ZQ5WGQYFPFD5VJRWXR72VA33VFNL225PL5"}

	// orderbook for XLM/USD
	hmock.On(
		"GET",
		"https://localhost/order_book?buying_asset_type=native&selling_asset_code=USD&selling_asset_issuer=GBVOL67TMUQBGL4TZYNMY3ZQ5WGQYFPFD5VJRWXR72VA33VFNL225PL5&selling_asset_type=credit_alphanum4",
	).ReturnString(200, orderbookResponse)

	obs, err := client.OrderBook(orderBookRequest)
	if assert.NoError(t, err) {
		assert.IsType(t, obs, hProtocol.OrderBookSummary{})
		bids := obs.Bids
		asks := obs.Asks
		assert.Equal(t, bids[0].Price, "0.0000251")
		assert.Equal(t, asks[0].Price, "0.0000256")
		assert.Equal(t, obs.Selling.Type, "native")
		assert.Equal(t, obs.Buying.Type, "credit_alphanum4")
	}

	// failure response
	orderBookRequest = OrderBookRequest{}
	hmock.On(
		"GET",
		"https://localhost/order_book",
	).ReturnString(400, orderBookNotFound)

	_, err = client.OrderBook(orderBookRequest)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "horizon error")
		horizonError, ok := err.(*Error)
		assert.Equal(t, ok, true)
		assert.Equal(t, horizonError.Problem.Title, "Invalid Order Book Parameters")
	}

}

func TestFetchTimebounds(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
		clock: &clock.Clock{
			Source: clocktest.FixedSource(time.Unix(1560947096, 0)),
		},
	}

	// When no saved server time, return local time
	st, err := client.FetchTimebounds(100)
	if assert.NoError(t, err) {
		serverTimeMapMutex.Lock()
		assert.IsType(t, ServerTimeMap["localhost"], ServerTimeRecord{})
		serverTimeMapMutex.Unlock()
		assert.Equal(t, st.MinTime, int64(0))
	}

	// server time is saved on requests to horizon
	header := http.Header{}
	header.Add("Date", "Wed, 19 Jun 2019 12:24:56 GMT") //unix time: 1560947096
	hmock.On(
		"GET",
		"https://localhost/",
	).ReturnStringWithHeader(200, metricsResponse, header)
	_, err = client.Root()
	assert.NoError(t, err)

	// get saved server time
	st, err = client.FetchTimebounds(100)
	if assert.NoError(t, err) {
		serverTimeMapMutex.Lock()
		assert.IsType(t, ServerTimeMap["localhost"], ServerTimeRecord{})
		serverTimeMapMutex.Unlock()
		assert.Equal(t, st.MinTime, int64(0))
		// serverTime + 100seconds
		assert.Equal(t, st.MaxTime, int64(1560947196))
	}

	// mock server time
	newRecord := ServerTimeRecord{ServerTime: 100, LocalTimeRecorded: 1560947096}
	serverTimeMapMutex.Lock()
	ServerTimeMap["localhost"] = newRecord
	serverTimeMapMutex.Unlock()
	st, err = client.FetchTimebounds(100)
	assert.NoError(t, err)
	assert.IsType(t, st, txnbuild.TimeBounds{})
	assert.Equal(t, st.MinTime, int64(0))
	// time should be 200, serverTime + 100seconds
	assert.Equal(t, st.MaxTime, int64(200))
}

func TestVersion(t *testing.T) {
	hmock := httptest.NewClient()
	client := &Client{
		HorizonURL: "https://localhost/",
		HTTP:       hmock,
	}

	assert.Equal(t, "2.1.0", client.Version())
}

var accountsResponse = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/accounts?cursor=\u0026limit=10\u0026order=asc\u0026signer=GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP"
    },
    "next": {
      "href": "https://horizon-testnet.stellar.org/accounts?cursor=GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP\u0026limit=10\u0026order=asc\u0026signer=GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP"
    },
    "prev": {
      "href": "https://horizon-testnet.stellar.org/accounts?cursor=GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP\u0026limit=10\u0026order=desc\u0026signer=GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP"
    }
  },
  "_embedded": {
    "records": [
      {
        "_links": {
          "self": {
            "href": "https://horizon-testnet.stellar.org/accounts/GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP"
          },
          "transactions": {
            "href": "https://horizon-testnet.stellar.org/accounts/GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP/transactions{?cursor,limit,order}",
            "templated": true
          },
          "operations": {
            "href": "https://horizon-testnet.stellar.org/accounts/GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP/operations{?cursor,limit,order}",
            "templated": true
          },
          "payments": {
            "href": "https://horizon-testnet.stellar.org/accounts/GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP/payments{?cursor,limit,order}",
            "templated": true
          },
          "effects": {
            "href": "https://horizon-testnet.stellar.org/accounts/GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP/effects{?cursor,limit,order}",
            "templated": true
          },
          "offers": {
            "href": "https://horizon-testnet.stellar.org/accounts/GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP/offers{?cursor,limit,order}",
            "templated": true
          },
          "trades": {
            "href": "https://horizon-testnet.stellar.org/accounts/GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP/trades{?cursor,limit,order}",
            "templated": true
          },
          "data": {
            "href": "https://horizon-testnet.stellar.org/accounts/GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP/data/{key}",
            "templated": true
          }
        },
        "id": "GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP",
        "account_id": "GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP",
        "sequence": "47236050321450",
        "subentry_count": 0,
        "last_modified_ledger": 116787,
        "thresholds": {
          "low_threshold": 0,
          "med_threshold": 0,
          "high_threshold": 0
        },
        "flags": {
          "auth_required": false,
          "auth_revocable": false,
          "auth_immutable": false
        },
        "balances": [
          {
            "balance": "100.8182300",
            "buying_liabilities": "0.0000000",
            "selling_liabilities": "0.0000000",
            "asset_type": "native"
          }
        ],
        "signers": [
          {
            "weight": 1,
            "key": "GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP",
            "type": "ed25519_public_key"
          }
        ],
        "data": {},
        "paging_token": "GAI3SO3S4E67HAUZPZ2D3VBFXY4AT6N7WQI7K5WFGRXWENTZJG2B6CYP"
      }
    ]
  }
}
`

var accountResponse = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU"
    },
    "transactions": {
      "href": "https://horizon-testnet.stellar.org/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/transactions{?cursor,limit,order}",
      "templated": true
    },
    "operations": {
      "href": "https://horizon-testnet.stellar.org/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/operations{?cursor,limit,order}",
      "templated": true
    },
    "payments": {
      "href": "https://horizon-testnet.stellar.org/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/payments{?cursor,limit,order}",
      "templated": true
    },
    "effects": {
      "href": "https://horizon-testnet.stellar.org/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/effects{?cursor,limit,order}",
      "templated": true
    },
    "offers": {
      "href": "https://horizon-testnet.stellar.org/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/offers{?cursor,limit,order}",
      "templated": true
    },
    "trades": {
      "href": "https://horizon-testnet.stellar.org/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/trades{?cursor,limit,order}",
      "templated": true
    },
    "data": {
      "href": "https://horizon-testnet.stellar.org/accounts/GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU/data/{key}",
      "templated": true
    }
  },
  "id": "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
  "paging_token": "1",
  "account_id": "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
  "sequence": "9865509814140929",
  "subentry_count": 1,
  "thresholds": {
    "low_threshold": 0,
    "med_threshold": 0,
    "high_threshold": 0
  },
  "flags": {
    "auth_required": false,
    "auth_revocable": false,
    "auth_immutable": false
  },
  "balances": [
    {
      "balance": "9999.9999900",
      "buying_liabilities": "0.0000000",
      "selling_liabilities": "0.0000000",
      "asset_type": "native"
    }
  ],
  "signers": [
    {
      "public_key": "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
      "weight": 1,
      "key": "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
      "type": "ed25519_public_key"
    }
  ],
  "data": {
    "test": "dGVzdA=="
  },
  "last_modified_ledger": 103307,
  "last_modified_time": "2019-03-05T13:23:50Z"
}`

var memoRequiredResponse = map[string]string{
	"value": "MQ==",
}

var notFoundResponse = `{
  "type": "https://stellar.org/horizon-errors/not_found",
  "title": "Resource Missing",
  "status": 404,
  "detail": "The resource at the url requested was not found.  This is usually occurs for one of two reasons:  The url requested is not valid, or no data in our database could be found with the parameters provided.",
  "instance": "horizon-live-001/61KdRW8tKi-18408110"
}`

var accountData = `{
  "value": "dGVzdA=="
}`

var effectsResponse = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/operations/43989725060534273/effects?cursor=&limit=10&order=asc"
    },
    "next": {
      "href": "https://horizon-testnet.stellar.org/operations/43989725060534273/effects?cursor=43989725060534273-3&limit=10&order=asc"
    },
    "prev": {
      "href": "https://horizon-testnet.stellar.org/operations/43989725060534273/effects?cursor=43989725060534273-1&limit=10&order=desc"
    }
  },
  "_embedded": {
    "records": [
      {
        "_links": {
          "operation": {
            "href": "https://horizon-testnet.stellar.org/operations/43989725060534273"
          },
          "succeeds": {
            "href": "https://horizon-testnet.stellar.org/effects?order=desc&cursor=43989725060534273-1"
          },
          "precedes": {
            "href": "https://horizon-testnet.stellar.org/effects?order=asc&cursor=43989725060534273-1"
          }
        },
        "id": "0043989725060534273-0000000001",
        "paging_token": "43989725060534273-1",
        "account": "GANHAS5OMPLKD6VYU4LK7MBHSHB2Q37ZHAYWOBJRUXGDHMPJF3XNT45Y",
        "type": "account_debited",
        "type_i": 3,
        "created_at": "2018-07-27T21:00:12Z",
        "asset_type": "native",
        "amount": "9999.9999900"
      },
      {
        "_links": {
          "operation": {
            "href": "https://horizon-testnet.stellar.org/operations/43989725060534273"
          },
          "succeeds": {
            "href": "https://horizon-testnet.stellar.org/effects?order=desc&cursor=43989725060534273-2"
          },
          "precedes": {
            "href": "https://horizon-testnet.stellar.org/effects?order=asc&cursor=43989725060534273-2"
          }
        },
        "id": "0043989725060534273-0000000002",
        "paging_token": "43989725060534273-2",
        "account": "GBO7LQUWCC7M237TU2PAXVPOLLYNHYCYYFCLVMX3RBJCML4WA742X3UB",
        "type": "account_credited",
        "type_i": 2,
        "created_at": "2018-07-27T21:00:12Z",
        "asset_type": "native",
        "amount": "9999.9999900"
      },
      {
        "_links": {
          "operation": {
            "href": "https://horizon-testnet.stellar.org/operations/43989725060534273"
          },
          "succeeds": {
            "href": "https://horizon-testnet.stellar.org/effects?order=desc&cursor=43989725060534273-3"
          },
          "precedes": {
            "href": "https://horizon-testnet.stellar.org/effects?order=asc&cursor=43989725060534273-3"
          }
        },
        "id": "0043989725060534273-0000000003",
        "paging_token": "43989725060534273-3",
        "account": "GANHAS5OMPLKD6VYU4LK7MBHSHB2Q37ZHAYWOBJRUXGDHMPJF3XNT45Y",
        "type": "account_removed",
        "type_i": 1,
        "created_at": "2018-07-27T21:00:12Z"
      }
    ]
  }
}`

var assetsResponse = `{
    "_links": {
        "self": {
            "href": "https://horizon-testnet.stellar.org/assets?cursor=&limit=1&order=desc"
        },
        "next": {
            "href": "https://horizon-testnet.stellar.org/assets?cursor=ABC_GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU_credit_alphanum12&limit=1&order=desc"
        },
        "prev": {
            "href": "https://horizon-testnet.stellar.org/assets?cursor=ABC_GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU_credit_alphanum12&limit=1&order=asc"
        }
    },
    "_embedded": {
        "records": [
            {
                "_links": {
                    "toml": {
                        "href": ""
                    }
                },
                "asset_type": "credit_alphanum12",
                "asset_code": "ABC",
                "asset_issuer": "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU",
                "paging_token": "1",
                "amount": "105.0000000",
                "num_accounts": 3,
                "flags": {
                    "auth_required": true,
                    "auth_revocable": false,
                    "auth_immutable": false
                }
            }
        ]
    }
}`

var ledgerResponse = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/ledgers/69859"
    },
    "transactions": {
      "href": "https://horizon-testnet.stellar.org/ledgers/69859/transactions{?cursor,limit,order}",
      "templated": true
    },
    "operations": {
      "href": "https://horizon-testnet.stellar.org/ledgers/69859/operations{?cursor,limit,order}",
      "templated": true
    },
    "payments": {
      "href": "https://horizon-testnet.stellar.org/ledgers/69859/payments{?cursor,limit,order}",
      "templated": true
    },
    "effects": {
      "href": "https://horizon-testnet.stellar.org/ledgers/69859/effects{?cursor,limit,order}",
      "templated": true
    }
  },
  "id": "71a40c0581d8d7c1158e1d9368024c5f9fd70de17a8d277cdd96781590cc10fb",
  "paging_token": "300042120331264",
  "hash": "71a40c0581d8d7c1158e1d9368024c5f9fd70de17a8d277cdd96781590cc10fb",
  "prev_hash": "78979bed15463bfc3b0c1915acc6aec866565d360ba6565d26ffbb3dc484f18c",
  "sequence": 69859,
  "successful_transaction_count": 0,
  "failed_transaction_count": 1,
  "operation_count": 0,
  "closed_at": "2019-03-03T13:38:16Z",
  "total_coins": "100000000000.0000000",
  "fee_pool": "10.7338093",
  "base_fee_in_stroops": 100,
  "base_reserve_in_stroops": 5000000,
  "max_tx_set_size": 100,
  "protocol_version": 10,
  "header_xdr": "AAAACniXm+0VRjv8OwwZFazGrshmVl02C6ZWXSb/uz3EhPGMLuFhI0sVqAG57WnGMUKmOUk/J8TAktUB97VgrgEsZuEAAAAAXHvYyAAAAAAAAAAAcvWzXsmT72oXZ7QPC1nZLJei+lFwYRXF4FIz/PQguubMDKGRJrT/0ofTHlZjWAMWjABeGgup7zhfZkm0xrthCAABEOMN4Lazp2QAAAAAAAAGZdltAAAAAAAAAAAABOqvAAAAZABMS0AAAABk4Vse3u3dDM9UWfoH9ooQLLSXYEee8xiHu/k9p6YLlWR2KT4hYGehoHGmp04rhMRMAEp+GHE+KXv0UUxAPmmNmwGYK2HFCnl5a931YmTQYrHQzEeCHx+aI4+TLjTlFjMqAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
}`

var metricsResponse = `{
  "_links": {
    "self": {
      "href": "/metrics"
    }
  },
  "goroutines": {
    "value": 1893
  },
  "history.elder_ledger": {
    "value": 1
  },
  "history.latest_ledger": {
    "value": 22826153
  },
  "history.open_connections": {
    "value": 27
	},
  "ingest.ledger_graph_only_ingestion": {
    "15m.rate": 0,
    "1m.rate": 0,
    "5m.rate": 0,
    "75%": 0,
    "95%": 0,
    "99%": 0,
    "99.9%": 0,
    "count": 0,
    "max": 0,
    "mean": 0,
    "mean.rate": 0,
    "median": 0,
    "min": 10,
    "stddev": 0
  },
  "ingest.ledger_ingestion": {
    "15m.rate": 4.292383845297832,
    "1m.rate": 1.6828538278349856,
    "5m.rate": 3.7401206537727854,
    "75%": 0.0918039395,
    "95%": 0.11669889484999994,
    "99%": 0.143023258,
    "99.9%": 0.143023258,
    "count": 36,
    "max": 0.143023258,
    "mean": 0.074862138,
    "mean.rate": 0.48723881363424204,
    "median": 0.0706217925,
    "min": 0.03396778,
    "stddev": 0.023001478
  },
  "ingest.state_verify": {
    "15m.rate": 0,
    "1m.rate": 0,
    "5m.rate": 0,
    "75%": 0,
    "95%": 0,
    "99%": 0,
    "99.9%": 230.123456,
    "count": 0,
    "max": 0,
    "mean": 0,
    "mean.rate": 0,
    "median": 0,
    "min": 0,
    "stddev": 0
  },
  "logging.debug": {
    "15m.rate": 0,
    "1m.rate": 0,
    "5m.rate": 0,
    "count": 0,
    "mean.rate": 0
  },
  "logging.error": {
    "15m.rate": 0,
    "1m.rate": 0,
    "5m.rate": 0,
    "count": 0,
    "mean.rate": 0
  },
  "logging.info": {
    "15m.rate": 232.85916859415772,
    "1m.rate": 242.7785273104503,
    "5m.rate": 237.74161591995696,
    "count": 133049195,
    "mean.rate": 227.30356525388274
  },
  "logging.panic": {
    "15m.rate": 0,
    "1m.rate": 0,
    "5m.rate": 0,
    "count": 0,
    "mean.rate": 0
  },
  "logging.warning": {
    "15m.rate": 0.00002864686194423444,
    "1m.rate": 4.5629799451093754e-41,
    "5m.rate": 3.714334583072108e-10,
    "count": 6995,
    "mean.rate": 0.011950421578867764
  },
  "requests.failed": {
    "15m.rate": 46.27434280564861,
    "1m.rate": 48.559342299629265,
    "5m.rate": 47.132925275045295,
    "count": 26002133,
    "mean.rate": 44.42250383043155
  },
  "requests.succeeded": {
    "15m.rate": 69.36681910982539,
    "1m.rate": 72.38504433912904,
    "5m.rate": 71.00293298710338,
    "count": 39985482,
    "mean.rate": 68.31190342961553
  },
  "requests.total": {
    "15m.rate": 115.64116191547402,
    "1m.rate": 120.94438663875829,
    "5m.rate": 118.13585826214866,
    "75%": 4628801.75,
    "95%": 55000011530.4,
    "99%": 55004856745.49,
    "99.9%": 55023166974.193,
    "count": 65987615,
    "max": 55023405838,
    "mean": 3513634813.836576,
    "mean.rate": 112.73440653824905,
    "median": 1937564.5,
    "min": 20411,
    "stddev": 13264750988.737148
  },
  "stellar_core.latest_ledger": {
    "value": 22826156
  },
  "stellar_core.open_connections": {
    "value": 94
  },
  "txsub.buffered": {
    "value": 1
  },
  "txsub.failed": {
    "15m.rate": 0.02479237361888591,
    "1m.rate": 0.03262394685483348,
    "5m.rate": 0.026600772194616953,
    "count": 13977,
    "mean.rate": 0.02387863835950965
  },
  "txsub.open": {
    "value": 0
  },
  "txsub.succeeded": {
    "15m.rate": 0.3684477520175787,
    "1m.rate": 0.3620036969560598,
    "5m.rate": 0.3669857018510689,
    "count": 253242,
    "mean.rate": 0.43264464015537746
  },
  "txsub.total": {
    "15m.rate": 0.3932401256364647,
    "1m.rate": 0.3946276438108932,
    "5m.rate": 0.3935864740456858,
    "75%": 30483683.25,
    "95%": 88524119.3999999,
    "99%": 320773244.6300014,
    "99.9%": 1582447912.6680026,
    "count": 267219,
    "max": 1602906917,
    "mean": 34469463.39785992,
    "mean.rate": 0.45652327851684915,
    "median": 18950996.5,
    "min": 3156355,
    "stddev": 79193338.90936844
  }
}`

var feesResponse = `{
  "last_ledger": "22606298",
  "last_ledger_base_fee": "100",
  "ledger_capacity_usage": "0.97",
  "max_fee": {
    "min": "130",
    "max": "8000",
    "mode": "250",
    "p10": "150",
    "p20": "200",
    "p30": "300",
    "p40": "400",
    "p50": "500",
    "p60": "1000",
    "p70": "2000",
    "p80": "3000",
    "p90": "4000",
    "p95": "5000",
    "p99": "8000"
  },
  "fee_charged": {
    "min": "100",
    "max": "100",
    "mode": "100",
    "p10": "100",
    "p20": "100",
    "p30": "100",
    "p40": "100",
    "p50": "100",
    "p60": "100",
    "p70": "100",
    "p80": "100",
    "p90": "100",
    "p95": "100",
    "p99": "100"
  }
}`

var offersResponse = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/accounts/GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F/offers?cursor=&limit=10&order=asc"
    },
    "next": {
      "href": "https://horizon-testnet.stellar.org/accounts/GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F/offers?cursor=432323&limit=10&order=asc"
    },
    "prev": {
      "href": "https://horizon-testnet.stellar.org/accounts/GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F/offers?cursor=432323&limit=10&order=desc"
    }
  },
  "_embedded": {
    "records": [
      {
        "_links": {
          "self": {
            "href": "https://horizon-testnet.stellar.org/offers/432323"
          },
          "offer_maker": {
            "href": "https://horizon-testnet.stellar.org/accounts/GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F"
          }
        },
        "id": "432323",
        "paging_token": "432323",
        "seller": "GDOJCPYIB66RY4XNDLRRHQQXB27YLNNAGAYV5HMHEYNYY4KUNV5FDV2F",
        "selling": {
          "asset_type": "credit_alphanum4",
          "asset_code": "ABC",
          "asset_issuer": "GDP6QEA4A5CRNGUIGYHHFETDPNEESIZFW53RVISXGSALI7KXNUC4YBWD"
        },
        "buying": {
          "asset_type": "native"
        },
        "amount": "1999979.8700000",
        "price_r": {
          "n": 100,
          "d": 1
        },
        "price": "100.0000000",
        "last_modified_ledger": 103307,
        "last_modified_time": "2019-03-05T13:23:50Z"
      }
    ]
  }
}`

var offerResponse = `
{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/offers/5635"
    },
    "offer_maker": {
      "href": "https://horizon-testnet.stellar.org/accounts/GD6UOZ3FGFI5L2X6F52YPJ6ICSW375BNBZIQC4PCLSEOO6SMX7CUS5MB"
    }
  },
  "id": "5635",
  "paging_token": "5635",
  "seller": "GD6UOZ3FGFI5L2X6F52YPJ6ICSW375BNBZIQC4PCLSEOO6SMX7CUS5MB",
  "selling": {
    "asset_type": "native"
  },
  "buying": {
    "asset_type": "credit_alphanum12",
    "asset_code": "AstroDollar",
    "asset_issuer": "GDA2EHKPDEWZTAL6B66FO77HMOZL3RHZTIJO7KJJK5RQYSDUXEYMPJYY"
  },
  "amount": "100.0000000",
  "price_r": {
    "n": 10,
    "d": 1
  },
  "price": "10.0000000",
  "last_modified_ledger": 356183,
  "last_modified_time": "2020-02-20T20:44:55Z"
}
`

var multipleOpsResponse = `{
  "_links": {
    "self": {
      "href": "https://horizon.stellar.org/transactions/b63307ef92bb253df13361a72095156d19fc0713798bc2e6c3bd9ee63cc3ca53/operations?cursor=&limit=10&order=asc"
    },
    "next": {
      "href": "https://horizon.stellar.org/transactions/b63307ef92bb253df13361a72095156d19fc0713798bc2e6c3bd9ee63cc3ca53/operations?cursor=98447788659970049&limit=10&order=asc"
    },
    "prev": {
      "href": "https://horizon.stellar.org/transactions/b63307ef92bb253df13361a72095156d19fc0713798bc2e6c3bd9ee63cc3ca53/operations?cursor=98447788659970049&limit=10&order=desc"
    }
  },
  "_embedded": {
    "records": [
      {
        "_links": {
          "self": {
            "href": "https://horizon.stellar.org/operations/98447788659970049"
          },
          "transaction": {
            "href": "https://horizon.stellar.org/transactions/b63307ef92bb253df13361a72095156d19fc0713798bc2e6c3bd9ee63cc3ca53"
          },
          "effects": {
            "href": "https://horizon.stellar.org/operations/98447788659970049/effects"
          },
          "succeeds": {
            "href": "https://horizon.stellar.org/effects?order=desc&cursor=98447788659970049"
          },
          "precedes": {
            "href": "https://horizon.stellar.org/effects?order=asc&cursor=98447788659970049"
          }
        },
        "id": "98447788659970049",
        "paging_token": "98447788659970049",
        "transaction_successful": true,
        "source_account": "GDZST3XVCDTUJ76ZAV2HA72KYQODXXZ5PTMAPZGDHZ6CS7RO7MGG3DBM",
        "type": "payment",
        "type_i": 1,
        "created_at": "2019-03-14T09:44:26Z",
        "transaction_hash": "b63307ef92bb253df13361a72095156d19fc0713798bc2e6c3bd9ee63cc3ca53",
        "asset_type": "credit_alphanum4",
        "asset_code": "DRA",
        "asset_issuer": "GCJKSAQECBGSLPQWAU7ME4LVQVZ6IDCNUA5NVTPPCUWZWBN5UBFMXZ53",
        "from": "GDZST3XVCDTUJ76ZAV2HA72KYQODXXZ5PTMAPZGDHZ6CS7RO7MGG3DBM",
        "to": "GDTCW47BX2ELQ76KAZIA5Z6V4IEHUUD44ABJ66JTRZRMINEJY3OUCNEO",
        "amount": "1.1200000"
      },
      {
        "_links": {
          "self": {
            "href": "https://horizon.stellar.org/operations/98448467264811009"
          },
          "transaction": {
            "href": "https://horizon.stellar.org/transactions/af68055329e570bf461f384e2cd40db023be32f1c38a756ba2db08b6baf66148"
          },
          "effects": {
            "href": "https://horizon.stellar.org/operations/98448467264811009/effects"
          },
          "succeeds": {
            "href": "https://horizon.stellar.org/effects?order=desc&cursor=98448467264811009"
          },
          "precedes": {
            "href": "https://horizon.stellar.org/effects?order=asc&cursor=98448467264811009"
          }
        },
        "id": "98448467264811009",
        "paging_token": "98448467264811009",
        "transaction_successful": true,
        "source_account": "GDD7ABRF7BCK76W33RXDQG5Q3WXVSQYVLGEMXSOWRGZ6Z3G3M2EM2TCP",
        "type": "manage_offer",
        "type_i": 3,
        "created_at": "2019-03-14T09:58:33Z",
        "transaction_hash": "af68055329e570bf461f384e2cd40db023be32f1c38a756ba2db08b6baf66148",
        "amount": "7775.0657728",
        "price": "3.0058511",
        "price_r": {
          "n": 30058511,
          "d": 10000000
        },
        "buying_asset_type": "native",
        "selling_asset_type": "credit_alphanum4",
        "selling_asset_code": "XRP",
        "selling_asset_issuer": "GBVOL67TMUQBGL4TZYNMY3ZQ5WGQYFPFD5VJRWXR72VA33VFNL225PL5",
        "offer_id": "73938565"
      },  
      {
        "_links": {
          "self": {
            "href": "http://horizon-mon.stellar-ops.com/operations/98455906148208641"
          },
          "transaction": {
            "href": "http://horizon-mon.stellar-ops.com/transactions/ade3c60f1b581e8744596673d95bffbdb8f68f199e0e2f7d63b7c3af9fd8d868"
          },
          "effects": {
            "href": "http://horizon-mon.stellar-ops.com/operations/98455906148208641/effects"
          },
          "succeeds": {
            "href": "http://horizon-mon.stellar-ops.com/effects?order=desc\u0026cursor=98455906148208641"
          },
          "precedes": {
            "href": "http://horizon-mon.stellar-ops.com/effects?order=asc\u0026cursor=98455906148208641"
          }
        },
        "id": "98455906148208641",
        "paging_token": "98455906148208641",
        "transaction_successful": true,
        "source_account": "GD7C4MQJDM3AHXKO2Z2OF7BK3FYL6QMNBGVEO4H2DHM65B7JMHD2IU2E",
        "type": "create_account",
        "type_i": 0,
        "created_at": "2019-03-14T12:30:40Z",
        "transaction_hash": "ade3c60f1b581e8744596673d95bffbdb8f68f199e0e2f7d63b7c3af9fd8d868",
        "starting_balance": "2.0000000",
        "funder": "GD7C4MQJDM3AHXKO2Z2OF7BK3FYL6QMNBGVEO4H2DHM65B7JMHD2IU2E",
        "account": "GD6LCN37TNJZW3JF2R7N5EYGQGVWRPMSGQHR6RZD4X4NATEQLP7RFAMA"
      }          
    ]
  }
}`

var opsResponse = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/operations/1103965508866049"
    },
    "transaction": {
      "href": "https://horizon-testnet.stellar.org/transactions/93c2755ec61c8b01ac11daa4d8d7a012f56be172bdfcaf77a6efd683319ca96d"
    },
    "effects": {
      "href": "https://horizon-testnet.stellar.org/operations/1103965508866049/effects"
    },
    "succeeds": {
      "href": "https://horizon-testnet.stellar.org/effects?order=desc\u0026cursor=1103965508866049"
    },
    "precedes": {
      "href": "https://horizon-testnet.stellar.org/effects?order=asc\u0026cursor=1103965508866049"
    }
  },
  "id": "1103965508866049",
  "paging_token": "1103965508866049",
  "transaction_successful": true,
  "source_account": "GBMVGXJXJ7ZBHIWMXHKR6IVPDTYKHJPXC2DHZDPJBEZWZYAC7NKII7IB",
  "type": "change_trust",
  "type_i": 6,
  "created_at": "2019-03-14T15:58:57Z",
  "transaction_hash": "93c2755ec61c8b01ac11daa4d8d7a012f56be172bdfcaf77a6efd683319ca96d",
  "asset_type": "credit_alphanum4",
  "asset_code": "UAHd",
  "asset_issuer": "GDDETPGV4OJVNBTB6GQICCPGH5DZRYYB7XQCSAZO2ZQH6HO7SWXHKKJN",
  "limit": "922337203685.4775807",
  "trustee": "GDDETPGV4OJVNBTB6GQICCPGH5DZRYYB7XQCSAZO2ZQH6HO7SWXHKKJN",
  "trustor": "GBMVGXJXJ7ZBHIWMXHKR6IVPDTYKHJPXC2DHZDPJBEZWZYAC7NKII7IB"
}`

var txSuccess = `{
	"_links": {
		"self": {
		  "href": "https://horizon-testnet.stellar.org/transactions/5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c"
		},
		"account": {
		  "href": "https://horizon-testnet.stellar.org/accounts/GC3IMK2BSHNZZ4WAC3AXQYA7HQTZKUUDJ7UYSA2HTNCIX5S5A5NVD3FD"
		},
		"ledger": {
		  "href": "https://horizon-testnet.stellar.org/ledgers/438134"
		},
		"operations": {
		  "href": "https://horizon-testnet.stellar.org/transactions/5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c/operations{?cursor,limit,order}",
		  "templated": true
		},
		"effects": {
		  "href": "https://horizon-testnet.stellar.org/transactions/5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c/effects{?cursor,limit,order}",
		  "templated": true
		},
		"precedes": {
		  "href": "https://horizon-testnet.stellar.org/transactions?order=asc&cursor=1881771201282048"
		},
		"succeeds": {
		  "href": "https://horizon-testnet.stellar.org/transactions?order=desc&cursor=1881771201282048"
		},
		"transaction": {
          "href": "https://horizon-testnet.stellar.org/transactions/bcc7a97264dca0a51a63f7ea971b5e7458e334489673078bb2a34eb0cce910ca"
		}
	},
	"id": "bcc7a97264dca0a51a63f7ea971b5e7458e334489673078bb2a34eb0cce910ca",
    "hash": "bcc7a97264dca0a51a63f7ea971b5e7458e334489673078bb2a34eb0cce910ca",
    "ledger": 354811,
	"successful": true,
	"created_at": "2019-03-25T10:27:53Z",
	"source_account": "GC3IMK2BSHNZZ4WAC3AXQYA7HQTZKUUDJ7UYSA2HTNCIX5S5A5NVD3FD",
	"source_account_sequence": "1881766906298369",
	"fee_charged": 100,
	"max_fee": 100,
	"operation_count": 1,
	"signatures": [
	"kOZumR7L/Pxnf2kSdhDC7qyTMRcp0+ymw+dU+4A/dRqqf387ER4pUhqFUsOc7ZrSW9iz+6N20G4mcp0IiT5fAg=="
	],
    "envelope_xdr": "AAAAABB90WssODNIgi6BHveqzxTRmIpvAFRyVNM+Hm2GVuCcAAAAZAAABD0AAuV/AAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAyTBGxOgfSApppsTnb/YRr6gOR8WT0LZNrhLh4y3FCgoAAAAXSHboAAAAAAAAAAABhlbgnAAAAEAivKe977CQCxMOKTuj+cWTFqc2OOJU8qGr9afrgu2zDmQaX5Q0cNshc3PiBwe0qw/+D/qJk5QqM5dYeSUGeDQP",
    "result_xdr": "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA=",
    "result_meta_xdr": "AAAAAQAAAAIAAAADAAVp+wAAAAAAAAAAEH3Rayw4M0iCLoEe96rPFNGYim8AVHJU0z4ebYZW4JwACBP/TuycHAAABD0AAuV+AAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAVp+wAAAAAAAAAAEH3Rayw4M0iCLoEe96rPFNGYim8AVHJU0z4ebYZW4JwACBP/TuycHAAABD0AAuV/AAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAwAAAAMABWn7AAAAAAAAAAAQfdFrLDgzSIIugR73qs8U0ZiKbwBUclTTPh5thlbgnAAIE/9O7JwcAAAEPQAC5X8AAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEABWn7AAAAAAAAAAAQfdFrLDgzSIIugR73qs8U0ZiKbwBUclTTPh5thlbgnAAIE+gGdbQcAAAEPQAC5X8AAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAABWn7AAAAAAAAAADJMEbE6B9ICmmmxOdv9hGvqA5HxZPQtk2uEuHjLcUKCgAAABdIdugAAAVp+wAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA=="
}`

var transactionFailure = `{
  "type": "https://stellar.org/horizon-errors/transaction_failed",
  "title": "Transaction Failed",
  "status": 400,
  "detail": "The transaction failed when submitted to the stellar network. The extras.result_codes field on this response contains further details.  Descriptions of each code can be found at: https://developers.stellar.org/docs/start/list-of-operations/",
  "instance": "horizon-testnet-001.prd.stellar001.internal.stellar-ops.com/4elYz2fHhC-528285",
  "extras": {
    "envelope_xdr": "AAAAAKpmDL6Z4hvZmkTBkYpHftan4ogzTaO4XTB7joLgQnYYAAAAZAAAAAAABeoyAAAAAAAAAAEAAAAAAAAAAQAAAAAAAAABAAAAAD3sEVVGZGi/NoC3ta/8f/YZKMzyi9ZJpOi0H47x7IqYAAAAAAAAAAAF9eEAAAAAAAAAAAA=",
    "result_codes": {
      "transaction": "tx_no_source_account"
    },
    "result_xdr": "AAAAAAAAAAD////4AAAAAA=="
  }
}`

var txPageResponse = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/transactions?cursor=&limit=10&order=desc"
    },
    "next": {
      "href": "https://horizon-testnet.stellar.org/transactions?cursor=1881762611335168&limit=10&order=desc"
    },
    "prev": {
      "href": "https://horizon-testnet.stellar.org/transactions?cursor=1881771201286144&limit=10&order=asc"
    }
  },
  "_embedded": {
    "records": [
      {
        "_links": {
          "self": {
            "href": "https://horizon-testnet.stellar.org/transactions/3274f131af56ecb6d8668acf6eb0b31b5f8faeca785cbce0a911a5a81308a599"
          },
          "account": {
            "href": "https://horizon-testnet.stellar.org/accounts/GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR"
          },
          "ledger": {
            "href": "https://horizon-testnet.stellar.org/ledgers/438134"
          },
          "operations": {
            "href": "https://horizon-testnet.stellar.org/transactions/3274f131af56ecb6d8668acf6eb0b31b5f8faeca785cbce0a911a5a81308a599/operations{?cursor,limit,order}",
            "templated": true
          },
          "effects": {
            "href": "https://horizon-testnet.stellar.org/transactions/3274f131af56ecb6d8668acf6eb0b31b5f8faeca785cbce0a911a5a81308a599/effects{?cursor,limit,order}",
            "templated": true
          },
          "precedes": {
            "href": "https://horizon-testnet.stellar.org/transactions?order=asc&cursor=1881771201286144"
          },
          "succeeds": {
            "href": "https://horizon-testnet.stellar.org/transactions?order=desc&cursor=1881771201286144"
          }
        },
        "id": "3274f131af56ecb6d8668acf6eb0b31b5f8faeca785cbce0a911a5a81308a599",
        "paging_token": "1881771201286144",
        "successful": true,
        "hash": "3274f131af56ecb6d8668acf6eb0b31b5f8faeca785cbce0a911a5a81308a599",
        "ledger": 438134,
        "created_at": "2019-03-25T10:27:53Z",
        "source_account": "GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR",
        "source_account_sequence": "4660039787356",
        "fee_charged": 100,
        "max_fee": 100,
        "operation_count": 1,
        "envelope_xdr": "AAAAABB90WssODNIgi6BHveqzxTRmIpvAFRyVNM+Hm2GVuCcAAAAZAAABD0ABCNcAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAzvbakxhsAWYE0gRDf2pfXaYUCnH8vEwyQiNOJYLmNRIAAAAXSHboAAAAAAAAAAABhlbgnAAAAEBw2qecm0C4q7xi8+43NjuExfspCtA1ki2Jq2lWuNSLArJ0qcOhz/HnszFppaCBHkFf/37557MbF4NbFZXlVv4P",
        "result_xdr": "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAA=",
        "result_meta_xdr": "AAAAAQAAAAIAAAADAAavdgAAAAAAAAAAEH3Rayw4M0iCLoEe96rPFNGYim8AVHJU0z4ebYZW4JwAHtF2q1bgcAAABD0ABCNbAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAavdgAAAAAAAAAAEH3Rayw4M0iCLoEe96rPFNGYim8AVHJU0z4ebYZW4JwAHtF2q1bgcAAABD0ABCNcAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAAAwAAAAMABq92AAAAAAAAAAAQfdFrLDgzSIIugR73qs8U0ZiKbwBUclTTPh5thlbgnAAe0XarVuBwAAAEPQAEI1wAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEABq92AAAAAAAAAAAQfdFrLDgzSIIugR73qs8U0ZiKbwBUclTTPh5thlbgnAAe0V9i3/hwAAAEPQAEI1wAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAABq92AAAAAAAAAADO9tqTGGwBZgTSBEN/al9dphQKcfy8TDJCI04lguY1EgAAABdIdugAAAavdgAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==",
        "fee_meta_xdr": "AAAAAgAAAAMABq92AAAAAAAAAAAQfdFrLDgzSIIugR73qs8U0ZiKbwBUclTTPh5thlbgnAAe0Y3zzcjUAAAEPQAEI1oAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEABq92AAAAAAAAAAAQfdFrLDgzSIIugR73qs8U0ZiKbwBUclTTPh5thlbgnAAe0Y3zzchwAAAEPQAEI1oAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==",
        "memo_type": "none",
        "signatures": [
          "cNqnnJtAuKu8YvPuNzY7hMX7KQrQNZItiatpVrjUiwKydKnDoc/x57MxaaWggR5BX/9++eezGxeDWxWV5Vb+Dw=="
        ]
      },
      {
        "memo": "2A1V6J5703G47XHY",
        "_links": {
          "self": {
            "href": "https://horizon-testnet.stellar.org/transactions/5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c"
          },
          "account": {
            "href": "https://horizon-testnet.stellar.org/accounts/GC3IMK2BSHNZZ4WAC3AXQYA7HQTZKUUDJ7UYSA2HTNCIX5S5A5NVD3FD"
          },
          "ledger": {
            "href": "https://horizon-testnet.stellar.org/ledgers/438134"
          },
          "operations": {
            "href": "https://horizon-testnet.stellar.org/transactions/5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c/operations{?cursor,limit,order}",
            "templated": true
          },
          "effects": {
            "href": "https://horizon-testnet.stellar.org/transactions/5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c/effects{?cursor,limit,order}",
            "templated": true
          },
          "precedes": {
            "href": "https://horizon-testnet.stellar.org/transactions?order=asc&cursor=1881771201282048"
          },
          "succeeds": {
            "href": "https://horizon-testnet.stellar.org/transactions?order=desc&cursor=1881771201282048"
          }
        },
        "id": "5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c",
        "paging_token": "1881771201282048",
        "successful": true,
        "hash": "5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c",
        "ledger": 438134,
        "created_at": "2019-03-25T10:27:53Z",
        "source_account": "GC3IMK2BSHNZZ4WAC3AXQYA7HQTZKUUDJ7UYSA2HTNCIX5S5A5NVD3FD",
        "source_account_sequence": "1881766906298369",
        "fee_charged": 100,
        "max_fee": 100,
        "operation_count": 1,
        "envelope_xdr": "AAAAALaGK0GR25zywBbBeGAfPCeVUoNP6YkDR5tEi/ZdB1tRAAAAZAAGr3UAAAABAAAAAAAAAAEAAAAQMkExVjZKNTcwM0c0N1hIWQAAAAEAAAABAAAAALaGK0GR25zywBbBeGAfPCeVUoNP6YkDR5tEi/ZdB1tRAAAAAQAAAADMSEvcRKXsaUNna++Hy7gWm/CfqTjEA7xoGypfrFGUHAAAAAAAAAACBo93AAAAAAAAAAABXQdbUQAAAECQ5m6ZHsv8/Gd/aRJ2EMLurJMxFynT7KbD51T7gD91Gqp/fzsRHilSGoVSw5ztmtJb2LP7o3bQbiZynQiJPl8C",
        "result_xdr": "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA=",
        "result_meta_xdr": "AAAAAQAAAAIAAAADAAavdgAAAAAAAAAAtoYrQZHbnPLAFsF4YB88J5VSg0/piQNHm0SL9l0HW1EAAAAXSHbnnAAGr3UAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAavdgAAAAAAAAAAtoYrQZHbnPLAFsF4YB88J5VSg0/piQNHm0SL9l0HW1EAAAAXSHbnnAAGr3UAAAABAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAABAAAABAAAAAMABq9zAAAAAAAAAADMSEvcRKXsaUNna++Hy7gWm/CfqTjEA7xoGypfrFGUHAAAAAUQ/z+cAABeBgAASuQAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEABq92AAAAAAAAAADMSEvcRKXsaUNna++Hy7gWm/CfqTjEA7xoGypfrFGUHAAAAAcXjracAABeBgAASuQAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAMABq92AAAAAAAAAAC2hitBkduc8sAWwXhgHzwnlVKDT+mJA0ebRIv2XQdbUQAAABdIduecAAavdQAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEABq92AAAAAAAAAAC2hitBkduc8sAWwXhgHzwnlVKDT+mJA0ebRIv2XQdbUQAAABVB53CcAAavdQAAAAEAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==",
        "fee_meta_xdr": "AAAAAgAAAAMABq91AAAAAAAAAAC2hitBkduc8sAWwXhgHzwnlVKDT+mJA0ebRIv2XQdbUQAAABdIdugAAAavdQAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEABq92AAAAAAAAAAC2hitBkduc8sAWwXhgHzwnlVKDT+mJA0ebRIv2XQdbUQAAABdIduecAAavdQAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==",
        "memo_type": "text",
        "signatures": [
          "kOZumR7L/Pxnf2kSdhDC7qyTMRcp0+ymw+dU+4A/dRqqf387ER4pUhqFUsOc7ZrSW9iz+6N20G4mcp0IiT5fAg=="
        ]
			}
		]
	}
}`

var txDetailResponse = `{
  "memo": "2A1V6J5703G47XHY",
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/transactions/5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c"
    },
    "account": {
      "href": "https://horizon-testnet.stellar.org/accounts/GC3IMK2BSHNZZ4WAC3AXQYA7HQTZKUUDJ7UYSA2HTNCIX5S5A5NVD3FD"
    },
    "ledger": {
      "href": "https://horizon-testnet.stellar.org/ledgers/438134"
    },
    "operations": {
      "href": "https://horizon-testnet.stellar.org/transactions/5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c/operations{?cursor,limit,order}",
      "templated": true
    },
    "effects": {
      "href": "https://horizon-testnet.stellar.org/transactions/5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c/effects{?cursor,limit,order}",
      "templated": true
    },
    "precedes": {
      "href": "https://horizon-testnet.stellar.org/transactions?order=asc&cursor=1881771201282048"
    },
    "succeeds": {
      "href": "https://horizon-testnet.stellar.org/transactions?order=desc&cursor=1881771201282048"
    }
  },
  "id": "5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c",
  "paging_token": "1881771201282048",
  "successful": true,
  "hash": "5131aed266a639a6eb4802a92fba310454e711ded830ed899745b9e777d7110c",
  "ledger": 438134,
  "created_at": "2019-03-25T10:27:53Z",
  "source_account": "GC3IMK2BSHNZZ4WAC3AXQYA7HQTZKUUDJ7UYSA2HTNCIX5S5A5NVD3FD",
  "source_account_sequence": "1881766906298369",
  "fee_charged": 100,
  "max_fee": 100,
  "operation_count": 1,
  "envelope_xdr": "AAAAALaGK0GR25zywBbBeGAfPCeVUoNP6YkDR5tEi/ZdB1tRAAAAZAAGr3UAAAABAAAAAAAAAAEAAAAQMkExVjZKNTcwM0c0N1hIWQAAAAEAAAABAAAAALaGK0GR25zywBbBeGAfPCeVUoNP6YkDR5tEi/ZdB1tRAAAAAQAAAADMSEvcRKXsaUNna++Hy7gWm/CfqTjEA7xoGypfrFGUHAAAAAAAAAACBo93AAAAAAAAAAABXQdbUQAAAECQ5m6ZHsv8/Gd/aRJ2EMLurJMxFynT7KbD51T7gD91Gqp/fzsRHilSGoVSw5ztmtJb2LP7o3bQbiZynQiJPl8C",
  "result_xdr": "AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA=",
  "result_meta_xdr": "<result_meta_xdr>",
  "fee_meta_xdr": "AAAAAgAAAAMABq91AAAAAAAAAAC2hitBkduc8sAWwXhgHzwnlVKDT+mJA0ebRIv2XQdbUQAAABdIdugAAAavdQAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAEABq92AAAAAAAAAAC2hitBkduc8sAWwXhgHzwnlVKDT+mJA0ebRIv2XQdbUQAAABdIduecAAavdQAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAA==",
  "memo_type": "text",
  "signatures": [
    "kOZumR7L/Pxnf2kSdhDC7qyTMRcp0+ymw+dU+4A/dRqqf387ER4pUhqFUsOc7ZrSW9iz+6N20G4mcp0IiT5fAg=="
  ]
}`

var orderbookResponse = `{
  "bids": [
    {
      "price_r": {
        "n": 48904,
        "d": 1949839975
      },
      "price": "0.0000251",
      "amount": "0.0841405"
    },
    {
      "price_r": {
        "n": 273,
        "d": 10917280
      },
      "price": "0.0000250",
      "amount": "0.0005749"
    }
  ],
  "asks": [
    {
      "price_r": {
        "n": 2,
        "d": 78125
      },
      "price": "0.0000256",
      "amount": "3354.7460938"
    },
    {
      "price_r": {
        "n": 10178,
        "d": 394234000
      },
      "price": "0.0000258",
      "amount": "1.7314070"
    }
  ],
  "base": {
    "asset_type": "native"
  },
  "counter": {
    "asset_type": "credit_alphanum4",
    "asset_code": "BTC",
    "asset_issuer": "GBVOL67TMUQBGL4TZYNMY3ZQ5WGQYFPFD5VJRWXR72VA33VFNL225PL5"
  }
}`

var orderBookNotFound = `{
  "type": "https://stellar.org/horizon-errors/invalid_order_book",
  "title": "Invalid Order Book Parameters",
  "status": 400,
  "detail": "The parameters that specify what order book to view are invalid in some way. Please ensure that your type parameters (selling_asset_type and buying_asset_type) are one the following valid values: native, credit_alphanum4, credit_alphanum12.  Also ensure that you have specified selling_asset_code and selling_asset_issuer if selling_asset_type is not 'native', as well as buying_asset_code and buying_asset_issuer if buying_asset_type is not 'native'"
}`

var paymentsResponse = `{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/payments?cursor=&limit=2&order=desc"
    },
    "next": {
      "href": "https://horizon-testnet.stellar.org/payments?cursor=2024660468248577&limit=2&order=desc"
    },
    "prev": {
      "href": "https://horizon-testnet.stellar.org/payments?cursor=2024660468256769&limit=2&order=asc"
    }
  },
  "_embedded": {
    "records": [
      {
        "_links": {
          "self": {
            "href": "https://horizon-testnet.stellar.org/operations/2024660468256769"
          },
          "transaction": {
            "href": "https://horizon-testnet.stellar.org/transactions/a0207513c372146bae8cdb299975047216cb1ffb393074b2015b39496e8767c2"
          },
          "effects": {
            "href": "https://horizon-testnet.stellar.org/operations/2024660468256769/effects"
          },
          "succeeds": {
            "href": "https://horizon-testnet.stellar.org/effects?order=desc&cursor=2024660468256769"
          },
          "precedes": {
            "href": "https://horizon-testnet.stellar.org/effects?order=asc&cursor=2024660468256769"
          }
        },
        "id": "2024660468256769",
        "paging_token": "2024660468256769",
        "transaction_successful": true,
        "source_account": "GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR",
        "type": "create_account",
        "type_i": 0,
        "created_at": "2019-03-27T09:55:41Z",
        "transaction_hash": "a0207513c372146bae8cdb299975047216cb1ffb393074b2015b39496e8767c2",
        "starting_balance": "10000.0000000",
        "funder": "GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR",
        "account": "GB4OHVQE7OZH4HLCHFNR7OHDMZVNKOJT3RCRAXRNGGCNUHFRVGUGKW36"
      },
      {
        "_links": {
          "self": {
            "href": "https://horizon-testnet.stellar.org/operations/2024660468248577"
          },
          "transaction": {
            "href": "https://horizon-testnet.stellar.org/transactions/87d7a29539e7902b14a6c720094856f74a77128ab332d8629432c5a176a9fe7b"
          },
          "effects": {
            "href": "https://horizon-testnet.stellar.org/operations/2024660468248577/effects"
          },
          "succeeds": {
            "href": "https://horizon-testnet.stellar.org/effects?order=desc&cursor=2024660468248577"
          },
          "precedes": {
            "href": "https://horizon-testnet.stellar.org/effects?order=asc&cursor=2024660468248577"
          }
        },
        "id": "2024660468248577",
        "paging_token": "2024660468248577",
        "transaction_successful": true,
        "source_account": "GAL6CXEVI3Y4O4J3FIX3KCRF7HSUG5RW2IRQRUUFC6XHZOLNV3NU35TL",
        "type": "payment",
        "type_i": 1,
        "created_at": "2019-03-27T09:55:41Z",
        "transaction_hash": "87d7a29539e7902b14a6c720094856f74a77128ab332d8629432c5a176a9fe7b",
        "asset_type": "native",
        "from": "GAL6CXEVI3Y4O4J3FIX3KCRF7HSUG5RW2IRQRUUFC6XHZOLNV3NU35TL",
        "to": "GDGEQS64ISS6Y2KDM5V67B6LXALJX4E7VE4MIA54NANSUX5MKGKBZM5G",
        "amount": "177.0000000"
      }
    ]
  }
}`
