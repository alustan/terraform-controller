package controller

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"controller/pkg/container"
	"controller/pkg/kubernetes"
	"controller/pkg/terraform"
	"controller/pkg/util"
	"controller/plugin"


	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynclient "k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	maxRetries   = 5
)

var syncInterval time.Duration

type Controller struct {
	clientset *k8sclient.Clientset
	dynClient dynclient.Interface
}

type TerraformConfigSpec struct {
	Variables       map[string]string `json:"variables"`
	Backend         map[string]string `json:"backend"`
	Scripts         Scripts           `json:"scripts"`
	GitRepo         GitRepo           `json:"gitRepo"`
	ContainerRegistry ContainerRegistry `json:"containerRegistry"`
}

type Scripts struct {
	Apply   Script `json:"apply"`
	Destroy Script `json:"destroy"`
}

type Script struct {
	Inline       string       `json:"inline"`
	ConfigMapRef ConfigMapRef `json:"configMapRef"`
}

type ConfigMapRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type GitRepo struct {
	URL          string       `json:"url"`
	Branch       string       `json:"branch"`
	SSHKeySecret SSHKeySecret `json:"sshKeySecret"`
}

type ContainerRegistry struct {
	ImageName string    `json:"imageName"`
	SecretRef SecretRef `json:"secretRef"`
}

type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type SSHKeySecret struct {
	Name string `json:"name"`
	Key  string `json:"key"`
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
	envVars := util.ExtractEnvVars(observed.Parent.Spec.Variables, observed.Parent.Spec.Backend)

	log.Printf("Observed Parent Spec: %+v", observed.Parent.Spec)

	var script map[string]interface{}
	if observed.Parent.Spec.Scripts.Apply.Inline != "" {
		log.Printf("Using inline script: %s", observed.Parent.Spec.Scripts.Apply.Inline)
		script = map[string]interface{}{
			"inline": observed.Parent.Spec.Scripts.Apply.Inline,
		}
	} else if observed.Parent.Spec.Scripts.Apply.ConfigMapRef.Name != "" && observed.Parent.Spec.Scripts.Apply.ConfigMapRef.Key != "" {
		log.Printf("Using ConfigMapRef with name: %s and key: %s", observed.Parent.Spec.Scripts.Apply.ConfigMapRef.Name, observed.Parent.Spec.Scripts.Apply.ConfigMapRef.Key)
		script = map[string]interface{}{
			"configMapRef": map[string]interface{}{
				"name": observed.Parent.Spec.Scripts.Apply.ConfigMapRef.Name,
				"key":  observed.Parent.Spec.Scripts.Apply.ConfigMapRef.Key,
			},
		}
	} else {
		log.Println("No script provided for apply operation")
		return
	}

	scriptContent, err := terraform.ExtractScriptContent(c.clientset, observed.Parent.Metadata.Namespace, script)
	if err != nil {
		log.Printf("Error extracting apply script: %v", err)
		return
	}

	if observed.Finalizing {
		if observed.Parent.Spec.Scripts.Destroy.Inline != "" {
			script = map[string]interface{}{
				"inline": observed.Parent.Spec.Scripts.Destroy.Inline,
			}
		} else if observed.Parent.Spec.Scripts.Destroy.ConfigMapRef.Name != "" && observed.Parent.Spec.Scripts.Destroy.ConfigMapRef.Key != "" {
			script = map[string]interface{}{
				"configMapRef": map[string]interface{}{
					"name": observed.Parent.Spec.Scripts.Destroy.ConfigMapRef.Name,
					"key":  observed.Parent.Spec.Scripts.Destroy.ConfigMapRef.Key,
				},
			}
		} else {
			log.Println("No script provided for destroy operation")
			return
		}

		scriptContent, err = terraform.ExtractScriptContent(c.clientset, observed.Parent.Metadata.Namespace, script)
		if err != nil {
			log.Printf("Error extracting destroy script: %v", err)
			return
		}
	}

	repoDir := filepath.Join("/tmp", observed.Parent.Metadata.Name)

      var sshKey string
    // Check if GitRepo is provided
    gitRepo := observed.Parent.Spec.GitRepo
    if gitRepo != nil && gitRepo.URL != "" && gitRepo.SSHKeySecret != nil && gitRepo.SSHKeySecret.Name != "" && gitRepo.SSHKeySecret.Key != "" {
        sshKey, err = util.GetDataFromSecret(c.clientset, observed.Parent.Metadata.Namespace, gitRepo.SSHKeySecret.Name, gitRepo.SSHKeySecret.Key)
        if err != nil {
            log.Fatalf("Failed to get SSH key from secret: %v", err)
        }
	}

	err = terraform.CloneOrPullRepo(observed.Parent.Spec.GitRepo.URL, observed.Parent.Spec.GitRepo.Branch, repoDir, sshKey)
	if err != nil {
		log.Printf("Error cloning Git repository: %v", err)
		return
	}

	backend := observed.Parent.Spec.Backend

	var provider plugin.BackendProvider

	if backend == nil || len(backend) == 0 {
		// No backend provided, continue without backend setup
		log.Println("No backend provided, continuing without backend setup")
	} else {
		providerType, providerExists := backend["provider"]

		if !providerExists || providerType == "" {
			log.Println("Backend provided without specifying provider, continuing without backend setup")
		} else {
			// Get the appropriate provider
			provider, err = plugin.GetProvider(providerType)
			if err != nil {
				log.Fatalf("Error getting provider: %v", err)
			}
		
		   err = provider.SetupBackend(backend)
            if err != nil {
                log.Printf("Error setting up %s backend: %v", providerType, err)
                return
            }

		} 
	}

	
	configMapName, err := container.CreateDockerfileConfigMap(c.clientset, observed.Parent.Metadata.Namespace, repoDir, provider.GetDockerfileAdditions())
	if err != nil {
		log.Printf("Error creating Dockerfile ConfigMap: %v", err)
		return
	}

   imageName := observed.Parent.Spec.ContainerRegistry.ImageName
	err = container.CreateBuildJob(c.clientset, observed.Parent.Metadata.Namespace, configMapName, imageName, observed.Parent.Spec.ContainerRegistry.SecretRef.Name)
	if err != nil {
		log.Printf("Error creating build job: %v", err)
		return
	}

	pvcName := "terraform-pvc"
	var terraformErr error
	for i := 0; i < maxRetries; i++ {
		terraformErr = container.CreateRunPod(c.clientset, observed.Parent.Metadata.Namespace, envVars, scriptContent, imageName, pvcName, observed.Parent.Spec.ContainerRegistry.SecretRef.Name)
		if terraformErr == nil {
			break
		}
		log.Printf("Retrying Terraform command due to error: %v", terraformErr)
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
		log.Printf("Error updating status: %v", err)
		return
	}
}


func (c *Controller) Reconcile(syncInterval time.Duration) {
	for {
		c.reconcileLoop()
		time.Sleep(syncInterval)
	}
}

func (c *Controller) reconcileLoop() {
	resourceList, err := c.dynClient.Resource(schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "terraforms",
	}).Namespace("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error fetching Terraform resources: %v", err)
		return
	}

	for _, item := range resourceList.Items {
		var observed SyncRequest
		raw, err := item.MarshalJSON()
		if err != nil {
			log.Printf("Error marshalling item: %v", err)
			continue
		}
		err = json.Unmarshal(raw, &observed)
		if err != nil {
			log.Printf("Error unmarshalling item: %v", err)
			continue
		}

		c.handleSyncRequest(observed)
	}
}
