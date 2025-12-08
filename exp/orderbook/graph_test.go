package orderbook

import (
	"bytes"
	"context"
	"encoding"
	"math"
	"sort"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/price"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
)

var (
	issuer, _ = xdr.NewAccountId(xdr.PublicKeyTypePublicKeyTypeEd25519, xdr.Uint256{})

	nativeAsset = xdr.Asset{
		Type: xdr.AssetTypeAssetTypeNative,
	}

	usdAsset = xdr.Asset{
		Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
		AlphaNum4: &xdr.AlphaNum4{
			AssetCode: [4]byte{'u', 's', 'd', 0},
			Issuer:    issuer,
		},
	}

	eurAsset = xdr.Asset{
		Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
		AlphaNum4: &xdr.AlphaNum4{
			AssetCode: [4]byte{'e', 'u', 'r', 0},
			Issuer:    issuer,
		},
	}

	chfAsset = xdr.Asset{
		Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
		AlphaNum4: &xdr.AlphaNum4{
			AssetCode: [4]byte{'c', 'h', 'f', 0},
			Issuer:    issuer,
		},
	}

	yenAsset = xdr.Asset{
		Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
		AlphaNum4: &xdr.AlphaNum4{
			AssetCode: [4]byte{'y', 'e', 'n', 0},
			Issuer:    issuer,
		},
	}

	fiftyCentsOffer = xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(1),
		Buying:   usdAsset,
		Selling:  nativeAsset,
		Price: xdr.Price{
			N: 1,
			D: 2,
		},
		Amount: xdr.Int64(500),
	}
	quarterOffer = xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(2),
		Buying:   usdAsset,
		Selling:  nativeAsset,
		Price: xdr.Price{
			N: 1,
			D: 4,
		},
		Amount: xdr.Int64(500),
	}
	dollarOffer = xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(3),
		Buying:   usdAsset,
		Selling:  nativeAsset,
		Price: xdr.Price{
			N: 1,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}

	eurOffer = xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(4),
		Buying:   eurAsset,
		Selling:  nativeAsset,
		Price: xdr.Price{
			N: 1,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}
	twoEurOffer = xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(5),
		Buying:   eurAsset,
		Selling:  nativeAsset,
		Price: xdr.Price{
			N: 2,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}
	threeEurOffer = xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(6),
		Buying:   eurAsset,
		Selling:  nativeAsset,
		Price: xdr.Price{
			N: 3,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}

	eurUsdLiquidityPool = makePool(eurAsset, usdAsset, 1000, 1000)
	eurYenLiquidityPool = makePool(eurAsset, yenAsset, 1000, 1000)
	usdChfLiquidityPool = makePool(chfAsset, usdAsset, 500, 1000)
	nativeEurPool       = makePool(xdr.MustNewNativeAsset(), eurAsset, 1500, 30)
	nativeUsdPool       = makePool(xdr.MustNewNativeAsset(), usdAsset, 120, 30)
)

func assertBinaryMarshalerEquals(t *testing.T, a, b encoding.BinaryMarshaler) {
	serializedA, err := a.MarshalBinary()
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	serializedB, err := b.MarshalBinary()
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	if !assert.Truef(t, bytes.Equal(serializedA, serializedB),
		"expected lists to be equal but got %v %v", a, b) {
		t.FailNow()
	}
}

func assertOfferListEquals(t *testing.T, a, b []xdr.OfferEntry) {
	assert.Equalf(t, len(a), len(b),
		"expected lists to have same length but got %v %v", a, b)

	for i := 0; i < len(a); i++ {
		assertBinaryMarshalerEquals(t, a[i], b[i])
	}
}

// assertGraphEquals ensures two graphs are identical
func assertGraphEquals(t *testing.T, a, b *OrderBookGraph) {
	assert.Equalf(t, len(a.assetStringToID), len(b.assetStringToID),
		"expected same # of asset string to id entries but got %v %v",
		a.assetStringToID, b.assetStringToID)

	assert.Equalf(t, len(a.tradingPairForOffer), len(b.tradingPairForOffer),
		"expected same # of trading pairs but got %v %v", a, b)

	assert.Equalf(t, len(a.liquidityPools), len(b.liquidityPools),
		"expected same # of liquidity pools but got %v %v", a, b)

	for assetString := range a.assetStringToID {
		asset := a.assetStringToID[assetString]
		otherAsset, ok := b.assetStringToID[assetString]
		if !ok {
			t.Fatalf("asset %v is not present in assetStringToID", assetString)
		}
		es := a.venuesForSellingAsset[asset]
		other := b.venuesForSellingAsset[otherAsset]

		assertEdgeSetEquals(t, a, b, es, other, assetString)

		es = a.venuesForBuyingAsset[asset]
		other = b.venuesForBuyingAsset[otherAsset]

		assert.Equalf(t, len(es), len(other),
			"expected edge set for %v to have same length but got %v %v",
			assetString, es, other)

		assertEdgeSetEquals(t, a, b, es, other, assetString)
	}

	for offerID, pair := range a.tradingPairForOffer {
		otherPair := b.tradingPairForOffer[offerID]

		assert.Equalf(
			t,
			a.idToAssetString[pair.buyingAsset],
			b.idToAssetString[otherPair.buyingAsset],
			"expected trading pair to match but got %v %v", pair, otherPair)

		assert.Equalf(
			t,
			a.idToAssetString[pair.sellingAsset],
			b.idToAssetString[otherPair.sellingAsset],
			"expected trading pair to match but got %v %v", pair, otherPair)
	}

	for pair, pool := range a.liquidityPools {
		otherPair := tradingPair{
			buyingAsset:  b.assetStringToID[a.idToAssetString[pair.buyingAsset]],
			sellingAsset: b.assetStringToID[a.idToAssetString[pair.sellingAsset]],
		}
		otherPool := b.liquidityPools[otherPair]
		assert.Equalf(t, pool, otherPool, "expected pool to match but got %v %v", pool, otherPool)
	}
}

func assertEdgeSetEquals(
	t *testing.T, a *OrderBookGraph, b *OrderBookGraph,
	es edgeSet, other edgeSet, assetString string) {
	assert.Equalf(t, len(es), len(other),
		"expected edge set for %v to have same length but got %v %v",
		assetString, es, other)

	for _, edge := range es {
		venues := edge.value
		otherVenues := findByAsset(b, other, a.idToAssetString[edge.key])

		assert.Equalf(t, venues.pool.LiquidityPoolEntry, otherVenues.pool.LiquidityPoolEntry,
			"expected pools for %v to be equal")

		assert.Equalf(t, len(venues.offers), len(otherVenues.offers),
			"expected offers for %v to have same length but got %v %v",
			edge.key, venues.offers, otherVenues.offers,
		)

		assertOfferListEquals(t, venues.offers, otherVenues.offers)
	}
}

func assertPathEquals(t *testing.T, a, b []Path) {
	if !assert.Equalf(t, len(a), len(b),
		"expected paths to have same length but got %v != %v", a, b) {
		t.FailNow()
	}

	for i := 0; i < len(a); i++ {
		assert.Equalf(t, a[i].SourceAmount, b[i].SourceAmount,
			"expected src amounts to be same got %v %v", a[i], b[i])

		assert.Equalf(t, a[i].DestinationAmount, b[i].DestinationAmount,
			"expected dest amounts to be same got %v %v", a[i], b[i])

		assert.Equalf(t, a[i].DestinationAsset, b[i].DestinationAsset,
			"expected dest assets to be same got %v %v", a[i], b[i])

		assert.Equalf(t, a[i].SourceAsset, b[i].SourceAsset,
			"expected source assets to be same got %v %v", a[i], b[i])

		assert.Equalf(t, len(a[i].InteriorNodes), len(b[i].InteriorNodes),
			"expected interior nodes have same length got %v %v", a[i], b[i])

		for j := 0; j > len(a[i].InteriorNodes); j++ {
			assert.Equalf(t,
				a[i].InteriorNodes[j], b[i].InteriorNodes[j],
				"expected interior nodes to be same got %v %v", a[i], b[i])
		}
	}
}

func findByAsset(g *OrderBookGraph, edges edgeSet, assetString string) Venues {
	asset, ok := g.assetStringToID[assetString]
	if !ok {
		return Venues{}
	}
	i := edges.find(asset)
	if i >= 0 {
		return edges[i].value
	}
	return Venues{}
}

