## Introduction

This document provides a detailed design for a Kubernetes controller integrated with Terraform. 

The controller manages infrastructure as code (IaC) by applying Terraform configurations stored in Git repositories. This integration automates drift detection and  reconciliation of infrastructure states defined by custom resources in Kubernetes.

## Objectives

> - Automate Infrastructure Management: Automatically apply, reconcile and manage infrastructure using Terraform based on custom Kubernetes resources.

> - Scalability: Handle multiple infrastructure configurations efficiently.

> - Extensibility: Allow easy addition of new features and backends using plugins.

> - Reliability: Ensure robust error handling and retry mechanisms.

> - Observability: Provide comprehensive logging and monitoring for debugging and performance analysis.

## Architecture

The project architecture consists of the following main components:

- Controller: The central component responsible for managing custom resources with constant drift detection and reconciliation.

- Container: in-cluster container build using kaniko with state persistence

- Git Integration: Clones or pulls latest changes from Git repositories.

- plugin: plugs in new backend for any given cloud provider

- Terraform Integration: Executes Terraform scripts based on the provided configurations.

- Kubernetes Integration: Interacts with the Kubernetes API to manage resources and update statuses.

- API Server: Exposes endpoints to handle incoming sync requests from metacontroller

- Helm package: package container into a helm chart and deploy to an OCI registry


## Components

#### Controller

- Controller Struct: Manages Kubernetes clients used in  interacting with resources.

- ServeHTTP Method: Handles incoming HTTP requests for synchronization from metacontroller.

- Reconcile Method: Periodically reconciles the state of custom resources.

- handleSyncRequest Method: Processes the synchronization requests and executes Terraform commands.

#### API Server

- Gin Framework: Used for setting up the HTTP server and routing.

- Sync Endpoint: Exposes a POST endpoint /sync to receive synchronization requests.

#### Terraform Integration

- TerraformConfigSpec: Defines the structure of the custom resource spec for Terraform configurations.

- Scripts: Handles execution of Terraform  `apply`, and `destroy` scripts.

- Backend Setup: Configures backends for  storing Terraform states.

#### Git Integration

- CloneGitRepo Method: Clones or pulls latest changes from  specified Git repository containing Terraform configurations.

#### Kubernetes Integration

- Kubernetes Clients: Handles interactions with the Kubernetes API.

- UpdateStatus Method: Updates the status of custom resources based on the outcome of Terraform commands.

#### Container 

- In-cluster container build using kaniko with state persistence

#### Plugin
- extensible backend plugin for different cloud providers

#### Logging and Monitoring

- Standard Logging: Uses the log package to write logs to standard error, ensuring compatibility with Kubernetes logging mechanisms.

- Log Levels: Info, error, and debug logs to provide detailed insights into the controller's operations.

#### Testing and Coverage

- Unit Tests: Located in the test directory, covering individual components and methods.

#### Security Considerations

- Authentication and Authorization: Ensures that the controller has the necessary permissions to interact with Kubernetes resources.

- Secret Management: Securely handles sensitive information like Git SSH keys and backend credentials.

#### Workflow

##### - Initialization:

> The controller initializes Kubernetes clients and dynamic clients.
> Sets up the API server using the Gin framework.

##### - Sync Request Handling:

> The API server listens for incoming sync requests at /sync.
> Upon receiving a request, the controller decodes the request body to a SyncRequest struct.

##### - Terraform Execution:

> Clones/pulls the Terraform configuration from the specified Git repository.

> Sets up the backend (e.g., AWS S3, dyanmoDB) for Terraform state management.

> The controller determines the appropriate Terraform script to run (apply or destroy).

> Executes the Terraform script with environment variables.

##### - Status Update:

> Following execution, the controller updates the status of the custom resource with the outcome.
> If errors occur, the controller retries the operation with a maximum retry limit.

##### - Reconciliation Loop:

> The controller periodically reconciles the state of all custom resources to ensure consistency.

#### Environment Setup

- fork and clone the repo

- `make setup ` to initialize the environment.



```yaml
apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: example-terraformconfig
  namespace: default
spec:
  variables:
    var1: value1
    var2: value2
  backend:
    provider: aws
    s3: s3-store
    dynamoDB: db-table
    region: us-east-1
  scripts:
    apply: 
     inline: |
       terraform apply -auto-approve
    destroy: 
     inline: |
        terraform destroy -auto-approve
  gitRepo:
    url: git@github.com:example/terraform-repo/terraform
    branch: main
    sshKeySecret:
      name: my-ssh-secret
      key: ssh-privatekey
  containerRegistry:
    imageName: docker.io/alustan/terrform (image name to be built and push by the controller)
    secretRef:
      name: my-dockerhub-secret
      key: dockerhub-cred

# status:
#   state: "Pending"
#   message: "Awaiting processing"
```

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: terraform-scripts
data:
  apply-script.sh: |
    #!/bin/bash
    echo "This is the apply script"
  destroy-script.sh: |
    #!/bin/bash
    echo "This is the destroy script"

---

apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: example-terraformconfig
  namespace: default
spec:
  variables:
    var1: value1
    var2: value2
  backend:
    provider: aws
    s3: s3-store
    dynamoDB: db-table
    region: us-east-1
    
  scripts:
    apply: 
     configMapRef:
      name: terraform-scripts
      key: apply-script.sh
    destroy:
     configMapRef:
      name: terraform-scripts
      key: destroy-script.sh
   
  gitRepo:
    url: https://github.com/example/terraform-repo/terraform
    branch: main
  containerRegistry:
    imageName: docker.io/alustan/terrform (image name to be built and push by the controller)
    secretRef:
      name: my-dockerhub-secret
      key: dockerhub-cred

  
# status:
#   state: "Pending"
#   message: "Awaiting processing"
```



#### Future Enhancements

- Support for Additional Backends: using the extensible plugin capability.

- Enhanced Error Handling: Improve error handling and retry mechanisms.

- Custom Metrics: Expose custom metrics for better observability.

- Webhooks: Implement webhooks for real-time notifications and updates.



**This design document outlines the architecture, components, and workflow for the Kubernetes controller integrated with Terraform. It serves as a reference for development, deployment, and future enhancements.**