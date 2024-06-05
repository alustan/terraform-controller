package controller_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"controller/pkg/controller"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestServeHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	clientset := k8sfake.NewSimpleClientset()
	scheme := runtime.NewScheme()
	dynClient := dynfake.NewSimpleDynamicClient(scheme, &unstructured.Unstructured{})

	ctrl := controller.NewController(clientset, dynClient)

	router := gin.Default()
	router.POST("/sync", ctrl.ServeHTTP)

	t.Run("Successful Sync Request", func(t *testing.T) {
		syncRequest := controller.SyncRequest{
			Parent: controller.ParentResource{
				ApiVersion: "alustan.io/v1alpha1",
				Kind:       "Terraform",
				Metadata: metav1.ObjectMeta{
					Name:      "test-resource",
					Namespace: "default",
				},
				Spec: controller.TerraformConfigSpec{
					Variables: map[string]string{
						"var1": "value1",
					},
					Backend: map[string]string{
						"provider": "aws",
					},
					Scripts: controller.Scripts{
						Apply: controller.Script{
							Inline: "apply script content",
						},
					},
					GitRepo: controller.GitRepo{
						URL:    "git@github.com:example/test.git",
						Branch: "main",
						SSHKeySecret: controller.SSHKeySecret{
							Name: "ssh-secret",
							Key:  "ssh-key",
						},
					},
					ContainerRegistry: controller.ContainerRegistry{
						ImageName: "example/image",
						SecretRef: controller.SecretRef{
							Name: "registry-secret",
							Key:  "secret-key",
						},
					},
				},
			},
			Finalizing: false,
		}

		w := httptest.NewRecorder()
		body, _ := json.Marshal(syncRequest)
		req, _ := http.NewRequest(http.MethodPost, "/sync", bytes.NewBuffer(body))
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Terraform applied successfully")
	})

	t.Run("Bad Request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/sync", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "EOF")
	})
}

func TestHandleSyncRequest(t *testing.T) {
	clientset := k8sfake.NewSimpleClientset()
	scheme := runtime.NewScheme()
	dynClient := dynfake.NewSimpleDynamicClient(scheme, &unstructured.Unstructured{})

	ctrl := controller.NewController(clientset, dynClient)

	t.Run("Handle Sync Request with Apply Script", func(t *testing.T) {
		syncRequest := controller.SyncRequest{
			Parent: controller.ParentResource{
				ApiVersion: "alustan.io/v1alpha1",
				Kind:       "Terraform",
				Metadata: metav1.ObjectMeta{
					Name:      "test-resource",
					Namespace: "default",
				},
				Spec: controller.TerraformConfigSpec{
					Variables: map[string]string{
						"var1": "value1",
					},
					Backend: map[string]string{
						"provider": "aws",
					},
					Scripts: controller.Scripts{
						Apply: controller.Script{
							Inline: "apply script content",
						},
					},
					GitRepo: controller.GitRepo{
						URL:    "git@github.com:example/repo.git",
						Branch: "main",
						SSHKeySecret: controller.SSHKeySecret{
							Name: "ssh-secret",
							Key:  "ssh-key",
						},
					},
					ContainerRegistry: controller.ContainerRegistry{
						ImageName: "example/image",
						SecretRef: controller.SecretRef{
							Name: "registry-secret",
							Key:  "secret-key",
						},
					},
				},
			},
			Finalizing: false,
		}

		ctrl.HandleSyncRequest(syncRequest)
		// Add assertions to check for expected behavior
	})
}
