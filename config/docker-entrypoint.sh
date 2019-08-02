#!/bin/sh
set -eo pipefail

# Require a kustomization.yaml file to exist for edits
if [ ! -e kustomization.yaml ] ; then
    echo "Error: unable to find 'kustomization.yaml' in directory '$(pwd)'"
fi


# Update the Kustomization to account for mounted files
# This only applies to the "base" (default from Dockerfile) root, so do it before processing arguments
find . -type f \( -name "*_resource.yaml" -o -path "./resources/*.yaml" \) -exec kustomize edit add resource {} +
find . -type f \( -name "*_patch.yaml" -o -path "./patches/*.yaml" \) -exec kustomize edit add patch {} +


# Process arguments
while [ "$#" != "0" ] ; do
    case "$1" in
    install)
        cd /workspace/install
        handle () { kubectl apply -f - ; }
        shift
        ;;
    uninstall)
        cd /workspace/install
        handle () { kubectl delete -f - ; }
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
    --dry-run)
        # TODO Should this just add --dry-run to the kubectl invocation?
        handle () { cat ; }
        shift
        ;;
    *)
        echo "unknown argument: $1"
        exit 1
        ;;
    esac
done


# Namespace support
if [ -n "$NAMESPACE" ] ; then
    kustomize edit set namespace "$NAMESPACE"
fi


# Helm support
if [ -n "$CHART" ] ; then
    if [ ! -d "$(helm home)" ]; then
        helm init --client-only > /dev/null
    fi

    mkdir -p /workspace/helm
    cd /workspace/helm
    touch kustomization.yaml

    kustomize edit add base ../base
    find . -type f -name "*patch.yaml" -exec kustomize edit add patch {} +
    values=$(find . -type f -name "*values.yaml" -exec echo -n "--values {} " \;)

    helm fetch "$CHART"
    for c in *.tgz ; do
        # TODO HELM_OPTS can't be trusted, how do we sanitize that?
        eval helm template $values $HELM_OPTS $c > ${c%%.tgz}.yaml 2> /dev/null
        kustomize edit add resource ${c%%.tgz}.yaml
    done
fi


# TODO Should we apply a label to all created objects so we can tie it back to the trial? Only if this isn't "install"? Only if $TRIAL is defined?


# Run Kustomize and pipe it into the handler
kustomize build | handle
