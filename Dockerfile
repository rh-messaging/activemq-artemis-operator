# Build the manager binary
FROM registry-proxy.engineering.redhat.com/rh-osbs/rhel8-go-toolset:1.21.11 as builder

ARG TARGETOS
ARG TARGETARCH

ENV GO_MODULE=github.com/artemiscloud/activemq-artemis-operator

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
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -ldflags="-X '${GO_MODULE}/version.BuildTimestamp=`date '+%Y-%m-%dT%H:%M:%S'`'" -o manager main.go

FROM registry-proxy.engineering.redhat.com/rh-osbs/ubi8-minimal@sha256:c12e67af6a7e15113d76bc72f10bef2045c026c71ec8b7124c8a075458188a83 as base-env

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

LABEL name="amq8/amq-broker-rhel8-operator"
LABEL description="Red Hat AMQ Broker 8.0 Operator"
LABEL maintainer="Roddie Kieley <rkieley@redhat.com>"
LABEL version="8.0.0"
LABEL summary="Red Hat AMQ Broker 8.0 Operator"
LABEL amq.broker.version="8.0.0.OPR.1.SR1"
LABEL com.redhat.component="amq-broker-rhel8-operator-container"
LABEL io.k8s.display-name="Red Hat AMQ Broker 8.0 Operator"
LABEL io.openshift.tags="messaging,amq,integration,operator,golang"
