#!/bin/sh
set -eo pipefail


# Generate installation manifests
if [ "$1" == "install" ] ; then
    shift && /workspace/install/install.sh $@
    exit $?
fi


# Package the Helm chart
if [ "$1" == "chart" ] ; then
    shift && /workspace/chart/build.sh $@
    exit $?
fi


# Create the "base" root
kustomize create --autodetect --recursive --namespace "$NAMESPACE"


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
    create)
        handle () {
            kubectl create -f -
            #if [ -n "$TRIAL" ] && [ -n "$NAMESPACE" ] ; then
            #    kubectl get sts,deploy,ds --namespace "$NAMESPACE" --selector "redskyops.dev/trial=$TRIAL,redskyops.dev/trial-role=trialResource" -o name | xargs -n 1 kubectl rollout status --namespace "$NAMESPACE"
            #fi
        }
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
        handle () { cat ; }
        shift
        ;;
    *)
        echo "unknown argument: $1"
        exit 1
        ;;
    esac
done


# Run Kustomize and pipe it into the handler
kustomize build --enable_alpha_plugins | handle
