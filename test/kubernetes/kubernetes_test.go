package kubernetes_test

import (
	"context"
	"testing"

	"controller/pkg/kubernetes"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

func TestUpdateStatus(t *testing.T) {
	namespace := "test-namespace"
	name := "test-name"
	status := map[string]interface{}{
		"phase": "Running",
	}

	resource := schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "terraforms",
	}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	dynClient := fake.NewSimpleDynamicClient(scheme)

	// Create an unstructured object to be returned by the fake client
	existingResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "alustan.io/v1alpha1",
			"kind":       "Terraform",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"status": map[string]interface{}{
				"phase": "Pending",
			},
		},
	}

	// Add the resource to the fake client
	_, err := dynClient.Resource(resource).Namespace(namespace).Create(context.Background(), existingResource, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Call the UpdateStatus function
	err = kubernetes.UpdateStatus(dynClient, namespace, name, status)
	assert.NoError(t, err)

	// Fetch the updated resource
	updatedResource, err := dynClient.Resource(resource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	assert.NoError(t, err)

	// Verify the status has been updated
	updatedStatus, found, err := unstructured.NestedMap(updatedResource.Object, "status")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, status, updatedStatus)
}

func TestUpdateStatusResourceNotFound(t *testing.T) {
	namespace := "test-namespace"
	name := "test-name"
	status := map[string]interface{}{
		"phase": "Running",
	}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	dynClient := fake.NewSimpleDynamicClient(scheme)

	// Call the UpdateStatus function
	err := kubernetes.UpdateStatus(dynClient, namespace, name, status)

	// Verify an error is returned for a non-existent resource
	assert.Error(t, err)
}

func TestUpdateStatusUpdateFails(t *testing.T) {
	namespace := "test-namespace"
	name := "test-name"
	status := map[string]interface{}{
		"phase": "Running",
	}

	resource := schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "terraforms",
	}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	dynClient := fake.NewSimpleDynamicClient(scheme)

	// Create an unstructured object to be returned by the fake client
	existingResource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "alustan.io/v1alpha1",
			"kind":       "Terraform",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"status": map[string]interface{}{
				"phase": "Pending",
			},
		},
	}

	// Add the resource to the fake client
	_, err := dynClient.Resource(resource).Namespace(namespace).Create(context.Background(), existingResource, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Call the UpdateStatus function
	err = kubernetes.UpdateStatus(dynClient, namespace, name, status)

	// Verify an error is returned for the update failure
	assert.Error(t, err)
}
