#!/usr/bin/env bash
set -euo pipefail

echo "Installing make"
apt-get update -yq && apt-get install -yq make

function defineEnvvar {
    echo "  $1=$2"
    echo "export $1=\"$2\"" >> $BASH_ENV
}

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
defineEnvvar DOCKER_TAG "${DOCKER_TAG}"
defineEnvvar IMG "gcr.io/${GOOGLE_PROJECT_ID}/${CIRCLE_PROJECT_REPONAME}:${DOCKER_TAG}"
