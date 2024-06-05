package terraform_test

import (
	"errors"
	"testing"

	"controller/pkg/util"
	"controller/pkg/terraform"
	"k8s.io/client-go/kubernetes"
    "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// MockClient is a mock implementation of the util package's GetConfigMapContent function.
type MockClient struct {
	mock.Mock
}

func (m *MockClient) GetConfigMapContent(clientset *kubernetes.Clientset, namespace, name, key string) (string, error) {
	args := m.Called(clientset, namespace, name, key)
	return args.String(0), args.Error(1)
}

func TestExtractScriptContent(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	namespace := "default"

	t.Run("extract inline script", func(t *testing.T) {
		script := map[string]interface{}{
			"inline": "echo Hello, World!",
		}

		content, err := terraform.ExtractScriptContent(clientset, namespace, script)
		assert.NoError(t, err)
		assert.Equal(t, "echo Hello, World!", content)
	})

	t.Run("extract script from ConfigMap", func(t *testing.T) {
		mockClient := new(MockClient)
		util.GetConfigMapContent = mockClient.GetConfigMapContent

		script := map[string]interface{}{
			"configMapRef": map[string]interface{}{
				"name": "my-configmap",
				"key":  "my-key",
			},
		}

		mockClient.On("GetConfigMapContent", clientset, namespace, "my-configmap", "my-key").Return("configmap content", nil)

		content, err := terraform.ExtractScriptContent(clientset, namespace, script)
		assert.NoError(t, err)
		assert.Equal(t, "configmap content", content)

		mockClient.AssertExpectations(t)
	})

	t.Run("missing script definition", func(t *testing.T) {
		script := map[string]interface{}{}

		content, err := terraform.ExtractScriptContent(clientset, namespace, script)
		assert.Error(t, err)
		assert.Equal(t, "", content)
	})

	t.Run("missing name or key in ConfigMap reference", func(t *testing.T) {
		script := map[string]interface{}{
			"configMapRef": map[string]interface{}{
				"key": "my-key",
			},
		}

		content, err := terraform.ExtractScriptContent(clientset, namespace, script)
		assert.Error(t, err)
		assert.Equal(t, "", content)
	})

	t.Run("error retrieving ConfigMap content", func(t *testing.T) {
		mockClient := new(MockClient)
		util.GetConfigMapContent = mockClient.GetConfigMapContent

		script := map[string]interface{}{
			"configMapRef": map[string]interface{}{
				"name": "my-configmap",
				"key":  "my-key",
			},
		}

		mockClient.On("GetConfigMapContent", clientset, namespace, "my-configmap", "my-key").Return("", errors.New("failed to retrieve configmap content"))

		content, err := terraform.ExtractScriptContent(clientset, namespace, script)
		assert.Error(t, err)
		assert.Equal(t, "", content)

		mockClient.AssertExpectations(t)
	})
}
