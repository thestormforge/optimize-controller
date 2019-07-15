# Build the manager binary
FROM golang:1.12.5 as builder

# Get arguments
ARG LDFLAGS=""

# Copy in the go src
WORKDIR /go/src/github.com/gramLabs/k8s-experiment
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -a -o manager github.com/gramLabs/k8s-experiment/cmd/manager

# Copy the controller-manager into a thin image
FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=builder /go/src/github.com/gramLabs/k8s-experiment/manager .
ENTRYPOINT ["/manager"]
