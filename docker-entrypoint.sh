#!/bin/sh

if [ "$1" == "install" ]; then
    kubectl create -k /cordelia/crd
    kubectl create -k /cordelia/client
    exit
fi


# Handle shell scripts; do this first so they can have side effects
if [ -d "/setup.d" ]; then
    for f in /setup.d/*.sh; do
        /bin/sh -c "$f"
    done
fi

if [ ! -z "$CHART" ]; then
    # Handle Helm Chart
    case "$1" in
    create)
        # Only run `helm init` if it hasn't already been run so you can change the repo using setup scripts
        if [ ! -d "$(helm home)" ]; then
            helm init --client-only
        fi

        # Check if a values file was mounted for this chart
        if [ -f "/values/$NAME.yaml" ]; then
            HELM_OPTS="${HELM_OPTS} --values /values/$NAME.yaml"
        fi

        helm install $HELM_OPTS --namespace "$NAMESPACE" --name "$NAME" "$CHART"
        ;;
    delete)
        helm delete $HELM_OPTS --purge "$NAME"
        ;;
    esac
elif [ -d "/manifests" ]; then
    if [ -n "$(ls -A /manifests)" ]; then
        # Handle Kubectl manifests
        case "$1" in
        create)
            kubectl create --namespace "$NAMESPACE" --filename "/manifests"
            ;;
        delete)
            kubectl delete --namespace "$NAMESPACE" --filename "/manifests"
            ;;
        esac
    fi
fi

