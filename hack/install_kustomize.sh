#!/bin/bash -x

set -e

echo "Install kustomize"
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | \
  bash
[[ ! -f /usr/local/bin/kustomize ]] && sudo mv ./kustomize /usr/local/bin/kustomize