func TestAddEdgeSet(t *testing.T) {
	set := edgeSet{}
	g := NewOrderBookGraph()

	set = set.addOffer(g.getOrCreateAssetID(dollarOffer.Buying), dollarOffer)
	set = set.addOffer(g.getOrCreateAssetID(eurOffer.Buying), eurOffer)
	set = set.addOffer(g.getOrCreateAssetID(twoEurOffer.Buying), twoEurOffer)
	set = set.addOffer(g.getOrCreateAssetID(threeEurOffer.Buying), threeEurOffer)
	set = set.addOffer(g.getOrCreateAssetID(quarterOffer.Buying), quarterOffer)
	set = set.addOffer(g.getOrCreateAssetID(fiftyCentsOffer.Buying), fiftyCentsOffer)
	set = set.addPool(g.getOrCreateAssetID(usdAsset), g.poolFromEntry(eurUsdLiquidityPool))
	set = set.addPool(g.getOrCreateAssetID(eurAsset), g.poolFromEntry(eurUsdLiquidityPool))

	assert.Lenf(t, set, 2, "expected set to have 2 entries but got %v", set)
	assert.Equal(t, findByAsset(g, set, usdAsset.String()).pool.LiquidityPoolEntry, eurUsdLiquidityPool)
	assert.Equal(t, findByAsset(g, set, eurAsset.String()).pool.LiquidityPoolEntry, eurUsdLiquidityPool)

	assertOfferListEquals(t, findByAsset(g, set, usdAsset.String()).offers, []xdr.OfferEntry{
		quarterOffer,
		fiftyCentsOffer,
		dollarOffer,
	})

	assertOfferListEquals(t, findByAsset(g, set, eurAsset.String()).offers, []xdr.OfferEntry{
		eurOffer,
		twoEurOffer,
		threeEurOffer,
	})
}

func TestRemoveEdgeSet(t *testing.T) {
	set := edgeSet{}
	g := NewOrderBookGraph()

	var found bool
	set, found = set.removeOffer(g.getOrCreateAssetID(usdAsset), dollarOffer.OfferId)
	assert.Falsef(t, found, "expected set to not contain asset but is %v", set)

	set = set.addOffer(g.getOrCreateAssetID(dollarOffer.Buying), dollarOffer)
	set = set.addOffer(g.getOrCreateAssetID(eurOffer.Buying), eurOffer)
	set = set.addOffer(g.getOrCreateAssetID(twoEurOffer.Buying), twoEurOffer)
	set = set.addOffer(g.getOrCreateAssetID(threeEurOffer.Buying), threeEurOffer)
	set = set.addOffer(g.getOrCreateAssetID(quarterOffer.Buying), quarterOffer)
	set = set.addOffer(g.getOrCreateAssetID(fiftyCentsOffer.Buying), fiftyCentsOffer)
	set = set.addPool(g.getOrCreateAssetID(usdAsset), g.poolFromEntry(eurUsdLiquidityPool))

	set = set.removePool(g.getOrCreateAssetID(usdAsset))
	assert.Nil(t, findByAsset(g, set, usdAsset.String()).pool.Body.ConstantProduct)

	set, found = set.removeOffer(g.getOrCreateAssetID(usdAsset), dollarOffer.OfferId)
	assert.Truef(t, found, "expected set to contain dollar offer but is %v", set)
	set, found = set.removeOffer(g.getOrCreateAssetID(usdAsset), dollarOffer.OfferId)
	assert.Falsef(t, found, "expected set to not contain dollar offer after deletion but is %v", set)
	set, found = set.removeOffer(g.getOrCreateAssetID(eurAsset), threeEurOffer.OfferId)
	assert.Truef(t, found, "expected set to contain three euro offer but is %v", set)
	set, found = set.removeOffer(g.getOrCreateAssetID(eurAsset), eurOffer.OfferId)
	assert.Truef(t, found, "expected set to contain euro offer but is %v", set)
	set, found = set.removeOffer(g.getOrCreateAssetID(eurAsset), twoEurOffer.OfferId)
	assert.Truef(t, found, "expected set to contain two euro offer but is %v", set)
	set, found = set.removeOffer(g.getOrCreateAssetID(eurAsset), eurOffer.OfferId)
	assert.Falsef(t, found, "expected set to not contain euro offer after deletion but is %v", set)

	assert.Lenf(t, set, 1, "%v", set)

	assertOfferListEquals(t, findByAsset(g, set, usdAsset.String()).offers, []xdr.OfferEntry{
		quarterOffer,
		fiftyCentsOffer,
	})
}

func TestApplyOutdatedLedger(t *testing.T) {
	graph := NewOrderBookGraph()
	if graph.lastLedger != 0 {
		t.Fatalf("expected last ledger to be %v but got %v", 0, graph.lastLedger)
	}

	graph.AddOffers(fiftyCentsOffer)
	err := graph.Apply(2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if graph.lastLedger != 2 {
		t.Fatalf("expected last ledger to be %v but got %v", 2, graph.lastLedger)
	}

	graph.AddOffers(eurOffer)
	err = graph.Apply(1)
	if err != errUnexpectedLedger {
		t.Fatalf("expected error %v but got %v", errUnexpectedLedger, err)
	}
	if graph.lastLedger != 2 {
		t.Fatalf("expected last ledger to be %v but got %v", 2, graph.lastLedger)
	}

	graph.Discard()

	graph.AddOffers(eurOffer)
	err = graph.Apply(2)
	if err != errUnexpectedLedger {
		t.Fatalf("expected error %v but got %v", errUnexpectedLedger, err)
	}
	if graph.lastLedger != 2 {
		t.Fatalf("expected last ledger to be %v but got %v", 2, graph.lastLedger)
	}

	graph.Discard()

	err = graph.Apply(4)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if graph.lastLedger != 4 {
		t.Fatalf("expected last ledger to be %v but got %v", 4, graph.lastLedger)
	}
}

func TestAddOffersOrderBook(t *testing.T) {
	graph := NewOrderBookGraph()
	graph.AddOffers(dollarOffer, threeEurOffer, eurOffer, twoEurOffer,
		quarterOffer, fiftyCentsOffer)
	if !assert.NoError(t, graph.Apply(1)) ||
		!assert.EqualValues(t, 1, graph.lastLedger) {
		t.FailNow()
	}

	eurUsdOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(9),
		Buying:   eurAsset,
		Selling:  usdAsset,
		Price: xdr.Price{
			N: 1,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}
	otherEurUsdOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(10),
		Buying:   eurAsset,
		Selling:  usdAsset,
		Price: xdr.Price{
			N: 2,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}

	usdEurOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(11),
		Buying:   usdAsset,
		Selling:  eurAsset,
		Price: xdr.Price{
			N: 1,
			D: 3,
		},
		Amount: xdr.Int64(500),
	}

	graph.AddOffers(eurUsdOffer, otherEurUsdOffer, usdEurOffer)
	if !assert.NoError(t, graph.Apply(2)) ||
		!assert.EqualValues(t, 2, graph.lastLedger) {
		t.FailNow()
	}

	assetStringToID := map[string]int32{}
	idToAssetString := []string{}
	for i, asset := range []xdr.Asset{
		nativeAsset,
		usdAsset,
		eurAsset,
	} {
		assetStringToID[asset.String()] = int32(i)
		idToAssetString = append(idToAssetString, asset.String())
	}

	expectedGraph := &OrderBookGraph{
		assetStringToID: assetStringToID,
		idToAssetString: idToAssetString,
		venuesForSellingAsset: []edgeSet{
			{
				{
					assetStringToID[usdAsset.String()],
					makeVenues(quarterOffer, fiftyCentsOffer, dollarOffer),
					false,
				},
				{
					assetStringToID[eurAsset.String()],
					makeVenues(eurOffer, twoEurOffer, threeEurOffer),
					false,
				},
			},
			{
				{
					assetStringToID[eurAsset.String()],
					makeVenues(eurUsdOffer, otherEurUsdOffer),
					false,
				},
			},
			{
				{
					assetStringToID[usdAsset.String()],
					makeVenues(usdEurOffer),
					false,
				},
			},
		},
		venuesForBuyingAsset: []edgeSet{
			{},
			{
				{
					assetStringToID[eurAsset.String()],
					makeVenues(usdEurOffer),
					false,
				},
				{
					assetStringToID[nativeAsset.String()],
					makeVenues(quarterOffer, fiftyCentsOffer, dollarOffer),
					false,
				},
			},
			{
				{
					assetStringToID[usdAsset.String()],
					makeVenues(eurUsdOffer, otherEurUsdOffer),
					false,
				},
				{
					assetStringToID[nativeAsset.String()],
					makeVenues(eurOffer, twoEurOffer, threeEurOffer),
					false,
				},
			},
		},
		tradingPairForOffer: map[xdr.Int64]tradingPair{
			quarterOffer.OfferId:     makeTradingPair(assetStringToID, usdAsset, nativeAsset),
			fiftyCentsOffer.OfferId:  makeTradingPair(assetStringToID, usdAsset, nativeAsset),
			dollarOffer.OfferId:      makeTradingPair(assetStringToID, usdAsset, nativeAsset),
			eurOffer.OfferId:         makeTradingPair(assetStringToID, eurAsset, nativeAsset),
			twoEurOffer.OfferId:      makeTradingPair(assetStringToID, eurAsset, nativeAsset),
			threeEurOffer.OfferId:    makeTradingPair(assetStringToID, eurAsset, nativeAsset),
			eurUsdOffer.OfferId:      makeTradingPair(assetStringToID, eurAsset, usdAsset),
			otherEurUsdOffer.OfferId: makeTradingPair(assetStringToID, eurAsset, usdAsset),
			usdEurOffer.OfferId:      makeTradingPair(assetStringToID, usdAsset, eurAsset),
		},
	}

	// adding the same orders multiple times should have no effect
	graph.AddOffers(otherEurUsdOffer, usdEurOffer, dollarOffer, threeEurOffer)
	assert.NoError(t, graph.Apply(3))
	assert.EqualValues(t, 3, graph.lastLedger)

	assertGraphEquals(t, expectedGraph, graph)
}

