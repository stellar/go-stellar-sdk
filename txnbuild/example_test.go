package txnbuild

import (
	"fmt"
	"time"

	"github.com/stellar/go-stellar-sdk/price"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	horizonclient "github.com/stellar/go-stellar-sdk/txnbuild/examplehorizonclient"
)

func ExampleInflation() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := Inflation{}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAACQAAAAAAAAAB6i5yxQAAAED9zR1l78yiBwd/o44RyE3XP7QT57VmI90qE46TjfncYyqlOaIRWpkh3qouTjV5IRPVGo6+bFWV40H1HE087FgA
}

func ExampleCreateAccount() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := CreateAccount{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "10",
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAF9eEAAAAAAAAAAAHqLnLFAAAAQKsrlxt6Ri/WuDGcK1+Tk1hdYHdPeK7KMIds10mcwzw6BpQFZYxP8o6O6ejJFGO06TAGt2PolwuWnpeiVQ9Kcg0=
}

func ExamplePayment() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := Payment{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "10",
		Asset:       NativeAsset{},
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAQAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAAAAAABfXhAAAAAAAAAAAB6i5yxQAAAEB2/C066OEFac3Bszk6FtvKd+NKOeCl+f8caHQATPos8HkJW1Sm/WyEkVDrvrDX4udMHl3gHhlS/qE0EuWEeJYC
}

func ExamplePayment_setBaseFee() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op1 := Payment{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "10",
		Asset:       NativeAsset{},
	}

	op2 := Payment{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "100",
		Asset:       NativeAsset{},
	}

	// get fees from network
	feeStats, err := client.FeeStats()
	check(err)

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op1, &op2},
			BaseFee:              feeStats.MaxFee.P50,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAABLAADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAAQAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAAAAAABfXhAAAAAAAAAAABAAAAAITg3tq8G0kvnvoIhZPMYJsY+9KVV8xAA6NxhtKxIXZUAAAAAAAAAAA7msoAAAAAAAAAAAHqLnLFAAAAQMmOXP+k93ENYtu7evNTu2h63UkNrQnF6ci49Oh1XufQ3rhzS4Dd1+6AXqgWa4FbcvlTVRjxCurkflI4Rov2xgQ=
}

func ExampleBumpSequence() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := BumpSequence{
		BumpTo: 9606132444168300,
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAACwAiILoAAABsAAAAAAAAAAHqLnLFAAAAQEIvyOHdPn82ckKXISGF6sR4YU5ox735ivKrC/wS4615j1AA42vbXSLqShJA5/7/DX56UUv+Lt7vlcu9M7jsRw4=
}

func ExampleAccountMerge() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := AccountMerge{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAACAAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAAAAAAB6i5yxQAAAEAvOx3WHzmaTsf4rK+yRDsvXn9xh+dU6CkpAum+FCXQ5LZQqhxQg9HErbSfxeTFMdknEpMKXgJRFUfAetl+jf4O
}

func ExampleManageData() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := ManageData{
		Name:  "Fruit preference",
		Value: []byte("Apple"),
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAACgAAABBGcnVpdCBwcmVmZXJlbmNlAAAAAQAAAAVBcHBsZQAAAAAAAAAAAAAB6i5yxQAAAEDtRCyQRKKgQ8iLEu7kicHtSzoplfxPtPTMhdRv/sq8UoIBVTxIw+S13Jv+jzs3tyLDLiGCVNXreUNlbfX+980K
}

func ExampleManageData_removeDataEntry() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := ManageData{
		Name: "Fruit preference",
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAACgAAABBGcnVpdCBwcmVmZXJlbmNlAAAAAAAAAAAAAAAB6i5yxQAAAEDFpI1vphzG8Dny4aVDA7tyOlP579d9kWO0U/vmq6pWTrNocd6+xTiU753W50ksEscA6f1WNwUsQf+DCwmZfqIA
}

