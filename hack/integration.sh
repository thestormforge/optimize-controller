#!/bin/bash -x

set -e

REDSKYCTL_BIN="${REDSKYCTL_BIN:=dist/redskyctl_linux_amd64/redskyctl}"

echo "Upload image to KinD"
[[ -n "${IMG}" ]] && kind load docker-image "${IMG}" --name chart-testing
[[ -n "${SETUPTOOLS_IMG}" ]] && kind load docker-image "${SETUPTOOLS_IMG}" --name chart-testing

echo "Init redskyops"
${REDSKYCTL_BIN} init

echo "Wait for controller"
# TODO
## This can fail if the resource does not exist yet because we have all the Ghz
## + ./redskyctl-bin check controller --wait
## error: no matching resources found
## Error: exit status 1
sleep 5
${REDSKYCTL_BIN} check controller --wait

echo "Create nginx deployment"
kubectl apply -f hack/nginx.yaml

stats() {
  ${REDSKYCTL_BIN} generate experiment -f hack/app.yaml
  kubectl get pods -o wide
  kubectl get trial -o wide
  kubectl logs -n redsky-system -l control-plane=controller-manager --tail=-1
}

generateAndWait() {
  echo "Create ci experiment"
  ${REDSKYCTL_BIN} generate experiment -f ${1} | \
    kubectl apply -f -

  echo "Create new trial"
  ${REDSKYCTL_BIN} generate experiment -f ${1} | \
  ${REDSKYCTL_BIN} generate trial \
    --default base \
    -f - | \
    kubectl create -f -

  kubectl get pods -o wide
  kubectl get trial -o wide

  waitTime=300s
  echo "Wait for trial to complete (${waitTime} timeout)"
  kubectl wait trial \
    -l redskyops.dev/application=ci \
    --for condition=redskyops.dev/trial-complete \
    --timeout ${waitTime}

  echo "Wait for trial deletion tasks to finish running"
  kubectl wait deployment \
    redsky-default-prometheus-server \
    --for=delete \
    --timeout ${waitTime}
  kubectl wait job \
    -l redskyops.dev/trial-role=trialSetup \
    --for condition=complete \
    --timeout ${waitTime}

  echo "Remove default experiment"
  ${REDSKYCTL_BIN} generate experiment -f ${1} | \
    kubectl delete -f -
}

trap stats EXIT

generateAndWait hack/app.yaml
generateAndWait hack/app_kube.yaml

