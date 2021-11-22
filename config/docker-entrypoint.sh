#!/bin/sh
set -ex

waitFn() { :; }

while [ "$#" != "0" ] ; do
  case "$1" in
    prometheus)
      shift

      export HELM_CONFIG="$(cat <<-EOF | base64
			apiVersion: konjure.carbonrelay.com/v1beta1
			kind: HelmGenerator
			metadata:
			  name: prometheus
			releaseName: optimize-${NAMESPACE}-prometheus
			releaseNamespace: ${NAMESPACE}
			chart: ../prometheus
			EOF
      )"
    ;;
    build)
      shift
      export MODE=build
    ;;
    *)
      shift
    ;;
  esac
done

if [ -z "${MODE}" ]; then
  export MODE=build
fi

# Create the "base" root
if [ ! -f kustomization.yaml ]; then
  kustomize create

  # TODO --autodetect fails with symlinked directories
  find . -type f -name "*.yaml" ! -name "kustomization.yaml" ! -name "helm.yaml" -exec kustomize edit add resource {} +
fi


if [ -n "$NAMESPACE" ]; then
  kustomize edit set namespace "$NAMESPACE"
fi


# Add Helm configuration
if [ -n "$HELM_CONFIG" ] ; then
    echo "$HELM_CONFIG" | base64 -d > helm.yaml

    if [ -n "$(grep -e 'chart: .*../prometheus' helm.yaml)" ] && \
       [ -z "$(grep -w releaseName helm.yaml)" ] && \
       [ -n "$NAMESPACE" ]; then
      echo "releaseName: optimize-${NAMESPACE}-prometheus" >> helm.yaml
    fi

    # Ensure releaseNamespace is present
    if [ -z "$(grep -w releaseNamespace helm.yaml)" ] && [ -n "$NAMESPACE" ]; then
      echo "releaseNamespace: ${NAMESPACE}" >> helm.yaml
    fi

    # Add a wait function for prometheus resources
    if [ -n "$(grep -e 'chart: .*../prometheus' helm.yaml)" ]; then
      releaseName=$(awk '/releaseName/ { print $NF }' helm.yaml)

      waitFn() {
        # Wait on {{ releaseName }}-server
        kubectl wait --for condition=Available=true --namespace "${NAMESPACE}" --timeout 5m deployment.apps "${releaseName}"
      }
    fi

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
case "${MODE}" in
  create)
    handle () {
        # Note, this *must* be create for `generateName` to work properly
        kubectl create -f -
    }

    ;;
  delete)
    handle () {
        kubectl delete -f -
    }

    waitFn () {
        if [ -n "$TRIAL" ] && [ -n "$NAMESPACE" ] ; then
            kubectl wait pods --for=delete --namespace "$NAMESPACE" --selector "stormforge.io/trial=$TRIAL,stormforge.io/trial-role=trialResource" --timeout=5m
        fi
    }

    ;;
  build)
    handle () { cat ; }
    waitFn () { :; }
    ;;
  *)
    echo "unknown argument: $1"
    exit 1
    ;;
esac

# Run Kustomize and pipe it into the handler
kustomize build --enable_alpha_plugins | handle
waitFn
