FROM alpine:latest

ARG IMG

ENV HELM_VERSION="v2.14.2" \
    HELM_URL="https://get.helm.sh/helm-v2.14.2-linux-amd64.tar.gz" \
    HELM_SHA256="9f50e69cf5cfa7268b28686728ad0227507a169e52bf59c99ada872ddd9679f0"

ENV KUBECTL_VERSION="v1.14.4" \
    KUBECTL_URL="https://storage.googleapis.com/kubernetes-release/release/v1.14.4/bin/linux/amd64/kubectl" \
    KUBECTL_SHA256="409bdecd3b8025c8d35b6c188d2015a6328a1a8e86f244bcd6703a65fe7dfb5e"

ENV KUSTOMIZE_VERSION="v3.1.0" \
    KUSTOMIZE_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/v3.1.0/kustomize_3.1.0_linux_amd64" \
    KUSTOMIZE_SHA256="73acc575cf4e035a91da63ecffcabe58f9572562b772c1eb7ed863991950afe8"

RUN apk --no-cache add curl && \
    curl -L "$HELM_URL" | tar xz --exclude '*/*[^helm]' --strip-components=1 -C /usr/local/bin/ && \
    curl -L "$KUBECTL_URL" -o /usr/local/bin/kubectl && \
    curl -L "$KUSTOMIZE_URL" -o /usr/local/bin/kustomize && \
    chmod +x /usr/local/bin/kubectl /usr/local/bin/kustomize

COPY . /workspace/

RUN cd /workspace/manager && kustomize edit set image controller=${IMG} && \
    if [ -n "${IMG##*/*}" ] ; then cd /workspace/default; kustomize edit add patch manager_image_pull_policy.yaml; fi && \
    mkdir -p /workspace/base && touch /workspace/base/kustomization.yaml

WORKDIR "/workspace/base"
ENTRYPOINT ["/redskyops/docker-entrypoint.sh"]
CMD ["install", "--dry-run"]
