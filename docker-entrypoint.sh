#!/bin/sh
set -eo pipefail


# Update the Kustomization to account for mounted files
# This only applies to the "base" (default from Dockerfile) root, so do it before processing arguments
if [ -e kustomization.yaml ]; then
    find . -name "*_resource.yaml" -o -path "./resources/*.yaml" -exec kustomize edit add resource {} +
    find . -name "*_patch.yaml" -o -path "./patches/*.yaml" -exec kustomize edit add patch {} +
fi


# Process arguments
while [ "$#" != "0" ] ; do
    case "$1" in
    install)
        cd /cordelia/client
        # TODO This is temporary until the CRD is part of the default Kustomization
        kustomize edit add base ../crd
        handle () { kubectl create -f - ; }
        shift
        ;;
    create)
        handle () { kubectl create -f - ; }
        shift
        ;;
    apply)
        handle () { kubectl apply -f - ; }
        shift
        ;;
    delete)
        handle () { kubectl delete -f - ; }
        shift
        ;;
    --manifests)
        handle () { cat ; }
        shift
        ;;
    *)
        echo "unknown argument: $1"
        exit 1
        ;;
    esac
done


# Helm support
if [ -n "$CHART" ] ; then
    if [ ! -d "$(helm home)" ]; then
        echo "Helm home ($(helm home)) is not a directory, initializing"
        helm init --client-only
    fi

    mkdir -p /workspace/helm
    cd /workspace/helm
    touch kustomization.yaml
    kustomize edit add base ../base

    find . -name "*patch.yaml" -exec kustomize edit add patch {} +
    values=$(find . -name "*values.yaml" -exec echo -n "--values {} " \;)

    helm fetch "$CHART"
    for c in *.tgz ; do
        # TODO HELM_OPTS can't be trusted, how do we sanitize that?
        helm template $values $HELM_OPTS $c > ${c%%.tgz}.yaml
        kustomize edit add resource ${c%%.tgz}.yaml
    done
fi


# Run Kustomize and pipe it into the handler
kustomize build | handle
