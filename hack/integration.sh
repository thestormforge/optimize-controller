#!/usr/bin/env bash

set -ex

make cli
CLI_BIN="${CLI_BIN:-dist/optimize-controller_$(go env GOOS)_amd64_v1/stormforge}"
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

generate() {
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
}

waitFn() {
  waitTime=300s
  echo "Wait for trial to complete (${waitTime} timeout)"
  kubectl wait trial \
    -l stormforge.io/application=ci \
    --for condition=stormforge.io/trial-complete \
    --timeout ${waitTime}

  echo "Wait for trial deletion tasks to finish running"
  kubectl wait deployment \
    -l app.kubernetes.io/name=optimize-prometheus \
    --for=delete \
    --timeout ${waitTime}
  kubectl wait job \
    -l stormforge.io/trial-role=trialSetup \
    --for condition=complete \
    --timeout ${waitTime}
}

cleanup() {
  echo "Remove default experiment"
  ${CLI_BIN} ${CLI_CONFIG} generate experiment -f "${1}" | \
    kubectl delete -f -
}

apply() {
  echo "Create ci experiment"
  kubectl apply -f "${1}"

  echo "Create new trial"
  ${CLI_BIN} ${CLI_CONFIG} generate trial \
    --default base \
    -f "${1}" | \
    kubectl create -f -

  kubectl get trial -o wide
}

trap stats EXIT

# Application using local resources
generate hack/app.yaml; waitFn; cleanup hack/app.yaml
# Application using in cluster resources
generate hack/app_kube.yaml; waitFn; cleanup hack/app_kube.yaml
# Experiment using custom settings for included prometheus stack
apply hack/experiment.yaml; waitFn; kubectl delete -f hack/experiment.yaml
