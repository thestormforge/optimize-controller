FROM alpine:latest

ENV HELM_VERSION="v2.14.3" \
    HELM_URL="https://get.helm.sh/helm-v2.14.3-linux-amd64.tar.gz" \
    HELM_SHA256="3f907cac93c6b18c36910b15af02ddc3d454b59a870899afdfa75548f9a7658d"

ENV KUBECTL_VERSION="v1.14.5" \
    KUBECTL_URL="https://storage.googleapis.com/kubernetes-release/release/v1.14.5/bin/linux/amd64/kubectl" \
    KUBECTL_SHA256="26681319de56820a8467c9407e9203d5b15fb010ffc75ac5b99c9945ad0bd28c"

ENV KUSTOMIZE_VERSION="v3.2.2" \
    KUSTOMIZE_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv3.2.2/kustomize_kustomize.v3.2.2_linux_amd64" \
    KUSTOMIZE_SHA256="c25c41d2295326d0af7c2613ff9d0ce7d3432dfc7a6426da6242213a1fd60e0e"

ENV KONJURE_VERSION="v0.1.0-beta.4" \
    KONJURE_URL="https://github.com/carbonrelay/konjure/releases/download/v0.1.0-beta.4/konjure-linux-amd64" \
    KONJURE_SHA256="e979d6317898c5988f4d6169e2bc320ab228925a722059debce38a28c1feb130"

RUN apk --no-cache add curl && \
    curl -L "$HELM_URL" | tar xz --exclude '*/*[^helm]' --strip-components=1 -C /usr/local/bin/ && \
    curl -L "$KUBECTL_URL" -o /usr/local/bin/kubectl && \
    curl -L "$KUSTOMIZE_URL" -o /usr/local/bin/kustomize && \
    curl -L "$KONJURE_URL" -o /usr/local/bin/konjure && \
    chmod +x /usr/local/bin/kubectl /usr/local/bin/kustomize /usr/local/bin/konjure && \
    addgroup -g 1000 -S setup && \
    adduser -u 1000 -S setup -G setup

COPY . /workspace/

ARG IMG
ARG PULL_POLICY
ARG VERSION
RUN cd /workspace/manager && kustomize edit set image controller=${IMG} && \
    sed -i "s|VERSION|${VERSION:-unknown}|g" /workspace/install/metadata_labels.yaml && \
    sed -i "s|PULL_POLICY|${PULL_POLICY}|g" /workspace/install/manager_patch.yaml /workspace/chart/redskyops/values.yaml && \
    sed -i "s|IMG|${IMG%:*}|g" /workspace/chart/redskyops/values.yaml /workspace/chart/redskyops/questions.yml && \
    sed -i "s|TAG|${IMG##*:}|g" /workspace/chart/redskyops/values.yaml /workspace/chart/redskyops/questions.yml && \
    echo "appVersion: ${VERSION}" >> /workspace/chart/redskyops/Chart.yaml && \
    mkdir -p /workspace/base && touch /workspace/base/kustomization.yaml && \
    mkdir -p /workspace/helm && touch /workspace/helm/kustomization.yaml && \
    chown -R setup /workspace

USER setup:setup
RUN konjure kustomize init

WORKDIR "/workspace/base"
ENTRYPOINT ["/workspace/docker-entrypoint.sh"]
CMD ["install", "--dry-run"]