func clonePool(entry xdr.LiquidityPoolEntry) xdr.LiquidityPoolEntry {
	clone := entry
	body := entry.Body.MustConstantProduct()
	clone.Body.ConstantProduct = &body
	return clone
}

func setupGraphWithLiquidityPools(t *testing.T) (*OrderBookGraph, []xdr.LiquidityPoolEntry) {
	graph := NewOrderBookGraph()
	graph.AddLiquidityPools(nativeEurPool, nativeUsdPool)
	if !assert.NoError(t, graph.Apply(1)) {
		t.FailNow()
	}

	expectedLiquidityPools := []xdr.LiquidityPoolEntry{nativeEurPool, nativeUsdPool}
	return graph, expectedLiquidityPools
}

func assertLiquidityPoolsEqual(t *testing.T, expectedLiquidityPools, liquidityPools []xdr.LiquidityPoolEntry) {
	sort.Slice(liquidityPools, func(i, j int) bool {
		return liquidityPools[i].Body.MustConstantProduct().Params.AssetB.String() <
			liquidityPools[j].Body.MustConstantProduct().Params.AssetB.String()
	})

	if !assert.Equal(t, len(expectedLiquidityPools), len(liquidityPools)) {
		t.FailNow()
	}

	for i, expected := range expectedLiquidityPools {
		liquidityPool := liquidityPools[i]
		liquidityPoolBase64, err := xdr.MarshalBase64(liquidityPool)
		assert.NoError(t, err)

		expectedBase64, err := xdr.MarshalBase64(expected)
		assert.NoError(t, err)

		assert.Equalf(t, expectedBase64, liquidityPoolBase64,
			"pool mismatch: %v != %v", expected, liquidityPool)
	}
}

func TestAddLiquidityPools(t *testing.T) {
	graph, expectedLiquidityPools := setupGraphWithLiquidityPools(t)
	assertLiquidityPoolsEqual(t, expectedLiquidityPools, graph.LiquidityPools())
}

func TestUpdateLiquidityPools(t *testing.T) {
	graph, expectedLiquidityPools := setupGraphWithLiquidityPools(t)
	p0 := clonePool(expectedLiquidityPools[0])
	p1 := clonePool(expectedLiquidityPools[1])
	p0.Body.ConstantProduct.ReserveA += 100
	p1.Body.ConstantProduct.ReserveB -= 2
	expectedLiquidityPools[0] = p0
	expectedLiquidityPools[1] = p1

	graph.AddLiquidityPools(expectedLiquidityPools[:2]...)
	if !assert.NoError(t, graph.Apply(2)) {
		t.FailNow()
	}

	assertLiquidityPoolsEqual(t, expectedLiquidityPools, graph.LiquidityPools())
}

func TestRemoveLiquidityPools(t *testing.T) {
	graph, expectedLiquidityPools := setupGraphWithLiquidityPools(t)
	p0 := clonePool(expectedLiquidityPools[0])
	p0.Body.ConstantProduct.ReserveA += 100
	expectedLiquidityPools[0] = p0

	graph.AddLiquidityPools(expectedLiquidityPools[0])
	graph.RemoveLiquidityPool(expectedLiquidityPools[1])

	if !assert.NoError(t, graph.Apply(2)) {
		t.FailNow()
	}

	assertLiquidityPoolsEqual(t, expectedLiquidityPools[:1], graph.LiquidityPools())
}

func TestUpdateOfferOrderBook(t *testing.T) {
	graph := NewOrderBookGraph()

	if !graph.IsEmpty() {
		t.Fatal("expected graph to be empty")
	}

	graph.AddOffers(dollarOffer, threeEurOffer, eurOffer, twoEurOffer,
		quarterOffer, fiftyCentsOffer)
	err := graph.Apply(1)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if graph.lastLedger != 1 {
		t.Fatalf("expected last ledger to be %v but got %v", 1, graph.lastLedger)
	}

	if graph.IsEmpty() {
		t.Fatal("expected graph to not be empty")
	}

	eurUsdOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(9),
		Buying:   eurAsset,
		Selling:  usdAsset,
		Price: xdr.Price{
			N: 1,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}
	otherEurUsdOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(10),
		Buying:   eurAsset,
		Selling:  usdAsset,
		Price: xdr.Price{
			N: 2,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}

	usdEurOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(11),
		Buying:   usdAsset,
		Selling:  eurAsset,
		Price: xdr.Price{
			N: 1,
			D: 3,
		},
		Amount: xdr.Int64(500),
	}

	graph.AddOffers(eurUsdOffer, otherEurUsdOffer, usdEurOffer)
	err = graph.Apply(2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if graph.lastLedger != 2 {
		t.Fatalf("expected last ledger to be %v but got %v", 2, graph.lastLedger)
	}

	usdEurOffer.Price.N = 4
	usdEurOffer.Price.D = 1

	otherEurUsdOffer.Price.N = 1
	otherEurUsdOffer.Price.D = 2

	dollarOffer.Amount = 12

	graph.AddOffers(usdEurOffer, otherEurUsdOffer, dollarOffer)
	err = graph.Apply(3)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if graph.lastLedger != 3 {
		t.Fatalf("expected last ledger to be %v but got %v", 3, graph.lastLedger)
	}

	assetStringToID := map[string]int32{}
	idToAssetString := []string{}
	for i, asset := range []xdr.Asset{
		nativeAsset,
		usdAsset,
		eurAsset,
	} {
		assetStringToID[asset.String()] = int32(i)
		idToAssetString = append(idToAssetString, asset.String())
	}
	expectedGraph := &OrderBookGraph{
		idToAssetString: idToAssetString,
		assetStringToID: assetStringToID,
		venuesForSellingAsset: []edgeSet{
			{
				{
					assetStringToID[usdAsset.String()],
					makeVenues(quarterOffer, fiftyCentsOffer, dollarOffer),
					false,
				},
				{
					assetStringToID[eurAsset.String()],
					makeVenues(eurOffer, twoEurOffer, threeEurOffer),
					false,
				},
			},
			{
				{
					assetStringToID[eurAsset.String()],
					makeVenues(otherEurUsdOffer, eurUsdOffer),
					false,
				},
			},
			{
				{
					assetStringToID[usdAsset.String()],
					makeVenues(usdEurOffer),
					false,
				},
			},
		},
		venuesForBuyingAsset: []edgeSet{
			{},
			{
				{
					assetStringToID[nativeAsset.String()],
					makeVenues(quarterOffer, fiftyCentsOffer, dollarOffer),
					false,
				},
				{
					assetStringToID[eurAsset.String()],
					makeVenues(usdEurOffer),
					false,
				},
			},
			{
				{
					assetStringToID[nativeAsset.String()],
					makeVenues(eurOffer, twoEurOffer, threeEurOffer),
					false,
				},
				{
					assetStringToID[usdAsset.String()],
					makeVenues(otherEurUsdOffer, eurUsdOffer),
					false,
				},
			},
		},
		tradingPairForOffer: map[xdr.Int64]tradingPair{
			quarterOffer.OfferId:     makeTradingPair(assetStringToID, usdAsset, nativeAsset),
			fiftyCentsOffer.OfferId:  makeTradingPair(assetStringToID, usdAsset, nativeAsset),
			dollarOffer.OfferId:      makeTradingPair(assetStringToID, usdAsset, nativeAsset),
			eurOffer.OfferId:         makeTradingPair(assetStringToID, eurAsset, nativeAsset),
			twoEurOffer.OfferId:      makeTradingPair(assetStringToID, eurAsset, nativeAsset),
			threeEurOffer.OfferId:    makeTradingPair(assetStringToID, eurAsset, nativeAsset),
			eurUsdOffer.OfferId:      makeTradingPair(assetStringToID, eurAsset, usdAsset),
			otherEurUsdOffer.OfferId: makeTradingPair(assetStringToID, eurAsset, usdAsset),
			usdEurOffer.OfferId:      makeTradingPair(assetStringToID, usdAsset, eurAsset),
		},
	}

	assertGraphEquals(t, expectedGraph, graph)
}

