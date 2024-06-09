package container

import (
	"context"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

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
    // Create ConfigMap with the Dockerfile content
    configMap := &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name: configMapName,
        },
        Data: map[string]string{
            "Dockerfile": content,
        },
    }

    // Check if the ConfigMap exists
    _, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
    if err == nil {
        // ConfigMap exists, delete it
        err := clientset.CoreV1().ConfigMaps(namespace).Delete(context.Background(), configMapName, metav1.DeleteOptions{})
        if err != nil {
            log.Printf("Failed to delete existing ConfigMap: %v", err)
            return "", err
        }
        log.Printf("Deleted existing ConfigMap: %s", configMapName)
    } else if !apierrors.IsNotFound(err) {
        log.Printf("Failed to get ConfigMap: %v", err)
        return "", err
    } else {
        log.Printf("No existing ConfigMap to delete: %s", configMapName)
    }

    // Create the new ConfigMap
    _, err = clientset.CoreV1().ConfigMaps(namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
    if err != nil {
        log.Printf("Failed to create ConfigMap: %v", err)
        return "", err
    }

    log.Printf("Created ConfigMap: %s", configMapName)
    return configMap.Name, nil
}
