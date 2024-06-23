package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/alustan/terraform-controller/pkg/container"
	"github.com/alustan/terraform-controller/pkg/kubernetes"
	"github.com/alustan/terraform-controller/pkg/util"
	"github.com/alustan/terraform-controller/pluginregistry"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	dynclient "k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	maxRetries = 5
)

type Controller struct {
	clientset *k8sclient.Clientset
	dynClient dynclient.Interface
}

type TerraformConfigSpec struct {
	Provider  string            `json:"provider"`
	Variables map[string]string `json:"variables"`
	Scripts   Scripts           `json:"scripts"`
	GitRepo   GitRepo           `json:"gitRepo"`
	ContainerRegistry ContainerRegistry `json:"containerRegistry"`
}

type Scripts struct {
	Deploy  string `json:"deploy"`
	Destroy string `json:"destroy"`
}

type GitRepo struct {
	URL    string `json:"url"`
	Branch string `json:"branch"`
}

type ContainerRegistry struct {
	ImageName string `json:"imageName"`
}

type ParentResource struct {
	ApiVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   metav1.ObjectMeta `json:"metadata"`
	Spec       TerraformConfigSpec `json:"spec"`
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

	// Initial status update: processing started
	initialStatus := map[string]interface{}{
		"state":   "Progressing",
		"message": "Starting processing",
	}
	c.updateStatus(observed, initialStatus)

	scriptContent := observed.Parent.Spec.Scripts.Deploy
	if scriptContent == "" {
		status := c.errorResponse("executing deploy script", fmt.Errorf("deploy script is missing"))
		c.updateStatus(observed, status)
		return status
	}

	if observed.Finalizing {
		scriptContent = observed.Parent.Spec.Scripts.Destroy
		if scriptContent == "" {
			status := c.errorResponse("executing destroy script", fmt.Errorf("destroy script is missing"))
			c.updateStatus(observed, status)
			return status
		}
	}

	// Status update: setting up provider
	c.updateStatus(observed, map[string]interface{}{
		"state":   "Progressing",
		"message": "Setting up provider",
	})

	repoDir := filepath.Join("/workspace", "tmp", observed.Parent.Metadata.Name)
	sshKey := os.Getenv("GIT_SSH_SECRET")

	// Setup provider and get Dockerfile additions
	dockerfileAdditions, providerExists, err := c.setupProvider(observed.Parent.Spec.Provider, observed.Parent.Metadata.Labels["workspace"], observed.Parent.Metadata.Labels["region"])
	if err != nil {
		status := c.errorResponse("setting up backend", err)
		c.updateStatus(observed, status)
		return status
	}

	// Status update: creating Dockerfile ConfigMap
	c.updateStatus(observed, map[string]interface{}{
		"state":   "Progressing",
		"message": "Creating Dockerfile ConfigMap",
	})

	configMapName, err := container.CreateDockerfileConfigMap(c.clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, dockerfileAdditions, providerExists)
	if err != nil {
		status := c.errorResponse("creating Dockerfile ConfigMap", err)
		c.updateStatus(observed, status)
		return status
	}

	// Status update: creating Docker config secret
	c.updateStatus(observed, map[string]interface{}{
		"state":   "Progressing",
		"message": "Creating Docker config secret",
	})

	encodedDockerConfigJSON := os.Getenv("CONTAINER_REGISTRY_SECRET")
	if encodedDockerConfigJSON == "" {
		log.Println("Environment variable CONTAINER_REGISTRY_SECRET is not set")
		status := c.errorResponse("creating Docker config secret", fmt.Errorf("CONTAINER_REGISTRY_SECRET is not set"))
		c.updateStatus(observed, status)
		return status
	}
	secretName := fmt.Sprintf("%s-container-secret", observed.Parent.Metadata.Name)
	err = container.CreateDockerConfigSecret(c.clientset, secretName, observed.Parent.Metadata.Namespace, encodedDockerConfigJSON)
	if err != nil {
		status := c.errorResponse("creating Docker config secret", err)
		c.updateStatus(observed, status)
		return status
	}

	// Status update: creating PVC
	c.updateStatus(observed, map[string]interface{}{
		"state":   "Progressing",
		"message": "Creating PVC",
	})

	pvcName := fmt.Sprintf("pvc-%s", observed.Parent.Metadata.Name)

	err = container.EnsurePVC(c.clientset, observed.Parent.Metadata.Namespace, pvcName)
	if err != nil {
		status := c.errorResponse("creating PVC", err)
		c.updateStatus(observed, status)
		return status
	}

	// Status update: building and tagging image
	c.updateStatus(observed, map[string]interface{}{
		"state":   "Progressing",
		"message": "Building and tagging image",
	})

	taggedImageName, _, err := c.buildAndTagImage(observed, configMapName, repoDir, sshKey, secretName, pvcName)
	if err != nil {
		status := c.errorResponse("creating build job", err)
		c.updateStatus(observed, status)
		return status
	}

	// Status update: running Terraform
	c.updateStatus(observed, map[string]interface{}{
		"state":   "Progressing",
		"message": "Running Terraform",
	})

	status := c.runTerraform(observed, scriptContent, taggedImageName, secretName, envVars)

	c.updateStatus(observed, status)

	if observed.Parent.Spec.Provider != "" {
		// Execute the plugin after running Terraform
		resources, err := c.executePlugin(observed.Parent.Spec.Provider, observed.Parent.Metadata.Labels["workspace"], observed.Parent.Metadata.Labels["region"])
		if err != nil {
			finalStatus := c.errorResponse("executing plugin", err)
			c.updateStatus(observed, finalStatus)
			return finalStatus
		}

		// Update status with plugin credentials
		pluginStatus := map[string]interface{}{
			"state":       "Completed",
			"message":     "Processing completed successfully",
			"cloudResources": resources,
		}
		c.updateStatus(observed, pluginStatus)
		return pluginStatus
	}

	finalStatus := map[string]interface{}{
		"state":   "Completed",
		"message": "Processing completed successfully",
	}

	c.updateStatus(observed, finalStatus)
	return finalStatus
}

