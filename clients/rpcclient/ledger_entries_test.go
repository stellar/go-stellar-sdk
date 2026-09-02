package rpcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const (
	testAccountAddress = "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z"
	testAssetIssuer    = "GBUKBCG5VLRKAVYAIREJRUJHOKLIADZJOICRW43WVJCLES52BDOTCQZU"

	testAccountKeyXDR   = "AAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VA=="
	testTrustlineKeyXDR = "AAAAAQAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2V" +
		"AAAAAFVU0QAAAAAAGigiN2q4qBXAERImNEncpaADylyBRtzdqpEsku6CN0x"
	testLiquidityPoolTrustlineKeyXDR = "AAAAAQAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2V" +
		"AAAAAM2h/HuftmNL8PqE9QSypx6zRyJZaQZgYGcKMeF/PM5Pw=="
	testClaimableBalanceKeyXDR = "AAAABAAAAAAAAQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHw=="
	testLiquidityPoolIDHex     = "3687f1ee7ed98d2fc3ea13d412ca9c7acd1c8965a41981819c28c785fcf3393f"

	testClaimableBalanceStrkey  = "BAAAAAICAMCAKBQHBAEQUCYMBUHA6EARCIJRIFIWC4MBSGQ3DQOR4H2TOM"
	testClaimableBalanceHashHex = "000102030405060708090a0b0c0d0e0f" +
		"101112131415161718191a1b1c1d1e1f"
	testClaimableBalanceXDRHex = "00000000" + testClaimableBalanceHashHex
)

type ledgerEntryTestRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      any             `json:"id"`
}

type ledgerEntryTestResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
	ID      any    `json:"id"`
}

