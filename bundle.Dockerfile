FROM scratch

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=amq-broker-rhel8
LABEL operators.operatorframework.io.bundle.channels.v1=7.10.x
LABEL operators.operatorframework.io.bundle.channel.default.v1=7.10.x

COPY manifests /manifests/
COPY metadata/annotations.yaml /metadata/annotations.yaml

LABEL com.redhat.component="amq-broker-rhel8-operator-bundle-container"
LABEL com.redhat.delivery.operator.bundle="true"
LABEL com.redhat.delivery.backport=false
LABEL com.redhat.openshift.versions="v4.6-v4.11"
LABEL description="Red Hat AMQ Broker 7.10 Operator Bundle"
LABEL io.k8s.description="An associated operator bundle of metadata."
LABEL io.k8s.display-name="Red Hat AMQ Broker 7.10 Operator Bundle"
LABEL io.openshift.tags="messaging,amq,integration,operator,golang"
LABEL maintainer="Roddie Kieley <rkieley@redhat.com>"
LABEL name="amq7/amq-broker-rhel8-operator-bundle"
LABEL summary="Red Hat AMQ Broker 7.10 Operator Bundle"
LABEL version="7.10"