func TestDiscard(t *testing.T) {
	graph := NewOrderBookGraph()

	graph.AddOffers(dollarOffer, threeEurOffer, eurOffer, twoEurOffer,
		quarterOffer, fiftyCentsOffer)
	graph.Discard()
	if graph.lastLedger != 0 {
		t.Fatalf("expected last ledger to be %v but got %v", 0, graph.lastLedger)
	}

	if err := graph.Apply(1); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !graph.IsEmpty() {
		t.Fatal("expected graph to be empty")
	}
	if graph.lastLedger != 1 {
		t.Fatalf("expected last ledger to be %v but got %v", 1, graph.lastLedger)
	}

	graph.AddOffers(dollarOffer)
	err := graph.Apply(2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if graph.IsEmpty() {
		t.Fatal("expected graph to be not empty")
	}
	if graph.lastLedger != 2 {
		t.Fatalf("expected last ledger to be %v but got %v", 2, graph.lastLedger)
	}

	expectedOffers := []xdr.OfferEntry{dollarOffer}
	assertOfferListEquals(t, graph.Offers(), expectedOffers)

	graph.AddOffers(threeEurOffer)
	graph.Discard()
	assertOfferListEquals(t, graph.Offers(), expectedOffers)
}

func TestRemoveOfferOrderBook(t *testing.T) {
	graph := NewOrderBookGraph()

	graph.AddOffers(dollarOffer, threeEurOffer, eurOffer, twoEurOffer,
		quarterOffer, fiftyCentsOffer)
	if !assert.NoError(t, graph.Apply(1)) ||
		!assert.EqualValues(t, 1, graph.lastLedger) {
		t.FailNow()
	}

	eurUsdOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(9),
		Buying:   eurAsset,
		Selling:  usdAsset,
		Price: xdr.Price{
			N: 1,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}
	otherEurUsdOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(10),
		Buying:   eurAsset,
		Selling:  usdAsset,
		Price: xdr.Price{
			N: 2,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}

	usdEurOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(11),
		Buying:   usdAsset,
		Selling:  eurAsset,
		Price: xdr.Price{
			N: 1,
			D: 3,
		},
		Amount: xdr.Int64(500),
	}

	graph.AddOffers(eurUsdOffer, otherEurUsdOffer, usdEurOffer)
	graph.RemoveOffer(usdEurOffer.OfferId)
	graph.RemoveOffer(otherEurUsdOffer.OfferId)
	graph.RemoveOffer(dollarOffer.OfferId)

	if !assert.NoError(t, graph.Apply(2)) ||
		!assert.EqualValues(t, 2, graph.lastLedger) {
		t.FailNow()
	}

	assetStringToID := map[string]int32{}
	idToAssetString := []string{}
	for i, asset := range []xdr.Asset{
		nativeAsset,
		usdAsset,
		eurAsset,
	} {
		assetStringToID[asset.String()] = int32(i)
		idToAssetString = append(idToAssetString, asset.String())
	}
	expectedGraph := &OrderBookGraph{
		idToAssetString: idToAssetString,
		assetStringToID: assetStringToID,
		venuesForSellingAsset: []edgeSet{
			{
				{
					assetStringToID[usdAsset.String()],
					makeVenues(quarterOffer, fiftyCentsOffer),
					false,
				},
				{
					assetStringToID[eurAsset.String()],
					makeVenues(eurOffer, twoEurOffer, threeEurOffer),
					false,
				},
			},
			{
				{
					assetStringToID[eurAsset.String()],
					makeVenues(eurUsdOffer),
					false,
				},
			},
			{},
		},
		venuesForBuyingAsset: []edgeSet{
			{},
			{
				{
					assetStringToID[nativeAsset.String()],
					makeVenues(quarterOffer, fiftyCentsOffer),
					false,
				},
			},
			{
				{
					assetStringToID[nativeAsset.String()],
					makeVenues(eurOffer, twoEurOffer, threeEurOffer),
					false,
				},
				{
					assetStringToID[usdAsset.String()],
					makeVenues(eurUsdOffer),
					false,
				},
			},
		},
		tradingPairForOffer: map[xdr.Int64]tradingPair{
			quarterOffer.OfferId:    makeTradingPair(assetStringToID, usdAsset, nativeAsset),
			fiftyCentsOffer.OfferId: makeTradingPair(assetStringToID, usdAsset, nativeAsset),
			eurOffer.OfferId:        makeTradingPair(assetStringToID, eurAsset, nativeAsset),
			twoEurOffer.OfferId:     makeTradingPair(assetStringToID, eurAsset, nativeAsset),
			threeEurOffer.OfferId:   makeTradingPair(assetStringToID, eurAsset, nativeAsset),
			eurUsdOffer.OfferId:     makeTradingPair(assetStringToID, eurAsset, usdAsset),
		},
	}

	assertGraphEquals(t, expectedGraph, graph)

	graph.
		RemoveOffer(quarterOffer.OfferId).
		RemoveOffer(fiftyCentsOffer.OfferId).
		RemoveOffer(eurOffer.OfferId).
		RemoveOffer(twoEurOffer.OfferId).
		RemoveOffer(threeEurOffer.OfferId).
		RemoveOffer(eurUsdOffer.OfferId)

	assert.NoError(t, graph.Apply(3))
	assert.EqualValues(t, 3, graph.lastLedger)

	// Skip over offer ids which are not present in the graph
	assert.NoError(t, graph.RemoveOffer(988888).Apply(4))

	expectedGraph.Clear()
	assertGraphEquals(t, expectedGraph, graph)
	assert.True(t, graph.IsEmpty())
}

