#!/bin/sh
set -eo pipefail


# Parse arguments and set variables
if [ -z "$1" -o -z "$2" ]; then
    echo "usage: $(basename $0) [CHART_FORMAT] [CHART_VERSION]"
    exit 1
fi
CHART_FORMAT=$1
CHART_VERSION=$2
WORKSPACE=${WORKSPACE:-/workspace}


# Post process the deployment manifest
function templatizeDeployment {
    sed '/namespace: redsky-system/d' | \
        sed 's/SECRET_SHA256/{{ include (print $.Template.BasePath "\/secret.yaml") . | sha256sum }}/g' | \
        sed 's/RELEASE_NAME/{{ .Release.Name | quote }}/g' | \
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
    sed '/creationTimestamp: null/d' | \
    sed '/^  labels:$/,/^    app\.kubernetes\.io\/name: redskyops$/c\
  labels:\
    app.kubernetes.io/name: redskyops\
    app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}\
    app.kubernetes.io/instance: {{ .Release.Name | quote }}\
    app.kubernetes.io/managed-by: {{ .Release.Service | quote }}\
    helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}"'
}


# For the installation root we remove the labels that are typically added during
# an install because the chart will contain templatized labels instead.
cd "$WORKSPACE/install"
konjure kustomize edit remove transformer metadata_labels.yaml


# For the default root we are going to relocate the HTTPS proxy used for serving
# Prometheus metrics since it would otherwise be in RBAC (which would put it in
# a conditional block in the chart template). We also separate out the different
# resources so they can be built individually.
cd "$WORKSPACE/default"
mv "$WORKSPACE/rbac/auth_proxy_service.yaml" .
kustomize edit add resource "auth_proxy_service.yaml"
kustomize edit remove resource "../crd"
kustomize edit remove resource "../rbac"


# For the manager we need to replace the image name with something that will
# match the filters later.
cd "$WORKSPACE/manager"
kustomize edit set image controller="IMG:TAG"


# For the CRD resources we need to add back the "name" label so the label filters
# will find it. We do not add the Helm CRD hook annotation because the CRD isn't
# used during the installation process.
cd "$WORKSPACE/crd"
kustomize edit add label "app.kubernetes.io/name:redskyops"


# For the RBAC resources we need to add back the "name" label so the label filters
# will find it and because we removed it from the default root, we need to add
# back the name prefix and namespace transformations.
cd "$WORKSPACE/rbac"
kustomize edit remove resource "auth_proxy_service.yaml"
kustomize edit add label "app.kubernetes.io/name:redskyops"
kustomize edit set namespace "redsky-system"
kustomize edit set nameprefix "redsky-"


# Build the templates for the chart
cd "$WORKSPACE"
kustomize build crd | label > "$WORKSPACE/chart/redskyops/templates/crds.yaml"
kustomize build rbac | templatizeRBAC | label > "$WORKSPACE/chart/redskyops/templates/rbac.yaml"
kustomize build chart | templatizeDeployment | label > "$WORKSPACE/chart/redskyops/templates/deployment.yaml"


# Remove icon reference from Rancher chart
if [ "$CHART_FORMAT" == "rancher" ] ; then
    sed -i 's|^icon: .*/icon.png$|icon: file://../icon.png|' "$WORKSPACE/chart/redskyops/Chart.yaml"
fi


# Package everything together using Helm
helm package --save=false --version "$CHART_VERSION" "$WORKSPACE/chart/redskyops" > /dev/null


# Output the chart in the expected format
case "$CHART_FORMAT" in
helm)
    tar c -z -C "$WORKSPACE" "redskyops-$CHART_VERSION.tgz" | base64
    ;;
rancher)
    BASE="$WORKSPACE/chart/rancher"
    DEST="$BASE/charts/redskyops/v$CHART_VERSION"
    mkdir -p "$DEST"
    tar x -z --strip-components 1 -f "$WORKSPACE/redskyops-$CHART_VERSION.tgz" -C "$DEST"
    tar c -z -C "$BASE" . | base64
    ;;
digitalocean)
    BASE="$WORKSPACE/chart/digitalocean"
    DEST="$BASE/src/redskyops/$CHART_VERSION"
    mkdir -p "$DEST"
    tar x -z --strip-components 1 -f "$WORKSPACE/redskyops-$CHART_VERSION.tgz" -C "$DEST"
    tar c -z -C "$BASE" . | base64
    ;;
*)
    echo "Unknown chart format: $CHART_FORMAT"
    exit 1
esac
