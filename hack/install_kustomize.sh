#!/bin/bash -x

set -e

echo "Install kustomize"
sudo snap remove kustomize
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | \
  bash
sudo mv ./kustomize /usr/local/bin/kustomize
