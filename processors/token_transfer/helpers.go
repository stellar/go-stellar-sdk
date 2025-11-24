package token_transfer

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

/*
Helper function to convert LiquidityPoolId to generate ClaimableBalanceIds in a deterministic way as per
https://github.com/stellar/stellar-protocol/blob/master/core/cap-0038.md#settrustlineflagsop-and-allowtrustop
Whenever a trustline is revoked for an asset for an account, via setTrustlineFalgs or allowTrust operations,
if that account has deposited into a Liquidity Pool, then, claimable balances are created for those assets from the LP

The generated ClaimableBalanceId is derived from:
- Liquidity Pool Id from which the pool shares are withdrawn
- The asset for which CB is created
- the accountSequence number of the transaction account
- The transaction accountId
- The operationIndex of the setTrustlineFlags or allowTrust operation within the transaction
*/
func ClaimableBalanceIdFromRevocation(liquidityPoolId xdr.PoolId, asset xdr.Asset, accountSequence xdr.SequenceNumber, accountId xdr.AccountId, opIndex uint32) (xdr.ClaimableBalanceId, error) {
	preImageId := xdr.HashIdPreimage{
		Type: xdr.EnvelopeTypeEnvelopeTypePoolRevokeOpId,
		RevokeId: &xdr.HashIdPreimageRevokeId{
			SourceAccount:   accountId,
			SeqNum:          accountSequence,
			OpNum:           xdr.Uint32(opIndex),
			LiquidityPoolId: liquidityPoolId,
			Asset:           asset,
		},
	}
	binaryDump, e := preImageId.MarshalBinary()
	if e != nil {
		return xdr.ClaimableBalanceId{}, fmt.Errorf("failed to convert HashIdPreimage to claimable balanceId:%w", e)
	}
	sha256hash := xdr.Hash(sha256.Sum256(binaryDump))
	cbId := xdr.ClaimableBalanceId{
		Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		V0:   &sha256hash,
	}
	return cbId, nil
}

// Helper functions
func operationSourceAccount(tx ingest.LedgerTransaction, op xdr.Operation) xdr.MuxedAccount {
	acc := op.SourceAccount
	if acc != nil {
		return *acc
	}
	res := tx.Envelope.SourceAccount()
	return res
}

// Even though these functions simply call the corresponding proto, these are helpful to reduce clutter when being used in the unit test
// otherwise the entire imported path alias needs to be added and it is distracting
func protoAddressFromAccount(account xdr.MuxedAccount) string {
	return account.ToAccountId().Address()
}

func lpIdToStrkey(lpId xdr.PoolId) string {
	return strkey.MustEncode(strkey.VersionByteLiquidityPool, lpId[:])
}

func cbIdToStrkey(cbId xdr.ClaimableBalanceId) string {
	return cbId.MustEncodeToStrkey()
}

// This operation is used to only find CB entries that are either created or deleted, not updated
func getClaimableBalanceEntriesFromOperationChanges(changeType xdr.LedgerEntryChangeType, tx ingest.LedgerTransaction, opIndex uint32) ([]xdr.ClaimableBalanceEntry, error) {
	if changeType == xdr.LedgerEntryChangeTypeLedgerEntryUpdated {
		return nil, fmt.Errorf("LEDGER_ENTRY_UPDATED is not a valid filter")
	}

	changes, err := tx.GetOperationChanges(opIndex)
	if err != nil {
		return nil, err
	}

	var entries []xdr.ClaimableBalanceEntry
	/*
		This function is expected to be called only to get details of newly created claimable balance
		(for e.g as a result of allowTrust or setTrustlineFlags  operations)
		or claimable balances that are be deleted
		(for e.g due to clawback claimable balance operation)
	*/
	var cb xdr.ClaimableBalanceEntry
	for _, change := range changes {
		if change.Type != xdr.LedgerEntryTypeClaimableBalance || change.ChangeType != changeType {
			continue
		}
		if change.Pre != nil {
			cb = change.Pre.Data.MustClaimableBalance()
			entries = append(entries, cb)
		} else if change.Post != nil {
			cb = change.Post.Data.MustClaimableBalance()
			entries = append(entries, cb)
		}
	}

	return entries, nil
}

