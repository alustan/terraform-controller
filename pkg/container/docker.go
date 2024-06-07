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

func CreateDockerfileConfigMap(clientset *kubernetes.Clientset, name, namespace, terraformDir, additionalTools string, providerExists bool) (string, error) {
    // Initialize Dockerfile content with the terraformDir
    content := fmt.Sprintf(`
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
`)

    // Include additionalTools if the provider exists
    if providerExists {
        content += additionalTools
    }

    // Append terraformDir to the Dockerfile content
    content += fmt.Sprintf(`
WORKDIR /app

COPY %s/. ./

CMD ["/bin/bash"]
`, terraformDir)

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

   
    existingConfigMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
    if err != nil {
        if !apierrors.IsNotFound(err) {
            log.Printf("Failed to get ConfigMap: %v", err)
            return "", err
        }

        // ConfigMap does not exist, create it
        _, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
        if err != nil {
            log.Printf("Failed to create ConfigMap: %v", err)
            return "", err
        }
    } else {
        // ConfigMap exists, update it
        existingConfigMap.Data = configMap.Data
        _, err := clientset.CoreV1().ConfigMaps(namespace).Update(context.Background(), existingConfigMap, metav1.UpdateOptions{})
        if err != nil {
            log.Printf("Failed to update ConfigMap: %v", err)
            return "", err
        }
    }

    return configMap.Name, nil
}


