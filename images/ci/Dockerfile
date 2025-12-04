# Build the external-secrets-operator binary
FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.24-openshift-4.20 AS builder

ARG SRC_DIR=/go/src/github.com/openshift/external-secrets-operator
ENV GO_BUILD_TAGS=strictfipsruntime,openssl
ENV GOEXPERIMENT=strictfipsruntime
ENV CGO_ENABLED=1
ENV GOFLAGS=""

WORKDIR $SRC_DIR

# Copy the source
COPY . .

RUN go build -tags $GO_BUILD_TAGS -o external-secrets-operator cmd/external-secrets-operator/main.go

FROM registry.ci.openshift.org/rhcos-devel/ocp-4.21-10.1:4.21.0-ec.3-x86_64
ARG SRC_DIR=/go/src/github.com/openshift/external-secrets-operator
COPY --from=builder $SRC_DIR/external-secrets-operator /bin/external-secrets-operator
USER 65534:65534

ENTRYPOINT ["/bin/external-secrets-operator"]