func ExampleSetOptions() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := SetOptions{
		InflationDestination: NewInflationDestination("GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z"),
		ClearFlags:           []AccountFlag{AuthRevocable},
		SetFlags:             []AccountFlag{AuthRequired, AuthImmutable},
		MasterWeight:         NewThreshold(10),
		LowThreshold:         NewThreshold(1),
		MediumThreshold:      NewThreshold(2),
		HighThreshold:        NewThreshold(2),
		HomeDomain:           NewHomeDomain("LovelyLumensLookLuminous.com"),
		Signer:               &Signer{Address: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z", Weight: Threshold(4)},
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABQAAAAEAAAAAhODe2rwbSS+e+giFk8xgmxj70pVXzEADo3GG0rEhdlQAAAABAAAAAgAAAAEAAAAFAAAAAQAAAAoAAAABAAAAAQAAAAEAAAACAAAAAQAAAAIAAAABAAAAHExvdmVseUx1bWVuc0xvb2tMdW1pbm91cy5jb20AAAABAAAAAITg3tq8G0kvnvoIhZPMYJsY+9KVV8xAA6NxhtKxIXZUAAAABAAAAAAAAAAB6i5yxQAAAEBxncRuLogeNQ8sG9TojUMB6QmKDWYmhF00Wz43UX90pAQnSNcJAQxur0RA7Fn6LjJLObqyjcdIc4P2DC02u08G
}

func ExampleChangeTrust() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	asset, err := CreditAsset{"ABCD", "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z"}.ToChangeTrustAsset()
	check(err)

	op := ChangeTrust{
		Line:  asset,
		Limit: "10",
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABgAAAAFBQkNEAAAAAITg3tq8G0kvnvoIhZPMYJsY+9KVV8xAA6NxhtKxIXZUAAAAAAX14QAAAAAAAAAAAeoucsUAAABAqqUuIlFMrlElYnGSLHlaI/A41oGA3rdtc1EHhza9bXk35ZwlEvmsBUOZTasZfgBzwd+CczekWKBCEqBCHzaSBw==
}

func ExampleChangeTrust_removeTrustline() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := RemoveTrustlineOp(CreditAsset{"ABCD", "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z"}.MustToChangeTrustAsset())

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABgAAAAFBQkNEAAAAAITg3tq8G0kvnvoIhZPMYJsY+9KVV8xAA6NxhtKxIXZUAAAAAAAAAAAAAAAAAAAAAeoucsUAAABAKLmUWcLjxeY+vG8jEXMNprU6EupxbMRiXGYzuKBptnVlbFUtTBqhYa/ibyCZTEVCinT8bWQKDvZI0m6VLKVHAg==
}

func ExampleAllowTrust() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := AllowTrust{
		Trustor:   "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Type:      CreditAsset{"ABCD", "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z"},
		Authorize: true,
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABwAAAACE4N7avBtJL576CIWTzGCbGPvSlVfMQAOjcYbSsSF2VAAAAAFBQkNEAAAAAQAAAAAAAAAB6i5yxQAAAEAY3MnWiMcL18SxRITSuI5tZSXmEo0Q38UZg0jiJGU2U6kSnsCNTTJiGACGQlIrPfAMYt9koarrX11w7HLBosQN
}

func ExampleManageSellOffer() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	selling := NativeAsset{}
	buying := CreditAsset{"ABCD", "GAS4V4O2B7DW5T7IQRPEEVCRXMDZESKISR7DVIGKZQYYV3OSQ5SH5LVP"}
	sellAmount := "100"
	op, err := CreateOfferOp(selling, buying, sellAmount, price.MustParse("0.01"))
	check(err)

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAwAAAAAAAAABQUJDRAAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAAA7msoAAAAAAQAAAGQAAAAAAAAAAAAAAAAAAAAB6i5yxQAAAEBtfrN+VUE7iCwBk0+rmg0/Ua4DItMWEy6naGWxoDBi4ksCIJSZPzkv79Q65rIaFyIcC/zuyJcnIcv73AP+HQEK
}

