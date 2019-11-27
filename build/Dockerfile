FROM registry.access.redhat.com/ubi7/go-toolset:1.11.13-10 AS build-env

ENV GOPATH=/tmp/go 
ENV GOOS=linux
ENV CGO_ENABLED=0
ENV BROKER_NAME=activemq-artemis

RUN mkdir -p /tmp/go/src/github.com/rh-messaging/activemq-artemis-operator/bin
RUN mkdir -p /tmp/activemq-artemis-operator

ADD . ${GOPATH}/src/github.com/rh-messaging/activemq-artemis-operator
WORKDIR ${GOPATH}/src/github.com/rh-messaging/activemq-artemis-operator

RUN /opt/rh/go-toolset-1.11/root/usr/bin/go build -o /tmp/activemq-artemis-operator/${BROKER_NAME}-operator /tmp/go/src/github.com/rh-messaging/activemq-artemis-operator/cmd/manager


FROM registry.access.redhat.com/ubi8:8.1-277 AS base-env

ENV BROKER_NAME=activemq-artemis
ENV OPERATOR=/home/${BROKER_NAME}-operator/bin/${BROKER_NAME}-operator
ENV USER_UID=1000
ENV USER_NAME=activemq-artemis-operator
ENV CGO_ENABLED=0
ENV GOPATH=/tmp/go
ENV JBOSS_IMAGE_NAME="amq7/amq-broker-rhel8-operator"
ENV JBOSS_IMAGE_VERSION="0.10"

WORKDIR /

RUN mkdir -p /manifests/0.9.1
RUN mkdir -p /manifests/0.10.0
RUN chown -R `id -u`:0 /manifests
COPY --from=build-env /tmp/activemq-artemis-operator /home/${BROKER_NAME}-operator/bin
COPY --from=build-env /tmp/go/src/github.com/rh-messaging/activemq-artemis-operator/build/bin/entrypoint /home/${BROKER_NAME}-operator/bin

COPY --from=build-env /tmp/go/src/github.com/rh-messaging/activemq-artemis-operator/deploy/olm-catalog/activemq-artemis-operator/${BROKER_NAME}.package.yaml /manifests
COPY --from=build-env /tmp/go/src/github.com/rh-messaging/activemq-artemis-operator/deploy/olm-catalog/activemq-artemis-operator/0.9.1/ /manifests/0.9.1
COPY --from=build-env /tmp/go/src/github.com/rh-messaging/activemq-artemis-operator/deploy/olm-catalog/activemq-artemis-operator/0.10.0/ /manifests/0.10.0
COPY --from=build-env /tmp/go/src/github.com/rh-messaging/activemq-artemis-operator/deploy/crds/* /manifests/0.10.0

RUN useradd activemq-artemis-operator
RUN mkdir -p /home/${BROKER_NAME}-operator/bin && chown -R `id -u`:0 /home/${BROKER_NAME}-operator/bin && chmod -R 755 /home/${BROKER_NAME}-operator/bin

USER ${USER_UID}
ENTRYPOINT ["/home/${BROKER_NAME}-operator/bin/entrypoint"]

LABEL \
      com.redhat.component="amq-broker-rhel8-operator-container"  \
      com.redhat.delivery.appregistry="true" \
      description="Red Hat AMQ Broker 7.5 Operator"  \
      io.k8s.description="An associated operator that handles broker installation, updates and scaling."  \
      io.k8s.display-name="Red Hat AMQ Broker 7.5 Operator"  \
      io.openshift.expose-services=""  \
      io.openshift.s2i.scripts-url="image:///usr/local/s2i"  \
      io.openshift.tags="messaging,amq,integration,operator,golang"  \
      maintainer="Roddie Kieley <rkieley@redhat.com>"  \
      name="amq7/amq-broker-rhel8-operator" \
      summary="Red Hat AMQ Broker 7.5 Operator"  \
      version="0.10"