func TestClient_GetAccountEntry(t *testing.T) {
	expected := xdr.AccountEntry{
		AccountId: xdr.MustAddress(testAccountAddress),
		Balance:   1234567,
		SeqNum:    42,
		Flags:     3,
	}
	data := xdr.LedgerEntryData{
		Type:    xdr.LedgerEntryTypeAccount,
		Account: &expected,
	}
	server := newLedgerEntryTestServer(t, testAccountKeyXDR, ledgerEntryResponse(t, data))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	actual, err := client.GetAccountEntry(context.Background(), testAccountAddress)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestClient_LoadAccount_UsesAccountEntry(t *testing.T) {
	entry := xdr.AccountEntry{
		AccountId: xdr.MustAddress(testAccountAddress),
		SeqNum:    42,
	}
	data := xdr.LedgerEntryData{
		Type:    xdr.LedgerEntryTypeAccount,
		Account: &entry,
	}
	server := newLedgerEntryTestServer(t, testAccountKeyXDR, ledgerEntryResponse(t, data))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	account, err := client.LoadAccount(context.Background(), testAccountAddress)
	require.NoError(t, err)
	assert.Equal(t, testAccountAddress, account.GetAccountID())
	sequence, err := account.GetSequenceNumber()
	require.NoError(t, err)
	assert.Equal(t, int64(42), sequence)
}

func TestClient_LoadAccount_NotFound(t *testing.T) {
	server := newLedgerEntryTestServer(t, testAccountKeyXDR, protocol.GetLedgerEntriesResponse{
		Entries: []protocol.LedgerEntryResult{},
	})
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	_, err := client.LoadAccount(context.Background(), testAccountAddress)
	require.EqualError(t, err, "account "+testAccountAddress+" not found")
}

func TestClient_GetTrustline(t *testing.T) {
	asset := txnbuild.CreditAsset{Code: "USD", Issuer: testAssetIssuer}
	assetXDR, err := asset.ToXDR()
	require.NoError(t, err)
	expected := xdr.TrustLineEntry{
		AccountId: xdr.MustAddress(testAccountAddress),
		Asset:     assetXDR.ToTrustLineAsset(),
		Balance:   25000000,
		Limit:     100000000,
		Flags:     1,
	}
	data := xdr.LedgerEntryData{
		Type:      xdr.LedgerEntryTypeTrustline,
		TrustLine: &expected,
	}
	server := newLedgerEntryTestServer(t, testTrustlineKeyXDR, ledgerEntryResponse(t, data))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	actual, err := client.GetTrustline(context.Background(), testAccountAddress, asset)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestClient_GetTrustline_LiquidityPoolShare(t *testing.T) {
	poolID, trustlineAsset, changeTrustAsset := testLiquidityPoolAssets(t)
	xdrPoolID, err := poolID.ToXDR()
	require.NoError(t, err)
	assetXDR, err := xdr.NewTrustLineAsset(xdr.AssetTypeAssetTypePoolShare, xdrPoolID)
	require.NoError(t, err)
	expected := xdr.TrustLineEntry{
		AccountId: xdr.MustAddress(testAccountAddress),
		Asset:     assetXDR,
		Balance:   25000000,
		Limit:     100000000,
		Flags:     1,
	}
	data := xdr.LedgerEntryData{
		Type:      xdr.LedgerEntryTypeTrustline,
		TrustLine: &expected,
	}

	for _, testCase := range []struct {
		name  string
		asset txnbuild.BasicAsset
	}{
		{name: "trustline asset", asset: trustlineAsset},
		{name: "change trust asset", asset: changeTrustAsset},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := newLedgerEntryTestServer(
				t,
				testLiquidityPoolTrustlineKeyXDR,
				ledgerEntryResponse(t, data),
			)
			defer server.Close()

			client := NewClient(server.URL, nil)
			defer client.Close()

			actual, err := client.GetTrustline(context.Background(), testAccountAddress, testCase.asset)
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestClient_GetClaimableBalance_IDFormats(t *testing.T) {
	balanceID := testClaimableBalanceID(t)
	nativeAsset, err := txnbuild.NativeAsset{}.ToXDR()
	require.NoError(t, err)
	expected := xdr.ClaimableBalanceEntry{
		BalanceId: balanceID,
		Asset:     nativeAsset,
		Amount:    7654321,
	}
	data := xdr.LedgerEntryData{
		Type:             xdr.LedgerEntryTypeClaimableBalance,
		ClaimableBalance: &expected,
	}

	for _, testCase := range []struct {
		name string
		id   string
	}{
		{name: "strkey", id: testClaimableBalanceStrkey},
		{name: "72-character XDR hex", id: testClaimableBalanceXDRHex},
		{name: "64-character hash hex", id: testClaimableBalanceHashHex},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := newLedgerEntryTestServer(
				t,
				testClaimableBalanceKeyXDR,
				ledgerEntryResponse(t, data),
			)
			defer server.Close()

			client := NewClient(server.URL, nil)
			defer client.Close()

			actual, err := client.GetClaimableBalance(context.Background(), testCase.id)
			require.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestClient_LedgerEntryHelpers_Validation(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", nil)
	defer client.Close()

	t.Run("invalid account entry address", func(t *testing.T) {
		_, err := client.GetAccountEntry(context.Background(), "not-an-account")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a valid Stellar account")
	})

	t.Run("nil trustline asset", func(t *testing.T) {
		_, err := client.GetTrustline(context.Background(), testAccountAddress, nil)
		require.EqualError(t, err, "asset must not be nil")
	})

	t.Run("native trustline asset", func(t *testing.T) {
		_, err := client.GetTrustline(context.Background(), testAccountAddress, txnbuild.NativeAsset{})
		require.EqualError(t, err, "native asset does not have a trustline")
	})

	t.Run("invalid trustline account", func(t *testing.T) {
		asset := txnbuild.CreditAsset{Code: "USD", Issuer: testAssetIssuer}
		_, err := client.GetTrustline(context.Background(), "not-an-account", asset)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a valid Stellar account")
	})

	for _, testCase := range []struct {
		name  string
		asset txnbuild.BasicAsset
	}{
		{
			name:  "invalid trustline asset code",
			asset: txnbuild.CreditAsset{Code: "TOO-LONG-CODE", Issuer: testAssetIssuer},
		},
		{
			name:  "invalid trustline asset issuer",
			asset: txnbuild.CreditAsset{Code: "USD", Issuer: "not-an-issuer"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := client.GetTrustline(context.Background(), testAccountAddress, testCase.asset)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid trustline asset")
		})
	}

	for _, testCase := range []struct {
		name string
		id   string
	}{
		{name: "unsupported format", id: "not-a-balance-id"},
		{name: "non-hex hash", id: strings.Repeat("z", 64)},
		{name: "invalid XDR discriminator", id: "00000001" + strings.Repeat("00", 32)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := client.GetClaimableBalance(context.Background(), testCase.id)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid claimable balance ID")
		})
	}
}

func TestClient_LedgerEntryHelpers_NotFound(t *testing.T) {
	asset := txnbuild.CreditAsset{Code: "USD", Issuer: testAssetIssuer}
	_, liquidityPoolAsset, _ := testLiquidityPoolAssets(t)
	testCases := []struct {
		name      string
		key       string
		invoke    func(*Client) error
		wantError string
	}{
		{
			name: "account",
			key:  testAccountKeyXDR,
			invoke: func(client *Client) error {
				_, err := client.GetAccountEntry(context.Background(), testAccountAddress)
				return err
			},
			wantError: "account " + testAccountAddress + " not found",
		},
		{
			name: "trustline",
			key:  testTrustlineKeyXDR,
			invoke: func(client *Client) error {
				_, err := client.GetTrustline(context.Background(), testAccountAddress, asset)
				return err
			},
			wantError: "trustline for USD:" + testAssetIssuer + " not found for account " + testAccountAddress,
		},
		{
			name: "liquidity pool trustline",
			key:  testLiquidityPoolTrustlineKeyXDR,
			invoke: func(client *Client) error {
				_, err := client.GetTrustline(context.Background(), testAccountAddress, liquidityPoolAsset)
				return err
			},
			wantError: "trustline for liquidity pool " + testLiquidityPoolIDHex +
				" not found for account " + testAccountAddress,
		},
		{
			name: "claimable balance",
			key:  testClaimableBalanceKeyXDR,
			invoke: func(client *Client) error {
				_, err := client.GetClaimableBalance(context.Background(), testClaimableBalanceStrkey)
				return err
			},
			wantError: "claimable balance " + testClaimableBalanceStrkey + " not found",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := newLedgerEntryTestServer(t, testCase.key, protocol.GetLedgerEntriesResponse{
				Entries: []protocol.LedgerEntryResult{},
			})
			defer server.Close()

			client := NewClient(server.URL, nil)
			defer client.Close()

			require.EqualError(t, testCase.invoke(client), testCase.wantError)
		})
	}
}

func TestClient_GetAccountEntry_MultipleEntries(t *testing.T) {
	data := xdr.LedgerEntryData{
		Type: xdr.LedgerEntryTypeAccount,
		Account: &xdr.AccountEntry{
			AccountId: xdr.MustAddress(testAccountAddress),
		},
	}
	response := ledgerEntryResponse(t, data)
	response.Entries = append(response.Entries, response.Entries[0])
	server := newLedgerEntryTestServer(t, testAccountKeyXDR, response)
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	_, err := client.GetAccountEntry(context.Background(), testAccountAddress)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one ledger entry")
}

func TestClient_LedgerEntryHelpers_MalformedResponse(t *testing.T) {
	for _, testCase := range ledgerEntryHelperTestCases() {
		t.Run(testCase.name, func(t *testing.T) {
			server := newLedgerEntryTestServer(t, testCase.key, protocol.GetLedgerEntriesResponse{
				Entries: []protocol.LedgerEntryResult{{DataXDR: "not-base64"}},
			})
			defer server.Close()

			client := NewClient(server.URL, nil)
			defer client.Close()

			err := testCase.invoke(client)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to decode ledger entry")
		})
	}
}

func TestClient_LedgerEntryHelpers_WrongEntryType(t *testing.T) {
	wrongData := xdr.LedgerEntryData{
		Type: xdr.LedgerEntryTypeData,
		Data: &xdr.DataEntry{
			AccountId: xdr.MustAddress(testAccountAddress),
			DataName:  "wrong-type",
			DataValue: []byte{1, 2, 3},
		},
	}

	for _, testCase := range ledgerEntryHelperTestCases() {
		t.Run(testCase.name, func(t *testing.T) {
			server := newLedgerEntryTestServer(t, testCase.key, ledgerEntryResponse(t, wrongData))
			defer server.Close()

			client := NewClient(server.URL, nil)
			defer client.Close()

			err := testCase.invoke(client)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "expected "+testCase.entryType+" ledger entry")
		})
	}
}

func TestClient_GetAccountEntry_RPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ledgerEntryTestRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, protocol.GetLedgerEntriesMethodName, request.Method)

		response := ledgerEntryTestResponse{
			JSONRPC: "2.0",
			Error: map[string]any{
				"code":    -32603,
				"message": "ledger backend unavailable",
			},
			ID: request.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	defer client.Close()

	_, err := client.GetAccountEntry(context.Background(), testAccountAddress)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get account entry")
	assert.Contains(t, err.Error(), "ledger backend unavailable")
}

type ledgerEntryHelperTestCase struct {
	name      string
	key       string
	entryType string
	invoke    func(*Client) error
}

func ledgerEntryHelperTestCases() []ledgerEntryHelperTestCase {
	asset := txnbuild.CreditAsset{Code: "USD", Issuer: testAssetIssuer}
	return []ledgerEntryHelperTestCase{
		{
			name:      "account",
			key:       testAccountKeyXDR,
			entryType: "account",
			invoke: func(client *Client) error {
				_, err := client.GetAccountEntry(context.Background(), testAccountAddress)
				return err
			},
		},
		{
			name:      "trustline",
			key:       testTrustlineKeyXDR,
			entryType: "trustline",
			invoke: func(client *Client) error {
				_, err := client.GetTrustline(context.Background(), testAccountAddress, asset)
				return err
			},
		},
		{
			name:      "claimable balance",
			key:       testClaimableBalanceKeyXDR,
			entryType: "claimable balance",
			invoke: func(client *Client) error {
				_, err := client.GetClaimableBalance(context.Background(), testClaimableBalanceStrkey)
				return err
			},
		},
	}
}

func newLedgerEntryTestServer(
	t *testing.T,
	expectedKey string,
	result protocol.GetLedgerEntriesResponse,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ledgerEntryTestRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "2.0", request.JSONRPC)
		require.Equal(t, protocol.GetLedgerEntriesMethodName, request.Method)

		var params protocol.GetLedgerEntriesRequest
		require.NoError(t, json.Unmarshal(request.Params, &params))
		require.Equal(t, []string{expectedKey}, params.Keys)
		require.Empty(t, params.Format)

		response := ledgerEntryTestResponse{
			JSONRPC: "2.0",
			Result:  result,
			ID:      request.ID,
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
}

func ledgerEntryResponse(t *testing.T, data xdr.LedgerEntryData) protocol.GetLedgerEntriesResponse {
	t.Helper()
	dataXDR, err := xdr.MarshalBase64(data)
	require.NoError(t, err)
	return protocol.GetLedgerEntriesResponse{
		Entries: []protocol.LedgerEntryResult{{DataXDR: dataXDR}},
	}
}

func testClaimableBalanceID(t *testing.T) xdr.ClaimableBalanceId {
	t.Helper()
	var balanceID xdr.ClaimableBalanceId
	require.NoError(t, xdr.SafeUnmarshalHex(testClaimableBalanceXDRHex, &balanceID))
	return balanceID
}

func testLiquidityPoolAssets(t *testing.T) (
	txnbuild.LiquidityPoolId,
	txnbuild.LiquidityPoolShareTrustLineAsset,
	txnbuild.LiquidityPoolShareChangeTrustAsset,
) {
	t.Helper()
	assetA := txnbuild.NativeAsset{}
	assetB := txnbuild.CreditAsset{Code: "USD", Issuer: testAssetIssuer}
	poolID, err := txnbuild.NewLiquidityPoolId(assetA, assetB)
	require.NoError(t, err)
	assert.Equal(t, testLiquidityPoolIDHex, fmt.Sprintf("%x", poolID))
	return poolID,
		txnbuild.LiquidityPoolShareTrustLineAsset{LiquidityPoolID: poolID},
		txnbuild.LiquidityPoolShareChangeTrustAsset{
			LiquidityPoolParameters: txnbuild.LiquidityPoolParameters{
				AssetA: assetA,
				AssetB: assetB,
				Fee:    txnbuild.LiquidityPoolFeeV18,
			},
		}
}