func TestConsumeOffersForSellingAsset(t *testing.T) {
	kp := keypair.MustRandom()
	ignoreOffersFrom := xdr.MustAddress(kp.Address())
	otherSellerTwoEurOffer := twoEurOffer
	otherSellerTwoEurOffer.SellerId = ignoreOffersFrom

	denominatorZeroOffer := twoEurOffer
	denominatorZeroOffer.Price.D = 0

	overflowOffer := twoEurOffer
	overflowOffer.Amount = math.MaxInt64
	overflowOffer.Price.N = math.MaxInt32
	overflowOffer.Price.D = 1

	for _, testCase := range []struct {
		name               string
		offers             []xdr.OfferEntry
		ignoreOffersFrom   *xdr.AccountId
		currentAssetAmount xdr.Int64
		result             xdr.Int64
		err                error
	}{
		{
			"offers must not be empty",
			[]xdr.OfferEntry{},
			&issuer,
			100,
			0,
			errEmptyOffers,
		},
		{
			"currentAssetAmount must be positive",
			[]xdr.OfferEntry{eurOffer},
			&ignoreOffersFrom,
			0,
			0,
			errAssetAmountIsZero,
		},
		{
			"ignore all offers",
			[]xdr.OfferEntry{eurOffer},
			&issuer,
			1,
			-1,
			nil,
		},
		{
			"offer denominator cannot be zero",
			[]xdr.OfferEntry{denominatorZeroOffer},
			&ignoreOffersFrom,
			10000,
			0,
			price.ErrDivisionByZero,
		},
		{
			"ignore some offers",
			[]xdr.OfferEntry{eurOffer, otherSellerTwoEurOffer},
			&issuer,
			100,
			200,
			nil,
		},
		{
			"ignore overflow offers",
			[]xdr.OfferEntry{overflowOffer},
			nil,
			math.MaxInt64,
			-1,
			nil,
		},
		{
			"not enough offers to consume",
			[]xdr.OfferEntry{eurOffer, twoEurOffer},
			nil,
			1001,
			-1,
			nil,
		},
		{
			"consume all offers",
			[]xdr.OfferEntry{eurOffer, twoEurOffer, threeEurOffer},
			nil,
			1500,
			3000,
			nil,
		},
		{
			"consume offer partially",
			[]xdr.OfferEntry{eurOffer, twoEurOffer},
			nil,
			2,
			2,
			nil,
		},
		{
			"round up",
			[]xdr.OfferEntry{quarterOffer},
			nil,
			5,
			2,
			nil,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := consumeOffersForSellingAsset(
				testCase.offers,
				testCase.ignoreOffersFrom,
				testCase.currentAssetAmount,
				0,
			)
			if err != testCase.err {
				t.Fatalf("expected error %v but got %v", testCase.err, err)
			}
			if err == nil {
				if result != testCase.result {
					t.Fatalf("expected %v but got %v", testCase.result, result)
				}
			}
		})
	}

}

func TestConsumeOffersForBuyingAsset(t *testing.T) {
	kp := keypair.MustRandom()
	ignoreOffersFrom := xdr.MustAddress(kp.Address())
	otherSellerTwoEurOffer := twoEurOffer
	otherSellerTwoEurOffer.SellerId = ignoreOffersFrom

	denominatorZeroOffer := twoEurOffer
	denominatorZeroOffer.Price.D = 0

	overflowOffer := twoEurOffer
	overflowOffer.Price.N = 1
	overflowOffer.Price.D = math.MaxInt32

	for _, testCase := range []struct {
		name               string
		offers             []xdr.OfferEntry
		currentAssetAmount xdr.Int64
		result             xdr.Int64
		err                error
	}{
		{
			"offers must not be empty",
			[]xdr.OfferEntry{},
			100,
			0,
			errEmptyOffers,
		},
		{
			"currentAssetAmount must be positive",
			[]xdr.OfferEntry{eurOffer},
			0,
			0,
			errAssetAmountIsZero,
		},
		{
			"offer denominator cannot be zero",
			[]xdr.OfferEntry{denominatorZeroOffer},
			10000,
			-1,
			nil,
		},
		{
			"balance too low to consume offers",
			[]xdr.OfferEntry{twoEurOffer},
			1,
			-1,
			nil,
		},
		{
			"not enough offers to consume",
			[]xdr.OfferEntry{eurOffer, twoEurOffer},
			1502,
			-1,
			nil,
		},
		{
			"ignore overflow offers",
			[]xdr.OfferEntry{overflowOffer},
			math.MaxInt64,
			-1,
			nil,
		},
		{
			"consume all offers",
			[]xdr.OfferEntry{eurOffer, twoEurOffer, threeEurOffer},
			3000,
			1500,
			nil,
		},
		{
			"consume offer partially",
			[]xdr.OfferEntry{eurOffer, twoEurOffer},
			2,
			2,
			nil,
		},
		{
			"round down",
			[]xdr.OfferEntry{eurOffer, twoEurOffer},
			1501,
			1000,
			nil,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := consumeOffersForBuyingAsset(
				testCase.offers,
				testCase.currentAssetAmount,
			)
			assert.Equal(t, testCase.err, err)
			if err == nil {
				assert.Equal(t, testCase.result, result)
			}
		})
	}

}

func TestSortAndFilterPathsBySourceAsset(t *testing.T) {
	allPaths := []Path{
		{
			SourceAmount:      3,
			SourceAsset:       eurAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 1000,
		},
		{
			SourceAmount:      4,
			SourceAsset:       eurAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 1000,
		},
		{
			SourceAmount:      1,
			SourceAsset:       usdAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 1000,
		},
		{
			SourceAmount:      2,
			SourceAsset:       eurAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 1000,
		},
		{
			SourceAmount: 2,
			SourceAsset:  eurAsset.String(),
			InteriorNodes: []string{
				nativeAsset.String(),
			},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 1000,
		},
		{
			SourceAmount:      10,
			SourceAsset:       nativeAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 1000,
		},
	}
	sortedAndFiltered, err := sortAndFilterPaths(
		allPaths,
		3,
		sortBySourceAsset,
	)
	assert.NoError(t, err)

	expectedPaths := []Path{
		{
			SourceAmount:      2,
			SourceAsset:       eurAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 1000,
		},
		{
			SourceAmount: 2,
			SourceAsset:  eurAsset.String(),
			InteriorNodes: []string{
				nativeAsset.String(),
			},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 1000,
		},
		{
			SourceAmount:      3,
			SourceAsset:       eurAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 1000,
		},
		{
			SourceAmount:      1,
			SourceAsset:       usdAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 1000,
		},
		{
			SourceAmount:      10,
			SourceAsset:       nativeAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 1000,
		},
	}

	assertPathEquals(t, sortedAndFiltered, expectedPaths)
}

func TestSortAndFilterPathsByDestinationAsset(t *testing.T) {
	allPaths := []Path{
		{
			SourceAmount:      1000,
			SourceAsset:       yenAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  eurAsset.String(),
			DestinationAmount: 3,
		},
		{
			SourceAmount:      1000,
			SourceAsset:       yenAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  eurAsset.String(),
			DestinationAmount: 4,
		},
		{
			SourceAmount:      1000,
			SourceAsset:       yenAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  usdAsset.String(),
			DestinationAmount: 1,
		},
		{
			SourceAmount:      1000,
			SourceAsset:       yenAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  eurAsset.String(),
			DestinationAmount: 2,
		},
		{
			SourceAmount: 1000,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				nativeAsset.String(),
			},
			DestinationAsset:  eurAsset.String(),
			DestinationAmount: 2,
		},
		{
			SourceAmount:      1000,
			SourceAsset:       yenAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 10,
		},
	}
	sortedAndFiltered, err := sortAndFilterPaths(
		allPaths,
		3,
		sortByDestinationAsset,
	)
	assert.NoError(t, err)

	expectedPaths := []Path{
		{
			SourceAmount:      1000,
			SourceAsset:       yenAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  eurAsset.String(),
			DestinationAmount: 4,
		},
		{
			SourceAmount:      1000,
			SourceAsset:       yenAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  eurAsset.String(),
			DestinationAmount: 3,
		},
		{
			SourceAmount:      1000,
			SourceAsset:       yenAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  eurAsset.String(),
			DestinationAmount: 2,
		},
		{
			SourceAmount:      1000,
			SourceAsset:       yenAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  usdAsset.String(),
			DestinationAmount: 1,
		},
		{
			SourceAmount:      1000,
			SourceAsset:       yenAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 10,
		},
	}

	assertPathEquals(t, sortedAndFiltered, expectedPaths)
}

