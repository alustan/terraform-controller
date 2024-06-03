package container

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateDockerfileConfigMap(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	namespace := "default"
	terraformDir := "/path/to/terraform"

	configMapName, err := CreateDockerfileConfigMap(clientset, namespace, terraformDir)
	assert.NoError(t, err)
	assert.Equal(t, "dockerfile-configmap", configMapName)

	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Contains(t, configMap.Data["Dockerfile"], "FROM ubuntu:latest")
}
