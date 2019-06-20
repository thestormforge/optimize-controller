FROM alpine:latest

ENV HELM_VERSION="v2.14.1" \
    HELM_URL="https://get.helm.sh/helm-v2.14.1-linux-amd64.tar.gz" \
    HELM_SHA256="804f745e6884435ef1343f4de8940f9db64f935cd9a55ad3d9153d064b7f5896"

ENV KUBECTL_VERSION="v1.14.3" \
    KUBECTL_URL="https://storage.googleapis.com/kubernetes-release/release/v1.14.3/bin/linux/amd64/kubectl" \
    KUBECTL_SHA256="ebc8c2fadede148c2db1b974f0f7f93f39f19c8278619893fd530e20e9bec98f"

RUN apk --no-cache add curl && \
    curl -L "$HELM_URL" | tar xz --exclude '*/*[^helm]' --strip-components=1 -C /usr/local/bin/ && \
    curl -L "$KUBECTL_URL" -o /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl

COPY hack/setuptools/docker-entrypoint.sh /
ENTRYPOINT ["/docker-entrypoint.sh"]