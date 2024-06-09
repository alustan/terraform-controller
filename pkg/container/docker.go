package container

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

// DeleteConfigMapIfExists deletes the ConfigMap if it already exists.
func DeleteConfigMapIfExists(clientset *kubernetes.Clientset, namespace, configMapName string) error {
	// Check if the ConfigMap exists
	_, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	if err == nil {
		// ConfigMap exists, retry deletion up to 5 times with a 1-minute interval
		maxAttempts := 5
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			err := clientset.CoreV1().ConfigMaps(namespace).Delete(context.Background(), configMapName, metav1.DeleteOptions{})
			if err == nil {
				log.Printf("Deleted existing ConfigMap: %s", configMapName)
				break
			} else if apierrors.IsNotFound(err) {
				log.Printf("No existing ConfigMap to delete: %s", configMapName)
				break
			} else {
				log.Printf("Attempt %d: Failed to delete existing ConfigMap: %v", attempt, err)
				if attempt < maxAttempts {
					time.Sleep(1 * time.Minute)
				} else {
					log.Printf("Max attempts reached. Giving up on deleting ConfigMap: %s", configMapName)
					return err
				}
			}
		}
	} else if !apierrors.IsNotFound(err) {
		log.Printf("Failed to get ConfigMap: %v", err)
		return err
	} else {
		log.Printf("No existing ConfigMap to delete: %s", configMapName)
	}
	return nil
}

// CreateDockerfileConfigMap creates a Kubernetes ConfigMap with the provided Dockerfile content.
func CreateDockerfileConfigMap(clientset *kubernetes.Clientset, name, namespace, additionalTools string, providerExists bool) (string, error) {
	// Initialize Dockerfile content
	content := `
FROM ubuntu:latest

RUN apt-get update && \
    apt-get install -y \
    wget \
    curl \
    git \
    unzip \
    jq \
    openssh-client \
    && rm -rf /var/lib/apt/lists/*

RUN wget https://releases.hashicorp.com/terraform/1.8.1/terraform_1.8.1_linux_amd64.zip && \
    unzip terraform_1.8.1_linux_amd64.zip -d /usr/local/bin/ && \
    rm terraform_1.8.1_linux_amd64.zip

RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && \
    rm kubectl
`

	// Include additionalTools if the provider exists
	if providerExists {
		content += additionalTools
	}

	// Append default content to the Dockerfile
	content += `
WORKDIR /app

COPY . ./

CMD ["/bin/bash"]
`

	configMapName := fmt.Sprintf("%s-dockerfile-configmap", name)

	// Attempt to delete the existing ConfigMap if it exists
	err := DeleteConfigMapIfExists(clientset, namespace, configMapName)
	if err != nil {
		return "", err
	}

	// Create the new ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: configMapName,
		},
		Data: map[string]string{
			"Dockerfile": content,
		},
	}

	_, err = clientset.CoreV1().ConfigMaps(namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create ConfigMap: %v", err)
		return "", err
	}

	log.Printf("Created ConfigMap: %s", configMapName)
	return configMapName, nil
}
