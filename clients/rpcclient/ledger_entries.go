package rpcclient

import (
	"context"
	"errors"
	"fmt"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var errLedgerEntryNotFound = errors.New("ledger entry not found")

// GetAccountEntry retrieves the full account ledger entry for address.
func (c *Client) GetAccountEntry(ctx context.Context, address string) (xdr.AccountEntry, error) {
	accountID, err := xdr.AddressToAccountId(address)
	if err != nil {
		return xdr.AccountEntry{}, fmt.Errorf("address %s is not a valid Stellar account: %w", address, err)
	}

	key, err := accountID.LedgerKey()
	if err != nil {
		return xdr.AccountEntry{}, fmt.Errorf("failed to build ledger key for account %s: %w", address, err)
	}

	data, err := c.getLedgerEntry(ctx, key)
	if errors.Is(err, errLedgerEntryNotFound) {
		return xdr.AccountEntry{}, fmt.Errorf("account %s not found", address)
	}
	if err != nil {
		return xdr.AccountEntry{}, fmt.Errorf("failed to get account entry for %s: %w", address, err)
	}

	entry, ok := data.GetAccount()
	if !ok {
		return xdr.AccountEntry{}, fmt.Errorf("expected account ledger entry for %s, got %s", address, data.Type)
	}
	return entry, nil
}

// GetTrustline retrieves the full trustline ledger entry for address and asset.
func (c *Client) GetTrustline(
	ctx context.Context,
	address string,
	asset txnbuild.BasicAsset,
) (xdr.TrustLineEntry, error) {
	if asset == nil {
		return xdr.TrustLineEntry{}, errors.New("asset must not be nil")
	}
	if asset.IsNative() {
		return xdr.TrustLineEntry{}, errors.New("native asset does not have a trustline")
	}

	accountID, err := xdr.AddressToAccountId(address)
	if err != nil {
		return xdr.TrustLineEntry{}, fmt.Errorf("address %s is not a valid Stellar account: %w", address, err)
	}

	trustlineAsset, err := asset.ToTrustLineAsset()
	if err != nil {
		return xdr.TrustLineEntry{}, fmt.Errorf("invalid trustline asset: %w", err)
	}
	assetXDR, err := trustlineAsset.ToXDR()
	if err != nil {
		return xdr.TrustLineEntry{}, fmt.Errorf("invalid trustline asset: %w", err)
	}

	var key xdr.LedgerKey
	if err := key.SetTrustline(accountID, assetXDR); err != nil {
		return xdr.TrustLineEntry{}, fmt.Errorf("failed to build trustline ledger key: %w", err)
	}

	data, err := c.getLedgerEntry(ctx, key)
	assetID := fmt.Sprintf("%s:%s", asset.GetCode(), asset.GetIssuer())
	if poolID, ok := trustlineAsset.GetLiquidityPoolID(); ok {
		assetID = fmt.Sprintf("liquidity pool %x", poolID)
	}
	if errors.Is(err, errLedgerEntryNotFound) {
		return xdr.TrustLineEntry{}, fmt.Errorf("trustline for %s not found for account %s", assetID, address)
	}
	if err != nil {
		return xdr.TrustLineEntry{}, fmt.Errorf("failed to get trustline for %s on account %s: %w", assetID, address, err)
	}

	entry, ok := data.GetTrustLine()
	if !ok {
		return xdr.TrustLineEntry{}, fmt.Errorf("expected trustline ledger entry for %s on account %s, got %s", assetID, address, data.Type)
	}
	return entry, nil
}

// GetClaimableBalance retrieves the full claimable balance ledger entry for id.
// The id may be a B-prefixed strkey, a 72-character XDR hex string, or a
// 64-character hex hash without the XDR type prefix.
func (c *Client) GetClaimableBalance(ctx context.Context, id string) (xdr.ClaimableBalanceEntry, error) {
	balanceID, err := parseClaimableBalanceID(id)
	if err != nil {
		return xdr.ClaimableBalanceEntry{}, err
	}

	var key xdr.LedgerKey
	if err := key.SetClaimableBalance(balanceID); err != nil {
		return xdr.ClaimableBalanceEntry{}, fmt.Errorf("failed to build claimable balance ledger key: %w", err)
	}

	data, err := c.getLedgerEntry(ctx, key)
	if errors.Is(err, errLedgerEntryNotFound) {
		return xdr.ClaimableBalanceEntry{}, fmt.Errorf("claimable balance %s not found", id)
	}
	if err != nil {
		return xdr.ClaimableBalanceEntry{}, fmt.Errorf("failed to get claimable balance %s: %w", id, err)
	}

	entry, ok := data.GetClaimableBalance()
	if !ok {
		return xdr.ClaimableBalanceEntry{}, fmt.Errorf("expected claimable balance ledger entry for %s, got %s", id, data.Type)
	}
	return entry, nil
}

func (c *Client) getLedgerEntry(ctx context.Context, key xdr.LedgerKey) (xdr.LedgerEntryData, error) {
	keyXDR, err := xdr.MarshalBase64(key)
	if err != nil {
		return xdr.LedgerEntryData{}, fmt.Errorf("failed to encode ledger key: %w", err)
	}

	response, err := c.GetLedgerEntries(ctx, protocol.GetLedgerEntriesRequest{
		Keys: []string{keyXDR},
	})
	if err != nil {
		return xdr.LedgerEntryData{}, err
	}

	switch len(response.Entries) {
	case 0:
		return xdr.LedgerEntryData{}, errLedgerEntryNotFound
	case 1:
		var data xdr.LedgerEntryData
		if err := xdr.SafeUnmarshalBase64(response.Entries[0].DataXDR, &data); err != nil {
			return xdr.LedgerEntryData{}, fmt.Errorf("failed to decode ledger entry: %w", err)
		}
		return data, nil
	default:
		return xdr.LedgerEntryData{}, fmt.Errorf(
			"expected one ledger entry for key %s, got %d",
			keyXDR,
			len(response.Entries),
		)
	}
}

func parseClaimableBalanceID(id string) (xdr.ClaimableBalanceId, error) {
	var balanceID xdr.ClaimableBalanceId
	if strkey.IsValidClaimableBalance(id) {
		if err := balanceID.DecodeFromStrkey(id); err != nil {
			return xdr.ClaimableBalanceId{}, fmt.Errorf("invalid claimable balance ID %q: %w", id, err)
		}
		return balanceID, nil
	}

	hexID := id
	switch len(hexID) {
	case 64:
		hexID = "00000000" + hexID
	case 72:
	default:
		return xdr.ClaimableBalanceId{}, fmt.Errorf(
			"invalid claimable balance ID %q: expected a B-prefixed strkey, 64-character hex hash, or 72-character XDR hex string",
			id,
		)
	}

	if err := xdr.SafeUnmarshalHex(hexID, &balanceID); err != nil {
		return xdr.ClaimableBalanceId{}, fmt.Errorf("invalid claimable balance ID %q: %w", id, err)
	}
	return balanceID, nil
}
