package test

import (
	"bytes"
	"controller/pkg/controller"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/dynamic/fake"
)

func TestServeHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	clientset := k8sfake.NewSimpleClientset()
	dynClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	ctrl := controller.NewController(clientset, dynClient)

	r := gin.Default()
	r.POST("/sync", ctrl.ServeHTTP)

	tests := []struct {
		name           string
		input          controller.SyncRequest
		expectedStatus int
	}{
		{
			name: "ValidRequest",
			input: controller.SyncRequest{
				Parent: controller.ParentResource{
					ApiVersion: "v1",
					Kind:       "ParentResource",
					Metadata: metav1.ObjectMeta{
						Name: "example",
					},
					Spec: controller.TerraformConfigSpec{
						Variables: map[string]string{
							"TF_VAR_example": "value",
						},
						Backend: map[string]string{
							"bucket":         "mybucket",
							"dynamodb_table": "mytable",
							"region":         "us-west-2",
						},
						Scripts: struct {
							Apply   string `json:"apply"`
							Destroy string `json:"destroy"`
						}{
							Apply: "apply.sh",
						},
						GitRepo: struct {
							URL    string `json:"url"`
							Branch string `json:"branch"`
							SSHKey string `json:"sshKey"`
						}{
							URL:    "https://github.com/example/repo.git",
							Branch: "main",
							SSHKey: "ssh-key",
						},
					},
					Status: map[string]interface{}{},
				},
				Finalizing: false,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "InvalidRequest",
			input: controller.SyncRequest{
				Parent: controller.ParentResource{
					ApiVersion: "v1",
					Kind:       "ParentResource",
					Metadata: metav1.ObjectMeta{
						Name: "example",
					},
					Spec: controller.TerraformConfigSpec{
						Variables: map[string]string{},
						Backend:   map[string]string{},
						Scripts: struct {
							Apply   string `json:"apply"`
							Destroy string `json:"destroy"`
						}{},
						GitRepo: struct {
							URL    string `json:"url"`
							Branch string `json:"branch"`
							SSHKey string `json:"sshKey"`
						}{},
					},
					Status: map[string]interface{}{},
				},
				Finalizing: false,
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.input)
			assert.NoError(t, err)

			req, err := http.NewRequest("POST", "/sync", bytes.NewBuffer(body))
			assert.NoError(t, err)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
