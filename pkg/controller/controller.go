package controller

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"time"
	"fmt"

	"github.com/gin-gonic/gin"
	"controller/pkg/container"
	"controller/pkg/kubernetes"
	"controller/pkg/terraform"
	"controller/pkg/util"
	"controller/plugin"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	dynclient "k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	maxRetries = 10
	
)



type Controller struct {
	clientset *k8sclient.Clientset
	dynClient dynclient.Interface
}

type TerraformConfigSpec struct {
	Variables        map[string]string `json:"variables"`
	Backend          map[string]string `json:"backend"`
	Scripts          Scripts           `json:"scripts"`
	GitRepo          GitRepo           `json:"gitRepo"`
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
	defer func() {
		if err := r.Request.Body.Close(); err != nil {
			log.Printf("Error closing request body: %v", err)
		}
	}()

	response := c.handleSyncRequest(observed)

	r.Writer.Header().Set("Content-Type", "application/json")
	r.JSON(http.StatusOK, gin.H{"body": response})
}



func (c *Controller) handleSyncRequest(observed SyncRequest) map[string]interface{} {
	envVars := c.extractEnvVars(observed.Parent.Spec.Variables)
	log.Printf("Observed Parent Spec: %+v", observed.Parent.Spec)

	script, err := c.getScript(observed.Parent.Spec.Scripts.Apply)
	if err != nil {
		return c.errorResponse("apply", err)
	}

	scriptContent, err := terraform.ExtractScriptContent(c.clientset, observed.Parent.Metadata.Namespace, script)
	if err != nil {
		return c.errorResponse("extracting apply script", err)
	}

	if observed.Finalizing {
		script, err = c.getScript(observed.Parent.Spec.Scripts.Destroy)
		if err != nil {
			return c.errorResponse("destroy", err)
		}

		scriptContent, err = terraform.ExtractScriptContent(c.clientset, observed.Parent.Metadata.Namespace, script)
		if err != nil {
			return c.errorResponse("extracting destroy script", err)
		}
	}

	repoDir, sshKey, err := c.prepareRepo(observed.Parent)
	if err != nil {
		return c.errorResponse("preparing repository", err)
	}

	err = terraform.CloneOrPullRepo(observed.Parent.Spec.GitRepo.URL, observed.Parent.Spec.GitRepo.Branch, repoDir, sshKey)
	if err != nil {
		return c.errorResponse("cloning Git repository", err)
	}

	dockerfileAdditions, providerExists, err := c.setupBackend(observed.Parent.Spec.Backend)
	if err != nil {
		return c.errorResponse("setting up backend", err)
	}

	configMapName, err := container.CreateDockerfileConfigMap(c.clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, dockerfileAdditions, providerExists)
	if err != nil {
		return c.errorResponse("creating Dockerfile ConfigMap", err)
	}

	taggedImageName,_, err := c.buildAndTagImage(observed, configMapName, repoDir)
	if err != nil {
		return c.errorResponse("creating build job", err)
	}

   status := c.runTerraform(observed, scriptContent, taggedImageName, envVars)
	if err := kubernetes.UpdateStatus(c.dynClient, observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name, status); err != nil {
		return c.errorResponse("updating status", err)
	}

	return status
}

func (c *Controller) extractEnvVars(variables map[string]string) map[string]string {
	if variables == nil {
		return nil
	}
	return util.ExtractEnvVars(variables)
}

func (c *Controller) getScript(scriptSpec Script) (map[string]interface{}, error) {
	if scriptSpec.Inline != "" {
		log.Printf("Using inline script: %s", scriptSpec.Inline)
		return map[string]interface{}{
			"inline": scriptSpec.Inline,
		}, nil
	}
	if scriptSpec.ConfigMapRef.Name != "" && scriptSpec.ConfigMapRef.Key != "" {
		log.Printf("Using ConfigMapRef with name: %s and key: %s", scriptSpec.ConfigMapRef.Name, scriptSpec.ConfigMapRef.Key)
		return map[string]interface{}{
			"configMapRef": map[string]interface{}{
				"name": scriptSpec.ConfigMapRef.Name,
				"key":  scriptSpec.ConfigMapRef.Key,
			},
		}, nil
	}
	return nil, fmt.Errorf("no script provided for operation")
}

