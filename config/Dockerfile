FROM alpine:latest

ENV HELM_VERSION="v2.14.3" \
    HELM_URL="https://get.helm.sh/helm-v2.14.3-linux-amd64.tar.gz" \
    HELM_SHA256="3f907cac93c6b18c36910b15af02ddc3d454b59a870899afdfa75548f9a7658d"

ENV KUBECTL_VERSION="v1.14.5" \
    KUBECTL_URL="https://storage.googleapis.com/kubernetes-release/release/v1.14.5/bin/linux/amd64/kubectl" \
    KUBECTL_SHA256="26681319de56820a8467c9407e9203d5b15fb010ffc75ac5b99c9945ad0bd28c"

ENV KUSTOMIZE_VERSION="v3.5.3" \
    KUSTOMIZE_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv3.5.3/kustomize_v3.5.3_linux_amd64.tar.gz" \
    KUSTOMIZE_SHA256="e0b86d6fc5f46b83b9ee339e98bf265354b66d1ea9bf168a2077d6b5914afd78"

ENV KONJURE_VERSION="v0.1.0" \
    KONJURE_URL="https://github.com/carbonrelay/konjure/releases/download/v0.1.0/konjure-linux-amd64.tar.gz" \
    KONJURE_SHA256="2b5243c30c8f4e4f5451109b9ddeeeb85e57d638dfb7b34112c57e3099a75ed8"

RUN apk --no-cache add curl && \
    curl -L "$HELM_URL" | tar xz -C /usr/local/bin --exclude '*/*[^helm]' --strip-components=1 && \
    curl -L "$KUBECTL_URL" -o /usr/local/bin/kubectl && chmod +x /usr/local/bin/kubectl && \
    curl -L "$KUSTOMIZE_URL" | tar xz -C /usr/local/bin && \
    curl -L "$KONJURE_URL" | tar xz -C /usr/local/bin && \
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
    mkdir -p /workspace/base && \
    chown -R setup /workspace

USER setup:setup
RUN konjure kustomize init

WORKDIR "/workspace/base"
ENTRYPOINT ["/workspace/docker-entrypoint.sh"]
CMD ["install"]
