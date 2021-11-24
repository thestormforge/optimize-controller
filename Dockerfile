# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY ./manager .

# Numeric identifers for nobody:nobody (allows Kube to infer "run as")
USER 65534:65534

ENTRYPOINT ["/manager"]
