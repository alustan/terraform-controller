package kubernetes_test

import (
	"context"
	"controller/pkg/kubernetes"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "TerraformConfig",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
			},
			"status": map[string]interface{}{},
		},
	}

	_, err := client.Resource(schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "terraformconfigs",
	}).Namespace("default").Create(context.Background(), resource, metav1.CreateOptions{})
	assert.NoError(t, err)

	status := map[string]interface{}{
		"state":   "Success",
		"message": "Terraform applied successfully",
	}

	err = kubernetes.UpdateStatus(client, "default", "test", status)
	assert.NoError(t, err)
}
