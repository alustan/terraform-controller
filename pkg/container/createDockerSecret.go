package container

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateDockerConfigSecret creates a Kubernetes Secret of type kubernetes.io/dockerconfigjson
// dockerConfigJSON should be base64-encoded JSON string.
func CreateDockerConfigSecret(clientset *kubernetes.Clientset, secretName, namespace, encodedDockerConfigJSON string) error {
	// Define the secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte(encodedDockerConfigJSON),
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}

	// Create the secret in the Kubernetes cluster
	_, err := clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret: %v", err)
	}

	return nil
}
