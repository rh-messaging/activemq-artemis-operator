# Build the manager binary
FROM openshift/golang-builder:1.17 as builder

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

# Set up the workspace
RUN mkdir -p /workspace
RUN mv $REMOTE_SOURCE_DIR/app /workspace
WORKDIR /workspace/app

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-X '${GO_MODULE}/version.BuildTimestamp=`date '+%Y-%m-%dT%H:%M:%S'`'" -o /workspace/manager main.go

FROM registry.access.redhat.com/ubi8:8.6-855 as base-env

ENV BROKER_NAME=amq-broker
ENV USER_UID=1000
ENV USER_NAME=${BROKER_NAME}-operator
ENV USER_HOME=/home/${USER_NAME}
ENV OPERATOR=${USER_HOME}/bin/${BROKER_NAME}-operator

WORKDIR /

# Create operator user
RUN useradd --uid ${USER_UID} --home-dir ${USER_HOME} --shell /sbin/nologin ${USER_NAME}

# Copy the manager binary
RUN mkdir -p ${USER_HOME}/bin
COPY --from=builder /workspace/manager ${OPERATOR}

# Copy the entrypoint script
COPY --from=builder /workspace/app/entrypoint/entrypoint ${USER_HOME}/bin/entrypoint

# Upgrade packages
RUN dnf update -y --setopt=install_weak_deps=0 && rm -rf /var/cache/yum

USER ${USER_UID}
ENTRYPOINT ["${USER_HOME}/bin/entrypoint"]

LABEL name="amq7/amq-broker-rhel8-operator"
LABEL description="Red Hat AMQ Broker 7.11 Operator"
LABEL maintainer="Roddie Kieley <rkieley@redhat.com>"
LABEL version="7.11"
LABEL summary="Red Hat AMQ Broker 7.11 Operator"
LABEL amq.broker.version="7.11.0.OPR.1.CR1"
LABEL com.redhat.component="amq-broker-rhel8-operator-container"
LABEL io.k8s.display-name="Red Hat AMQ Broker 7.11 Operator"
LABEL io.openshift.tags="messaging,amq,integration,operator,golang"
