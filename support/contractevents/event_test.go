package contractevents

import (
	_ "embed"
	"math"
	"math/big"
	"testing"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/randxdr"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const passphrase = "passphrase"

var (
	randomIssuer     = keypair.MustRandom()
	randomAsset      = xdr.MustNewCreditAsset("TESTING", randomIssuer.Address())
	randomAccount    = keypair.MustRandom().Address()
	zeroContractHash = xdr.Hash([32]byte{})
	zeroContract     = strkey.MustEncode(strkey.VersionByteContract, zeroContractHash[:])
)

func TestScValCreators(t *testing.T) {
	val := makeSymbol("hello")
	assert.Equal(t, val.Type, xdr.ScValTypeScvSymbol)
	assert.NotNil(t, val.Sym)
	assert.EqualValues(t, *val.Sym, "hello")

	val = makeAmount(1234)
	amt, ok := val.GetI128()
	assert.True(t, ok)
	assert.EqualValues(t, 0, amt.Hi)
	assert.EqualValues(t, 1234, amt.Lo)

	// make an amount which is 2^65 + 1234 to check both hi and lo parts
	amount := big.NewInt(math.MaxInt64)
	amount. // this is 2^63-1
		Add(amount, big.NewInt(1)).            // 2^63
		Or(amount, big.NewInt(math.MaxInt64)). // 2^64-1 (max uint64)
		Lsh(amount, 2).                        // now it's (2^64 - 1) * 4 = 2^66 - 4
		Add(amount, big.NewInt(1234+4))        // now it's 2^66 + 1234

	val = makeBigAmount(amount)
	amt, ok = val.GetI128()
	assert.True(t, ok)
	assert.EqualValues(t, 4, amt.Hi)
	assert.EqualValues(t, 1234, amt.Lo)
}

func TestEventGenerator(t *testing.T) {
	passphrase := "This is a passphrase."
	issuer := keypair.MustRandom().Address()
	from, to, admin := issuer, issuer, issuer

	for _, type_ := range []EventType{
		EventTypeTransfer,
		EventTypeMint,
		EventTypeClawback,
		EventTypeBurn,
	} {
		event := GenerateEvent(type_, from, to, admin, xdr.MustNewNativeAsset(), big.NewInt(12345), passphrase)
		parsedEvent, err := NewStellarAssetContractEvent(&event, passphrase)
		require.NoErrorf(t, err, "generating an event of type %v failed", type_)
		require.Equal(t, type_, parsedEvent.GetType())
		require.Equal(t, xdr.AssetTypeAssetTypeNative, parsedEvent.GetAsset().Type)

		event = GenerateEvent(type_, from, to, admin,
			xdr.MustNewCreditAsset("TESTER", issuer),
			big.NewInt(12345), passphrase)
		parsedEvent, err = NewStellarAssetContractEvent(&event, passphrase)
		require.NoErrorf(t, err, "generating an event of type %v failed", type_)
		require.Equal(t, type_, parsedEvent.GetType())
		require.Equal(t, xdr.AssetTypeAssetTypeCreditAlphanum12, parsedEvent.GetAsset().Type)
	}
}

func TestSACTransferEvent(t *testing.T) {
	xdrEvent := GenerateEvent(EventTypeTransfer, randomAccount, zeroContract, "", randomAsset, big.NewInt(10000), passphrase)

	// Ensure the happy path for transfer events works
	sacEvent, err := NewStellarAssetContractEvent(&xdrEvent, passphrase)
	require.NoError(t, err)
	require.NotNil(t, sacEvent)
	require.Equal(t, EventTypeTransfer, sacEvent.GetType())

	transferEvent := sacEvent.(*TransferEvent)
	require.Equal(t, randomAccount, transferEvent.From)
	require.Equal(t, zeroContract, transferEvent.To)
	require.EqualValues(t, 10000, transferEvent.Amount.Lo)
	require.EqualValues(t, 0, transferEvent.Amount.Hi)
}

func TestSACEventCreation(t *testing.T) {
	var xdrEvent xdr.ContractEvent
	resetEvent := func(from string, to string, asset xdr.Asset) {
		xdrEvent = GenerateEvent(EventTypeTransfer, from, to, "", asset, big.NewInt(10000), passphrase)
	}

	// Ensure that changing the passphrase invalidates the event
	t.Run("wrong passphrase", func(t *testing.T) {
		resetEvent(randomAccount, zeroContract, randomAsset)
		_, err := NewStellarAssetContractEvent(&xdrEvent, "different")
		require.Error(t, err)
	})

	// Ensure that the native asset still works
	t.Run("native transfer", func(t *testing.T) {
		resetEvent(randomAccount, zeroContract, xdr.MustNewNativeAsset())
		sacEvent, err := NewStellarAssetContractEvent(&xdrEvent, passphrase)
		require.NoError(t, err)
		require.Equal(t, xdr.AssetTypeAssetTypeNative, sacEvent.GetAsset().Type)
	})

	// Ensure that invalid asset binaries are rejected
	t.Run("bad asset binary", func(t *testing.T) {
		resetEvent(randomAccount, zeroContract, randomAsset)
		rawBsAsset := xdr.ScString("no bueno")
		xdrEvent.Body.V0.Topics[3].Str = &rawBsAsset
		_, err := NewStellarAssetContractEvent(&xdrEvent, passphrase)
		require.Error(t, err)
	})

	// Ensure that valid asset binaries that mismatch the contract are rejected
	t.Run("mismatching ID", func(t *testing.T) {
		resetEvent(randomAccount, zeroContract, randomAsset)
		// change the ID but keep the asset
		rawNativeContractId, err := xdr.MustNewNativeAsset().ContractID(passphrase)
		require.NoError(t, err)
		nativeContractId := xdr.ContractId(rawNativeContractId)
		xdrEvent.ContractId = &nativeContractId
		_, err = NewStellarAssetContractEvent(&xdrEvent, passphrase)
		require.Error(t, err)

		// now change the asset but keep the ID
		resetEvent(randomAccount, zeroContract, randomAsset)
		diffRandomAsset := xdr.MustNewCreditAsset("TESTING", keypair.MustRandom().Address())
		xdrEvent.Body.V0.Topics = makeTransferTopic(diffRandomAsset)
		_, err = NewStellarAssetContractEvent(&xdrEvent, passphrase)
		require.Error(t, err)
	})

	// Ensure that system events are rejected
	t.Run("system events", func(t *testing.T) {
		resetEvent(randomAccount, zeroContract, randomAsset)
		xdrEvent.Type = xdr.ContractEventTypeSystem
		_, err := NewStellarAssetContractEvent(&xdrEvent, passphrase)
		require.Error(t, err)
	})
}

func TestSACMintEvent(t *testing.T) {
	xdrEvent := GenerateEvent(EventTypeMint, "", zeroContract, randomAccount, randomAsset, big.NewInt(10000), passphrase)

	// Ensure the happy path for mint events works
	sacEvent, err := NewStellarAssetContractEvent(&xdrEvent, passphrase)
	require.NoError(t, err)
	require.NotNil(t, sacEvent)
	require.Equal(t, EventTypeMint, sacEvent.GetType())

	mintEvent := sacEvent.(*MintEvent)
	require.Equal(t, randomAccount, mintEvent.Admin)
	require.Equal(t, zeroContract, mintEvent.To)
	require.EqualValues(t, 10000, mintEvent.Amount.Lo)
	require.EqualValues(t, 0, mintEvent.Amount.Hi)
}

func TestSACClawbackEvent(t *testing.T) {
	xdrEvent := GenerateEvent(EventTypeClawback, zeroContract, "", randomAccount, randomAsset, big.NewInt(10000), passphrase)

	// Ensure the happy path for clawback events works
	sacEvent, err := NewStellarAssetContractEvent(&xdrEvent, passphrase)
	require.NoError(t, err)
	require.NotNil(t, sacEvent)
	require.Equal(t, EventTypeClawback, sacEvent.GetType())

	clawEvent := sacEvent.(*ClawbackEvent)
	require.Equal(t, randomAccount, clawEvent.Admin)
	require.Equal(t, zeroContract, clawEvent.From)
	require.EqualValues(t, 10000, clawEvent.Amount.Lo)
	require.EqualValues(t, 0, clawEvent.Amount.Hi)
}

func TestSACBurnEvent(t *testing.T) {
	xdrEvent := GenerateEvent(EventTypeBurn, randomAccount, "", "", randomAsset, big.NewInt(10000), passphrase)

	// Ensure the happy path for burn events works
	sacEvent, err := NewStellarAssetContractEvent(&xdrEvent, passphrase)
	require.NoError(t, err)
	require.NotNil(t, sacEvent)
	require.Equal(t, EventTypeBurn, sacEvent.GetType())

	burnEvent := sacEvent.(*BurnEvent)
	require.Equal(t, randomAccount, burnEvent.From)
	require.EqualValues(t, 10000, burnEvent.Amount.Lo)
	require.EqualValues(t, 0, burnEvent.Amount.Hi)
}

func TestFuzzingSACEventParser(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := 0; i < 100_000; i++ {
		event, shape := xdr.ContractEvent{}, &gxdr.ContractEvent{}

		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		assert.NoError(t, gxdr.Convert(shape, &event))

		// return values are ignored, but this should never panic
		NewStellarAssetContractEvent(&event, "passphrase")
	}
}

//
// Test suite helpers below
//

func makeEvent() xdr.ContractEvent {
	rawContractId, err := randomAsset.ContractID(passphrase)
	if err != nil {
		panic(err)
	}
	contractId := xdr.ContractId(rawContractId)

	baseXdrEvent := xdr.ContractEvent{
		Ext:        xdr.ExtensionPoint{V: 0},
		ContractId: &contractId,
		Type:       xdr.ContractEventTypeContract,
		Body: xdr.ContractEventBody{
			V:  0,
			V0: &xdr.ContractEventV0{},
		},
	}

	return baseXdrEvent
}

func makeTransferTopic(asset xdr.Asset) xdr.ScVec {
	contractStr := strkey.MustEncode(strkey.VersionByteContract, zeroContractHash[:])

	return xdr.ScVec([]xdr.ScVal{
		makeSymbol("transfer"),     // event name
		makeAddress(randomAccount), // from
		makeAddress(contractStr),   // to
		makeAsset(asset),           // asset details
	})
}

func makeMintTopic(asset xdr.Asset) xdr.ScVec {
	// mint is just transfer but with an admin instead of a from... nice
	topics := makeTransferTopic(asset)
	topics[0] = makeSymbol("mint")
	return topics
}

func makeClawbackTopic(asset xdr.Asset) xdr.ScVec {
	// clawback is just mint but with a from instead of a to
	topics := makeTransferTopic(asset)
	topics[0] = makeSymbol("clawback")
	return topics
}

func makeBurnTopic(asset xdr.Asset) xdr.ScVec {
	// burn is like clawback but without a "to", so we drop that topic
	topics := makeTransferTopic(asset)
	topics[0] = makeSymbol("burn")
	topics = append(topics[:2], topics[3:]...)
	return topics
}

func makeAmount(amount int64) xdr.ScVal {
	return makeBigAmount(big.NewInt(amount))
}
