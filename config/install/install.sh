#!/bin/sh
set -e

WORKSPACE=${WORKSPACE:-/workspace}
cd "$WORKSPACE/install"

# Namespace support
if [ -n "$NAMESPACE" ] ; then
    kustomize edit set namespace "$NAMESPACE"
fi

kustomize build
