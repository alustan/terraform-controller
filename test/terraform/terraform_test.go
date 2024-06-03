package terraform_test

import (
	"controller/pkg/terraform"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractEnvVars(t *testing.T) {
	variables := map[string]string{"key1": "value1"}
	backend := map[string]string{"backend1": "value2"}
	envVars := terraform.ExtractEnvVars(variables, backend)

	expected := map[string]string{"key1": "value1", "backend1": "value2"}
	assert.Equal(t, expected, envVars)
}