func (c *Controller) updateStatus(observed SyncRequest, status map[string]interface{}) {
	err := kubernetes.UpdateStatus(c.dynClient, observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name, status)
	if err != nil {
		log.Printf("Error updating status for %s: %v", observed.Parent.Metadata.Name, err)
	}
}

func (c *Controller) extractEnvVars(variables map[string]string) map[string]string {
	if variables == nil {
		return nil
	}
	return util.ExtractEnvVars(variables)
}

func (c *Controller) setupProvider(providerType, workspace, region string) (string, bool, error) {
	if providerType == "" {
		// No provider specified, return without error
		return "", false, nil
	}
	provider, err := pluginregistry.SetupPlugin(providerType, workspace, region)
	if err != nil {
		return "", false, err
	}
	return provider.GetDockerfileAdditions(), true, nil
}

func (c *Controller) executePlugin(providerType, workspace, region string) (map[string]interface{}, error) {
	provider, err := pluginregistry.SetupPlugin(providerType, workspace, region)
	if err != nil {
		return nil, fmt.Errorf("error getting plugin: %v", err)
	}
	return provider.Execute()
}

func (c *Controller) buildAndTagImage(observed SyncRequest, configMapName, repoDir, sshKey, secretName, pvcName string) (string, string, error) {
	imageName := observed.Parent.Spec.ContainerRegistry.ImageName

	return container.CreateBuildPod(c.clientset,
		observed.Parent.Metadata.Name,
		observed.Parent.Metadata.Namespace,
		configMapName,
		imageName,
		secretName,
		repoDir,
		observed.Parent.Spec.GitRepo.URL,
		observed.Parent.Spec.GitRepo.Branch,
		sshKey,
		pvcName)
}

func (c *Controller) runTerraform(observed SyncRequest, scriptContent, taggedImageName, secretName string, envVars map[string]string) map[string]interface{} {
	var terraformErr error
	var podName string

	for i := 0; i < maxRetries; i++ {
		podName, terraformErr = container.CreateRunPod(c.clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, envVars, scriptContent, taggedImageName, secretName)
		if terraformErr == nil {
			break
		}
		log.Printf("Retrying Terraform command due to error: %v", terraformErr)
		time.Sleep(5 * time.Minute)
	}

	status := map[string]interface{}{
		"state":   "Success",
		"message": "Terraform applied successfully",
	}
	if terraformErr != nil {
		status["state"] = "Failed"
		status["message"] = terraformErr.Error()
		return status
	}

	// Wait for the pod to complete and retrieve the logs
	output, err := container.WaitForPodCompletion(c.clientset, observed.Parent.Metadata.Namespace, podName)
	if err != nil {
		status["state"] = "Failed"
		status["message"] = fmt.Sprintf("Error retrieving Terraform output: %v", err)
		return status
	}

	status["output"] = output

	// Retrieve ingress URLs and include them in the status
	ingressURLs, err := kubernetes.GetAllIngressURLs(c.clientset)
	if err != nil {
		status["state"] = "Failed"
		status["message"] = fmt.Sprintf("Error retrieving Ingress URLs: %v", err)
		return status
	}
	status["ingressURLs"] = ingressURLs

	// Retrieve credentials and include them in the status
	credentials, err := kubernetes.FetchCredentials(c.clientset)
	if err != nil {
		status["state"] = "Failed"
		status["message"] = fmt.Sprintf("Error retrieving credentials: %v", err)
		return status
	}
	status["credentials"] = credentials

	return status
}

func (c *Controller) errorResponse(action string, err error) map[string]interface{} {
	log.Printf("Error %s: %v", action, err)
	return map[string]interface{}{
		"state":   "error",
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
