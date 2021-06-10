# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
ARG BASE_IMAGE=gcr.io/distroless/static:nonroot
FROM ${BASE_IMAGE} AS base
WORKDIR /
COPY ./manager /usr/local/bin/
COPY ./LICENSE /licenses/
USER nobody:nobody

ENTRYPOINT ["manager"]
