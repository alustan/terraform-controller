package container

import (
    "context"
    "encoding/base64"
    "fmt"
    "log"

    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// CreateDockerConfigSecret creates a Kubernetes Secret of type kubernetes.io/dockerconfigjson
// dockerConfigJSON should be base64-encoded JSON string.
func CreateDockerConfigSecret(clientset *kubernetes.Clientset, secretName, namespace, encodedDockerConfigJSON string) error {
    // Decode the base64 string to verify it's correct
    decodedData, err := base64.StdEncoding.DecodeString(encodedDockerConfigJSON)
    if err != nil {
        return fmt.Errorf("invalid base64 encoded docker config JSON: %v", err)
    }

    // Log decoded data for debugging
    log.Printf("Decoded Docker Config JSON: %s\n", string(decodedData))

    // Define the secret
    secret := &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      secretName,
            Namespace: namespace,
        },
        Data: map[string][]byte{
            ".dockerconfigjson": decodedData,
        },
        Type: corev1.SecretTypeDockerConfigJson,
    }

    // Attempt to create the secret
    _, err = clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
    if err != nil {
        // If the secret already exists, update it
        if apierrors.IsAlreadyExists(err) {
            log.Printf("Secret %s already exists, updating it", secretName)
            _, err = clientset.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
            if err != nil {
                return fmt.Errorf("failed to update existing secret: %v", err)
            }
        } else {
            return fmt.Errorf("failed to create secret: %v", err)
        }
    }

    return nil
}
