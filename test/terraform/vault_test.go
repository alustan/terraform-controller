package terraform_test

import (
	"controller/pkg/terraform"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupVaultBackend(t *testing.T) {
	backendConfig := map[string]string{
		"address": "http://127.0.0.1:8200",
	}

	err := terraform.SetupVaultBackend(backendConfig)
	assert.NoError(t, err)
}