func TestFindPaths(t *testing.T) {
	graph := NewOrderBookGraph()

	graph.AddOffers(dollarOffer, threeEurOffer, eurOffer, twoEurOffer,
		quarterOffer, fiftyCentsOffer)
	if !assert.NoError(t, graph.Apply(1)) {
		t.FailNow()
	}

	eurUsdOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(9),
		Buying:   eurAsset,
		Selling:  usdAsset,
		Price: xdr.Price{
			N: 1,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}
	otherEurUsdOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(10),
		Buying:   eurAsset,
		Selling:  usdAsset,
		Price: xdr.Price{
			N: 2,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}

	usdEurOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(11),
		Buying:   usdAsset,
		Selling:  eurAsset,
		Price: xdr.Price{
			N: 1,
			D: 3,
		},
		Amount: xdr.Int64(500),
	}

	chfEurOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(12),
		Buying:   chfAsset,
		Selling:  eurAsset,
		Price: xdr.Price{
			N: 1,
			D: 2,
		},
		Amount: xdr.Int64(500),
	}

	yenChfOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(13),
		Buying:   yenAsset,
		Selling:  chfAsset,
		Price: xdr.Price{
			N: 1,
			D: 2,
		},
		Amount: xdr.Int64(500),
	}

	graph.AddOffers(eurUsdOffer, otherEurUsdOffer, usdEurOffer, chfEurOffer, yenChfOffer)
	if !assert.NoError(t, graph.Apply(2)) {
		t.FailNow()
	}

	kp := keypair.MustRandom()
	ignoreOffersFrom := xdr.MustAddress(kp.Address())

	paths, lastLedger, err := graph.FindPaths(
		context.TODO(),
		3,
		nativeAsset,
		20,
		&ignoreOffersFrom,
		[]xdr.Asset{
			yenAsset,
			usdAsset,
		},
		[]xdr.Int64{
			0,
			0,
		},
		true,
		5,
		true,
	)
	assert.NoError(t, err)
	assertPathEquals(t, paths, []Path{})
	assert.EqualValues(t, 2, lastLedger)

	paths, lastLedger, err = graph.FindPaths(
		context.TODO(),
		3,
		nativeAsset,
		20,
		&ignoreOffersFrom,
		[]xdr.Asset{
			yenAsset,
			usdAsset,
		},
		[]xdr.Int64{
			100000,
			60000,
		},
		true,
		5,
		true,
	)

	expectedPaths := []Path{
		{
			SourceAmount:      5,
			SourceAsset:       usdAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
		{
			SourceAmount: 5,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				eurAsset.String(),
				chfAsset.String(),
			},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
	}

	assert.NoError(t, err)
	assert.EqualValues(t, 2, lastLedger)
	assertPathEquals(t, paths, expectedPaths)

	paths, lastLedger, err = graph.FindPaths(
		context.TODO(),
		3,
		nativeAsset,
		20,
		&ignoreOffersFrom,
		[]xdr.Asset{
			yenAsset,
			usdAsset,
		},
		[]xdr.Int64{
			0,
			0,
		},
		false,
		5,
		true,
	)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, lastLedger)
	assertPathEquals(t, paths, expectedPaths)

	paths, lastLedger, err = graph.FindPaths(
		context.TODO(),
		4,
		nativeAsset,
		20,
		&ignoreOffersFrom,
		[]xdr.Asset{
			yenAsset,
			usdAsset,
		},
		[]xdr.Int64{
			100000,
			60000,
		},
		true,
		5,
		true,
	)

	expectedPaths = []Path{
		{
			SourceAmount:      5,
			SourceAsset:       usdAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
		{
			SourceAmount: 2,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				usdAsset.String(),
				eurAsset.String(),
				chfAsset.String(),
			},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
		{
			SourceAmount: 5,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				eurAsset.String(),
				chfAsset.String(),
			},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
	}

	assert.NoError(t, err)
	assert.EqualValues(t, 2, lastLedger)
	assertPathEquals(t, paths, expectedPaths)

	paths, lastLedger, err = graph.FindPaths(
		context.TODO(),
		4,
		nativeAsset,
		20,
		&ignoreOffersFrom,
		[]xdr.Asset{
			yenAsset,
			usdAsset,
			nativeAsset,
		},
		[]xdr.Int64{
			100000,
			60000,
			100000,
		},
		true,
		5,
		true,
	)

	expectedPaths = []Path{
		{
			SourceAmount:      5,
			SourceAsset:       usdAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
		{
			SourceAmount: 2,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				usdAsset.String(),
				eurAsset.String(),
				chfAsset.String(),
			},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
		{
			SourceAmount: 5,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				eurAsset.String(),
				chfAsset.String(),
			},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
		// include the empty path where xlm is transferred without any
		// conversions
		{
			SourceAmount:      20,
			SourceAsset:       nativeAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
	}

	assert.NoError(t, err)
	assert.EqualValues(t, 2, lastLedger)
	assertPathEquals(t, paths, expectedPaths)

	t.Run("find paths starting from non-existent asset", func(t *testing.T) {
		paths, lastLedger, err = graph.FindPaths(
			context.TODO(),
			4,
			xdr.MustNewCreditAsset("DNE", yenAsset.GetIssuer()),
			20,
			&ignoreOffersFrom,
			[]xdr.Asset{
				yenAsset,
				usdAsset,
			},
			[]xdr.Int64{
				100000,
				60000,
			},
			false,
			5,
			true,
		)
		assert.NoError(t, err)
		assert.EqualValues(t, 2, lastLedger)
		assert.Len(t, paths, 0)
	})

	t.Run("find paths ending at non-existent assets", func(t *testing.T) {
		paths, lastLedger, err = graph.FindPaths(
			context.TODO(),
			4,
			usdAsset,
			20,
			&ignoreOffersFrom,
			[]xdr.Asset{
				xdr.MustNewCreditAsset("DNE", yenAsset.GetIssuer()),
			},
			[]xdr.Int64{
				1000000000,
			},
			false,
			5,
			true,
		)
		assert.NoError(t, err)
		assert.EqualValues(t, 2, lastLedger)
		assert.Len(t, paths, 0)
	})
}

func TestFindPathsStartingAt(t *testing.T) {
	graph := NewOrderBookGraph()

	graph.AddOffers(dollarOffer, threeEurOffer, eurOffer, twoEurOffer,
		quarterOffer, fiftyCentsOffer)

	err := graph.Apply(1)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	eurUsdOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(9),
		Buying:   eurAsset,
		Selling:  usdAsset,
		Price: xdr.Price{
			N: 1,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}
	otherEurUsdOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(10),
		Buying:   eurAsset,
		Selling:  usdAsset,
		Price: xdr.Price{
			N: 2,
			D: 1,
		},
		Amount: xdr.Int64(500),
	}

	usdEurOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(11),
		Buying:   usdAsset,
		Selling:  eurAsset,
		Price: xdr.Price{
			N: 1,
			D: 3,
		},
		Amount: xdr.Int64(500),
	}

	chfEurOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(12),
		Buying:   chfAsset,
		Selling:  eurAsset,
		Price: xdr.Price{
			N: 1,
			D: 2,
		},
		Amount: xdr.Int64(500),
	}

	yenChfOffer := xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(13),
		Buying:   yenAsset,
		Selling:  chfAsset,
		Price: xdr.Price{
			N: 1,
			D: 2,
		},
		Amount: xdr.Int64(500),
	}

	graph.AddOffers(eurUsdOffer, otherEurUsdOffer, usdEurOffer, chfEurOffer, yenChfOffer)
	err = graph.Apply(2)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	paths, lastLedger, err := graph.FindFixedPaths(
		context.TODO(),
		3,
		usdAsset,
		5,
		[]xdr.Asset{nativeAsset},
		5,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if lastLedger != 2 {
		t.Fatalf("expected last ledger to be %v but got %v", 2, lastLedger)
	}

	expectedPaths := []Path{
		{
			SourceAmount:      5,
			SourceAsset:       usdAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
	}

	assertPathEquals(t, paths, expectedPaths)

	paths, lastLedger, err = graph.FindFixedPaths(
		context.TODO(),
		2,
		yenAsset,
		5,
		[]xdr.Asset{nativeAsset},
		5,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if lastLedger != 2 {
		t.Fatalf("expected last ledger to be %v but got %v", 2, lastLedger)
	}

	expectedPaths = []Path{}

	assertPathEquals(t, paths, expectedPaths)

	paths, lastLedger, err = graph.FindFixedPaths(
		context.TODO(),
		3,
		yenAsset,
		5,
		[]xdr.Asset{nativeAsset},
		5,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if lastLedger != 2 {
		t.Fatalf("expected last ledger to be %v but got %v", 2, lastLedger)
	}

	expectedPaths = []Path{
		{
			SourceAmount: 5,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				chfAsset.String(),
				eurAsset.String(),
			},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
	}

	assertPathEquals(t, paths, expectedPaths)

	paths, lastLedger, err = graph.FindFixedPaths(
		context.TODO(),
		5,
		yenAsset,
		5,
		[]xdr.Asset{nativeAsset},
		5,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if lastLedger != 2 {
		t.Fatalf("expected last ledger to be %v but got %v", 2, lastLedger)
	}

	expectedPaths = []Path{
		{
			SourceAmount: 5,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				chfAsset.String(),
				eurAsset.String(),
				usdAsset.String(),
			},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 80,
		},
		{
			SourceAmount: 5,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				chfAsset.String(),
				eurAsset.String(),
			},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
	}

	assertPathEquals(t, paths, expectedPaths)

	paths, lastLedger, err = graph.FindFixedPaths(
		context.TODO(),
		5,
		yenAsset,
		5,
		[]xdr.Asset{nativeAsset, usdAsset, yenAsset},
		5,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if lastLedger != 2 {
		t.Fatalf("expected last ledger to be %v but got %v", 2, lastLedger)
	}

	expectedPaths = []Path{
		{
			SourceAmount: 5,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				chfAsset.String(),
				eurAsset.String(),
			},
			DestinationAsset:  usdAsset.String(),
			DestinationAmount: 20,
		},
		// include the empty path where yen is transferred without any
		// conversions
		{
			SourceAmount:      5,
			SourceAsset:       yenAsset.String(),
			InteriorNodes:     []string{},
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 5,
		},
		{
			SourceAmount: 5,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				chfAsset.String(),
				eurAsset.String(),
				usdAsset.String(),
			},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 80,
		},
		{
			SourceAmount: 5,
			SourceAsset:  yenAsset.String(),
			InteriorNodes: []string{
				chfAsset.String(),
				eurAsset.String(),
			},
			DestinationAsset:  nativeAsset.String(),
			DestinationAmount: 20,
		},
	}
	assertPathEquals(t, paths, expectedPaths)

	t.Run("find fixed paths starting from non-existent asset", func(t *testing.T) {
		paths, lastLedger, err = graph.FindFixedPaths(
			context.TODO(),
			5,
			xdr.MustNewCreditAsset("DNE", yenAsset.GetIssuer()),
			5,
			[]xdr.Asset{nativeAsset, usdAsset},
			5,
			true,
		)

		assert.NoError(t, err)
		assert.EqualValues(t, 2, lastLedger)
		assert.Len(t, paths, 0)
	})

	t.Run("find fixed paths ending at non-existent assets", func(t *testing.T) {
		paths, lastLedger, err = graph.FindFixedPaths(
			context.TODO(),
			5,
			usdAsset,
			5,
			[]xdr.Asset{xdr.MustNewCreditAsset("DNE", yenAsset.GetIssuer())},
			5,
			true,
		)

		assert.NoError(t, err)
		assert.EqualValues(t, 2, lastLedger)
		assert.Len(t, paths, 0)
	})
}

func TestPathThroughLiquidityPools(t *testing.T) {
	graph := NewOrderBookGraph()
	graph.AddLiquidityPools(eurUsdLiquidityPool)
	graph.AddLiquidityPools(eurYenLiquidityPool)
	graph.AddLiquidityPools(usdChfLiquidityPool)
	if !assert.NoErrorf(t, graph.Apply(1), "applying LPs to graph failed") {
		t.FailNow()
	}

	kp := keypair.MustRandom()
	fakeSource := xdr.MustAddress(kp.Address())

	t.Run("happy path", func(t *testing.T) {
		paths, _, err := graph.FindPaths(
			context.TODO(),
			5,           // more than enough hops
			yenAsset,    // path should go USD -> EUR -> Yen
			100,         // less than LP reserves for either pool
			&fakeSource, // fake source account to ignore pools from
			[]xdr.Asset{usdAsset},
			[]xdr.Int64{127}, // we only exactly the right amount of $ to trade
			true,
			5, // irrelevant
			true,
		)

		// The path should go USD -> EUR -> Yen, jumping through both liquidity
		// pools. For a payout of 100 Yen from the EUR/Yen pool, we need to
		// exchange 112 Euros. To get 112 EUR, we need to exchange 127 USD.
		expectedPaths := []Path{
			{
				SourceAsset:       usdAsset.String(),
				SourceAmount:      127,
				DestinationAsset:  yenAsset.String(),
				DestinationAmount: 100,
				InteriorNodes:     []string{eurAsset.String()},
			},
		}

		assert.NoError(t, err)
		assertPathEquals(t, expectedPaths, paths)
	})

	t.Run("exclude pools", func(t *testing.T) {
		paths, _, err := graph.FindPaths(
			context.TODO(),
			5,           // more than enough hops
			yenAsset,    // path should go USD -> EUR -> Yen
			100,         // less than LP reserves for either pool
			&fakeSource, // fake source account to ignore pools from
			[]xdr.Asset{usdAsset},
			[]xdr.Int64{127}, // we only exactly the right amount of $ to trade
			true,
			5, // irrelevant
			false,
		)

		assert.NoError(t, err)
		assert.Empty(t, paths)
	})

	t.Run("not enough source balance", func(t *testing.T) {
		paths, _, err := graph.FindPaths(context.TODO(),
			5, yenAsset, 100, &fakeSource, []xdr.Asset{usdAsset},
			[]xdr.Int64{126}, // the only change: we're short on balance now
			true, 5,
			true,
		)

		assert.NoError(t, err)
		assertPathEquals(t, []Path{}, paths)
	})

	t.Run("more hops", func(t *testing.T) {
		// The conversion rate is different this time: one more more hop means
		// one more exchange rate to deal with.
		paths, _, err := graph.FindPaths(context.TODO(),
			5,
			yenAsset, // different path: CHF -> USD -> EUR -> Yen
			100,
			&fakeSource,
			[]xdr.Asset{chfAsset},
			[]xdr.Int64{73},
			true,
			5,
			true,
		)

		expectedPaths := []Path{{
			SourceAsset:       chfAsset.String(),
			SourceAmount:      73,
			DestinationAsset:  yenAsset.String(),
			DestinationAmount: 100,
			InteriorNodes:     []string{usdAsset.String(), eurAsset.String()},
		}}

		assert.NoError(t, err)
		assertPathEquals(t, expectedPaths, paths)
	})
}

func TestInterleavedPaths(t *testing.T) {
	graph := NewOrderBookGraph()
	graph.AddLiquidityPools(nativeUsdPool, eurUsdLiquidityPool, usdChfLiquidityPool)
	if !assert.NoError(t, graph.Apply(1)) {
		t.FailNow()
	}

	graph.AddOffers(xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(42),
		Selling:  nativeAsset,
		Buying:   eurAsset,
		Amount:   100,
		Price:    xdr.Price{1, 1},
	}, xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(43),
		Selling:  chfAsset,
		Buying:   usdAsset,
		Amount:   1,
		Price:    xdr.Price{1, 1},
	})
	if !assert.NoError(t, graph.Apply(2)) {
		t.FailNow()
	}

	kp := keypair.MustRandom()
	fakeSource := xdr.MustAddress(kp.Address())

	// The final graph looks like the following:
	//
	//  - XLM: Offer 100 for 1 EUR each
	//         LP        for USD, 50:1
	//
	//  - EUR: LP for USD, 1:1
	//
	//  - USD: LP for EUR, 1:1
	//         LP for XLM, 1:4
	//         LP for CHF, 2:1
	//
	//  - CHF: Offer 1 for 4 USD each
	//              LP for USD, 1:2

	paths, _, err := graph.FindPaths(context.TODO(),
		5,
		nativeAsset,
		100,
		&fakeSource,
		[]xdr.Asset{chfAsset},
		[]xdr.Int64{1000},
		true,
		5,
		true,
	)

	// There should be two paths: one that consumes the EUR/XLM offers and one
	// that goes through the USD/XLM liquidity pool.
	//
	// If we take up the offers, it's very efficient:
	//   64 CHF for 166 USD for 142 EUR for 100 XLM
	//
	// If we only go through pools, it's less-so:
	//   90 CHF for 152 USD for 100 XLM
	expectedPaths := []Path{{
		SourceAsset:       chfAsset.String(),
		SourceAmount:      64,
		DestinationAsset:  nativeAsset.String(),
		DestinationAmount: 100,
		InteriorNodes:     []string{usdAsset.String(), eurAsset.String()},
	}, {
		SourceAsset:       chfAsset.String(),
		SourceAmount:      90,
		DestinationAsset:  nativeAsset.String(),
		DestinationAmount: 100,
		InteriorNodes:     []string{usdAsset.String()},
	}}

	assert.NoError(t, err)
	assertPathEquals(t, expectedPaths, paths)

	// If we ask for more than the offer can handle, though, it should only go
	// through the LPs, not some sort of mix of the two:
	paths, _, err = graph.FindPaths(context.TODO(), 5,
		nativeAsset, 101, // only change: more than the offer has
		&fakeSource, []xdr.Asset{chfAsset}, []xdr.Int64{1000},
		true, 5, true,
	)

	expectedPaths = []Path{{
		SourceAsset:       chfAsset.String(),
		SourceAmount:      96,
		DestinationAsset:  nativeAsset.String(),
		DestinationAmount: 101,
		InteriorNodes:     []string{usdAsset.String()},
	}}

	assert.NoError(t, err)
	assertPathEquals(t, expectedPaths, paths)

	t.Run("without pools", func(t *testing.T) {
		paths, _, err = graph.FindPaths(context.TODO(), 5,
			nativeAsset, 100, &fakeSource,
			[]xdr.Asset{chfAsset}, []xdr.Int64{1000}, true, 5,
			false, // only change: no pools
		)
		assert.NoError(t, err)

		onlyOffersGraph := NewOrderBookGraph()
		onlyOffersGraph.AddOffers(graph.Offers()...)
		if !assert.NoError(t, onlyOffersGraph.Apply(2)) {
			t.FailNow()
		}
		expectedPaths, _, err = onlyOffersGraph.FindPaths(context.TODO(), 5,
			nativeAsset, 100, &fakeSource,
			[]xdr.Asset{chfAsset}, []xdr.Int64{1000}, true, 5,
			true,
		)
		assert.NoError(t, err)

		assertPathEquals(t, expectedPaths, paths)
	})
}