func ExampleManageSellOffer_deleteOffer() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	offerID := int64(2921622)
	op, err := DeleteOfferOp(offerID)
	check(err)

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAwAAAAAAAAABRkFLRQAAAABBB4BkxJWGYvNgJBoiXUo2tjgWlNmhHMMKdwGN7RSdsQAAAAAAAAAAAAAAAQAAAAEAAAAAACyUlgAAAAAAAAAB6i5yxQAAAEBnE+oILauqt6m8fj7DIBNW/XBmKJ34SLvHdxP04vb26aI8q9i/2p9/pJMnWPeOoIw0f6jreR306qPJFhjMtl4G
}

func ExampleManageSellOffer_updateOffer() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	selling := NativeAsset{}
	buying := CreditAsset{"ABCD", "GAS4V4O2B7DW5T7IQRPEEVCRXMDZESKISR7DVIGKZQYYV3OSQ5SH5LVP"}
	sellAmount := "50"
	offerID := int64(2497628)
	op, err := UpdateOfferOp(selling, buying, sellAmount, price.MustParse("0.02"), offerID)
	check(err)

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAwAAAAAAAAABQUJDRAAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAAAdzWUAAAAAAQAAADIAAAAAACYcXAAAAAAAAAAB6i5yxQAAAECmO+4yukAuLRtR4IRWPVtoyZ2LJeaipPuec+/M1JGDoTFPULDl3kgugPwV3mr0jvMNArBdR8S3NUw31gtT5TcO
}

func ExampleCreatePassiveSellOffer() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := CreatePassiveSellOffer{
		Selling: NativeAsset{},
		Buying:  CreditAsset{"ABCD", "GAS4V4O2B7DW5T7IQRPEEVCRXMDZESKISR7DVIGKZQYYV3OSQ5SH5LVP"},
		Amount:  "10",
		Price:   xdr.Price{1, 1},
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAABAAAAAAAAAABQUJDRAAAAAAlyvHaD8duz+iEXkJUUbsHkklIlH46oMrMMYrt0odkfgAAAAAF9eEAAAAAAQAAAAEAAAAAAAAAAeoucsUAAABAE4XbLdDVz1MwC9Bs84nkqK8hyHheVbYznNSiAP0hiP8auvcKAMnYz3HJvzM8H0q/K5MPvgBaehHZ/tQtaPSGBg==
}

