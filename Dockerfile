# Build the manager binary
FROM registry-proxy.engineering.redhat.com/rh-osbs/rhel9-go-toolset:1.21.11 AS builder

ARG TARGETOS
ARG TARGETARCH

ENV GO_MODULE=github.com/arkmq-org/activemq-artemis-operator

### BEGIN REMOTE SOURCE
# Use the COPY instruction only inside the REMOTE SOURCE block
# Use the COPY instruction only to copy files to the container path $REMOTE_SOURCE_DIR/app
ARG REMOTE_SOURCE_DIR=/tmp/remote_source
RUN mkdir -p $REMOTE_SOURCE_DIR/app
WORKDIR $REMOTE_SOURCE_DIR/app
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY entrypoint/ entrypoint/
COPY pkg/ pkg/
COPY version/ version/
### END REMOTE SOURCE

# Set up the workdir
WORKDIR /opt/app-root/src
RUN cp -r $REMOTE_SOURCE_DIR/app/* .

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
# CGO_ENABLED is set to 1 for dynamic linking to OpenSSL to use FIPS validated cryptographic modules
# when is executed on nodes that are booted into FIPS mode.
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -ldflags="-X '${GO_MODULE}/version.BuildTimestamp=`date '+%Y-%m-%dT%H:%M:%S'`'" -o manager main.go

# This OSBS Base Image is designed and engineered to be the base layer for
# Red Hat products. This base image is only supported for approved Red Hat
# products. This image is maintained by Red Hat and updated regularly.
FROM registry.redhat.io/rhel9-osbs/osbs-ubi9-minimal:9.5-1736404155 AS base-env

ENV BROKER_NAME=amq-broker
ENV USER_UID=1000
ENV USER_NAME=${BROKER_NAME}-operator
ENV USER_HOME=/home/${USER_NAME}
ENV OPERATOR=${USER_HOME}/bin/${BROKER_NAME}-operator

WORKDIR /

# Create operator bin
RUN mkdir -p ${USER_HOME}/bin

# Copy the manager binary
COPY --from=builder /opt/app-root/src/manager ${OPERATOR}

# Copy the entrypoint script
COPY --from=builder /opt/app-root/src/entrypoint/entrypoint ${USER_HOME}/bin/entrypoint

# Set operator bin owner and permissions
RUN chown -R `id -u`:0 ${USER_HOME}/bin && chmod -R 755 ${USER_HOME}/bin

# Upgrade packages
RUN microdnf update -y --setopt=install_weak_deps=0 && rm -rf /var/cache/yum

USER ${USER_UID}
ENTRYPOINT ["${USER_HOME}/bin/entrypoint"]

LABEL name="amq0/amq-broker-rhel9-operator"
LABEL description="ActiveMQ Artemis Broker Operator"
LABEL maintainer="Roddie Kieley <rkieley@redhat.com>"
LABEL version="0.0.0"
LABEL summary="Red Hat AMQ Broker 0.0 Operator"
LABEL amq.broker.version="0.0.0.OPR.1.CR1"
LABEL com.redhat.component="amq-broker-rhel9-operator-container"
LABEL io.k8s.display-name="Red Hat AMQ Broker 0.0 Operator"
LABEL io.openshift.tags="messaging,amq,integration,operator,golang"
