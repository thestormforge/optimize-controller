
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
REDSKYCTL_IMG ?= redskyctl:latest
SETUPTOOLS_IMG ?= setuptools:latest
PULL_POLICY ?= IfNotPresent
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Collect version information
ifdef VERSION
    LDFLAGS += -X github.com/redskyops/k8s-experiment/pkg/version.Version=${VERSION}
endif
ifneq ($(origin BUILD_METADATA), undefined)
    LDFLAGS += -X github.com/redskyops/k8s-experiment/pkg/version.BuildMetadata=${BUILD_METADATA}
endif
LDFLAGS += -X github.com/redskyops/k8s-experiment/pkg/version.GitCommit=$(shell git rev-parse HEAD)
LDFLAGS += -X github.com/redskyops/k8s-experiment/pkg/controller/trial.Image=${SETUPTOOLS_IMG}
LDFLAGS += -X github.com/redskyops/k8s-experiment/pkg/controller/trial.ImagePullPolicy=${PULL_POLICY}

all: manager tool

# Run tests
test: generate fmt vet manifests
	go test ./controllers/... ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -ldflags '$(LDFLAGS)' -o bin/manager cmd/manager/main.go

# Build tool binary for all supported platforms
tool: generate fmt vet
	GOOS=darwin GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/redskyctl-darwin-amd64 cmd/redskyctl/main.go
	GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/redskyctl-linux-amd64 cmd/redskyctl/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./pkg/apis/...;./controllers/...;./cmd/..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./controllers/... ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./controllers/... ./pkg/... ./cmd/...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./pkg/apis/...;./controllers/...;./cmd/..."

# Build the docker images
docker-build:
	docker build . -t ${IMG} --build-arg LDFLAGS='$(LDFLAGS)'
	docker build . -f Dockerfile.redskyctl -t ${REDSKYCTL_IMG} --build-arg LDFLAGS='$(LDFLAGS)'
	docker build config -t ${SETUPTOOLS_IMG} --build-arg IMG='$(IMG)' --build-arg PULL_POLICY='$(PULL_POLICY)' --build-arg VERSION='$(VERSION)'

# Push the docker images
docker-push:
	docker push ${IMG}
	docker push ${REDSKYCTL_IMG}
	docker push ${SETUPTOOLS_IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.4 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# Generate client code
generate-client:
	client-gen --clientset-name kubernetes --input-base "" --input github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1 --output-base "../../.." --output-package github.com/redskyops/k8s-experiment/pkg --go-header-file hack/boilerplate.go.txt

# Generate CLI and API documentation
generate-docs:
	go run -ldflags '$(LDFLAGS)' cmd/redskyctl/main.go docs --directory docs/redskyctl
	go run -ldflags '$(LDFLAGS)' cmd/redskyctl/main.go docs --directory docs/api --doc-type api
