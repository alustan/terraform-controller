package vault

import (
    vaultapi "github.com/hashicorp/vault/api"
)

type VaultClient struct {
    client *vaultapi.Client
}

func NewVaultClient(client *vaultapi.Client) *VaultClient {
    return &VaultClient{client: client}
}

func (v *VaultClient) ReadSecret(path string) (map[string]interface{}, error) {
    secret, err := v.client.Logical().Read(path)
    if err != nil {
        return nil, err
    }
    return secret.Data, nil
}

func (v *VaultClient) WriteSecret(path string, data map[string]interface{}) error {
    _, err := v.client.Logical().Write(path, data)
    return err
}
