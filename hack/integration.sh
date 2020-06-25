#!/bin/bash -x

set -e

echo "Upload image to KinD"
kind load docker-image "${IMG}" --name chart-testing
kind load docker-image "${SETUPTOOLS_IMG}" --name chart-testing
echo "Init redskyops"
dist/redskyctl_linux_amd64/redskyctl init --wait
echo "Create postgres experiment"
kustomize build hack | kubectl apply -f -
echo "Create new trial"
dist/redskyctl_linux_amd64/redskyctl generate trial \
  --assign memory=500 \
  --assign cpu=100 \
  -f <(kubectl get experiment postgres-example -o yaml) | \
    kubectl create -f -
kubectl get trial -o wide
# Change this back to a higher value when we can schedule the trial
echo "Wait for trial to complete (300s timeout)"
kubectl wait trial \
  -l redskyops.dev/experiment=postgres-example \
  --for condition=redskyops.dev/trial-complete \
  --timeout 300s
kubectl get trial -o wide
kubectl get pods -o wide -l redskyops.dev/experiment=postgres-example
