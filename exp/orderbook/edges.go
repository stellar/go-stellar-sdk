package orderbook

import (
	"sort"

	"github.com/stellar/go-stellar-sdk/xdr"
	"golang.org/x/exp/slices"
)

// edgeSet maintains a mapping of assets to a set of venues, which
// is composed of a sorted lists of offers and, optionally, a liquidity pool.
// The offers are sorted by ascending price (in terms of the buying asset).
type edgeSet []edge

type edge struct {
	key   int32
	value Venues

	// reallocate is set to true when some offers were removed from the edge.
	// See edgeSet.reallocate() godoc for more information.
	reallocate bool
}

func (e edgeSet) find(key int32) int {
	for i := 0; i < len(e); i++ {
		if e[i].key == key {
			return i
		}
	}
	return -1
}

// addOffer will insert the given offer into the edge set
func (e edgeSet) addOffer(key int32, offer xdr.OfferEntry) edgeSet {
	// The list of offers in a venue is sorted by cheapest to most expensive
	// price to convert buyingAsset to sellingAsset
	i := e.find(key)
	if i < 0 {
		return append(e, edge{key: key, value: Venues{offers: []xdr.OfferEntry{offer}}})
	}

	offers := e[i].value.offers
	// find the smallest i such that Price of offers[i] > Price of offer
	insertIndex := sort.Search(len(offers), func(j int) bool {
		return offer.Price.Cheaper(offers[j].Price)
	})

	offers = slices.Insert(offers, insertIndex, offer)
	e[i].value = Venues{offers: offers, pool: e[i].value.pool}
	return e
}

// addPool makes `pool` a viable venue at `key`.
func (e edgeSet) addPool(key int32, pool liquidityPool) edgeSet {
	i := e.find(key)
	if i < 0 {
		return append(e, edge{key: key, value: Venues{pool: pool}})
	}
	e[i].value.pool = pool
	return e
}

// removeOffer will delete the given offer from the edge set, returning whether
// or not the given offer was actually found.
func (e edgeSet) removeOffer(key int32, offerID xdr.Int64) (edgeSet, bool) {
	i := e.find(key)
	if i < 0 {
		return e, false
	}

	offers := e[i].value.offers
	updatedOffers := offers
	contains := false
	for i, offer := range offers {
		if offer.OfferId != offerID {
			continue
		}

		updatedOffers = slices.Delete(offers, i, i+1)
		contains = true
		break
	}

	if !contains {
		return e, false
	}

	if len(updatedOffers) == 0 && e[i].value.pool.Body.ConstantProduct == nil {
		return slices.Delete(e, i, i+1), true
	}
	e[i].value.offers = updatedOffers
	e[i].reallocate = true
	return e, true
}

func (e edgeSet) removePool(key int32) edgeSet {
	i := e.find(key)
	if i < 0 {
		return e
	}

	if len(e[i].value.offers) == 0 {
		return slices.Delete(e, i, i+1)
	}

	e[i].value = Venues{offers: e[i].value.offers}
	return e
}

// reallocate recreates offers slice when edge.reallocate is set to true and
// this is true after an offer is removed.
// Without periodic reallocations an arbitrary account could create 1000s of
// offers in an orderbook, then remove them but the space occupied by these
// offers would not be released by GC because an array used internally is
// the same. This can lead to DoS attack by OOM.
func (e edgeSet) reallocate() {
	for i := 0; i < len(e); i++ {
		if e[i].reallocate {
			e[i].value.offers = append([]xdr.OfferEntry(nil), e[i].value.offers[:]...)
			e[i].reallocate = false
		}
	}
}
