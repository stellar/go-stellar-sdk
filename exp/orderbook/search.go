package orderbook

import (
	"context"

	"github.com/stellar/go-stellar-sdk/price"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Path represents a payment path from a source asset to some destination asset
type Path struct {
	SourceAsset       string
	SourceAmount      xdr.Int64
	DestinationAsset  string
	DestinationAmount xdr.Int64

	InteriorNodes []string
}

type liquidityPool struct {
	xdr.LiquidityPoolEntry
	assetA int32
	assetB int32
}

type Venues struct {
	offers []xdr.OfferEntry
	pool   liquidityPool // can be empty, check body pointer
}

type searchState interface {
	// totalAssets returns the total number of assets in the search space.
	totalAssets() int32

	// considerPools returns true if we will consider liquidity pools in our path
	// finding search.
	considerPools() bool

	// isTerminalNode returns true if the current asset is a terminal node in our
	// path finding search.
	isTerminalNode(asset int32) bool

	// includePath returns true if the current path which ends at the given asset
	// and produces the given amount satisfies our search criteria.
	includePath(
		currentAsset int32,
		currentAssetAmount xdr.Int64,
	) bool

	// betterPathAmount returns true if alternativeAmount is better than currentAmount
	// Given two paths (current path and alternative path) which lead to the same asset
	// but possibly have different amounts of that asset, betterPathAmount will return
	// true if the alternative path is better than the current path.
	betterPathAmount(currentAmount, alternativeAmount xdr.Int64) bool

	// appendToPaths appends the current path to our result list.
	appendToPaths(
		path []int32,
		currentAsset int32,
		currentAssetAmount xdr.Int64,
	)

	// venues returns all possible trading opportunities for a particular asset.
	//
	// The result is grouped by the next asset hop, mapping to a sorted list of
	// offers (by price) and a liquidity pool (if one exists for that trading
	// pair).
	venues(currentAsset int32) edgeSet

	// consumeOffers will consume the given set of offers to trade our
	// current asset for a different asset.
	consumeOffers(
		currentAssetAmount xdr.Int64,
		currentBestAmount xdr.Int64,
		offers []xdr.OfferEntry,
	) (xdr.Int64, error)

	// consumePool will consume the given liquidity pool to trade our
	// current asset for a different asset.
	consumePool(
		pool liquidityPool,
		currentAsset int32,
		currentAssetAmount xdr.Int64,
	) (xdr.Int64, error)
}

type pathNode struct {
	asset int32
	prev  *pathNode
}

func (p *pathNode) contains(node int32) bool {
	for cur := p; cur != nil; cur = cur.prev {
		if cur.asset == node {
			return true
		}
	}
	return false
}

func reversePath(path []int32) {
	for i := len(path)/2 - 1; i >= 0; i-- {
		opp := len(path) - 1 - i
		path[i], path[opp] = path[opp], path[i]
	}
}

func (e *pathNode) path() []int32 {
	// Initialize slice capacity to minimize allocations.
	// 8 is the maximum path supported by stellar.
	result := make([]int32, 0, 8)
	for cur := e; cur != nil; cur = cur.prev {
		result = append(result, cur.asset)
	}

	reversePath(result)
	return result
}

func search(
	ctx context.Context,
	state searchState,
	maxPathLength int,
	sourceAsset int32,
	sourceAssetAmount xdr.Int64,
) error {
	totalAssets := state.totalAssets()
	bestAmount := make([]xdr.Int64, totalAssets)
	updateAmount := make([]xdr.Int64, totalAssets)
	bestPath := make([]*pathNode, totalAssets)
	updatePath := make([]*pathNode, totalAssets)
	updatedAssets := make([]int32, 0, totalAssets)
	// Used to minimize allocations
	slab := make([]pathNode, 0, totalAssets)
	bestAmount[sourceAsset] = sourceAssetAmount
	updateAmount[sourceAsset] = sourceAssetAmount
	bestPath[sourceAsset] = &pathNode{
		asset: sourceAsset,
		prev:  nil,
	}
	// Simple payments (e.g. payments where an asset is transferred from
	// one account to another without any conversions into another asset)
	// are also valid path payments. If the source asset is a valid
	// destination asset we include the empty path in the response.
	if state.includePath(sourceAsset, sourceAssetAmount) {
		state.appendToPaths(
			[]int32{sourceAsset},
			sourceAsset,
			sourceAssetAmount,
		)
	}

	for i := 0; i < maxPathLength; i++ {
		updatedAssets = updatedAssets[:0]

		for currentAsset := int32(0); currentAsset < totalAssets; currentAsset++ {
			currentAmount := bestAmount[currentAsset]
			if currentAmount == 0 {
				continue
			}
			pathToCurrentAsset := bestPath[currentAsset]
			edges := state.venues(currentAsset)
			for j := 0; j < len(edges); j++ {
				// Exit early if the context was canceled.
				if err := ctx.Err(); err != nil {
					return err
				}
				nextAsset, venues := edges[j].key, edges[j].value

				// If we're on our last step ignore any edges which don't lead to
				// our desired destination. This optimization will save us from
				// doing wasted computation.
				if i == maxPathLength-1 && !state.isTerminalNode(nextAsset) {
					continue
				}

				// Make sure we don't visit a node more than once.
				if pathToCurrentAsset.contains(nextAsset) {
					continue
				}

				nextAssetAmount, err := processVenues(state, currentAsset, currentAmount, venues)
				if err != nil {
					return err
				}
				if nextAssetAmount <= 0 {
					continue
				}

				if state.betterPathAmount(updateAmount[nextAsset], nextAssetAmount) {
					newEntry := updateAmount[nextAsset] == bestAmount[nextAsset]
					updateAmount[nextAsset] = nextAssetAmount

					if newEntry {
						updatedAssets = append(updatedAssets, nextAsset)
						// By piggybacking on slice appending (which uses exponential allocation)
						// we avoid allocating each node individually, which is much slower and
						// puts more pressure on the garbage collector.
						slab = append(slab, pathNode{
							asset: nextAsset,
							prev:  pathToCurrentAsset,
						})
						updatePath[nextAsset] = &slab[len(slab)-1]
					} else {
						updatePath[nextAsset].prev = pathToCurrentAsset
					}

					// We could avoid this step until the last iteration, but we would
					// like to include multiple paths in the response to give the user
					// other options in case the best path is already consumed.
					if state.includePath(nextAsset, nextAssetAmount) {
						state.appendToPaths(
							append(bestPath[currentAsset].path(), nextAsset),
							nextAsset,
							nextAssetAmount,
						)
					}
				}
			}
		}

		// Only update bestPath and bestAmount if we have more iterations left in
		// the algorithm. This optimization will save us from doing wasted
		// computation.
		if i < maxPathLength-1 {
			for _, asset := range updatedAssets {
				bestPath[asset] = updatePath[asset]
				bestAmount[asset] = updateAmount[asset]
			}
		}
	}

	return nil
}

// sellingGraphSearchState configures a DFS on the orderbook graph where only
// edges in `graph.edgesForSellingAsset` are traversed.
//
// The DFS maintains the following invariants:
//   - no node is repeated
//   - no offers are consumed from the `ignoreOffersFrom` account
//   - each payment path must begin with an asset in `targetAssets`
//   - also, the required source asset amount cannot exceed the balance in
//     `targetAssets`
type sellingGraphSearchState struct {
	graph                  *OrderBookGraph
	destinationAssetString string
	destinationAssetAmount xdr.Int64
	ignoreOffersFrom       *xdr.AccountId
	targetAssets           map[int32]xdr.Int64
	validateSourceBalance  bool
	paths                  []Path
	includePools           bool
}

func (state *sellingGraphSearchState) totalAssets() int32 {
	return int32(len(state.graph.idToAssetString))
}

func (state *sellingGraphSearchState) isTerminalNode(currentAsset int32) bool {
	_, ok := state.targetAssets[currentAsset]
	return ok
}

func (state *sellingGraphSearchState) includePath(currentAsset int32, currentAssetAmount xdr.Int64) bool {
	targetAssetBalance, ok := state.targetAssets[currentAsset]
	return ok && (!state.validateSourceBalance || targetAssetBalance >= currentAssetAmount)
}

func (state *sellingGraphSearchState) betterPathAmount(currentAmount, alternativeAmount xdr.Int64) bool {
	if currentAmount == 0 {
		return true
	}
	if alternativeAmount == 0 {
		return false
	}
	return alternativeAmount < currentAmount
}

func assetIDsToAssetStrings(graph *OrderBookGraph, path []int32) []string {
	result := make([]string, len(path))
	for i := 0; i < len(path); i++ {
		result[i] = graph.idToAssetString[path[i]]
	}
	return result
}

func (state *sellingGraphSearchState) appendToPaths(
	path []int32,
	currentAsset int32,
	currentAssetAmount xdr.Int64,
) {
	if len(path) > 2 {
		path = path[1 : len(path)-1]
		reversePath(path)
	} else {
		path = []int32{}
	}

	state.paths = append(state.paths, Path{
		SourceAmount:      currentAssetAmount,
		SourceAsset:       state.graph.idToAssetString[currentAsset],
		InteriorNodes:     assetIDsToAssetStrings(state.graph, path),
		DestinationAsset:  state.destinationAssetString,
		DestinationAmount: state.destinationAssetAmount,
	})
}

func (state *sellingGraphSearchState) venues(currentAsset int32) edgeSet {
	return state.graph.venuesForSellingAsset[currentAsset]
}

func (state *sellingGraphSearchState) consumeOffers(
	currentAssetAmount xdr.Int64,
	currentBestAmount xdr.Int64,
	offers []xdr.OfferEntry,
) (xdr.Int64, error) {
	nextAmount, err := consumeOffersForSellingAsset(
		offers, state.ignoreOffersFrom, currentAssetAmount, currentBestAmount)

	return positiveMin(currentBestAmount, nextAmount), err
}

func (state *sellingGraphSearchState) considerPools() bool {
	return state.includePools
}

func (state *sellingGraphSearchState) consumePool(
	pool liquidityPool,
	currentAsset int32,
	currentAssetAmount xdr.Int64,
) (xdr.Int64, error) {
	// How many of the previous hop do we need to get this amount?
	return makeTrade(pool, getOtherAsset(currentAsset, pool),
		tradeTypeExpectation, currentAssetAmount)
}

// buyingGraphSearchState configures a DFS on the orderbook graph where only
// edges in `graph.edgesForBuyingAsset` are traversed.
//
// The DFS maintains the following invariants:
//   - no node is repeated
//   - no offers are consumed from the `ignoreOffersFrom` account
//   - each payment path must terminate with an asset in `targetAssets`
//   - each payment path must begin with `sourceAsset`
type buyingGraphSearchState struct {
	graph             *OrderBookGraph
	sourceAssetString string
	sourceAssetAmount xdr.Int64
	targetAssets      map[int32]bool
	paths             []Path
	includePools      bool
}

func (state *buyingGraphSearchState) totalAssets() int32 {
	return int32(len(state.graph.idToAssetString))
}

func (state *buyingGraphSearchState) isTerminalNode(currentAsset int32) bool {
	return state.targetAssets[currentAsset]
}

func (state *buyingGraphSearchState) includePath(currentAsset int32, currentAssetAmount xdr.Int64) bool {
	return state.targetAssets[currentAsset]
}

func (state *buyingGraphSearchState) betterPathAmount(currentAmount, alternativeAmount xdr.Int64) bool {
	return alternativeAmount > currentAmount
}

func (state *buyingGraphSearchState) appendToPaths(
	path []int32,
	currentAsset int32,
	currentAssetAmount xdr.Int64,
) {
	if len(path) > 2 {
		path = path[1 : len(path)-1]
	} else {
		path = []int32{}
	}

	state.paths = append(state.paths, Path{
		SourceAmount:      state.sourceAssetAmount,
		SourceAsset:       state.sourceAssetString,
		InteriorNodes:     assetIDsToAssetStrings(state.graph, path),
		DestinationAsset:  state.graph.idToAssetString[currentAsset],
		DestinationAmount: currentAssetAmount,
	})
}

func (state *buyingGraphSearchState) venues(currentAsset int32) edgeSet {
	return state.graph.venuesForBuyingAsset[currentAsset]
}

func (state *buyingGraphSearchState) consumeOffers(
	currentAssetAmount xdr.Int64,
	currentBestAmount xdr.Int64,
	offers []xdr.OfferEntry,
) (xdr.Int64, error) {
	nextAmount, err := consumeOffersForBuyingAsset(offers, currentAssetAmount)

	return max(nextAmount, currentBestAmount), err
}

func (state *buyingGraphSearchState) considerPools() bool {
	return state.includePools
}

func (state *buyingGraphSearchState) consumePool(
	pool liquidityPool,
	currentAsset int32,
	currentAssetAmount xdr.Int64,
) (xdr.Int64, error) {
	return makeTrade(pool, currentAsset, tradeTypeDeposit, currentAssetAmount)
}

func consumeOffersForSellingAsset(
	offers []xdr.OfferEntry,
	ignoreOffersFrom *xdr.AccountId,
	currentAssetAmount xdr.Int64,
	currentBestAmount xdr.Int64,
) (xdr.Int64, error) {
	if len(offers) == 0 {
		return 0, errEmptyOffers
	}

	if currentAssetAmount == 0 {
		return 0, errAssetAmountIsZero
	}

	totalConsumed := xdr.Int64(0)
	for i := 0; i < len(offers); i++ {
		if ignoreOffersFrom != nil && ignoreOffersFrom.Equals(offers[i].SellerId) {
			continue
		}

		buyingUnitsFromOffer, sellingUnitsFromOffer, err := price.ConvertToBuyingUnits(
			int64(offers[i].Amount),
			int64(currentAssetAmount),
			int64(offers[i].Price.N),
			int64(offers[i].Price.D),
		)
		if err == price.ErrOverflow {
			// skip paths which would result in overflow errors
			// but still continue the path finding search
			return -1, nil
		} else if err != nil {
			return -1, err
		}

		totalConsumed += xdr.Int64(buyingUnitsFromOffer)

		// For sell-state, we are aiming to *minimize* the amount of the source
		// assets we need to get to the destination, so if we exceed the best
		// amount, it's time to bail.
		//
		// FIXME: Evaluate if this can work, and if it's actually performant.
		// if totalConsumed >= currentBestAmount && currentBestAmount > 0 {
		// 	return currentBestAmount, nil
		// }

		currentAssetAmount -= xdr.Int64(sellingUnitsFromOffer)

		if currentAssetAmount == 0 {
			return totalConsumed, nil
		}
		if currentAssetAmount < 0 {
			return -1, errSoldTooMuch
		}
	}

	return -1, nil
}

func consumeOffersForBuyingAsset(
	offers []xdr.OfferEntry,
	currentAssetAmount xdr.Int64,
) (xdr.Int64, error) {
	if len(offers) == 0 {
		return 0, errEmptyOffers
	}

	if currentAssetAmount == 0 {
		return 0, errAssetAmountIsZero
	}

	totalConsumed := xdr.Int64(0)
	for i := 0; i < len(offers); i++ {
		n := int64(offers[i].Price.N)
		d := int64(offers[i].Price.D)

		// check if we can spend all of currentAssetAmount on the current offer
		// otherwise consume entire offer and move on to the next one
		amountSold, err := price.MulFractionRoundDown(int64(currentAssetAmount), d, n)
		if err == nil {
			if amountSold == 0 {
				// not enough of the buying asset to consume the offer
				return -1, nil
			}
			if amountSold < 0 {
				return -1, errSoldTooMuch
			}

			amountSoldXDR := xdr.Int64(amountSold)
			if amountSoldXDR <= offers[i].Amount {
				totalConsumed += amountSoldXDR
				return totalConsumed, nil
			}
		} else if err != price.ErrOverflow {
			return -1, err
		}

		buyingUnitsFromOffer, sellingUnitsFromOffer, err := price.ConvertToBuyingUnits(
			int64(offers[i].Amount),
			int64(offers[i].Amount),
			n,
			d,
		)
		if err == price.ErrOverflow {
			// skip paths which would result in overflow errors
			// but still continue the path finding search
			return -1, nil
		} else if err != nil {
			return -1, err
		}

		totalConsumed += xdr.Int64(sellingUnitsFromOffer)
		currentAssetAmount -= xdr.Int64(buyingUnitsFromOffer)

		if currentAssetAmount == 0 {
			return totalConsumed, nil
		}
		if currentAssetAmount < 0 {
			return -1, errSoldTooMuch
		}
	}

	return -1, nil
}

func processVenues(
	state searchState,
	currentAsset int32,
	currentAssetAmount xdr.Int64,
	venues Venues,
) (xdr.Int64, error) {
	if currentAssetAmount == 0 {
		return 0, errAssetAmountIsZero
	}

	// We evaluate the pool venue (if any) before offers, because pool exchange
	// rates can only be evaluated with an amount.
	poolAmount := xdr.Int64(0)
	if pool := venues.pool; state.considerPools() && pool.Body.ConstantProduct != nil {
		amount, err := state.consumePool(pool, currentAsset, currentAssetAmount)
		if err == nil {
			poolAmount = amount
		}
		// It's only a true error if the offers fail later, too
	}

	if poolAmount == 0 && len(venues.offers) == 0 {
		return -1, nil // not really an error
	}

	// This will return the pool amount if the LP performs better.
	nextAssetAmount, err := state.consumeOffers(
		currentAssetAmount, poolAmount, venues.offers)

	// Only error out the offers if the LP trade didn't happen.
	if err != nil && poolAmount == 0 {
		return 0, err
	}

	return nextAssetAmount, nil
}
