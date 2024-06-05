package util_test

import (
	"context"
	"testing"
	"time"

	"controller/pkg/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// MockClientSet is a mock implementation of the Kubernetes ClientSet.
type MockClientSet struct {
	mock.Mock
}

func (m *MockClientSet) CoreV1() corev1.CoreV1Interface {
	return &MockCoreV1{}
}

type MockCoreV1 struct {
	mock.Mock
}

func (m *MockCoreV1) ConfigMaps(namespace string) corev1.ConfigMapInterface {
	return &MockConfigMap{}
}

func (m *MockCoreV1) Secrets(namespace string) corev1.SecretInterface {
	return &MockSecret{}
}

func (m *MockCoreV1) PersistentVolumes() corev1.PersistentVolumeInterface {
	return nil
}

func (m *MockCoreV1) PersistentVolumeClaims(namespace string) corev1.PersistentVolumeClaimInterface {
	return nil
}

func (m *MockCoreV1) Services(namespace string) corev1.ServiceInterface {
	return nil
}

func (m *MockCoreV1) Endpoints(namespace string) corev1.EndpointsInterface {
	return nil
}

func (m *MockCoreV1) Nodes() corev1.NodeInterface {
	return nil
}

func (m *MockCoreV1) Namespaces() corev1.NamespaceInterface {
	return nil
}

func (m *MockCoreV1) Pods(namespace string) corev1.PodInterface {
	return nil
}

func (m *MockCoreV1) LimitRanges(namespace string) corev1.LimitRangeInterface {
	return nil
}

func (m *MockCoreV1) ComponentStatuses() corev1.ComponentStatusInterface {
	return nil
}

func (m *MockCoreV1) Events(namespace string) corev1.EventInterface {
	return nil
}

func (m *MockCoreV1) PodTemplates(namespace string) corev1.PodTemplateInterface {
	return nil
}

type MockConfigMap struct {
	mock.Mock
}

func (m *MockConfigMap) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.ConfigMap, error) {
	args := m.Called(ctx, name, opts)
	return args.Get(0).(*v1.ConfigMap), args.Error(1)
}

func (m *MockConfigMap) Apply(ctx context.Context, configMap *v1.ConfigMap, opts metav1.ApplyOptions) (*v1.ConfigMap, error) {
	return nil, nil
}

func (m *MockConfigMap) Create(ctx context.Context, configMap *v1.ConfigMap, opts metav1.CreateOptions) (*v1.ConfigMap, error) {
	return nil, nil
}

func (m *MockConfigMap) Update(ctx context.Context, configMap *v1.ConfigMap, opts metav1.UpdateOptions) (*v1.ConfigMap, error) {
	return nil, nil
}

func (m *MockConfigMap) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return nil
}

func (m *MockConfigMap) List(ctx context.Context, opts metav1.ListOptions) (*v1.ConfigMapList, error) {
	return nil, nil
}

func (m *MockConfigMap) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}

func (m *MockConfigMap) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, subresources ...string) (*v1.ConfigMap, error) {
	return nil, nil
}

type MockSecret struct {
	mock.Mock
}

func (m *MockSecret) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Secret, error) {
	args := m.Called(ctx, name, opts)
	return args.Get(0).(*v1.Secret), args.Error(1)
}

func (m *MockSecret) Apply(ctx context.Context, secret *v1.Secret, opts metav1.ApplyOptions) (*v1.Secret, error) {
	return nil, nil
}

func (m *MockSecret) Create(ctx context.Context, secret *v1.Secret, opts metav1.CreateOptions) (*v1.Secret, error) {
	return nil, nil
}

func (m *MockSecret) Update(ctx context.Context, secret *v1.Secret, opts metav1.UpdateOptions) (*v1.Secret, error) {
	return nil, nil
}

func (m *MockSecret) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return nil
}

func (m *MockSecret) List(ctx context.Context, opts metav1.ListOptions) (*v1.SecretList, error) {
	return nil, nil
}

func (m *MockSecret) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}

func (m *MockSecret) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, subresources ...string) (*v1.Secret, error) {
	return nil, nil
}

func TestExtractEnvVars(t *testing.T) {
	variables := map[string]string{"VAR1": "value1", "VAR2": "value2"}
	backend := map[string]string{"VAR2": "override", "VAR3": "value3"}

	envVars := util.ExtractEnvVars(variables, backend)
	expected := map[string]string{"VAR1": "value1", "VAR2": "override", "VAR3": "value3"}

	assert.Equal(t, expected, envVars)
}

func TestGetConfigMapContent(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	namespace := "default"
	name := "test-configmap"
	key := "test-key"
	expectedContent := "test-content"

	clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       map[string]string{key: expectedContent},
	}, metav1.CreateOptions{})

	content, err := util.GetConfigMapContent(clientset, namespace, name, key)
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, content)
}

func TestGetDataFromSecret(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	namespace := "default"
	secretName := "test-secret"
	keyName := "ssh-key"
	expectedKey := "ssh-key-content"

	clientset.CoreV1().Secrets(namespace).Create(context.TODO(), &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName},
		Data:       map[string][]byte{keyName: []byte(expectedKey)},
	}, metav1.CreateOptions{})

	key, err := util.GetDataFromSecret(clientset, namespace, secretName, keyName)
	assert.NoError(t, err)
	assert.Equal(t, expectedKey, key)
}

func TestGetSyncInterval(t *testing.T) {
	defaultValue := util.GetSyncInterval()
	assert.Equal(t, 10*time.Minute, defaultValue)

	t.Setenv("SYNC_INTERVAL", "15m")
	interval := util.GetSyncInterval()
	assert.Equal(t, 15*time.Minute, interval)

	t.Setenv("SYNC_INTERVAL", "invalid")
	interval = util.GetSyncInterval()
	assert.Equal(t, 10*time.Minute, interval)
}
