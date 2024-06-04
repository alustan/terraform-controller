package container

import (
	"context"
	"log"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateDockerfileConfigMap(clientset *kubernetes.Clientset, namespace, terraformDir string) (string, error) {
	content := fmt.Sprintf(`
FROM ubuntu:latest

RUN apt-get update && \\
    apt-get install -y \\
    wget \\
    curl \\
    git \\
    unzip \\
    jq \\
    openssh-client \\
    && rm -rf /var/lib/apt/lists/*

RUN wget https://releases.hashicorp.com/terraform/1.8.1/terraform_1.8.1_linux_amd64.zip && \\
    unzip terraform_1.8.1_linux_amd64.zip -d /usr/local/bin/ && \\
    rm terraform_1.8.1_linux_amd64.zip

RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \\
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && \\
    rm kubectl

RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \\
    apt install unzip && \\
    unzip awscliv2.zip && \\
    ./aws/install && \\
    rm -rf awscliv2.zip aws

WORKDIR /app

COPY %s/. ./

CMD ["/bin/bash"]
`, terraformDir)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dockerfile-configmap",
		},
		Data: map[string]string{
			"Dockerfile": content,
		},
	}

	_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create ConfigMap: %v", err)
		return "", err
	}

	return configMap.Name, nil
}
