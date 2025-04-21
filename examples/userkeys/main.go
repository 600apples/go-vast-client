package main

import (
	"context"
	"fmt"
	client "github.com/600apples/go-vast-client/pkg/vast_client"
)

func main() {
	ctx := context.Background()
	config := &client.VMSConfig{
		Host:     "10.27.40.1",
		Username: "admin",
		Password: "123456",
	}
	rest := client.NewVMSRest(config)

	result, err := rest.UserKeys.CreateKey(ctx, 1)
	if err != nil {
		panic(err)
	}
	accessKey := result["access_key"].(string)

	fmt.Printf("access key: %s\n", accessKey)

	if _, err = rest.UserKeys.DeleteKey(ctx, 1, accessKey); err != nil {
		panic(err)
	}
}
