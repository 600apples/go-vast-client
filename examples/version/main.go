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

	version, err := rest.Versions.GetVersion(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Println(version)
}