func TestInterleavedFixedPaths(t *testing.T) {
	graph := NewOrderBookGraph()
	graph.AddLiquidityPools(nativeUsdPool, nativeEurPool,
		eurUsdLiquidityPool, usdChfLiquidityPool)
	if !assert.NoErrorf(t, graph.Apply(1), "applying LPs to graph failed") {
		t.FailNow()
	}
	graph.AddOffers(xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(42),
		Selling:  eurAsset,
		Buying:   nativeAsset,
		Amount:   10,
		Price:    xdr.Price{1, 1},
	}, xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(43),
		Selling:  chfAsset,
		Buying:   usdAsset,
		Amount:   1,
		Price:    xdr.Price{1, 1},
	})
	if !assert.NoErrorf(t, graph.Apply(2), "applying offers to graph failed") {
		t.FailNow()
	}

	paths, _, err := graph.FindFixedPaths(context.TODO(),
		5,
		nativeAsset,
		1234,
		[]xdr.Asset{chfAsset},
		5,
		true,
	)

	expectedPaths := []Path{
		{
			SourceAsset:       nativeAsset.String(),
			SourceAmount:      1234,
			DestinationAsset:  chfAsset.String(),
			DestinationAmount: 13,
			InteriorNodes:     []string{usdAsset.String()},
		},
	}

	assert.NoError(t, err)
	assertPathEquals(t, expectedPaths, paths)

	paths, _, err = graph.FindFixedPaths(context.TODO(),
		5,
		nativeAsset,
		1234,
		[]xdr.Asset{chfAsset},
		5,
		false,
	)
	assert.NoError(t, err)

	onlyOffersGraph := NewOrderBookGraph()
	for _, offer := range graph.Offers() {
		onlyOffersGraph.addOffer(offer)
	}
	if !assert.NoErrorf(t, onlyOffersGraph.Apply(2), "applying offers to graph failed") {
		t.FailNow()
	}
	expectedPaths, _, err = onlyOffersGraph.FindFixedPaths(context.TODO(),
		5,
		nativeAsset,
		1234,
		[]xdr.Asset{chfAsset},
		5,
		true,
	)

	assert.NoError(t, err)
	assertPathEquals(t, expectedPaths, paths)
}

