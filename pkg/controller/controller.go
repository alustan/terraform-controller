package controller

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
    "fmt"
	"path/filepath"
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

	response := c.handleSyncRequest(observed)

    r.Writer.Header().Set("Content-Type", "application/json")
	r.JSON(http.StatusOK, gin.H{"body": response})
	
}


func (c *Controller) handleSyncRequest(observed SyncRequest) map[string]interface{} {
    var envVars map[string]string

    if observed.Parent.Spec.Variables != nil {
        envVars = util.ExtractEnvVars(observed.Parent.Spec.Variables)
    }

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
        return map[string]interface{}{
            "status": "error",
            "message": "No script provided for apply operation",
        }
    }

    scriptContent, err := terraform.ExtractScriptContent(c.clientset, observed.Parent.Metadata.Namespace, script)
    if err != nil {
        log.Printf("Error extracting apply script: %v", err)
        return map[string]interface{}{
            "status": "error",
            "message": fmt.Sprintf("Error extracting apply script: %v", err),
        }
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
            return map[string]interface{}{
                "status": "error",
                "message": "No script provided for destroy operation",
            }
        }

        scriptContent, err = terraform.ExtractScriptContent(c.clientset, observed.Parent.Metadata.Namespace, script)
        if err != nil {
            log.Printf("Error extracting destroy script: %v", err)
            return map[string]interface{}{
                "status": "error",
                "message": fmt.Sprintf("Error extracting destroy script: %v", err),
            }
        }
    }

    repoDir := filepath.Join("/tmp", observed.Parent.Metadata.Name)

    var sshKey string
    gitRepo := observed.Parent.Spec.GitRepo

    if gitRepo.URL != "" && gitRepo.SSHKeySecret.Name != "" && gitRepo.SSHKeySecret.Key != "" {
        sshKey, err = util.GetDataFromSecret(c.clientset, observed.Parent.Metadata.Namespace, gitRepo.SSHKeySecret.Name, gitRepo.SSHKeySecret.Key)
        if err != nil {
            log.Fatalf("Failed to get SSH key from secret: %v", err)
        }
    }

    err = terraform.CloneOrPullRepo(gitRepo.URL, gitRepo.Branch, repoDir, sshKey)
    if err != nil {
        log.Printf("Error cloning Git repository: %v", err)
        return map[string]interface{}{
            "status": "error",
            "message": fmt.Sprintf("Error cloning Git repository: %v", err),
        }
    }

    backend := observed.Parent.Spec.Backend

    var provider plugin.BackendProvider
    providerType := ""
    providerExists := false

    if backend != nil && len(backend) > 0 {
        providerType, providerExists = backend["provider"]

        if providerExists && providerType != "" {
            provider, err = plugin.GetProvider(providerType)
            if err != nil {
                log.Fatalf("Error getting provider: %v", err)
            }

            err = provider.SetupBackend(backend)
            if err != nil {
                log.Printf("Error setting up %s backend: %v", providerType, err)
                return map[string]interface{}{
                    "status": "error",
                    "message": fmt.Sprintf("Error setting up %s backend: %v", providerType, err),
                }
            }
        } else {
            log.Println("Backend provided without specifying provider, continuing without backend setup")
        }
    } else {
        log.Println("No backend provided, continuing without backend setup")
    }

    // Ensure provider is only used if providerExists
    var dockerfileAdditions string
    if providerExists {
        dockerfileAdditions = provider.GetDockerfileAdditions()
    } else {
        dockerfileAdditions = ""
    }

    configMapName, err := container.CreateDockerfileConfigMap(c.clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace,  dockerfileAdditions, providerExists)
    if err != nil {
        log.Printf("Error creating Dockerfile ConfigMap: %v", err)
        return map[string]interface{}{
            "status": "error",
            "message": fmt.Sprintf("Error creating Dockerfile ConfigMap: %v", err),
        }
    }

    pvcName :=  fmt.Sprintf("%s-terraform-pvc", observed.Parent.Metadata.Name)
    imageName := observed.Parent.Spec.ContainerRegistry.ImageName
    
    taggedImageName, err := container.CreateBuildPod(c.clientset, observed.Parent.Metadata.Name,observed.Parent.Metadata.Namespace, configMapName, imageName, pvcName,observed.Parent.Spec.ContainerRegistry.SecretRef.Name,repoDir)
    if err != nil {
        log.Printf("Error creating build job: %v", err)
        return map[string]interface{}{
            "status": "error",
            "message": fmt.Sprintf("Error creating build job: %v", err),
        }
    }

    
    var terraformErr error
    for i := 0; i < maxRetries; i++ {
        // Create the run pod using the tagged image name
	terraformErr = container.CreateRunPod(c.clientset,observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, envVars, scriptContent, taggedImageName, pvcName, observed.Parent.Spec.ContainerRegistry.SecretRef.Name)
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

    err = kubernetes.UpdateStatus(c.dynClient, observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name, status)
     
    if err != nil {
        log.Printf("Error updating status: %v", err)
        return map[string]interface{}{
            "status": "error",
            "message": fmt.Sprintf("Error updating status: %v", err),
        }
    }

    return map[string]interface{}{
        "status": "success",
        "message": "Sync completed successfully",
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

		log.Printf("Handling resource: %s", observed.Parent.Metadata.Name)
		c.handleSyncRequest(observed)
	}
}