func ExamplePathPayment() {
	kp, _ := keypair.Parse("SBZVMB74Z76QZ3ZOY7UTDFYKMEGKW5XFJEB6PFKBF4UYSSWHG4EDH7PY")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	abcdAsset := CreditAsset{"ABCD", "GDQNY3PBOJOKYZSRMK2S7LHHGWZIUISD4QORETLMXEWXBI7KFZZMKTL3"}
	op := PathPayment{
		SendAsset:   NativeAsset{},
		SendMax:     "10",
		Destination: kp.Address(),
		DestAsset:   NativeAsset{},
		DestAmount:  "1",
		Path:        []Asset{abcdAsset},
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAgAAAAAAAAAABfXhAAAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAAAAAAAAAJiWgAAAAAEAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAAAAABLhVZmAAAAEDhhPsNm7yKfCUCDyBV1pOZDu+3DVDpT2cJSLQOVevP6pmU2yVqvMKnWbYxC5GbTXEEF+MfBE6EoW5+Z4rRt0QO
}

func ExamplePathPaymentStrictReceive() {
	kp, _ := keypair.Parse("SBZVMB74Z76QZ3ZOY7UTDFYKMEGKW5XFJEB6PFKBF4UYSSWHG4EDH7PY")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	abcdAsset := CreditAsset{"ABCD", "GDQNY3PBOJOKYZSRMK2S7LHHGWZIUISD4QORETLMXEWXBI7KFZZMKTL3"}
	op := PathPaymentStrictReceive{
		SendAsset:   NativeAsset{},
		SendMax:     "10",
		Destination: kp.Address(),
		DestAsset:   NativeAsset{},
		DestAmount:  "1",
		Path:        []Asset{abcdAsset},
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAgAAAAAAAAAABfXhAAAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAAAAAAAAAJiWgAAAAAEAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAAAAABLhVZmAAAAEDhhPsNm7yKfCUCDyBV1pOZDu+3DVDpT2cJSLQOVevP6pmU2yVqvMKnWbYxC5GbTXEEF+MfBE6EoW5+Z4rRt0QO
}

func ExamplePathPaymentStrictSend() {
	kp, _ := keypair.Parse("SBZVMB74Z76QZ3ZOY7UTDFYKMEGKW5XFJEB6PFKBF4UYSSWHG4EDH7PY")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	abcdAsset := CreditAsset{"ABCD", "GDQNY3PBOJOKYZSRMK2S7LHHGWZIUISD4QORETLMXEWXBI7KFZZMKTL3"}
	op := PathPaymentStrictSend{
		SendAsset:   NativeAsset{},
		SendAmount:  "1",
		Destination: kp.Address(),
		DestAsset:   NativeAsset{},
		DestMin:     "10",
		Path:        []Asset{abcdAsset},
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAADQAAAAAAAAAAAJiWgAAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAAAAAAAABfXhAAAAAAEAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAAAAAABLhVZmAAAAEDV6CmR4ATvtm2qBzHE9UqqS95ZnIIHgpuU7hTZO38DHhf+oeZQ02DGvst4vYMMAIPGkMAsLlfAN/AFinz74DAD
}

func ExampleManageBuyOffer() {
	kp, _ := keypair.Parse("SBZVMB74Z76QZ3ZOY7UTDFYKMEGKW5XFJEB6PFKBF4UYSSWHG4EDH7PY")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	buyOffer := ManageBuyOffer{
		Selling: NativeAsset{},
		Buying:  CreditAsset{"ABCD", "GDQNY3PBOJOKYZSRMK2S7LHHGWZIUISD4QORETLMXEWXBI7KFZZMKTL3"},
		Amount:  "100",
		Price:   price.MustParse("0.01"),
		OfferID: 0,
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&buyOffer},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	txe, err := tx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAAAgAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAADAAAAAAAAAABQUJDRAAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAAA7msoAAAAAAQAAAGQAAAAAAAAAAAAAAAAAAAABLhVZmAAAAED4fIdU68w6XIMwf1RPFdF9qRRlfPycrmK8dCOW0XwSbiya9JfMi9YrD9cGY7zHV+3zYpLcEi7lLo++PZ1gOsAK

}

func ExampleFeeBumpTransaction() {
	kp, _ := keypair.Parse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := BumpSequence{
		BumpTo: 9606132444168300,
	}

	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()}, // Use a real timeout in production!
		},
	)
	check(err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
	check(err)

	feeBumpKP, _ := keypair.Parse("SBZVMB74Z76QZ3ZOY7UTDFYKMEGKW5XFJEB6PFKBF4UYSSWHG4EDH7PY")
	feeBumpTx, err := NewFeeBumpTransaction(
		FeeBumpTransactionParams{
			Inner:      tx,
			FeeAccount: feeBumpKP.Address(),
			BaseFee:    MinBaseFee,
		},
	)
	check(err)
	feeBumpTx, err = feeBumpTx.Sign(network.TestNetworkPassphrase, feeBumpKP.(*keypair.Full))
	check(err)

	txe, err := feeBumpTx.Base64()
	check(err)
	fmt.Println(txe)

	// Output: AAAABQAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAAAAAADIAAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAGQADKI/AAAABAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAACwAiILoAAABsAAAAAAAAAAHqLnLFAAAAQEIvyOHdPn82ckKXISGF6sR4YU5ox735ivKrC/wS4615j1AA42vbXSLqShJA5/7/DX56UUv+Lt7vlcu9M7jsRw4AAAAAAAAAAS4VWZgAAABAeD0gL6WpzSdGTzWd4c9yUu3r+W21hOTLT4ItHGBTHYPT20Wk3dytuqfP89EzlkZXvtG8/N0HH4w+oJCLOL/5Aw==
}

func ExampleBuildChallengeTx() {
	// Generate random nonce
	serverSignerSeed := "SBZVMB74Z76QZ3ZOY7UTDFYKMEGKW5XFJEB6PFKBF4UYSSWHG4EDH7PY"
	clientAccountID := "GDQNY3PBOJOKYZSRMK2S7LHHGWZIUISD4QORETLMXEWXBI7KFZZMKTL3"
	anchorName := "SDF"
	webAuthDomain := "webauthdomain.example.org"
	timebound := time.Duration(5 * time.Minute)

	tx, err := BuildChallengeTx(serverSignerSeed, clientAccountID, webAuthDomain, anchorName, network.TestNetworkPassphrase, timebound, nil)
	check(err)

	txeBase64, err := tx.Base64()
	check(err)
	ok, err := checkChallengeTx(txeBase64, anchorName)
	check(err)

	fmt.Println(ok)
	// Output: true
}

func ExampleCreateClaimableBalance() {
	A := "SCZANGBA5YHTNYVVV4C3U252E2B6P6F5T3U6MM63WBSBZATAQI3EBTQ4"
	B := "GA2C5RFPE6GCKMY3US5PAB6UZLKIGSPIUKSLRB6Q723BM2OARMDUYEJ5"

	aKeys := keypair.MustParseFull(A)
	aAccount := SimpleAccount{AccountID: aKeys.Address()}

	soon := time.Now().Add(time.Second * 60)
	bCanClaim := BeforeRelativeTimePredicate(60)
	aCanReclaim := NotPredicate(BeforeAbsoluteTimePredicate(soon.Unix()))

	claimants := []Claimant{
		NewClaimant(B, &bCanClaim),
		NewClaimant(aKeys.Address(), &aCanReclaim),
	}

	claimableBalanceEntry := CreateClaimableBalance{
		Destinations: claimants,
		Asset:        NativeAsset{},
		Amount:       "420",
	}

	// Build and sign the transaction
	tx, err := NewTransaction(
		TransactionParams{
			SourceAccount:        &aAccount,
			IncrementSequenceNum: true,
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			Operations:           []Operation{&claimableBalanceEntry},
		},
	)
	check(err)
	tx, err = tx.Sign(network.TestNetworkPassphrase, aKeys)
	check(err)

	balanceId, err := tx.ClaimableBalanceID(0)
	check(err)
	fmt.Println(balanceId)

	// Output: 000000000bf0a78c7ca2a980768b66980ba97934f3b3b45a05ce7a5195a44b64b7dedadb
}

func ExampleClaimClaimableBalance() {
	A := "SCZANGBA5YHTNYVVV4C3U252E2B6P6F5T3U6MM63WBSBZATAQI3EBTQ4"
	aKeys := keypair.MustParseFull(A)
	aAccount := SimpleAccount{AccountID: aKeys.Address()}

	balanceId := "000000000bf0a78c7ca2a980768b66980ba97934f3b3b45a05ce7a5195a44b64b7dedadb"
	claimBalance := ClaimClaimableBalance{BalanceID: balanceId}

	txb64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &aAccount, // or Account B, depending on the condition!
			IncrementSequenceNum: true,
			BaseFee:              MinBaseFee,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			Operations:           []Operation{&claimBalance},
		},
		network.TestNetworkPassphrase,
		aKeys,
	)
	check(err)
	fmt.Println(txb64)

	// Output: AAAAAgAAAAC0FS8Odh4yFSpaseK1sYMMVdTpVCJmylGJpMeYu9LOKAAAAGQAAAAAAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAADwAAAAAL8KeMfKKpgHaLZpgLqXk087O0WgXOelGVpEtkt97a2wAAAAAAAAABu9LOKAAAAEAesnN9L5oVpoZloBoUYfafhhuGSXAsJL2q15zyyWysc7fOADPdiQXQTEuySp12/ciGYWbZhw/fvyzLJlTgqmsI
}

