# Variables
APP_NAME := terraform-controller
DOCKER_IMAGE := $(APP_NAME):latest


# Commands
GO := go
DOCKER := docker


# Directories
SRC_DIR := ./cmd/controller
TEST_DIR := ./test

# Targets
.PHONY: all build test setup lint clean docker-build docker-push 

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
	@echo "  setup         setup script before build"
	@echo "  help          Display this help message"
