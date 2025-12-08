package stellarcore

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	proto "github.com/stellar/go-stellar-sdk/protocols/stellarcore"
	"github.com/stellar/go-stellar-sdk/support/http/httptest"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestSubmitTransaction(t *testing.T) {
	hmock := httptest.NewClient()
	c := &Client{HTTP: hmock, URL: "http://localhost:11626"}

	// happy path - new transaction
	hmock.On("GET", "http://localhost:11626/tx?blob=foo").
		ReturnJSON(http.StatusOK, proto.TXResponse{
			Status: proto.TXStatusPending,
		})

	resp, err := c.SubmitTransaction(context.Background(), "foo")

	if assert.NoError(t, err) {
		assert.Equal(t, proto.TXStatusPending, resp.Status)
	}
}

func TestSubmitTransactionError(t *testing.T) {
	hmock := httptest.NewClient()
	c := &Client{HTTP: hmock, URL: "http://localhost:11626"}

	// happy path - new transaction
	hmock.On("GET", "http://localhost:11626/tx?blob=foo").
		ReturnString(
			200,
			`{"diagnostic_events":"AAAAAQAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAA8AAAAFZXJyb3IAAAAAAAACAAAAAwAAAAUAAAAQAAAAAQAAAAMAAAAOAAAAU3RyYW5zYWN0aW9uIGBzb3JvYmFuRGF0YS5yZXNvdXJjZUZlZWAgaXMgbG93ZXIgdGhhbiB0aGUgYWN0dWFsIFNvcm9iYW4gcmVzb3VyY2UgZmVlAAAAAAUAAAAAAAEJcwAAAAUAAAAAAAG6fA==","error":"AAAAAAABCdf////vAAAAAA==","status":"ERROR"}`,
		)

	resp, err := c.SubmitTransaction(context.Background(), "foo")

	if assert.NoError(t, err) {
		assert.Equal(t, "ERROR", resp.Status)
		assert.Equal(t, resp.Error, "AAAAAAABCdf////vAAAAAA==")
		assert.Equal(t, "AAAAAQAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAgAAAA8AAAAFZXJyb3IAAAAAAAACAAAAAwAAAAUAAAAQAAAAAQAAAAMAAAAOAAAAU3RyYW5zYWN0aW9uIGBzb3JvYmFuRGF0YS5yZXNvdXJjZUZlZWAgaXMgbG93ZXIgdGhhbiB0aGUgYWN0dWFsIFNvcm9iYW4gcmVzb3VyY2UgZmVlAAAAAAUAAAAAAAEJcwAAAAUAAAAAAAG6fA==", resp.DiagnosticEvents)
	}
}

func TestManualClose(t *testing.T) {
	hmock := httptest.NewClient()
	c := &Client{HTTP: hmock, URL: "http://localhost:11626"}

	// happy path - new transaction
	hmock.On("GET", "http://localhost:11626/manualclose").
		ReturnString(http.StatusOK, "Manually triggered a ledger close with sequence number 7")

	err := c.ManualClose(context.Background())

	assert.NoError(t, err)
}

func TestManualClose_NotAvailable(t *testing.T) {
	hmock := httptest.NewClient()
	c := &Client{HTTP: hmock, URL: "http://localhost:11626"}

	// happy path - new transaction
	hmock.On("GET", "http://localhost:11626/manualclose").
		ReturnString(http.StatusOK,
			`{"exception": "Set MANUAL_CLOSE=true"}`,
		)

	err := c.ManualClose(context.Background())

	assert.EqualError(t, err, "exception in response: Set MANUAL_CLOSE=true")
}

func TestGetRawLedgerEntries(t *testing.T) {
	hmock := httptest.NewClient()
	c := &Client{HTTP: hmock, URL: "http://localhost:11626"}

	// build a fake response body
	mockResp := proto.GetLedgerEntryRawResponse{
		Ledger: 1215, // checkpoint align on expected request
		Entries: []proto.RawLedgerEntryResponse{
			{
				Entry: "pretend this is XDR lol",
			},
			{
				Entry: "pretend this is another XDR lol",
			},
		},
	}

	var key xdr.LedgerKey
	acc, err := xdr.AddressToAccountId(keypair.MustRandom().Address())
	require.NoError(t, err)
	key.SetAccount(acc)

	// happy path - fetch an entry
	ce := hmock.On("POST", "http://localhost:11626/getledgerentryraw")
	hmock.RegisterResponder(
		"POST",
		"http://localhost:11626/getledgerentryraw",
		func(r *http.Request) (*http.Response, error) {
			// Ensure the request has the correct POST body
			requestData, ierr := io.ReadAll(r.Body)
			require.NoError(t, ierr)

			keyB64, ierr := key.MarshalBinaryBase64()
			require.NoError(t, ierr)
			expected := fmt.Sprintf("key=%s&ledgerSeq=1234", url.QueryEscape(keyB64))
			require.Equal(t, expected, string(requestData))

			resp, ierr := httpmock.NewJsonResponse(http.StatusOK, &mockResp)
			require.NoError(t, ierr)
			ce.Return(httpmock.ResponderFromResponse(resp))
			return resp, nil
		})

	resp, err := c.GetLedgerEntryRaw(context.Background(), 1234, key)
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.EqualValues(t, 1215, resp.Ledger)
	require.Len(t, resp.Entries, 2)
	require.Equal(t, "pretend this is XDR lol", resp.Entries[0].Entry)
	require.Equal(t, "pretend this is another XDR lol", resp.Entries[1].Entry)
}

