package stellarcore

import (
	"context"
	"fmt"
)

func ExampleClient_Info() {
	client := &Client{URL: "http://localhost:11626"}

	info, err := client.Info(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Printf("synced: %v", info.IsSynced())
}
