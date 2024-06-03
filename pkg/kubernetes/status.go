package kubernetes

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UpdateStatus(dynClient dynamic.Interface, namespace, name string, status map[string]interface{}) error {
	resource := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "terraformconfigs",
	}

	// Fetch the existing resource
	unstructuredResource, err := dynClient.Resource(resource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get resource: %v", err)
	}

	// Update the status
	unstructuredResource.Object["status"] = status

	// Update the resource with the new status
	_, err = dynClient.Resource(resource).Namespace(namespace).UpdateStatus(context.Background(), unstructuredResource, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update status: %v", err)
	}

	return nil
}
