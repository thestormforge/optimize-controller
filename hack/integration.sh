#!/usr/bin/env bash

set -ex

CLI_BIN="${CLI_BIN:-dist/stormforge_linux_amd64/stormforge}"
#${CLI_CONFIG-...} allows us to test just for parameter existance which means we can rely on
# the default and overwrite as needed ( for those pesky cases where you want to do
# CLI_CONFIG="--stormforgeconfig /dev/null" hack/integration.sh
CLI_CONFIG="${CLI_CONFIG-}"
KIND_CLUSTER="${KIND_CLUSTER:-chart-testing}"

echo "Upload image to KinD"
[[ -n "${IMG}" ]] && kind load docker-image "${IMG}" --name="${KIND_CLUSTER}"
[[ -n "${SETUPTOOLS_IMG}" ]] && kind load docker-image "${SETUPTOOLS_IMG}" --name="${KIND_CLUSTER}"

echo "Initialize Controller"
${CLI_BIN} ${CLI_CONFIG} init

echo "Wait for controller"
${CLI_BIN} ${CLI_CONFIG} check controller --wait

echo "Create nginx deployment"
kubectl apply -f hack/nginx.yaml

stats() {
  # Experiment yaml
  ${CLI_BIN} ${CLI_CONFIG} generate experiment -f hack/app.yaml
	# Current cluster state
  kubectl describe trials,jobs,pods -l stormforge.io/application=ci
	# Controller logs
  kubectl logs -n stormforge-system -l control-plane=controller-manager --tail=-1
	# Setup Tasks logs
	kubectl logs -l stormforge.io/trial-role=trialSetup
}

generateAndWait() {
  echo "Create ci experiment"
  ${CLI_BIN} ${CLI_CONFIG} generate experiment -f "${1}" | \
    kubectl apply -f -

  echo "Create new trial"
  ${CLI_BIN} ${CLI_CONFIG} generate experiment -f "${1}" | \
  ${CLI_BIN} ${CLI_CONFIG} generate trial \
    --default base \
    -f - | \
    kubectl create -f -

  kubectl get trial -o wide

  waitTime=300s
  echo "Wait for trial to complete (${waitTime} timeout)"
  kubectl wait trial \
    -l stormforge.io/application=ci \
    --for condition=stormforge.io/trial-complete \
    --timeout ${waitTime}

  echo "Wait for trial deletion tasks to finish running"
  kubectl wait deployment \
    optimize-default-prometheus-server \
    --for=delete \
    --timeout ${waitTime}
  kubectl wait job \
    -l stormforge.io/trial-role=trialSetup \
    --for condition=complete \
    --timeout ${waitTime}

  echo "Remove default experiment"
  ${CLI_BIN} ${CLI_CONFIG} generate experiment -f "${1}" | \
    kubectl delete -f -
}

trap stats EXIT

generateAndWait hack/app.yaml
generateAndWait hack/app_kube.yaml

