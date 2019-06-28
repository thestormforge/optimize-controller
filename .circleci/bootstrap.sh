#!/usr/bin/env bash
set -euo pipefail

KUBEBUILDER_VERSION=1.0.8
KUSTOMIZE_VERSION=2.1.0
if [[ -n "${CIRCLE_TAG:-}" ]]; then
    export VERSION="${CIRCLE_TAG}"
    export BUILD_METADATA=""
    DOCKER_TAG="${CIRCLE_TAG#v}"
else
    defineEnvvar VERSION "$(sed -n 's/[[:blank:]]Version[[:blank:]]*=[[:blank:]]*"\(.*\)"/\1/p' pkg/version/version.go)"
    defineEnvvar BUILD_METADATA "build.${CIRCLE_BUILD_NUM}"
    DOCKER_TAG="${CIRCLE_SHA1:0:8}.${CIRCLE_BUILD_NUM}"
fi
export SETUPTOOLS_IMG="gcr.io/${GOOGLE_PROJECT_ID}/setuptools:${DOCKER_TAG}"
export IMG="gcr.io/${GOOGLE_PROJECT_ID}/${CIRCLE_PROJECT_REPONAME}:${DOCKER_TAG}"

echo "Using environment variables from bootstrap script"
echo "  VERSION=$VERSION"
echo "  BUILD_METADATA=$BUILD_METADATA"
echo "  SETUPTOOLS_IMG=$SETUPTOOLS_IMG"
echo "  IMG=$IMG"


echo "Installing Kubebuilder"
curl -LO https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
tar -zxvf kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
sudo mv kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64 /usr/local/kubebuilder
export PATH=$PATH:/usr/local/kubebuilder/bin


echo "Installing Kustomize"
curl -LO https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64
chmod +x kustomize_${KUSTOMIZE_VERSION}_linux_amd64
sudo mv kustomize_${KUSTOMIZE_VERSION}_linux_amd64 /usr/local/bin/kustomize


echo "Installing Google Cloud SDK"
curl https://sdk.cloud.google.com | bash -s -- --disable-prompts
export PATH=$PATH:~/google-cloud-sdk/bin


echo "Authorizing Google Cloud SDK"
echo $GCLOUD_SERVICE_KEY | gcloud auth activate-service-account --key-file=-
gcloud --quiet config set project ${GOOGLE_PROJECT_ID}
gcloud --quiet config set compute/zone ${GOOGLE_COMPUTE_ZONE}
gcloud --quiet auth configure-docker