func TestRepro(t *testing.T) {
	// A reproduction of the bug report:
	// https://github.com/stellar/go-stellar-sdk/issues/4014
	usdc := xdr.MustNewCreditAsset("USDC", "GAEB3HSAWRVILER6T5NMX5VAPTK4PPO2BAL37HR2EOUIK567GJFEO437")
	eurt := xdr.MustNewCreditAsset("EURT", "GABHG6C7YL2WA2ZJSONPD6ZBWLPAWKYDPYMK6BQRFLZXPQE7IBSTMPNN")

	ybx := xdr.MustNewCreditAsset("YBX", "GCIWMQHPST7LQ7V4LHAF2UP6ZSDCFRYYP7IM4BBAFSBZMVTR3BB4OQZ5")
	btc := xdr.MustNewCreditAsset("BTC", "GA2RETJWNREEUY4JHMZVXCE6EJG6MGBUEXK2QXXMNE5EYAQMG22XCXHA")
	eth := xdr.MustNewCreditAsset("ETH", "GATPY6X6OYTXKNRKVP6LEMUUQKFDUW5P7HL4XI3KWRCY52RAWYJ5FLMC")

	usdcYbxPool := makePool(usdc, ybx, 115066115, 9133346)
	eurtYbxPool := makePool(eurt, ybx, 871648100, 115067)
	btcYbxPool := makePool(btc, ybx, 453280, 19884933)
	ethYbxPool := makePool(eth, ybx, 900000, 10000000)
	usdcForBtcOffer := xdr.OfferEntry{
		OfferId: 42,
		Selling: usdc,
		Buying:  btc,
		Amount:  1000000000000000,
		Price:   xdr.Price{N: 81, D: 5000000},
	}

	graph := NewOrderBookGraph()
	graph.AddLiquidityPools(usdcYbxPool, eurtYbxPool, btcYbxPool, ethYbxPool)
	graph.AddOffers(usdcForBtcOffer)
	if !assert.NoError(t, graph.Apply(2)) {
		t.FailNow()
	}

	// get me 70000.0000000 USDC if I have some ETH
	paths, _, err := graph.FindPaths(context.TODO(), 5,
		usdc, 700000000000, nil, []xdr.Asset{eth}, []xdr.Int64{0},
		false, 5, true,
	)

	assert.NoError(t, err)
	assertPathEquals(t, []Path{}, paths)
	// can't, because BTC/YBX pool is too small
}

func makeVenues(offers ...xdr.OfferEntry) Venues {
	return Venues{offers: offers}
}

func makeTradingPair(assetStringToID map[string]int32, buying, selling xdr.Asset) tradingPair {
	return tradingPair{
		buyingAsset:  assetStringToID[buying.String()],
		sellingAsset: assetStringToID[selling.String()],
	}
}

func makePool(A, B xdr.Asset, a, b xdr.Int64) xdr.LiquidityPoolEntry {
	if !A.LessThan(B) {
		B, A = A, B
		b, a = a, b
	}

	poolId, _ := xdr.NewPoolId(A, B, xdr.LiquidityPoolFeeV18)
	return xdr.LiquidityPoolEntry{
		LiquidityPoolId: poolId,
		Body: xdr.LiquidityPoolEntryBody{
			Type: xdr.LiquidityPoolTypeLiquidityPoolConstantProduct,
			ConstantProduct: &xdr.LiquidityPoolEntryConstantProduct{
				Params: xdr.LiquidityPoolConstantProductParameters{
					AssetA: A,
					AssetB: B,
					Fee:    xdr.LiquidityPoolFeeV18,
				},
				ReserveA:                 a,
				ReserveB:                 b,
				TotalPoolShares:          123,
				PoolSharesTrustLineCount: 456,
			},
		},
	}
}

func getCode(asset xdr.Asset) string {
	code := asset.GetCode()
	if code == "" {
		return "xlm"
	}
	return code
}
