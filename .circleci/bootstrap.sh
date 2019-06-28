#!/usr/bin/env bash
set -euo pipefail

KUBEBUILDER_VERSION=1.0.8
KUSTOMIZE_VERSION=2.1.0
function defineEnvvar {
    echo "  $1=$2"
    echo "export $1=\"$2\"" >> $BASH_ENV
}


echo "Installing Kubebuilder"
curl -L -O https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
tar -zxvf kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
mv kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64 kubebuilder && sudo mv kubebuilder /usr/local/
export PATH=$PATH:/usr/local/kubebuilder/bin


echo "Installing Kustomize"
curl -L https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64 > kustomize
chmod +x kustomize
mv kustomize /usr/local/bin


echo "Installing Google Cloud SDK"
curl https://sdk.cloud.google.com | bash -s -- --disable-prompts
export PATH=$PATH:~/google-cloud-sdk/bin
echo $GCLOUD_SERVICE_KEY | gcloud auth activate-service-account --key-file=-
gcloud --quiet config set project ${GOOGLE_PROJECT_ID}
gcloud --quiet config set compute/zone ${GOOGLE_COMPUTE_ZONE}
gcloud --quiet auth configure-docker


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
