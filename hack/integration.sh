#!/bin/bash -x

set -e

REDSKYCTL_BIN="${REDSKYCTL_BIN:=dist/redskyctl_linux_amd64/redskyctl}"

echo "Upload image to KinD"
[[ -n "${IMG}" ]] && kind load docker-image "${IMG}" --name chart-testing
[[ -n "${SETUPTOOLS_IMG}" ]] && kind load docker-image "${SETUPTOOLS_IMG}" --name chart-testing

echo "Init redskyops"
${REDSKYCTL_BIN} init

echo "Wait for controller"
${REDSKYCTL_BIN} check controller --wait

echo "Create nginx deployment"
kubectl apply -f hack/nginx.yaml

echo "Create ci experiment"
${REDSKYCTL_BIN} generate experiment -f hack/app.yaml > hack/experiment.yaml
kubectl apply -f hack/experiment.yaml

echo "Create new trial"
${REDSKYCTL_BIN} generate trial \
  --assign cpu=50 \
  --assign memory=50 \
  -f hack/experiment.yaml | \
    kubectl create -f -

kubectl get trial -o wide

echo "Wait for trial to complete (120s timeout)"
kubectl wait trial \
  -l redskyops.dev/application=ci \
  --for condition=redskyops.dev/trial-complete \
  --timeout 120s
