FROM alpine:latest

ENV KUBECTL_VERSION="v1.14.10" \
    KUBECTL_URL="https://storage.googleapis.com/kubernetes-release/release/v1.14.10/bin/linux/amd64/kubectl" \
    KUBECTL_SHA256="7729c6612bec76badc7926a79b26e0d9b06cc312af46dbb80ea7416d1fce0b36"

RUN apk add --no-cache ca-certificates && \
    apk add --no-cache -t .build-deps curl && \
    curl -L "$KUBECTL_URL" -o /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl && \
    apk del .build-deps

COPY /stormforge /usr/local/bin/

ENTRYPOINT ["stormforge"]
CMD ["--help"]
