package main

import (
	"context"
	"fmt"
	client "github.com/600apples/go-vast-client/pkg/vast_client"
)

type ViewContainer struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	TenantID int64  `json:"tenant_id"`
}

func main() {
	ctx := context.Background()
	config := &client.VMSConfig{
		Host:     "10.27.40.1", // replace with your VAST address
		Username: "admin",
		Password: "123456",
	}

	rest := client.NewVMSRest(config)

	// --- CREATE ---
	createParams := client.Params{
		"name":       "myview",
		"path":       "/myview",
		"create_dir": true,
		"policy_id":  1,
		"protocols":  []string{"NFS"},
	}
	_, err := rest.Views.Create(ctx, createParams)
	if err != nil {
		panic(fmt.Errorf("failed to create view: %w", err))
	}
	fmt.Println("View created successfully.")

	// --- UPDATE ---
	updateParams := client.Params{
		"protocols": []string{"NFS", "NFS4"},
	}
	_, err = rest.Views.Update(ctx, 5, updateParams)
	if err != nil {
		panic(fmt.Errorf("failed to update view: %w", err))
	}
	fmt.Println("View updated successfully.")

	// --- GET + DESERIALIZE ---
	result, err := rest.Views.Get(ctx, client.Params{
		"path__endswith": "view",
		"tenant_id":      1,
	})
	if err != nil {
		panic(fmt.Errorf("failed to get view: %w", err))
	}

	var view ViewContainer
	if err := result.Fill(&view); err != nil {
		panic(fmt.Errorf("failed to fill ViewContainer: %w", err))
	}
	fmt.Printf("Fetched view: %+v\n", view)

	// --- DELETE ---
	_, err = rest.Views.Delete(ctx, client.Params{
		"path__endswith": "view",
	})
	if err != nil {
		panic(fmt.Errorf("failed to delete view: %w", err))
	}
	fmt.Println("View deleted successfully.")
}
