package orderbook

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const (
	_ = iota
	// the operationType enum values start at 1 because when constructing a
	// orderBookOperation struct, the operationType field should always be specified
	// explicitly. if the operationType enum values started at 0 then it would be
	// possible to create a valid orderBookOperation struct without specifying
	// the operationType field
	addOfferOperationType            = iota
	removeOfferOperationType         = iota
	addLiquidityPoolOperationType    = iota
	removeLiquidityPoolOperationType = iota
)

type orderBookOperation struct {
	operationType int
	offerID       xdr.Int64
	offer         *xdr.OfferEntry
	liquidityPool *xdr.LiquidityPoolEntry
}

type orderBookBatchedUpdates struct {
	operations []orderBookOperation
	orderbook  *OrderBookGraph
	committed  bool
}

// addOffer will queue an operation to add the given offer to the order book
func (tx *orderBookBatchedUpdates) addOffer(offer xdr.OfferEntry) *orderBookBatchedUpdates {
	tx.operations = append(tx.operations, orderBookOperation{
		operationType: addOfferOperationType,
		offerID:       offer.OfferId,
		offer:         &offer,
	})

	return tx
}

// addLiquidityPool will queue an operation to add the given liquidity pool to the order book graph
func (tx *orderBookBatchedUpdates) addLiquidityPool(pool xdr.LiquidityPoolEntry) *orderBookBatchedUpdates {
	tx.operations = append(tx.operations, orderBookOperation{
		operationType: addLiquidityPoolOperationType,
		liquidityPool: &pool,
	})

	return tx
}

// removeOffer will queue an operation to remove the given offer from the order book
func (tx *orderBookBatchedUpdates) removeOffer(offerID xdr.Int64) *orderBookBatchedUpdates {
	tx.operations = append(tx.operations, orderBookOperation{
		operationType: removeOfferOperationType,
		offerID:       offerID,
	})

	return tx
}

// removeLiquidityPool will queue an operation to remove the given liquidity pool from the order book
func (tx *orderBookBatchedUpdates) removeLiquidityPool(pool xdr.LiquidityPoolEntry) *orderBookBatchedUpdates {
	tx.operations = append(tx.operations, orderBookOperation{
		operationType: removeLiquidityPoolOperationType,
		liquidityPool: &pool,
	})

	return tx
}

// apply will attempt to apply all the updates in the batch to the order book
func (tx *orderBookBatchedUpdates) apply(ledger uint32) error {
	tx.orderbook.lock.Lock()
	defer tx.orderbook.lock.Unlock()

	if tx.committed {
		// This should never happen
		panic(errBatchAlreadyApplied)
	}
	tx.committed = true

	if tx.orderbook.lastLedger > 0 && ledger <= tx.orderbook.lastLedger {
		return errUnexpectedLedger
	}

	reallocatePairs := map[tradingPair]struct{}{}

	for _, operation := range tx.operations {
		switch operation.operationType {
		case addOfferOperationType:
			if err := tx.orderbook.addOffer(*operation.offer); err != nil {
				panic(errors.Wrap(err, "could not apply update in batch"))
			}
		case removeOfferOperationType:
			if pair, ok := tx.orderbook.tradingPairForOffer[operation.offerID]; !ok {
				continue
			} else {
				reallocatePairs[pair] = struct{}{}
			}
			if err := tx.orderbook.removeOffer(operation.offerID); err != nil {
				panic(errors.Wrap(err, "could not apply update in batch"))
			}

		case addLiquidityPoolOperationType:
			tx.orderbook.addPool(*operation.liquidityPool)

		case removeLiquidityPoolOperationType:
			tx.orderbook.removePool(*operation.liquidityPool)

		default:
			panic(errors.New("invalid operation type"))
		}
	}

	tx.orderbook.lastLedger = ledger

	for pair := range reallocatePairs {
		tx.orderbook.venuesForSellingAsset[pair.sellingAsset].reallocate()
		tx.orderbook.venuesForBuyingAsset[pair.buyingAsset].reallocate()
	}
	return nil
}
