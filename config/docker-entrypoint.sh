#!/bin/sh
set -e

case "$1" in
  prometheus)
    # Generate prometheus manifests
    shift && cd /workspace/prometheus

    namePrefix="optimize-"
    if [ -n "$NAMESPACE" ]; then
      namePrefix="optimize-$NAMESPACE-"
    fi

    kustomize edit set nameprefix "$namePrefix"
    waitFn() {
      kubectl wait --for condition=Available=true --timeout 120s deployment.apps ${namePrefix}prometheus-server
    }
  ;;
  *)
    waitFn() { :; }
  ;;
esac


# Create the "base" root
if [ ! -f kustomization.yaml ]; then
  kustomize create

  # TODO --autodetect fails with symlinked directories
  find . -type f -name "*.yaml" ! -name "kustomization.yaml" -exec kustomize edit add resource {} +
fi


if [ -n "$NAMESPACE" ]; then
  kustomize edit set namespace "$NAMESPACE"
fi


# Add Helm configuration
if [ -n "$HELM_CONFIG" ] ; then
    echo "$HELM_CONFIG" | base64 -d > helm.yaml
    konjure kustomize edit add generator helm.yaml
fi


# Add trial labels to the resulting manifests so they can be more easily located
if [ -n "$TRIAL" ]; then
    # Note, this heredoc block must be indented with tabs
    # <<- allows for indentation via tabs, if spaces are used it is no good.
    cat <<-EOF >"trial_labels.yaml"
		apiVersion: konjure.carbonrelay.com/v1beta1
		kind: LabelTransformer
		metadata:
		  name: trial-labels
		labels:
		  "stormforge.io/trial": $TRIAL
		  "stormforge.io/trial-role": trialResource
		EOF
    konjure kustomize edit add transformer trial_labels.yaml
fi


# Process arguments
while [ "$#" != "0" ] ; do
    case "$1" in
    create)
        handle () {
            # Note, this *must* be create for `generateName` to work properly
            kubectl create -f -
            #if [ -n "$TRIAL" ] && [ -n "$NAMESPACE" ] ; then
            #    kubectl get sts,deploy,ds --namespace "$NAMESPACE" --selector "stormforge.io/trial=$TRIAL,stormforge.io/trial-role=trialResource" -o name | xargs -n 1 kubectl rollout status --namespace "$NAMESPACE"
            #fi
        }
        shift
        ;;
    delete)
        handle () {
            kubectl delete -f -
            if [ -n "$TRIAL" ] && [ -n "$NAMESPACE" ] ; then
                kubectl wait pods --for=delete --namespace "$NAMESPACE" --selector "stormforge.io/trial=$TRIAL,stormforge.io/trial-role=trialResource"
            fi
        }
        waitFn () { :; }
        shift
        ;;
    build)
        handle () { cat ; }
        waitFn () { :; }
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
waitFn
