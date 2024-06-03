package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"

	"controller/pkg/controller"
)

func TestControllerIntegration(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)

	ctrl := controller.NewController(clientset, dynClient)

	// Create a dummy TerraformConfig resource
	terraformConfig := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "alustan.io/v1alpha1",
			"kind":       "Terraform",
			"metadata": map[string]interface{}{
				"name":      "test-config",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"variables": map[string]string{
					"var1": "value1",
				},
				"backend": map[string]string{
					"provider": "aws",
				},
				"gitRepo": map[string]interface{}{
					"url":    "https://github.com/alustan/platform-template.git",
					"branch": "main",
				},
			},
		},
	}

	_, err := dynClient.Resource(schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "terraform",
	}).Namespace("default").Create(context.Background(), terraformConfig, metav1.CreateOptions{})
	require.NoError(t, err)

	// Start the reconcile loop
	go ctrl.Reconcile()

	// Wait for a few seconds to allow the reconciliation loop to process
	time.Sleep(5 * time.Second)

	// Verify the status of the TerraformConfig resource
	updatedResource, err := dynClient.Resource(schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpaha1",
		Resource: "terraform",
	}).Namespace("default").Get(context.Background(), "test-config", metav1.GetOptions{})
	require.NoError(t, err)

	status := updatedResource.Object["status"].(map[string]interface{})
	assert.Equal(t, "Success", status["state"])
	assert.Equal(t, "Terraform applied successfully", status["message"])
}
