# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
ARG BASE_IMAGE=gcr.io/distroless/static:nonroot
FROM ${BASE_IMAGE} as base

# These labels are required for the partner guide
# https://redhat-connect.gitbook.io/partner-guide-for-red-hat-openshift-and-container/program-on-boarding/technical-prerequisites#dockerfile-requirements
LABEL name="Optimize" \
      maintainer="support@stormforge.io" \
      vendor="StormForge" \
      version="latest" \
      release="1" \
      summary="StormForge Optimize uses machine learning to improve the performance of your application" \
      description="StormForge Optimize uses machine learning to improve the performance of your application."

WORKDIR /
COPY ./manager .

USER nobody:nobody

ENTRYPOINT ["/manager"]
