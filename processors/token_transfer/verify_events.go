package token_transfer

import (
	"fmt"
	"io"

	"github.com/google/go-cmp/cmp"
	"github.com/stellar/go-stellar-sdk/amount"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// balanceKey represents a unique holder-asset pair for tracking balance changes
type balanceKey struct {
	holder string
	asset  string
}

// updateBalanceMap updates the map and removes the entry if the value becomes 0
func updateBalanceMap(m map[balanceKey]int64, key balanceKey, delta int64) {
	// We dont include movement to/from contract address is balance delta tracking, since there is no standard way to derive/verify from contractData
	if strkey.IsValidContractAddress(key.holder) {
		return
	}
	m[key] += delta
	if m[key] == 0 {
		delete(m, key)
	}
}

func fetchAccountDeltaFromChange(change ingest.Change, m map[balanceKey]int64) {
	var accountKey string
	var pre, post xdr.Int64

	if change.Pre != nil {
		entry := change.Pre.Data.MustAccount()
		accountKey = entry.AccountId.Address()
		pre = entry.Balance
	}
	if change.Post != nil {
		entry := change.Post.Data.MustAccount()
		accountKey = entry.AccountId.Address()
		post = entry.Balance
	}

	delta := int64(post - pre)
	updateBalanceMap(m, balanceKey{holder: accountKey, asset: xlmAsset.StringCanonical()}, delta)
}

func fetchTrustlineDeltaFromChange(change ingest.Change, m map[balanceKey]int64) {
	var trustlineKey string
	var asset string
	var pre, post xdr.Int64

	if change.Pre != nil {
		entry := change.Pre.Data.MustTrustLine()
		if entry.Asset.Type == xdr.AssetTypeAssetTypePoolShare {
			return // Skip pool share assets
		}
		trustlineKey = entry.AccountId.Address()
		pre = entry.Balance
		asset = entry.Asset.ToAsset().StringCanonical()
	}
	if change.Post != nil {
		entry := change.Post.Data.MustTrustLine()
		if entry.Asset.Type == xdr.AssetTypeAssetTypePoolShare {
			return // Skip pool share assets
		}
		trustlineKey = entry.AccountId.Address()
		post = entry.Balance
		asset = entry.Asset.ToAsset().StringCanonical()
	}

	delta := int64(post - pre)
	updateBalanceMap(m, balanceKey{holder: trustlineKey, asset: asset}, delta)
}

func fetchClaimableDeltaFromChange(change ingest.Change, m map[balanceKey]int64) {
	var cbKey string
	var asset string
	var pre, post xdr.Int64

	if change.Pre != nil {
		entry := change.Pre.Data.MustClaimableBalance()
		cbKey = cbIdToStrkey(entry.BalanceId)
		asset = entry.Asset.StringCanonical()
		pre = entry.Amount
	}
	if change.Post != nil {
		entry := change.Post.Data.MustClaimableBalance()
		cbKey = cbIdToStrkey(entry.BalanceId)
		asset = entry.Asset.StringCanonical()
		post = entry.Amount
	}

	delta := int64(post - pre)
	updateBalanceMap(m, balanceKey{holder: cbKey, asset: asset}, delta)
}

func fetchLiquidityPoolDeltaFromChange(change ingest.Change, m map[balanceKey]int64) {
	var lpKey string
	var assetA, assetB string
	var preA, preB, postA, postB xdr.Int64

	if change.Pre != nil {
		entry := change.Pre.Data.MustLiquidityPool()
		lpKey = lpIdToStrkey(entry.LiquidityPoolId)
		cp := entry.Body.ConstantProduct
		assetA, assetB = cp.Params.AssetA.StringCanonical(), cp.Params.AssetB.StringCanonical()
		preA, preB = cp.ReserveA, cp.ReserveB
	}

	if change.Post != nil {
		entry := change.Post.Data.MustLiquidityPool()
		lpKey = lpIdToStrkey(entry.LiquidityPoolId)
		cp := entry.Body.ConstantProduct
		assetA, assetB = cp.Params.AssetA.StringCanonical(), cp.Params.AssetB.StringCanonical()
		postA, postB = cp.ReserveA, cp.ReserveB
	}

	deltaA := int64(postA - preA)
	deltaB := int64(postB - preB)

	updateBalanceMap(m, balanceKey{holder: lpKey, asset: assetA}, deltaA)
	updateBalanceMap(m, balanceKey{holder: lpKey, asset: assetB}, deltaB)
}

// findBalanceDeltasFromChanges aggregates all balance changes from ledger entry changes
func findBalanceDeltasFromChanges(changes []ingest.Change) map[balanceKey]int64 {
	hashmap := make(map[balanceKey]int64)
	for _, change := range changes {
		switch change.Type {
		case xdr.LedgerEntryTypeAccount:
			fetchAccountDeltaFromChange(change, hashmap)
		case xdr.LedgerEntryTypeTrustline:
			fetchTrustlineDeltaFromChange(change, hashmap)
		case xdr.LedgerEntryTypeClaimableBalance:
			fetchClaimableDeltaFromChange(change, hashmap)
		case xdr.LedgerEntryTypeLiquidityPool:
			fetchLiquidityPoolDeltaFromChange(change, hashmap)
		}
	}
	return hashmap
}

// findBalanceDeltasFromEvents aggregates all balance changes from token transfer events
func findBalanceDeltasFromEvents(events []*TokenTransferEvent) map[balanceKey]int64 {
	hashmap := make(map[balanceKey]int64)

	for _, event := range events {
		if event.GetAsset() == nil { // needed check for custom token events which won't have an asset
			continue
		}

		switch event.GetEvent().(type) {
		case *TokenTransferEvent_Fee:
			ev := event.GetFee()
			address := ev.From
			asset := xlmAsset.StringCanonical()
			amt := amount.MustParseInt64Raw(ev.Amount)
			// Address' balance reduces by amt in FEE
			updateBalanceMap(hashmap, balanceKey{holder: address, asset: asset}, -amt)

		case *TokenTransferEvent_Transfer:
			ev := event.GetTransfer()
			fromAddress := ev.From
			toAddress := ev.To
			amt := amount.MustParseInt64Raw(ev.Amount)
			asset := event.GetAsset().ToXdrAsset().StringCanonical()
			// FromAddress' balance reduces by amt in TRANSFER
			updateBalanceMap(hashmap, balanceKey{holder: fromAddress, asset: asset}, -amt)
			// ToAddress' balance increases by amt in TRANSFER
			updateBalanceMap(hashmap, balanceKey{holder: toAddress, asset: asset}, amt)

		case *TokenTransferEvent_Mint:
			ev := event.GetMint()
			toAddress := ev.To
			asset := event.GetAsset().ToXdrAsset().StringCanonical()
			amt := amount.MustParseInt64Raw(ev.Amount)
			// ToAddress' balance increases by amt in MINT
			updateBalanceMap(hashmap, balanceKey{holder: toAddress, asset: asset}, amt)

		case *TokenTransferEvent_Burn:
			ev := event.GetBurn()
			fromAddress := ev.From
			asset := event.GetAsset().ToXdrAsset().StringCanonical()
			amt := amount.MustParseInt64Raw(ev.Amount)
			// FromAddress' balance reduces by amt in BURN
			updateBalanceMap(hashmap, balanceKey{holder: fromAddress, asset: asset}, -amt)

		case *TokenTransferEvent_Clawback:
			ev := event.GetClawback()
			fromAddress := ev.From
			asset := event.GetAsset().ToXdrAsset().StringCanonical()
			amt := amount.MustParseInt64Raw(ev.Amount)
			// FromAddress' balance reduces by amt in CLAWBACK
			updateBalanceMap(hashmap, balanceKey{holder: fromAddress, asset: asset}, -amt)

		default:
			panic(fmt.Errorf("unknown event type %s", event.GetEventType()))
		}
	}
	return hashmap
}

func VerifyEvents(ledger xdr.LedgerCloseMeta, passphrase string, readFromUnifiedEvents bool) error {
	var ttp *EventsProcessor
	if readFromUnifiedEvents {
		ttp = NewEventsProcessorForUnifiedEvents(passphrase)
	} else {
		ttp = NewEventsProcessor(passphrase)
	}
	txReader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(passphrase, ledger)
	if err != nil {
		return fmt.Errorf("error creating transaction reader: %w", err)
	}

	for {
		var tx ingest.LedgerTransaction
		var events []*TokenTransferEvent
		tx, err = txReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading transaction: %w", err)
		}

		txHash := tx.Hash.HexString()
		txEvents, err := ttp.EventsFromTransaction(tx)
		if err != nil {
			return fmt.Errorf("verifyEventsError: %w", err)
		}

		events = append(events, txEvents.FeeEvents...) // order here doesnt matter, since we are only using this to verify
		events = append(events, txEvents.OperationEvents...)

		feeChanges := tx.GetFeeChanges()
		txChanges, err := tx.GetChanges()
		if err != nil {
			return fmt.Errorf("verifyEventsError: %w", err)
		}
		postTxApplyFeeChanges := tx.GetPostApplyFeeChanges()

		changes := append(feeChanges, txChanges...)
		changes = append(changes, postTxApplyFeeChanges...)

		txEventsMap := findBalanceDeltasFromEvents(events)
		txChangesMap := findBalanceDeltasFromChanges(changes)

		if diff := cmp.Diff(txEventsMap, txChangesMap); diff != "" {
			return fmt.Errorf("balance delta mismatch between events and ledger changes for ledgerSequence: %v, closedAt: %v, txHash: %v\n"+
				"('-' indicates missing or different in events, '+' indicates missing or different in ledger changes)\n%s", ledger.LedgerSequence(), ledger.ClosedAt(), txHash, diff)
		}
	}
	return nil
}