type SponsorshipTestConfig struct {
	A  *keypair.Full
	S1 *keypair.Full
	S2 *keypair.Full

	Aaccount  SimpleAccount
	S1account SimpleAccount
	S2account SimpleAccount

	Assets []CreditAsset
}

func InitSponsorshipTestConfig() SponsorshipTestConfig {
	A := keypair.MustParseFull("SCZANGBA5YHTNYVVV4C3U252E2B6P6F5T3U6MM63WBSBZATAQI3EBTQ4")
	S1 := keypair.MustParseFull("SBZVMB74Z76QZ3ZOY7UTDFYKMEGKW5XFJEB6PFKBF4UYSSWHG4EDH7PY")
	S2 := keypair.MustParseFull("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")

	return SponsorshipTestConfig{
		A: A, S1: S1, S2: S2,
		Aaccount:  SimpleAccount{AccountID: A.Address()},
		S1account: SimpleAccount{AccountID: S1.Address()},
		S2account: SimpleAccount{AccountID: S2.Address()},
		Assets: []CreditAsset{
			{Code: "ABCD", Issuer: S1.Address()},
			{Code: "EFGH", Issuer: S1.Address()},
			{Code: "IJKL", Issuer: S2.Address()},
		},
	}
}

func ExampleBeginSponsoringFutureReserves() {
	test := InitSponsorshipTestConfig()

	asset, err := test.Assets[0].ToChangeTrustAsset()
	check(err)

	// If the sponsoree submits the transaction, the `SourceAccount` fields can
	// be omitted for the "sponsor sandwich" operations.
	sponsorTrustline := []Operation{
		&BeginSponsoringFutureReserves{SponsoredID: test.A.Address()},
		&ChangeTrust{
			SourceAccount: test.Aaccount.AccountID,
			Line:          asset,
			Limit:         MaxTrustlineLimit,
		},
		&EndSponsoringFutureReserves{},
	}

	// The sponsorer obviously must sign the tx, but so does the sponsoree, to
	// consent to the sponsored operation.
	txb64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &test.Aaccount,
			Operations:           sponsorTrustline,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			BaseFee:              MinBaseFee,
			IncrementSequenceNum: true,
		},
		network.TestNetworkPassphrase,
		test.S1,
		test.A,
	)
	check(err)
	fmt.Println(txb64)

	// Output: AAAAAgAAAAC0FS8Odh4yFSpaseK1sYMMVdTpVCJmylGJpMeYu9LOKAAAASwAAAAAAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAMAAAAAAAAAEAAAAAC0FS8Odh4yFSpaseK1sYMMVdTpVCJmylGJpMeYu9LOKAAAAAEAAAAAtBUvDnYeMhUqWrHitbGDDFXU6VQiZspRiaTHmLvSzigAAAAGAAAAAUFCQ0QAAAAAfhHLNNY19eGrAtSgLD3VpaRm2AjNjxIBWQg9zS4VWZh//////////wAAAAAAAAARAAAAAAAAAAIuFVmYAAAAQARLe8wjGKq6WwdOPGkw2jo4eltp6dAHXEum4kYKzIjYx9fs4kdNJAaJE0s3Fy6JAIo1ttrGWp8zq6VX6P5CcAW70s4oAAAAQNpzu6NxKgcYd70mJl6EHyRPdjNTfxGm1w4XIIyIfZElRpmuZ6aWpXA0wwS6BimT3UQizK55T1kt1B2Pi3KyPAw=
}

