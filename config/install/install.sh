#!/bin/sh
set -e

WORKSPACE=${WORKSPACE:-/workspace}
cd "$WORKSPACE/install"

# Namespace support
if [ -n "$NAMESPACE" ] ; then
    kustomize edit set namespace "$NAMESPACE"
fi

# Append file contents (or stdin for "-") to the manager secrets
if [ -n "$1" ]; then
    cat "$1" >> manager.env
fi

kustomize build
