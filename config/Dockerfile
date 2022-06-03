FROM alpine:latest

ENV HELM_VERSION="v3.4.1" \
    HELM_SHA256="538f85b4b73ac6160b30fd0ab4b510441aa3fa326593466e8bf7084a9c288420"

ENV KUBECTL_VERSION="v1.17.17" \
    KUBECTL_SHA256="8329fac94c66bf7a475b630972a8c0b036bab1f28a5584115e8dd26483de8349"

ENV KUSTOMIZE_VERSION="v3.8.7" \
    KUSTOMIZE_SHA256="4a3372d7bfdffe2eaf729e77f88bc94ce37dc84de55616bfe90aac089bf6fd02"

ENV KONJURE_VERSION="v0.2.1" \
    KONJURE_SHA256="8bf2a82b389076d80a9bd5f379c330e5d74353ef8fac95f851dd26c26349b61c"

ENV HELM_URL="https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz" \
    KUBECTL_URL="https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    KUSTOMIZE_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" \
    KONJURE_URL="https://github.com/carbonrelay/konjure/releases/download/${KONJURE_VERSION}/konjure-linux-amd64.tar.gz"

ENV KUSTOMIZE_PLUGIN_HOME="/home/setup/.kustomize"

RUN apk --no-cache add curl && \
    curl -L "$HELM_URL" | tar xoz -C /usr/local/bin --exclude '*/*[^helm]' --strip-components=1 && \
    curl -L "$KUBECTL_URL" -o /usr/local/bin/kubectl && chmod +x /usr/local/bin/kubectl && \
    curl -L "$KUSTOMIZE_URL" | tar xoz -C /usr/local/bin && \
    curl -L "$KONJURE_URL" | tar xoz -C /usr/local/bin && \
    addgroup -g 1000 -S setup && \
    adduser -u 1000 -S setup -G setup -h /home/setup

COPY --chown=setup:setup . /home/setup
WORKDIR "/home/setup/base"
RUN chown setup:setup . && chmod o+w .
USER 1000:1000
RUN konjure kustomize init && chmod -R go+rX "${KUSTOMIZE_PLUGIN_HOME}"

ENTRYPOINT ["/home/setup/docker-entrypoint.sh"]
CMD ["build"]
