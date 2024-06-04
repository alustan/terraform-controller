# Variables
APP_NAME := terraform-controller
DOCKER_IMAGE := $(APP_NAME):latest
K8S_NAMESPACE := default
K8S_DEPLOYMENT := terraform-controller-deployment

# Commands
GO := go
DOCKER := docker
KUBECTL := kubectl

# Directories
SRC_DIR := ./cmd/controller
PKG_DIR := ./pkg
TEST_DIR := ./test

# Targets
.PHONY: all build test lint clean docker-build docker-push deploy undeploy

all: build

## Build the application
build:
	$(GO) build -o bin/$(APP_NAME) $(SRC_DIR)

## Run tests
test:
	$(GO) test -v $(TEST_DIR)/...

setup:
	./hack/setup.sh

## Run linting
lint:
	golangci-lint run ./...

## Clean build artifacts
clean:
	rm -rf bin/
	rm -rf /tmp/$(APP_NAME)-*

## Build Docker image
docker-build:
	$(DOCKER) build -t $(DOCKER_IMAGE) .

## Push Docker image to registry (you need to be logged in)
docker-push:
	$(DOCKER) push $(DOCKER_IMAGE)

## Deploy application to Kubernetes
deploy: docker-build docker-push
	$(KUBECTL) apply -f k8s/

## Undeploy application from Kubernetes
undeploy:
	$(KUBECTL) delete -f k8s/

## Show logs from the application pod
logs:
	$(KUBECTL) logs -l app=$(APP_NAME) -n $(K8S_NAMESPACE) -f

## Port forward to the application pod
port-forward:
	$(KUBECTL) port-forward deployment/$(K8S_DEPLOYMENT) 8080:8080 -n $(K8S_NAMESPACE)

## Display help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all           Build the application"
	@echo "  build         Build the application binary"
	@echo "  test          Run tests"
	@echo "  lint          Run linting"
	@echo "  clean         Clean build artifacts"
	@echo "  docker-build  Build Docker image"
	@echo "  docker-push   Push Docker image to registry"
	@echo "  deploy        Deploy application to Kubernetes"
	@echo "  undeploy      Undeploy application from Kubernetes"
	@echo "  logs          Show logs from the application pod"
	@echo "  port-forward  Port forward to the application pod"
	@echo "  help          Display this help message"
