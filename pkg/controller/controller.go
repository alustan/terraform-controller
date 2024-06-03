package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"controller/pkg/container"
	"controller/pkg/kubernetes"
	"controller/pkg/terraform"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynclient "k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	maxRetries   = 5
	syncInterval = 10 * time.Minute // Set sync interval to 10 minutes
)

type Controller struct {
	clientset *k8sclient.Clientset
	dynClient dynclient.Interface
}

type TerraformConfigSpec struct {
	Variables map[string]string `json:"variables"`
	Backend   map[string]string `json:"backend"`
	Scripts   struct {
		Apply   string `json:"apply"`
		Destroy string `json:"destroy"`
	} `json:"scripts"`
	GitRepo struct {
		URL           string `json:"url"`
		Branch        string `json:"branch"`
		SSHKey        string `json:"sshKey"`
	} `json:"gitRepo"`
}

type ParentResource struct {
	ApiVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   metav1.ObjectMeta      `json:"metadata"`
	Spec       TerraformConfigSpec    `json:"spec"`
	Status     map[string]interface{} `json:"status"`
}

type SyncRequest struct {
	Parent     ParentResource `json:"parent"`
	Finalizing bool           `json:"finalizing"`
}


func NewController(clientset *k8sclient.Clientset, dynClient dynclient.Interface) *Controller {
	return &Controller{
		clientset: clientset,
		dynClient: dynClient,
	}
}

func NewInClusterController() *Controller {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error creating in-cluster config: %v", err)
	}

	clientset, err := k8sclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes clientset: %v", err)
	}

	dynClient, err := dynclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating dynamic Kubernetes client: %v", err)
	}

	return NewController(clientset, dynClient)
}

func (c *Controller) ServeHTTP(r *gin.Context) {
	var observed SyncRequest
	err := json.NewDecoder(r.Request.Body).Decode(&observed)
	if err != nil {
		r.String(http.StatusBadRequest, err.Error())
		return
	}
	defer r.Request.Body.Close()

	c.handleSyncRequest(observed)
}

func (c *Controller) handleSyncRequest(observed SyncRequest) {
	envVars := terraform.ExtractEnvVars(observed.Parent.Spec.Variables, observed.Parent.Spec.Backend)

	script := observed.Parent.Spec.Scripts.Apply
	if observed.Finalizing {
		script = observed.Parent.Spec.Scripts.Destroy
	}

	repoDir := fmt.Sprintf("/tmp/%s", observed.Parent.Metadata.Name)
	err := terraform.CloneGitRepo(observed.Parent.Spec.GitRepo.URL, observed.Parent.Spec.GitRepo.Branch, observed.Parent.Spec.GitRepo.SSHKey, repoDir)
	if err != nil {
		log.Printf("Error cloning Git repository: %s\n", err.Error())
		return
	}

	provider := observed.Parent.Spec.Backend["provider"]
	if provider == "vault" {
		err = terraform.SetupVaultBackend(observed.Parent.Spec.Backend)
		if err != nil {
			log.Printf("Error setting up Vault backend: %s\n", err.Error())
			return
		}
	} else if provider == "aws" {
		err = terraform.SetupAWSBackend(observed.Parent.Spec.Backend)
		if err != nil {
			log.Printf("Error setting up AWS backend: %s\n", err.Error())
			return
		}
	} else {
		log.Printf("Unsupported backend provider: %s\n", provider)
		return
	}

	configMapName, err := container.CreateDockerfileConfigMap(c.clientset, observed.Parent.Metadata.Namespace, repoDir)
	if err != nil {
		log.Printf("Error creating Dockerfile ConfigMap: %s\n", err.Error())
		return
	}

	imageName := fmt.Sprintf("terraform-image-%s", observed.Parent.Metadata.Name)
	err = container.CreateBuildJob(c.clientset, observed.Parent.Metadata.Namespace, configMapName, imageName)
	if err != nil {
		log.Printf("Error creating build job: %s\n", err.Error())
		return
	}

	pvcName := "terraform-pvc"
	var terraformErr error
	for i := 0; i < maxRetries; i++ {
		terraformErr = container.CreateRunPod(c.clientset, observed.Parent.Metadata.Namespace, envVars, script, imageName,pvcName)
		if terraformErr == nil {
			break
		}
		log.Printf("Retrying Terraform command due to error: %s\n", terraformErr.Error())
		time.Sleep(2 * time.Second)
	}

	status := map[string]interface{}{
		"state":   "Success",
		"message": "Terraform applied successfully",
	}
	if terraformErr != nil {
		status["state"] = "Failed"
		status["message"] = terraformErr.Error()
	}

	err = kubernetes.UpdateStatus(c.dynClient, observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name, status)
	if err != nil {
		log.Printf("Error updating status: %s\n", err.Error())
		return
	}
}

func (c *Controller) Reconcile() {
	for {
		c.reconcileLoop()
		time.Sleep(syncInterval)
	}
}

func (c *Controller) reconcileLoop() {
	// Fetch all TerraformConfig custom resources
	resourceList, err := c.dynClient.Resource(schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "terraformconfigs",
	}).Namespace("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error fetching TerraformConfig resources: %s\n", err.Error())
		return
	}

	for _, item := range resourceList.Items {
		var observed SyncRequest
		// Unmarshal the item to observed
		raw, err := item.MarshalJSON()
		if err != nil {
			log.Printf("Error marshalling item: %s\n", err.Error())
			continue
		}
		err = json.Unmarshal(raw, &observed)
		if err != nil {
			log.Printf("Error unmarshalling item: %s\n", err.Error())
			continue
		}

		// Re-use the ServeHTTP logic
		c.handleSyncRequest(observed)
	}
}
