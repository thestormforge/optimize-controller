
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
REDSKYCTL_IMG ?= redskyctl:latest
SETUPTOOLS_IMG ?= setuptools:latest
PULL_POLICY ?= Never
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,maxDescLen=0"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Collect version information
VERSION ?= $(shell git ls-remote --tags --refs origin 'v*' | tail -1 | awk -F/ '{ print $$3 }')-next
BUILD_METADATA ?=
GIT_COMMIT ?= $(shell git rev-parse HEAD)

# Define linker flags
LDFLAGS += -X github.com/thestormforge/optimize-controller/internal/version.Version=${VERSION}
LDFLAGS += -X github.com/thestormforge/optimize-controller/internal/version.BuildMetadata=${BUILD_METADATA}
LDFLAGS += -X github.com/thestormforge/optimize-controller/internal/version.GitCommit=${GIT_COMMIT}
LDFLAGS += -X github.com/thestormforge/optimize-controller/internal/setup.Image=${SETUPTOOLS_IMG}
LDFLAGS += -X github.com/thestormforge/optimize-controller/internal/setup.ImagePullPolicy=${PULL_POLICY}
LDFLAGS += -X github.com/thestormforge/optimize-controller/redskyctl/internal/kustomize.BuildImage=${IMG}

all: manager tool

# Run tests
test: generate manifests fmt vet
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -ldflags '$(LDFLAGS)' -o bin/manager main.go

# Build tool binary using GoReleaser in a local dev environment (in CI we just invoke GoReleaser directly)
tool: manifests
	BUILD_METADATA=${BUILD_METADATA} \
	SETUPTOOLS_IMG=${SETUPTOOLS_IMG} \
	PULL_POLICY=${PULL_POLICY} \
	REDSKYCTL_IMG=${REDSKYCTL_IMG} \
	IMG=${IMG} \
	goreleaser release --snapshot --skip-sign --rm-dist

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate manifests fmt vet
	go run ./main.go

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
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./api/v1alpha1;./api/v1beta1;./controllers/..." output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) schemapatch:manifests=config/crd/bases,maxDescLen=0  paths="./api/v1alpha1;./api/v1beta1" output:dir=./config/crd/bases
	go generate ./redskyctl/internal/kustomize

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen conversion-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	$(CONVERSION_GEN) --go-header-file "./hack/boilerplate.go.txt" --input-dirs "./api/v1alpha1" \
		--output-base "." --output-file-base="zz_generated.conversion" --skip-unsafe=true

build: manifests
	# Build on host so we can make use of the cache
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -ldflags "${LDFLAGS}" -o manager main.go

# Build the docker images
docker-build: test docker-build-ci

# Build the docker images
docker-build-ci: build docker-build-controller docker-build-setuptools

docker-build-controller:
	docker build . -t ${IMG} \
		--label "org.opencontainers.image.source=$(shell git remote get-url origin)"

docker-build-setuptools:
	docker build config -t ${SETUPTOOLS_IMG} \
		--label "org.opencontainers.image.source=$(shell git remote get-url origin)"

# Push the docker images
docker-push:
	docker push ${IMG}
	docker push ${SETUPTOOLS_IMG}
	docker push ${REDSKYCTL_IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.2 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# find or download conversion-gen
# download conversion-gen if necessary
conversion-gen:
ifeq (, $(shell which conversion-gen))
	@{ \
	set -e ;\
	CONVERSION_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONVERSION_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get k8s.io/code-generator/cmd/conversion-gen@v0.18.3 ;\
	rm -rf $$CONVERSION_GEN_TMP_DIR ;\
	}
CONVERSION_GEN=$(GOBIN)/conversion-gen
else
CONVERSION_GEN=$(shell which conversion-gen)
endif
