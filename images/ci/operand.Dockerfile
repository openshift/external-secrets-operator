FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.24-openshift-4.20 AS builder

ARG RELEASE_BRANCH=v0.19.0
ARG GO_BUILD_TAGS=strictfipsruntime,openssl
ARG SRC_DIR=/go/src/github.com/openshift/external-secrets

ENV GOEXPERIMENT=strictfipsruntime
ENV CGO_ENABLED=1

WORKDIR $SRC_DIR
RUN git clone --depth 1 --branch $RELEASE_BRANCH https://github.com/openshift/external-secrets.git $SRC_DIR
RUN go mod vendor
RUN go build -mod=vendor -tags $GO_BUILD_TAGS -o _output/external-secrets main.go

FROM registry.ci.openshift.org/rhcos-devel/ocp-4.21-10.1:4.21.0-ec.3-x86_64
ARG SRC_DIR=/go/src/github.com/openshift/external-secrets
COPY --from=builder $SRC_DIR/_output/external-secrets /bin/external-secrets

USER 65534:65534

ENTRYPOINT ["/bin/external-secrets"]
