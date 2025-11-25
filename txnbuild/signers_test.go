package txnbuild

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/price"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stretchr/testify/assert"
)

func TestAccountMergeMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	accountMerge := AccountMerge{
		Destination:   "GAS4V4O2B7DW5T7IQRPEEVCRXMDZESKISR7DVIGKZQYYV3OSQ5SH5LVP",
		SourceAccount: kp1.Address(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&accountMerge},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAACAAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAAAAAAAC6i5yxQAAAEABEvDME7nz+5dkZW4OPtZJcQHhoEsk2/r3RiOzq/y6ecRxmcEPyr1qNFtaLeIcvlpHSQQg9VRed7JAeGWEzxQJ0odkfgAAAEBj72ZPE9hg6dgaWBnkvOVQFdlBis8oxqMLfmDnycCm1uX46Phi3uO6G1xBGMQkA2SLJsBuLubSfRVG47r6ov4N"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestAllowTrustMultSigners(t *testing.T) {
	kp0 := newKeypair0()

	kp1 := newKeypair1()
	txSourceAccount := NewSimpleAccount(kp1.Address(), int64(9606132444168199))

	issuedAsset := CreditAsset{"ABCD", kp1.Address()}
	allowTrust := AllowTrust{
		Trustor:       kp1.Address(),
		Type:          issuedAsset,
		Authorize:     true,
		SourceAccount: kp0.Address(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&allowTrust},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAGQAIiC6AAAACAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAAODcbeFyXKxmUWK1L6znNbKKIkPkHRJNbLktcKPqLnLFAAAABwAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAAFBQkNEAAAAAQAAAAAAAAAC6i5yxQAAAEB5vvJHErjjFX7YWzUbuSLc6JwNAAry+fIeJQuitCRujgkkeYEWy1DjKlbtcaUGbvurfaR8CjfUKBD6F74k964A0odkfgAAAEAq9Ks21/ca6HhTs5YiYG+/nWSRI8mTKZhd2/dDcJRFrZuCj7vlNi76/dSJnjmLbdf1BpLA5Rgvt2hatxbGygYP"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestBumpSequenceMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	bumpSequence := BumpSequence{
		BumpTo:        9606132444168300,
		SourceAccount: kp1.Address(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&bumpSequence},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAACwAiILoAAABsAAAAAAAAAALqLnLFAAAAQOcGy1wxUHU5CdDqN5pFula3BXspTmoNLq4+pSl2kFd5hnRUAOCfTnswoceQ8p1vhcULbsl20gWE3IF1AA2qUgnSh2R+AAAAQLrmJprrsJDARgt6F+EQOmZDOT32K3VLrgIRLzp7mp38sp6zoA/0T7NETjqXezwDrmYkpFpSWT1AmiUwqPEGXQ4="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestChangeTrustMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	changeTrust := ChangeTrust{
		Line:          CreditAsset{"ABCD", kp0.Address()}.MustToChangeTrustAsset(),
		Limit:         "10",
		SourceAccount: kp1.Address(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&changeTrust},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAABgAAAAFBQkNEAAAAAODcbeFyXKxmUWK1L6znNbKKIkPkHRJNbLktcKPqLnLFAAAAAAX14QAAAAAAAAAAAuoucsUAAABA3nSc20C4tFs7nUZp/P4kTzpmPEHYaATNtzGcU4mOwOrxrCPJr1TpVnASi/8d3M0AhRXLa2c5tI9s79hc4/w+BNKHZH4AAABAtPLvu8OPMiaXEfDCZivyynR5Q/sFfMWwqOBIEq4wJSbzl24Dz4uqVdjlxyqKAOkdsefKINfrkcaETZrDYRU8BQ=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestCreateAccountMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	createAccount := CreateAccount{
		Destination:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:        "10",
		SourceAccount: kp1.Address(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createAccount},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAF9eEAAAAAAAAAAALqLnLFAAAAQDV8bLiIbfvgV6NtYoipI9Ja4VQmDXWw/7gT2y+wFyqJXk9XMp2ke5bgO+J6bDH8xPQFRa/lXJTmPnc0AaiFmQzSh2R+AAAAQNBEP2v1OPVYFzepAB58TCH8v+6wExgpPrLasptj2un3GyCiBcqE0VYvrj05CHEtLtcC9Rb5FrlOGG327VDyeQM="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestCreatePassiveSellOfferMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	createPassiveOffer := CreatePassiveSellOffer{
		Selling:       NativeAsset{},
		Buying:        CreditAsset{"ABCD", kp0.Address()},
		Amount:        "10",
		Price:         xdr.Price{1, 1},
		SourceAccount: kp1.Address(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createPassiveOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAABAAAAAAAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAF9eEAAAAAAQAAAAEAAAAAAAAAAuoucsUAAABA0APb892L3NYP8YyXZMonoBOYMOMtUZhpjOnfSnfouxQ/otFnRss5MX/Ro6w6a1EI9f4gxRhNh6WDm+WXeVFHD9KHZH4AAABAqvvW4IA+53gcWg2DuJMUf5bS46gbnKqgG2HCGO28Jxst9gmv477IJcJ1NlIF96oQhB0rITdtW7BiP4eX/sXFBw=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestInflationMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	inflation := Inflation{
		SourceAccount: kp1.Address(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&inflation},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAACQAAAAAAAAAC6i5yxQAAAEANdI2UgZ566jUekR+rW4r3ya6KQcV2tinB9sjfSd5gRqCMYAUsgQmBHPailp5K5mVBr5m0zvizTnfj3UOGPAgD0odkfgAAAECf29QWzDc7FzBqhhC61x/G3BDOZ12vo6tOsazJyG4DETUbI/jYUsion81j9D0ELx0OAtssOsvhwX1r8MwBT4UB"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageDataMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	manageData := ManageData{
		Name:          "Fruit preference",
		Value:         []byte("Apple"),
		SourceAccount: kp1.Address(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&manageData},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAACgAAABBGcnVpdCBwcmVmZXJlbmNlAAAAAQAAAAVBcHBsZQAAAAAAAAAAAAAC6i5yxQAAAECwrVa4S7aX0RqxYYohiavPdXsBbuo7ut6aNn4I52B4ANjIEhSea0aNx9PbiMlqXJhHngcF4oZ8egIYfUf6Q54O0odkfgAAAEDkq5kiNBo0g0oKdPkRcK2WAYKo1bRBOWngnm2dykdCQhGF8MyBv6vbdVhs+f88nfAZpqiNfqz9EekEqdZA8ocK"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageOfferCreateMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	selling := NativeAsset{}
	buying := CreditAsset{"ABCD", kp0.Address()}
	sellAmount := "100"
	createOffer, err := CreateOfferOp(selling, buying, sellAmount, price.MustParse("0.01"), kp1.Address())
	check(err)

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&createOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAAAwAAAAAAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAA7msoAAAAAAQAAAGQAAAAAAAAAAAAAAAAAAAAC6i5yxQAAAEAaOoWXzyhFoJqVov0wmaJ47EM/8N0wgoNkHJ9tfG/7wqujo03s07pAicyWboRCO5P0k6df3RKbaJT/crBrKnoI0odkfgAAAECwHJ6t67JJOKe7Icr30S7jZytV4Dp1bb4aNuFFuqan5b/sEWlViYO1afOPBouWwRQfJjyUWDGt5Wy+/J+MGCQN"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageOfferDeleteMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	offerID := int64(2921622)
	deleteOffer, err := DeleteOfferOp(offerID, kp1.Address())
	check(err)

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&deleteOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAAAwAAAAAAAAABRkFLRQAAAABBB4BkxJWGYvNgJBoiXUo2tjgWlNmhHMMKdwGN7RSdsQAAAAAAAAAAAAAAAQAAAAEAAAAAACyUlgAAAAAAAAAC6i5yxQAAAEBaditn57uAGNhrBW+QS/G/Lg8AqB73HR4vnu6HnRKeduLCQsLOJz8BFixbuQyXDKiwrxZK+VIMLUMBazSZjKsG0odkfgAAAEC7UNgojiThuTrJlsnRVhVGnbOkCY+dUXCWyW9Jgsg3sFgaWUS5oeOSDMjEZTCaMZPMCiSuFEdkn6Jc+2jJo68O"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestManageOfferUpdateMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	selling := NativeAsset{}
	buying := CreditAsset{"ABCD", kp0.Address()}
	sellAmount := "50"
	offerID := int64(2497628)
	updateOffer, err := UpdateOfferOp(selling, buying, sellAmount, price.MustParse("0.02"), offerID, kp1.Address())
	check(err)

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&updateOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAAAwAAAAAAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAdzWUAAAAAAQAAADIAAAAAACYcXAAAAAAAAAAC6i5yxQAAAEDhL3pD9+Veot1821y3cQuQRxYNaUJIQt+SlxySg2HV8Bm+WIx4eWpmC+/CS7a5rMLuzW6Vs9zGP628RZ/vCN4B0odkfgAAAEC1PuV3ntuZ0k20SZ1secwrZCEOysw52/1f6/Z4sx7Is53oraNuiUKnhCgR/6s/PHd5EMVlguC39Od7Tw+nfkgN"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestPathPaymentMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	abcdAsset := CreditAsset{"ABCD", kp0.Address()}
	pathPayment := PathPayment{
		SendAsset:     NativeAsset{},
		SendMax:       "10",
		Destination:   kp0.Address(),
		DestAsset:     NativeAsset{},
		DestAmount:    "1",
		Path:          []Asset{abcdAsset},
		SourceAccount: kp1.Address(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&pathPayment},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAAAgAAAAAAAAAABfXhAAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAAAAAAAJiWgAAAAAEAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAAAAAC6i5yxQAAAECmKj83TAGKOza6zjhNh510cwiAYsSE/Y1rXjcrI7tO1lXBqSYaCyVufe1KzJbEVViwf0CZOnuo8Oksy0Q18OcC0odkfgAAAEDM/Wano1U5PSolmQr9Hv4aFvheLmtpjOrR1f5LswgfR6lRoJWyvcTdGjhp60ML8JafNuHFTmJ1JFfPh38LJ0ID"

	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestPaymentMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	payment := Payment{
		Destination:   "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
		Amount:        "10",
		Asset:         NativeAsset{},
		SourceAccount: kp1.Address(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&payment},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAAAQAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAAAAAAAABfXhAAAAAAAAAAAC6i5yxQAAAEB82JGXqIIh87Wp6kb6118YjUoR/2X+RFI4Gm62+sMIF9XjlAUY6eSfdqqvLP6NQdbMazDYj6VYgKuNLQ/8hn8I0odkfgAAAEDVQumCyGwJxbNxv63X+yMa1mBTsYzilEmbDdKtQZvzF5Pu8nYXAm2AYKvlRmunmX/AXJICHQLQyPFTVj6E8oQD"
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

func TestSetOptionsMultSigners(t *testing.T) {
	kp0 := newKeypair0()
	txSourceAccount := NewSimpleAccount(kp0.Address(), int64(9605939170639898))

	kp1 := newKeypair1()

	setOptions := SetOptions{
		SetFlags:      []AccountFlag{AuthRequired, AuthRevocable},
		SourceAccount: kp1.Address(),
	}

	received, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &txSourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&setOptions},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
		},
		network.TestNetworkPassphrase,
		kp0, kp1,
	)
	assert.NoError(t, err)

	expected := "AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQAIiCNAAAAGwAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAACXK8doPx27P6IReQlRRuweSSUiUfjqgyswxiu3Sh2R+AAAABQAAAAAAAAAAAAAAAQAAAAMAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAuoucsUAAABAO1oK5K+qtaNQn/a836KapCFEFg/Unt02oFNhoTJ/Toxk++X5RgGjnUPpBywxkI04QyjDHQfIwiRnvCBnP3SED9KHZH4AAABA54vLHhDV5sodEIB5C4zOBJoR5ga+Tb1OlaSWlQX7+t9cmmhz+5TjX4PcfA8h48/LodN0u4qUoRyK0AxTfi/nDA=="
	assert.Equal(t, expected, received, "Base 64 XDR should match")
}

type transactionCommon interface {
	Signatures() []xdr.DecoratedSignature
	Hash(networkStr string) ([32]byte, error)
	Base64() (string, error)
	ToXDR() xdr.TransactionEnvelope
}

func verifySignatures(t *testing.T, tx transactionCommon, signers ...*keypair.Full) {
	assert.Len(t, tx.Signatures(), len(signers))

	hash, err := tx.Hash(network.TestNetworkPassphrase)
	assert.NoError(t, err)
	signatures := tx.Signatures()
	for i, kp := range signers {
		assert.NoError(t, kp.Verify(hash[:], signatures[i].Signature))
	}
}

func assertBase64(t *testing.T, tx transactionCommon) string {
	base64, err := tx.Base64()
	assert.NoError(t, err)

	envRef := tx.ToXDR()
	envRefBase64, err := xdr.MarshalBase64(envRef)
	assert.NoError(t, err)

	assert.Equal(t, base64, envRefBase64)

	return base64
}

func TestSigningImmutability(t *testing.T) {
	kp0, kp1, kp2 := newKeypair0(), newKeypair1(), newKeypair2()

	sourceAccount := NewSimpleAccount(kp0.Address(), 1)
	params := TransactionParams{
		SourceAccount: &sourceAccount,
		Operations:    []Operation{&Inflation{}},
		BaseFee:       MinBaseFee,
		Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
	}
	root, err := NewTransaction(params)
	assert.NoError(t, err)
	root, err = root.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)
	rootB64 := assertBase64(t, root)
	rootXDR := root.ToXDR()

	left, err := root.Sign(network.TestNetworkPassphrase, kp1)
	assert.NoError(t, err)
	leftB64 := assertBase64(t, left)

	right, err := root.Sign(network.TestNetworkPassphrase, kp2)
	assert.NoError(t, err)
	rightB64 := assertBase64(t, right)

	expectedRootB64, err := newSignedTransaction(
		params, network.TestNetworkPassphrase, kp0,
	)
	assert.NoError(t, err)
	expectedLeftB64, err := newSignedTransaction(
		params, network.TestNetworkPassphrase, kp0, kp1,
	)
	assert.NoError(t, err)
	expectedRightB64, err := newSignedTransaction(
		params, network.TestNetworkPassphrase, kp0, kp2,
	)
	assert.NoError(t, err)

	assert.Equal(t, expectedRootB64, rootB64)
	verifySignatures(t, root, kp0)
	assert.Equal(t, expectedLeftB64, leftB64)
	verifySignatures(t, left, kp0, kp1)
	assert.Equal(t, expectedRightB64, rightB64)
	verifySignatures(t, right, kp0, kp2)

	rootXDRB64, err := xdr.MarshalBase64(rootXDR)
	if assert.NoError(t, err) {
		assert.Equal(t, expectedRootB64, rootXDRB64)
	}
}

func TestFeeBumpSigningImmutability(t *testing.T) {
	kp0, kp1, kp2 := newKeypair0(), newKeypair1(), newKeypair2()

	sourceAccount := NewSimpleAccount(kp0.Address(), 1)
	innerParams := TransactionParams{
		SourceAccount: &sourceAccount,
		Operations:    []Operation{&Inflation{}},
		BaseFee:       MinBaseFee,
		Preconditions: Preconditions{TimeBounds: NewInfiniteTimeout()},
	}
	inner, err := NewTransaction(innerParams)
	assert.NoError(t, err)

	params := FeeBumpTransactionParams{
		Inner:      inner,
		FeeAccount: kp1.Address(),
		BaseFee:    MinBaseFee,
	}
	root, err := NewFeeBumpTransaction(params)
	assert.NoError(t, err)
	root, err = root.Sign(network.TestNetworkPassphrase, kp1)
	assert.NoError(t, err)
	rootB64 := assertBase64(t, root)

	left, err := root.Sign(network.TestNetworkPassphrase, kp0)
	assert.NoError(t, err)
	leftB64 := assertBase64(t, left)

	right, err := root.Sign(network.TestNetworkPassphrase, kp2)
	assert.NoError(t, err)
	rightB64 := assertBase64(t, right)

	expectedRootB64, err := newSignedFeeBumpTransaction(
		params, network.TestNetworkPassphrase, kp1,
	)
	assert.NoError(t, err)

	expectedLeftB64, err := newSignedFeeBumpTransaction(
		params, network.TestNetworkPassphrase, kp1, kp0,
	)
	assert.NoError(t, err)
	expectedRightB64, err := newSignedFeeBumpTransaction(
		params, network.TestNetworkPassphrase, kp1, kp2,
	)
	assert.NoError(t, err)

	assert.Equal(t, expectedRootB64, rootB64)
	verifySignatures(t, root, kp1)
	assert.Equal(t, expectedLeftB64, leftB64)
	verifySignatures(t, left, kp1, kp0)
	assert.Equal(t, expectedRightB64, rightB64)
	verifySignatures(t, right, kp1, kp2)
}
