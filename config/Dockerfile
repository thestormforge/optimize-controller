FROM alpine:latest

ENV HELM_VERSION="v3.2.1" \
    HELM_SHA256="018f9908cb950701a5d59e757653a790c66d8eda288625dbb185354ca6f41f6b"

ENV KUBECTL_VERSION="v1.14.10" \
    KUBECTL_SHA256="7729c6612bec76badc7926a79b26e0d9b06cc312af46dbb80ea7416d1fce0b36"

ENV KUSTOMIZE_VERSION="v3.5.5" \
    KUSTOMIZE_SHA256="23306e0c0fb24f5a9fea4c3b794bef39211c580e4cbaee9e21b9891cb52e73e7"

ENV KONJURE_VERSION="v0.2.1" \
    KONJURE_SHA256="8bf2a82b389076d80a9bd5f379c330e5d74353ef8fac95f851dd26c26349b61c"

ENV HELM_URL="https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz" \
    KUBECTL_URL="https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    KUSTOMIZE_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" \
    KONJURE_URL="https://github.com/carbonrelay/konjure/releases/download/${KONJURE_VERSION}/konjure-linux-amd64.tar.gz"

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
    sed -i "s|PULL_POLICY|${PULL_POLICY}|g" /workspace/chart/redskyops/values.yaml && \
    sed -i "s|IMG|${IMG%:*}|g" /workspace/chart/redskyops/values.yaml /workspace/chart/redskyops/questions.yml && \
    sed -i "s|TAG|${IMG##*:}|g" /workspace/chart/redskyops/values.yaml /workspace/chart/redskyops/questions.yml && \
    echo "appVersion: ${VERSION}" >> /workspace/chart/redskyops/Chart.yaml && \
    mkdir -p /workspace/base && \
    chown -R setup /workspace

USER setup:setup
RUN konjure kustomize init

# Add Helm stable until May 13, 2020
RUN helm repo add stable https://kubernetes-charts.storage.googleapis.com

WORKDIR "/workspace/base"
ENTRYPOINT ["/workspace/docker-entrypoint.sh"]
CMD ["install"]
