#!/bin/sh
set -eo pipefail

if [ -z "$1" ]; then
    echo "usage: chart [VERSION]"
    exit 1
fi

WORKSPACE=${WORKSPACE:-/workspace}

# Post process the deployment manifest
function templatizeDeployment {
    sed '/namespace: redsky-system/d' | \
        sed 's/SECRET_SHA256/{{ include (print $.Template.BasePath "\/secret.yaml") . | sha256sum }}/g' | \
        sed 's/IMG:TAG/{{ .Values.redskyImage }}:{{ .Values.redskyTag }}/g' | \
        sed 's/name: redsky-\(.*\)$/name: "{{ .Release.Name }}-\1"/g'
}

# Post process the RBAC manifest
function templatizeRBAC {
    sed 's/namespace: redsky-system/namespace: {{ .Release.Namespace | quote }}/g' | \
        sed 's/name: redsky-\(.*\)$/name: "{{ .Release.Name }}-\1"/g' | \
        cat "$WORKSPACE/chart/rbac_header.txt" - "$WORKSPACE/chart/rbac_footer.txt"
}


# TODO It seems like the proxy service is in the wrong spot
mv "$WORKSPACE/rbac/auth_proxy_service.yaml" "$WORKSPACE/default/."

cd "$WORKSPACE/install"
kustomize edit remove label "app.kubernetes.io/managed-by"

cd "$WORKSPACE/crd"
kustomize edit add annotation "helm.sh/hook:crd-install"

cd "$WORKSPACE/manager"
kustomize edit set image controller="IMG:TAG"

cd "$WORKSPACE/default"
kustomize edit remove resource "../crd"
kustomize edit remove resource "../rbac"
kustomize edit add resource "auth_proxy_service.yaml"

cd "$WORKSPACE/rbac"
kustomize edit set namespace "redsky-system"
kustomize edit set nameprefix "redsky-"
kustomize edit remove resource "auth_proxy_service.yaml"


# Build the templates for the chart
cd "$WORKSPACE"
kustomize build crd > "$WORKSPACE/chart/redskyops/templates/crds.yaml"
kustomize build rbac | templatizeRBAC > "$WORKSPACE/chart/redskyops/templates/rbac.yaml"
kustomize build chart | templatizeDeployment > "$WORKSPACE/chart/redskyops/templates/deployment.yaml"

# Package everything together using Helm
helm package --save=false --version "$1" "$WORKSPACE/chart/redskyops" > /dev/null
cat "/workspace/redskyops-$1.tgz" | base64
