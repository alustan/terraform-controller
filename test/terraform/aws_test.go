package terraform_test

import (
	"controller/pkg/terraform"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupAWSBackend(t *testing.T) {
	backendConfig := map[string]string{
		"region":        "us-west-2",
		"s3":        "test-bucket",
		"dynamoDB": "test-table",
	}

	err := terraform.SetupAWSBackend(backendConfig)
	assert.NoError(t, err)
}