func ExampleBeginSponsoringFutureReserves_transfer() {
	test := InitSponsorshipTestConfig()

	asset, err := test.Assets[1].ToTrustLineAsset()
	check(err)

	transferOps := []Operation{
		&BeginSponsoringFutureReserves{
			SourceAccount: test.S2account.AccountID,
			SponsoredID:   test.S1.Address(),
		},
		&RevokeSponsorship{
			SponsorshipType: RevokeSponsorshipTypeTrustLine,
			Account:         &test.Aaccount.AccountID,
			TrustLine: &TrustLineID{
				Account: test.A.Address(),
				Asset:   asset,
			},
		},
		&EndSponsoringFutureReserves{},
	}

	// For transfers, both the old and new sponsor need to sign.
	txb64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &test.S1account,
			Operations:           transferOps,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			BaseFee:              MinBaseFee,
			IncrementSequenceNum: true,
		},
		network.TestNetworkPassphrase,
		test.S1,
		test.S2,
	)
	check(err)
	fmt.Println(txb64)

	// Output: AAAAAgAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAASwAAAAAAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAMAAAABAAAAAODcbeFyXKxmUWK1L6znNbKKIkPkHRJNbLktcKPqLnLFAAAAEAAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAAAAAAASAAAAAAAAAAEAAAAAtBUvDnYeMhUqWrHitbGDDFXU6VQiZspRiaTHmLvSzigAAAABRUZHSAAAAAB+Ecs01jX14asC1KAsPdWlpGbYCM2PEgFZCD3NLhVZmAAAAAAAAAARAAAAAAAAAAIuFVmYAAAAQDx6tSWzDT5MCVpolKLvhBwM/PpV9d/Om8PlJ4GZekp+DY6H2XAZ+Rldlfa0DqK8KNuMF921Vha6fpmK7FY4/QrqLnLFAAAAQCxxzLrpHFwd+CS6xmAoytq+ORtrkxUy2k6B7wIuASrlJDnYAHZptf7bBKXPn5ImcpJIcB3E5Xl98s/lEA0+YAA=
}

