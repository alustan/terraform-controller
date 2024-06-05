package container_test

import (
	"context"
	"testing"

	"controller/pkg/container"
	"controller/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/dynamic/fake" 
	"github.com/stretchr/testify/assert"
)

func TestCreateDockerfileConfigMap(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	dynClient := fake.NewSimpleDynamicClient(runtime.NewScheme()) // Create a fake dynamic client
	ctrl := controller.NewTestController(clientset, dynClient) // Create the controller with fake clients

	namespace := "test-namespace"
	terraformDir := "test-dir"

	configMapName, err := container.CreateDockerfileConfigMap(ctrl.clientset, namespace, terraformDir)
	assert.NoError(t, err)
	assert.Equal(t, "dockerfile-configmap", configMapName)

	configMap, err := ctrl.clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, configMap)
	assert.Contains(t, configMap.Data["Dockerfile"], terraformDir)
}

func TestCreateBuildJob(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	dynClient := fake.NewSimpleDynamicClient(runtime.NewScheme()) // Create a fake dynamic client
	ctrl := controller.NewTestController(clientset, dynClient) // Create the controller with fake clients

	namespace := "test-namespace"
	configMapName := "test-configmap"
	imageName := "test-image"
	dockerSecretName := "test-secret"

	err := container.CreateBuildJob(ctrl.clientset, namespace, configMapName, imageName, dockerSecretName)
	assert.NoError(t, err)

	job, err := ctrl.clientset.BatchV1().Jobs(namespace).Get(context.Background(), "docker-build-job", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.Equal(t, "docker-build-job", job.Name)
}

func TestEnsurePVC(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	dynClient := fake.NewSimpleDynamicClient(runtime.NewScheme()) // Create a fake dynamic client
	ctrl := controller.NewTestController(clientset, dynClient) // Create the controller with fake clients

	namespace := "test-namespace"
	pvcName := "test-pvc"

	err := container.EnsurePVC(ctrl.clientset, namespace, pvcName)
	assert.NoError(t, err)

	pvc, err := ctrl.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, pvc)
	assert.Equal(t, pvcName, pvc.Name)
	assert.Equal(t, "5Gi", pvc.Spec.Resources.Requests[corev1.ResourceStorage].String())
}

func TestCreateRunPod(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	dynClient := fake.NewSimpleDynamicClient(runtime.NewScheme()) // Create a fake dynamic client
	ctrl := controller.NewTestController(clientset, dynClient) // Create the controller with fake clients

	namespace := "test-namespace"
	envVars := map[string]string{"ENV_VAR": "value"}
	script := "test-script.sh"
	imageName := "test-image"
	pvcName := "test-pvc"
	imagePullSecretName := "test-secret"

	clientset.PrependReactor("get", "persistentvolumeclaims", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvcName,
			},
		}, nil
	})

	err := container.CreateRunPod(ctrl.clientset, namespace, envVars, script, imageName, pvcName, imagePullSecretName)
	assert.NoError(t, err)

	pod, err := ctrl.clientset.CoreV1().Pods(namespace).Get(context.Background(), "docker-run-pod", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, pod)
	assert.Equal(t, "docker-run-pod", pod.Name)
	assert.Equal(t, imageName, pod.Spec.Containers[0].Image)
	assert.Contains(t, pod.Spec.Containers[0].Env, corev1.EnvVar{Name: "ENV_VAR", Value: "value"})
}
