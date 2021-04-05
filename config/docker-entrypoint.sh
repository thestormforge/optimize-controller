#!/bin/sh
set -e

if [ "${1}" == "prometheus" ]; then
  PROMETHEUS=true

  # Generate prometheus manifests
  shift && cd /workspace/prometheus

  namePrefix="redsky-"
  if [ -n "$NAMESPACE" ]; then
    namePrefix="redsky-$NAMESPACE-"
  fi

  kustomize edit set nameprefix "$namePrefix"

  function promCreateWaitFn() {
    kubectl wait deployment.apps \
      --for condition=Available=true \
      --timeout 120s \
      ${namePrefix}prometheus-server
  }

  function promDeleteWaitFn() {
    kubectl wait deployment.apps \
      --for delete \
      --timeout 120s \
      ${namePrefix}prometheus-server
  }
fi

function createFn() {
  # Note, this *must* be create for `generateName` to work properly
  kubectl create -f -
}

function createWaitFn() {
  #if [ -n "$TRIAL" ] && [ -n "$NAMESPACE" ] ; then
  #    kubectl get sts,deploy,ds --namespace "$NAMESPACE" --selector "redskyops.dev/trial=$TRIAL,redskyops.dev/trial-role=trialResource" -o name | xargs -n 1 kubectl rollout status --namespace "$NAMESPACE"
  #fi
  :
}

function deleteFn() {
  kubectl delete -f -
}

function deleteWaitFn() {
  if [ -n "$TRIAL" ] && [ -n "$NAMESPACE" ]; then
    kubectl wait pods \
      --for=delete \
      --namespace "$NAMESPACE" \
      --selector "redskyops.dev/trial=$TRIAL,redskyops.dev/trial-role=trialResource"
  fi
}


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
		  "redskyops.dev/trial": $TRIAL
		  "redskyops.dev/trial-role": trialResource
		EOF
    konjure kustomize edit add transformer trial_labels.yaml
fi


# Process arguments
while [ "$#" != "0" ] ; do
  case "$1" in
  create)
    if [ -n "${PROMETHEUS}" ]; then
      handle () {
        # Feel like this should be `cat -` but busybox cat doesnt recognize that.
        yamls=$(cat)

        # We may or may not have resources here, so allow this to fail.
        printf "${yamls}" | deleteFn || true
        # Cant use the generic deleteWaitFn because our trials will be different
        promDeleteWaitFn || true

        printf "${yamls}" | createFn
        promCreateWaitFn
      }
    else
      handle () {
        cat | createFn
        createWaitFn
      }
    fi

    break
    ;;
  delete)
    handle () {
        cat | deleteFn
        deleteWaitFn
    }

    break
    ;;
  build)
    handle () { cat ; }
    break
    ;;
  *)
    echo "unknown argument: $1"
    exit 1
    ;;
  esac
done


# Run Kustomize and pipe it into the handler
kustomize build --enable_alpha_plugins | handle