func ExampleRevokeSponsorship() {
	test := InitSponsorshipTestConfig()

	asset1, err := test.Assets[1].ToTrustLineAsset()
	check(err)

	asset2, err := test.Assets[2].ToTrustLineAsset()
	check(err)

	revokeOps := []Operation{
		&RevokeSponsorship{
			SponsorshipType: RevokeSponsorshipTypeTrustLine,
			Account:         &test.Aaccount.AccountID,
			TrustLine: &TrustLineID{
				Account: test.A.Address(),
				Asset:   asset1,
			},
		},
		&RevokeSponsorship{
			SponsorshipType: RevokeSponsorshipTypeTrustLine,
			Account:         &test.Aaccount.AccountID,
			TrustLine: &TrustLineID{
				Account: test.A.Address(),
				Asset:   asset2,
			},
		},
	}

	// With revocation, only the new sponsor needs to sign.
	txb64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &test.S2account,
			Operations:           revokeOps,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			BaseFee:              MinBaseFee,
			IncrementSequenceNum: true,
		},
		network.TestNetworkPassphrase,
		test.S2,
	)
	check(err)
	fmt.Println(txb64)

	// Output: AAAAAgAAAADg3G3hclysZlFitS+s5zWyiiJD5B0STWy5LXCj6i5yxQAAAMgAAAAAAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAAAAAAAEgAAAAAAAAABAAAAALQVLw52HjIVKlqx4rWxgwxV1OlUImbKUYmkx5i70s4oAAAAAUVGR0gAAAAAfhHLNNY19eGrAtSgLD3VpaRm2AjNjxIBWQg9zS4VWZgAAAAAAAAAEgAAAAAAAAABAAAAALQVLw52HjIVKlqx4rWxgwxV1OlUImbKUYmkx5i70s4oAAAAAUlKS0wAAAAA4Nxt4XJcrGZRYrUvrOc1sooiQ+QdEk1suS1wo+oucsUAAAAAAAAAAeoucsUAAABA9YO+xRc5Vb8ueP1U8go7ka+u/gZJd2z075c2pdFxYb+4AvQUQGvg+N4wvtNll43lPwXq5XAz74BfP99wugplDQ==
}

type LiquidityPoolTestConfig struct {
	A        *keypair.Full
	AAccount SimpleAccount
	Assets   []CreditAsset
}

func InitLiquidityPoolTestConfig() LiquidityPoolTestConfig {
	A := keypair.MustParseFull("SCZANGBA5YHTNYVVV4C3U252E2B6P6F5T3U6MM63WBSBZATAQI3EBTQ4")

	return LiquidityPoolTestConfig{
		A:        A,
		AAccount: SimpleAccount{AccountID: A.Address()},
		Assets: []CreditAsset{
			{Code: "ABCD", Issuer: A.Address()},
			{Code: "EFGH", Issuer: A.Address()},
			{Code: "IJKL", Issuer: A.Address()},
		},
	}
}

