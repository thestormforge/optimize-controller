#!/bin/sh
set -eo pipefail

# Parse arguments and set variables
if [ -z "$1" ]; then
    echo "usage: $(basename $0) [CHART_VERSION]"
    exit 1
fi
CHART_VERSION=$1
WORKSPACE=${WORKSPACE:-/workspace}


# Post process the deployment manifest
function templatizeDeployment {
    sed '/namespace: redsky-system/d' | \
        sed 's/SECRET_SHA256/{{ include (print $.Template.BasePath "\/secret.yaml") . | sha256sum }}/g' | \
        sed 's/VERSION/{{ .Chart.AppVersion | quote }}/g' | \
        sed 's/IMG:TAG/{{ .Values.redskyImage }}:{{ .Values.redskyTag }}/g' | \
        sed 's/PULL_POLICY/{{ .Values.redskyImagePullPolicy }}/g' | \
        sed 's/name: redsky-\(.*\)$/name: "{{ .Release.Name }}-\1"/g'
}

# Post process the RBAC manifest
function templatizeRBAC {
    sed 's/namespace: redsky-system/namespace: {{ .Release.Namespace | quote }}/g' | \
        sed 's/name: redsky-\(.*\)$/name: "{{ .Release.Name }}-\1"/g' | \
        cat "$WORKSPACE/chart/rbac_header.txt" - "$WORKSPACE/chart/rbac_footer.txt"
}

# Post processing to add recommended labels
function label {
    sed '/^  labels:$/,/^    app\.kubernetes\.io\/name: redskyops$/c\
  labels:\
    app.kubernetes.io/name: redskyops\
    helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}"\
    app.kubernetes.io/managed-by: {{ .Release.Service | quote }}\
    app.kubernetes.io/instance: {{ .Release.Name | quote }}\
    app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}'
}


# Move non-role resource out of RBAC
mv "$WORKSPACE/rbac/auth_proxy_service.yaml" "$WORKSPACE/default/."


# Edit the kustomizations for templatization

cd "$WORKSPACE/install"
konjure kustomize edit remove transformer metadata_labels.yaml

cd "$WORKSPACE/crd"
kustomize edit add label "app.kubernetes.io/name:redskyops"
# NOTE: Not using the crd-install hook is more consistent with `redskyctl reset`
# kustomize edit add annotation "helm.sh/hook:crd-install"

cd "$WORKSPACE/manager"
kustomize edit set image controller="IMG:TAG"

cd "$WORKSPACE/default"
kustomize edit remove resource "../crd"
kustomize edit remove resource "../rbac"
kustomize edit add resource "auth_proxy_service.yaml"

cd "$WORKSPACE/rbac"
kustomize edit add label "app.kubernetes.io/name:redskyops"
kustomize edit set namespace "redsky-system"
kustomize edit set nameprefix "redsky-"
kustomize edit remove resource "auth_proxy_service.yaml"


# Build the templates for the chart
cd "$WORKSPACE"
kustomize build crd | label > "$WORKSPACE/chart/redskyops/templates/crds.yaml"
kustomize build rbac | templatizeRBAC | label > "$WORKSPACE/chart/redskyops/templates/rbac.yaml"
kustomize build chart | templatizeDeployment | label > "$WORKSPACE/chart/redskyops/templates/deployment.yaml"


# Package everything together using Helm
helm package --save=false --version "$CHART_VERSION" "$WORKSPACE/chart/redskyops" > /dev/null
cat "/workspace/redskyops-$CHART_VERSION.tgz" | base64