func possibleClaimableBalanceIdsFromRevocation(lpEntry liquidityPoolEntryDelta, tx ingest.LedgerTransaction, txSrcAccount xdr.AccountId, opIndex uint32) ([]xdr.ClaimableBalanceId, error) {
	var possibleClaimableBalanceIds []xdr.ClaimableBalanceId
	lpId := lpEntry.liquidityPoolId
	seqNum := xdr.SequenceNumber(tx.Envelope.SeqNum())

	for _, asset := range []xdr.Asset{lpEntry.assetA, lpEntry.assetB} {
		cbId, err := ClaimableBalanceIdFromRevocation(lpId, asset, seqNum, txSrcAccount, opIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to generate claimable balance id from LiquidityPoolId: %v, for asset: %v: %w", lpIdToStrkey(lpId), asset.String(), err)
		}
		possibleClaimableBalanceIds = append(possibleClaimableBalanceIds, cbId)
	}
	return possibleClaimableBalanceIds, nil
}

type liquidityPoolEntryDelta struct {
	liquidityPoolId       xdr.PoolId
	assetA                xdr.Asset
	assetB                xdr.Asset
	amountChangeForAssetA xdr.Int64
	amountChangeForAssetB xdr.Int64
}

func getImpactedLiquidityPoolEntriesFromOperation(tx ingest.LedgerTransaction, opIndex uint32) ([]liquidityPoolEntryDelta, error) {
	changes, err := tx.GetOperationChanges(opIndex)
	if err != nil {
		return nil, err
	}

	var entries []liquidityPoolEntryDelta
	for _, c := range changes {
		if c.Type != xdr.LedgerEntryTypeLiquidityPool {
			continue
		}
		var lp *xdr.LiquidityPoolEntry
		var entry liquidityPoolEntryDelta

		var preA, preB xdr.Int64
		if c.Pre != nil {
			lp = c.Pre.Data.LiquidityPool
			entry.liquidityPoolId = lp.LiquidityPoolId
			cp := lp.Body.ConstantProduct
			entry.assetA, entry.assetB = cp.Params.AssetA, cp.Params.AssetB
			preA, preB = cp.ReserveA, cp.ReserveB
		}

		var postA, postB xdr.Int64
		if c.Post != nil {
			lp = c.Post.Data.LiquidityPool
			entry.liquidityPoolId = lp.LiquidityPoolId
			cp := lp.Body.ConstantProduct
			entry.assetA, entry.assetB = cp.Params.AssetA, cp.Params.AssetB
			postA, postB = cp.ReserveA, cp.ReserveB
		}

		entry.amountChangeForAssetA = abs64(postA - preA)
		entry.amountChangeForAssetB = abs64(postB - preB)
		entries = append(entries, entry)
	}

	return entries, nil
}

func formatError(err error, tx ingest.LedgerTransaction, opIndex uint32, op xdr.Operation) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to process token transfer events for ledgerSequence: %v, txHash: %v, operationIndex: %v, operationType: %v. error: %w",
		tx.Ledger.LedgerSequence(), tx.Hash.HexString(), opIndex, op.Body.Type.String(), err)
}

// extractAddress helps extract a string representation of an address from a ScVal
func extractAddress(val xdr.ScVal) (string, error) {
	addr, ok := val.GetAddress()
	if !ok {
		return "", errors.New("invalid address")
	}
	return addr.String()
}

func isAccountAddress(addr string) bool {
	return strkey.IsValidEd25519PublicKey(addr) || strkey.IsValidMuxedAccountEd25519PublicKey(addr)
}