func ExampleLiquidityPoolDeposit() {
	test := InitLiquidityPoolTestConfig()

	poolId, err := NewLiquidityPoolId(test.Assets[0], test.Assets[1])
	check(err)

	depositOps := []Operation{
		// Change of trust the first time ensures that the pool exists.
		&ChangeTrust{
			Line: LiquidityPoolShareChangeTrustAsset{
				LiquidityPoolParameters: LiquidityPoolParameters{
					AssetA: test.Assets[0],
					AssetB: test.Assets[1],
					Fee:    LiquidityPoolFeeV18,
				},
			},
			SourceAccount: test.AAccount.AccountID,
			Limit:         MaxTrustlineLimit,
		},

		// Add our deposit to the pool
		&LiquidityPoolDeposit{
			SourceAccount:   test.A.Address(),
			LiquidityPoolID: poolId,
			MaxAmountA:      "0.1000000",
			MaxAmountB:      "0.1000000",
			MinPrice:        price.MustParse("0.1000000"),
			MaxPrice:        price.MustParse("0.1000000"),
		},
	}

	// With revocation, only the new sponsor needs to sign.
	txb64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &test.AAccount,
			Operations:           depositOps,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			BaseFee:              MinBaseFee,
			IncrementSequenceNum: true,
		},
		network.TestNetworkPassphrase,
		test.A,
	)
	check(err)
	fmt.Println(txb64)

	// Output: AAAAAgAAAAC0FS8Odh4yFSpaseK1sYMMVdTpVCJmylGJpMeYu9LOKAAAAMgAAAAAAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAIAAAABAAAAALQVLw52HjIVKlqx4rWxgwxV1OlUImbKUYmkx5i70s4oAAAABgAAAAMAAAAAAAAAAUFCQ0QAAAAAtBUvDnYeMhUqWrHitbGDDFXU6VQiZspRiaTHmLvSzigAAAABRUZHSAAAAAC0FS8Odh4yFSpaseK1sYMMVdTpVCJmylGJpMeYu9LOKAAAAB5//////////wAAAAEAAAAAtBUvDnYeMhUqWrHitbGDDFXU6VQiZspRiaTHmLvSzigAAAAWeDFASRchw0mM6/TK0AbZw13MsKye384/nzGgEZhWzzsAAAAAAA9CQAAAAAAAD0JAAAAAAQAAAAoAAAABAAAACgAAAAAAAAABu9LOKAAAAECsEeCUf0w62cgGpgaZxR2cb47Ln3jvfUOvTXl2sJkmEM3CIHIZzkFkMz7RZCRGn70DUjl5TXeow0zxipPL1K0H

}

func ExampleLiquidityPoolWithdraw() {
	test := InitLiquidityPoolTestConfig()

	poolId, err := NewLiquidityPoolId(test.Assets[0], test.Assets[1])
	check(err)

	withdrawOps := []Operation{
		&LiquidityPoolWithdraw{
			SourceAccount:   test.A.Address(),
			LiquidityPoolID: poolId,
			Amount:          "0.1000000",
			MinAmountA:      "0.1000000",
			MinAmountB:      "0.1000000",
		},
	}

	// With revocation, only the new sponsor needs to sign.
	txb64, err := newSignedTransaction(
		TransactionParams{
			SourceAccount:        &test.AAccount,
			Operations:           withdrawOps,
			Preconditions:        Preconditions{TimeBounds: NewInfiniteTimeout()},
			BaseFee:              MinBaseFee,
			IncrementSequenceNum: true,
		},
		network.TestNetworkPassphrase,
		test.A,
	)
	check(err)
	fmt.Println(txb64)

	// Output: AAAAAgAAAAC0FS8Odh4yFSpaseK1sYMMVdTpVCJmylGJpMeYu9LOKAAAAGQAAAAAAAAAAQAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAABAAAAALQVLw52HjIVKlqx4rWxgwxV1OlUImbKUYmkx5i70s4oAAAAF3gxQEkXIcNJjOv0ytAG2cNdzLCsnt/OP58xoBGYVs87AAAAAAAPQkAAAAAAAA9CQAAAAAAAD0JAAAAAAAAAAAG70s4oAAAAQHc2K5XVrm6+ICFt3xOrJFbXZXV4jhCZ2kruYJnJ/JJatgRZerVjiCp6BI37hcrd9CM3yTnb8McOSNXHmtgEMQM=
}
