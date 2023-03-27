FROM scratch

# Core bundle labels.
LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=amq-broker-rhel8
LABEL operators.operatorframework.io.bundle.channels.v1=7.11.x
LABEL operators.operatorframework.io.bundle.channel.default.v1=7.11.x
LABEL operators.operatorframework.io.metrics.builder=operator-sdk-v1.14.0+git
LABEL operators.operatorframework.io.metrics.mediatype.v1=metrics+v1
LABEL operators.operatorframework.io.metrics.project_layout=go.kubebuilder.io/v3

# Labels for testing.
LABEL operators.operatorframework.io.test.mediatype.v1=scorecard+v1
LABEL operators.operatorframework.io.test.config.v1=tests/scorecard/

# Copy files to locations specified by labels.
COPY bundle/manifests /manifests/
COPY bundle/metadata /metadata/
COPY bundle/tests/scorecard /tests/scorecard/

LABEL name="amq7/amq-broker-rhel8-operator-bundle"
LABEL description="Red Hat AMQ Broker 7.11 Operator Bundle"
LABEL maintainer="Roddie Kieley <rkieley@redhat.com>"
LABEL version="7.11.0"
LABEL summary="Red Hat AMQ Broker 7.11 Operator Bundle"
LABEL amq.broker.version="7.11.0.OPR.1.CR3"
LABEL com.redhat.component="amq-broker-rhel8-operator-bundle-container"
LABEL com.redhat.delivery.backport=false
LABEL com.redhat.delivery.operator.bundle=true
LABEL com.redhat.openshift.versions="v4.9"
LABEL io.k8s.display-name="Red Hat AMQ Broker 7.11 Operator Bundle"
LABEL io.openshift.tags="messaging,amq,integration,operator,golang"
