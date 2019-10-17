#!/bin/sh
set -eo pipefail


# Package the Helm chart and do nothing else
if [ "$1" == "chart" ] ; then
    shift && /workspace/chart/build.sh $@
    exit $?
fi


# Create the "base" root
kustomize create --autodetect --recursive


# Detect and add patches
find . -type f \( -name "*_patch.yaml" -o -path "./patches/*.yaml" \) -exec kustomize edit add patch {} +


# Add Helm configuration
if [ -n "$HELM_CONFIG" ] ; then
    echo "$HELM_CONFIG" | base64 -d > helm.yaml
    konjure kustomize edit add generator helm.yaml
fi


# Add trial labels to the resulting manifests so they can be more easily located
if [ -n "$TRIAL" ]; then
    cat <<-EOF >"trial_labels.yaml"
		apiVersion: konjure.carbonrelay.com/v1beta1
		kind: LabelTransformer
		metadata:
		  name: trial-labels
		labels:
		  "redskyops.dev/trial": $TRIAL
		  "redskyops.dev/trial-role": trialResource
		EOF
    konjure kustomize edit add transformer trial_labels.yaml
fi


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
        handle () {
            kubectl delete -f -
            if [ -n "$TRIAL" ] && [ -n "$NAMESPACE" ] ; then
                kubectl wait pods --for=delete --namespace "$NAMESPACE" --selector "redskyops.dev/trial=$TRIAL,redskyops.dev/trial-role=trialResource"
            fi
        }
        shift
        ;;
    --dry-run)
        sed -i '\|app.kubernetes.io/managed-by|d' /workspace/install/metadata_labels.yaml
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


# Run Kustomize and pipe it into the handler
kustomize build --enable_alpha_plugins | handle
