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

Stats() {
  ${REDSKYCTL_BIN} generate experiment -f hack/app.yaml
  kubectl get pods -o wide
  kubectl get trial -o wide
  kubectl logs -n redsky-system -l control-plane=controller-manager --tail=-1
}

trap Stats EXIT

echo "Create ci experiment"
${REDSKYCTL_BIN} generate experiment -f hack/app.yaml | \
  kubectl apply -f -

echo "Create new trial"
${REDSKYCTL_BIN} generate experiment -f hack/app.yaml | \
${REDSKYCTL_BIN} generate trial \
  --default base \
  -f - | \
  kubectl create -f -

kubectl get trial -o wide


waitTime=300s
echo "Wait for trial to complete (${waitTime} timeout)"
kubectl wait trial \
  -l redskyops.dev/application=ci \
  --for condition=redskyops.dev/trial-complete \
  --timeout ${waitTime}
