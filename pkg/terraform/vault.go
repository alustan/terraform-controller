package terraform

import (
	"fmt"
	vault "github.com/hashicorp/vault/api"
)

func SetupVaultBackend(backendConfig map[string]string) error {
	client, err := createVaultClient(backendConfig["vaultAddress"])
	if err != nil {
		return fmt.Errorf("failed to create Vault client: %v", err)
	}

	// Assuming Vault is configured with a Key-Value Secrets Engine named "terraform"
	path := "secret/data/terraform"

	// Write the backend configuration to Vault
	_, err = client.Logical().Write(path, map[string]interface{}{
		"data": backendConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to write backend configuration to Vault: %v", err)
	}

	fmt.Println("Successfully configured Vault backend for Terraform")
	return nil
}

func createVaultClient(address string) (*vault.Client, error) {
	config := &vault.Config{
		Address: address,
	}
	client, err := vault.NewClient(config)
	if err != nil {
		return nil, err
	}

	// Set the token for authentication
	// This example assumes that the token is provided in the environment variable VAULT_TOKEN
	token := client.Token()
	if token == "" {
		return nil, fmt.Errorf("no Vault token found in environment")
	}

	return client, nil
}
