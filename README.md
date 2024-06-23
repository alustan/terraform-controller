## Introduction

> **Terraform drift detection and continuous reconciliation using native kubernetes**

## Design Goal

- controller should be able to sync and reconcile the infra code base with the desired state

>  Default sync interval `6 hrly`

- Custom resource should be as simple as possible

- Controller should be extensible via plugins

- Custom resources can still be synced using argocd

> However it is recommended to disable argocd autosync and implement argocd `git webhook ` triggred on push to manifest repo; since actual codebase is already being synced. 

## Environment Setup

- fork and clone the repo

- `make setup ` to initialize the environment.

## Usage

- install the helm chart into a kubernetes cluster

```sh
helm install my-terraform-controller-helm oci://registry-1.docker.io/alustan/terraform-controller-helm --version <version>
```


- Define your manifest

```yaml
apiVersion: alustan.io/v1alpha1
kind: Terraform
metadata:
  name: staging-cluster
  namespace: staging
spec:
  provider: aws
  variables:
    TF_VAR_provision_cluster: "true"
    TF_VAR_provision_db: "false"
    TF_VAR_vpc_cidr: "10.1.0.0/16"
  scripts:
    deploy: deploy.sh
    destroy: destroy.sh
  gitRepo:
    url: https://github.com/alustan/infrastructure
    branch: main
  containerRegistry:
    imageName: docker.io/alustan/terraform-control # imagename to be built by the controller
    
#  status:
#    state: ""
#    message: ""
#    ingressURLs: ""
#    credentials : ""
#    cloudResources: ""
```

**This is one of multiple projects that aims to setup a functional platform for seemless app deployment with less technical overhead**

**Check Out:**

1. [Infrastructure](https://github.com/alustan/infrastructure)

2. [App-controller](https://github.com/alustan/app-controller)

3. [Cluster-manifests](https://github.com/alustan/cluster-manifests)

4. [Alustan-Backstage](https://github.com/alustan/backstage)
