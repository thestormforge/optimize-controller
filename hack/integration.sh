#!/bin/bash -x

set -e

echo "Upload image to KinD"
[[ -n "${IMG}" ]] && kind load docker-image "${IMG}" --name chart-testing
[[ -n "${SETUPTOOLS_IMG}" ]] && kind load docker-image "${SETUPTOOLS_IMG}" --name chart-testing
echo "Init redskyops"
dist/redskyctl_linux_amd64/redskyctl init --wait
echo "Create ci experiment"
kubectl apply -f hack/experiment.yaml
echo "Create new trial"
dist/redskyctl_linux_amd64/redskyctl generate trial \
  --assign something=500 \
  --assign another=100 \
  -f hack/experiment.yaml | \
    kubectl create -f -
kubectl get trial -o wide
# Change this back to a higher value when we can schedule the trial
echo "Wait for trial to complete (300s timeout)"
kubectl wait trial \
  -l redskyops.dev/experiment=ci \
  --for condition=redskyops.dev/trial-complete \
  --timeout 300s
