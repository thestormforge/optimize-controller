#!/usr/bin/env bash

# Copyright 2019 GramLabs, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
set -euo pipefail

function defineEnvvar {
    echo "  $1=$2"
    echo "export $1=\"$2\"" >> "$BASH_ENV"
}

KUBEBUILDER_VERSION=2.3.0
GHR_VERSION=0.13.0

echo "Using environment variables from bootstrap script"
if [[ -n "${CIRCLE_TAG:-}" ]]; then
    defineEnvvar VERSION "${CIRCLE_TAG}"
    defineEnvvar BUILD_METADATA ""
    DOCKER_TAG="${CIRCLE_TAG#v}"
else
    defineEnvvar VERSION "$(sed -n 's/[[:blank:]]Version[[:blank:]]*=[[:blank:]]*"\(.*\)"/\1/p' internal/version/version.go)"
    defineEnvvar BUILD_METADATA "build.${CIRCLE_BUILD_NUM}"
    DOCKER_TAG="${CIRCLE_SHA1:0:8}.${CIRCLE_BUILD_NUM}"
fi
defineEnvvar SETUPTOOLS_IMG "gcr.io/${GOOGLE_PROJECT_ID}/setuptools:${DOCKER_TAG}"
defineEnvvar REDSKYCTL_IMG "gcr.io/${GOOGLE_PROJECT_ID}/redskyctl:${DOCKER_TAG}"
defineEnvvar IMG "gcr.io/${GOOGLE_PROJECT_ID}/${CIRCLE_PROJECT_REPONAME}:${DOCKER_TAG}"
defineEnvvar PULL_POLICY "Always"
echo


echo "Installing Kubebuilder"
curl -LOs https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
tar -zxvf kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
sudo mv kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64 /usr/local/kubebuilder
PATH=$PATH:/usr/local/kubebuilder/bin
echo


echo "Installing ghr"
curl -LOs https://github.com/tcnksm/ghr/releases/download/v${GHR_VERSION}/ghr_v${GHR_VERSION}_linux_amd64.tar.gz
tar -zxf ghr_v${GHR_VERSION}_linux_amd64.tar.gz --exclude '*/*[^ghr]' --strip-components=1
sudo mv ghr /usr/local/bin/
echo


echo "Installing Google Cloud SDK"
curl -s https://sdk.cloud.google.com | bash -s -- --disable-prompts > /dev/null
PATH=$PATH:~/google-cloud-sdk/bin
echo


echo "Updating PATH"
defineEnvvar PATH "$PATH"
export PATH