func (c *Controller) prepareRepo(parent ParentResource) (string, string, error) {
	repoDir := filepath.Join("/tmp", parent.Metadata.Name)
	gitRepo := parent.Spec.GitRepo
	var sshKey string

	if gitRepo.URL != "" && gitRepo.SSHKeySecret.Name != "" && gitRepo.SSHKeySecret.Key != "" {
		var err error
		sshKey, err = util.GetDataFromSecret(c.clientset, parent.Metadata.Namespace, gitRepo.SSHKeySecret.Name, gitRepo.SSHKeySecret.Key)
		if err != nil {
			return "", "", fmt.Errorf("failed to get SSH key from secret: %v", err)
		}
	}

	return repoDir, sshKey, nil
}

func (c *Controller) setupBackend(backend map[string]string) (string, bool, error) {
	if backend == nil || len(backend) == 0 {
		log.Println("No backend provided, continuing without backend setup")
		return "", false, nil
	}

	providerType, providerExists := backend["provider"]
	if !providerExists || providerType == "" {
		log.Println("Backend provided without specifying provider, continuing without backend setup")
		return "", false, nil
	}

	provider, err := plugin.GetProvider(providerType)
	if err != nil {
		return "", false, fmt.Errorf("error getting provider: %v", err)
	}

	if err := provider.SetupBackend(backend); err != nil {
		return "", false, fmt.Errorf("error setting up %s backend: %v", providerType, err)
	}

	return provider.GetDockerfileAdditions(), true, nil
}

func (c *Controller) buildAndTagImage(observed SyncRequest, configMapName, repoDir string) (string,string, error) {
	imageName := observed.Parent.Spec.ContainerRegistry.ImageName
	

	return container.CreateBuildPod(c.clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, configMapName, imageName, observed.Parent.Spec.ContainerRegistry.SecretRef.Name, repoDir)
}




func (c *Controller) runTerraform(observed SyncRequest, scriptContent, taggedImageName string, envVars map[string]string) map[string]interface{} {


	var terraformErr error
	for i := 0; i < maxRetries; i++ {
		terraformErr = container.CreateRunPod(c.clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, envVars, scriptContent, taggedImageName, observed.Parent.Spec.ContainerRegistry.SecretRef.Name)
		if terraformErr == nil {
			break
		}
		log.Printf("Retrying Terraform command due to error: %v", terraformErr)
		time.Sleep(1 * time.Minute)
	}

	status := map[string]interface{}{
		"state":   "Success",
		"message": "Terraform applied successfully",
	}
	if terraformErr != nil {
		status["state"] = "Failed"
		status["message"] = terraformErr.Error()
	}

	return status
}

func (c *Controller) errorResponse(action string, err error) map[string]interface{} {
	log.Printf("Error %s: %v", action, err)
	return map[string]interface{}{
		"status":  "error",
		"message": fmt.Sprintf("Error %s: %v", action, err),
	}
}


func (c *Controller) Reconcile(syncInterval time.Duration) {
	for {
		c.reconcileLoop()
		time.Sleep(syncInterval)
	}
}

func (c *Controller) reconcileLoop() {
	log.Println("Starting reconciliation loop")
	resourceList, err := c.dynClient.Resource(schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "terraforms",
	}).Namespace("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error fetching Terraform resources: %v", err)
		return
	}

	log.Printf("Fetched %d Terraform resources", len(resourceList.Items))

	for _, item := range resourceList.Items {
		go func(item unstructured.Unstructured) {
			var observed SyncRequest
			raw, err := item.MarshalJSON()
			if err != nil {
				log.Printf("Error marshalling item: %v", err)
				return
			}
			err = json.Unmarshal(raw, &observed)
			if err != nil {
				log.Printf("Error unmarshalling item: %v", err)
				return
			}

			log.Printf("Handling resource: %s", observed.Parent.Metadata.Name)
			c.handleSyncRequest(observed)
		}(item)
	}
}