func TestGetLedgerEntries(t *testing.T) {
	hmock := httptest.NewClient()
	c := &Client{HTTP: hmock, URL: "http://localhost:11626"}

	var hash xdr.ContractId
	_, err := rand.Read(hash[:])
	require.NoError(t, err)

	tr, err := xdr.NewScVal(xdr.ScValTypeScvBool, true)
	require.NoError(t, err)
	fl, err := xdr.NewScVal(xdr.ScValTypeScvBool, false)
	require.NoError(t, err)

	rawEntry := xdr.ContractDataEntry{
		Ext:        xdr.ExtensionPoint{},
		Contract:   xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: &hash},
		Key:        tr,
		Durability: xdr.ContractDataDurabilityPersistent,
		Val:        fl,
	}
	entry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: 1210,
		Data: xdr.LedgerEntryData{
			Type:         xdr.LedgerEntryTypeContractData,
			ContractData: &rawEntry,
		},
	}
	entryB64, err := xdr.MarshalBase64(entry)
	require.NoError(t, err)

	var key xdr.LedgerKey
	acc, err := xdr.AddressToAccountId(keypair.MustRandom().Address())
	require.NoError(t, err)
	key.SetAccount(acc)
	keyB64, err := key.MarshalBinaryBase64()
	require.NoError(t, err)

	// build a fake response body
	mockResp := proto.GetLedgerEntryResponse{
		Ledger: 1215, // checkpoint align on expected request
		Entries: []proto.LedgerEntryResponse{{
			Entry:              entryB64,
			State:              "live",
			LiveUntilLedgerSeq: 1234,
		}, {
			Entry: entryB64,
			State: "archived",
		}, {
			State: "new",
		}},
	}

	// happy path - fetch an entry
	ce := hmock.On("POST", "http://localhost:11626/getledgerentry")
	hmock.RegisterResponder(
		"POST",
		"http://localhost:11626/getledgerentry",
		func(r *http.Request) (*http.Response, error) {
			// Ensure the request has the correct POST body
			requestData, ierr := io.ReadAll(r.Body)
			require.NoError(t, ierr)

			expected := fmt.Sprintf(
				"key=%s&key=%s&key=%s&ledgerSeq=1234",
				url.QueryEscape(keyB64),
				url.QueryEscape(keyB64),
				url.QueryEscape(keyB64))
			require.Equal(t, expected, string(requestData))

			resp, ierr := httpmock.NewJsonResponse(http.StatusOK, &mockResp)
			require.NoError(t, ierr)
			ce.Return(httpmock.ResponderFromResponse(resp))
			return resp, nil
		})

	resp, err := c.GetLedgerEntries(context.Background(), 1234, key, key, key)
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.EqualValues(t, 1215, resp.Ledger)
	require.Len(t, resp.Entries, 3)
	require.Equal(t, entryB64, resp.Entries[0].Entry)
	require.Equal(t, entryB64, resp.Entries[1].Entry)
	require.Empty(t, resp.Entries[2].Entry)
	require.EqualValues(t, 1234, resp.Entries[0].LiveUntilLedgerSeq)
	require.EqualValues(t, 0, resp.Entries[1].LiveUntilLedgerSeq)
	require.EqualValues(t, 0, resp.Entries[2].LiveUntilLedgerSeq)
	require.EqualValues(t, "live", resp.Entries[0].State)
	require.EqualValues(t, "archived", resp.Entries[1].State)
	require.EqualValues(t, "new", resp.Entries[2].State)

	// TTL keys aren't returned
	key.Type = xdr.LedgerEntryTypeTtl
	_, err = c.GetLedgerEntries(context.Background(), 1234, key)
	require.Error(t, err)
}

func TestGenSorobanConfigUpgradeTxAndKey(t *testing.T) {
	key, err := keypair.ParseFull("SB6VZS57IY25334Y6F6SPGFUNESWS7D2OSJHKDPIZ354BK3FN5GBTS6V")
	require.NoError(t, err)
	funcConfig := GenSorobanConfig{
		BaseSeqNum:        1,
		NetworkPassphrase: network.TestNetworkPassphrase,
		SigningKey:        key,
		StellarCoreImage:  "stellar/stellar-core:22",
	}
	config := xdr.ConfigUpgradeSet{
		UpdatedEntry: []xdr.ConfigSettingEntry{
			{
				ConfigSettingId: xdr.ConfigSettingIdConfigSettingContractComputeV0,
				ContractCompute: &xdr.ConfigSettingContractComputeV0{
					LedgerMaxInstructions:           1000,
					TxMaxInstructions:               100,
					FeeRatePerInstructionsIncrement: 1000,
					TxMemoryLimit:                   10,
				},
			},
		},
	}

	txs, _, err := GenSorobanConfigUpgradeTxAndKey(funcConfig, config)
	require.NoError(t, err)
	require.Len(t, txs, 4)
}
