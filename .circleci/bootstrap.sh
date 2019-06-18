#!/usr/bin/env bash
set -euo pipefail

# Install make
apt-get update -y && apt-get install -yq make

# Determine what version information to embed
if [[ -n "${CIRCLE_TAG:-}" ]]; then
  VERSION="${CIRCLE_TAG}"
  BUILD_METADATA=""
  IMG_TAG="${VERSION}"
else
  VERSION="$(sed -n 's/[[:blank:]]Version[[:blank:]]*=[[:blank:]]*"\(.*\)"/\1/p' pkg/version/version.go)"
  BUILD_METADATA="build.${CIRCLE_BUILD_NUM:-0}"
  IMG_TAG="${VERSION}+${BUILD_METADATA}"
fi

# Expose the environment variables
echo "Cordelia tag is $IMG_TAG"
echo "export IMG=\"gcr.io/${GOOGLE_PROJECT_ID}/cordelia:${IMG_TAG}\"" >> $BASH_ENV
echo "export VERSION=\"${VERSION}\"" >> $BASH_ENV
echo "export BUILD_METADATA=\"${BUILD_METADATA}\"" >> $BASH_ENV


