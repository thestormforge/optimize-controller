#!/usr/bin/env bash
set -euo pipefail

function defineEnvvar {
    echo "  $1=$2"
    echo "export $1=\"$2\"" >> $BASH_ENV
}

KUBEBUILDER_VERSION=1.0.8
KUSTOMIZE_VERSION=2.1.0

echo "Using environment variables from bootstrap script"
if [[ -n "${CIRCLE_TAG:-}" ]]; then
    defineEnvvar VERSION "${CIRCLE_TAG}"
    defineEnvvar BUILD_METADATA ""
    DOCKER_TAG="${CIRCLE_TAG#v}"
else
    defineEnvvar VERSION "$(sed -n 's/[[:blank:]]Version[[:blank:]]*=[[:blank:]]*"\(.*\)"/\1/p' pkg/version/version.go)"
    defineEnvvar BUILD_METADATA "build.${CIRCLE_BUILD_NUM}"
    DOCKER_TAG="${CIRCLE_SHA1:0:8}.${CIRCLE_BUILD_NUM}"
fi
defineEnvvar SETUPTOOLS_IMG "gcr.io/${GOOGLE_PROJECT_ID}/setuptools:${DOCKER_TAG}"
defineEnvvar IMG "gcr.io/${GOOGLE_PROJECT_ID}/${CIRCLE_PROJECT_REPONAME}:${DOCKER_TAG}"
echo


echo "Installing Kubebuilder"
curl -LOq https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
tar -zxvf kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
sudo mv kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64 /usr/local/kubebuilder
export PATH=$PATH:/usr/local/kubebuilder/bin
echo


echo "Installing Kustomize"
curl -LOq https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64
chmod +x kustomize_${KUSTOMIZE_VERSION}_linux_amd64
sudo mv kustomize_${KUSTOMIZE_VERSION}_linux_amd64 /usr/local/bin/kustomize
echo


echo "Installing Google Cloud SDK"
curl -q https://sdk.cloud.google.com | bash -s -- --disable-prompts > /dev/null
export PATH=$PATH:~/google-cloud-sdk/bin
echo

