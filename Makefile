# Image URL to use all building/pushing image targets
IMG ?= controller:latest
SETUPTOOLS_IMG ?= setuptools:latest

# Collect version information
ifdef VERSION
    LDFLAGS += -X github.com/gramLabs/cordelia/pkg/version.Version=${VERSION}
endif
ifdef BUILD_METADATA
    LDFLAGS += -X github.com/gramLabs/cordelia/pkg/version.BuildMetadata=${BUILD_METADATA}
endif
LDFLAGS += -X github.com/gramLabs/cordelia/pkg/version.GitCommit=$(shell git rev-parse HEAD)
LDFLAGS += -X github.com/gramLabs/cordelia/pkg/controller/trial.DefaultImage=${SETUPTOOLS_IMG}

all: test manager

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -ldflags '$(LDFLAGS)' -o bin/manager github.com/gramLabs/cordelia/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go crd output:crd:dir="config/crd/bases" rbac:roleName=manager-role object webhook paths="./pkg/...;./cmd/..."

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif
	go generate ./pkg/... ./cmd/...

# Build the docker images
docker-build:
	docker build . -t ${IMG} --build-arg LDFLAGS='$(LDFLAGS)'
	docker build . -t ${SETUPTOOLS_IMG} -f Dockerfile.setuptools
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker images
docker-push:
	docker push ${IMG}
	docker push ${SETUPTOOLS_IMG}
