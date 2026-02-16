FROM scratch

# Core bundle labels.
LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=amq-broker-rhel9
LABEL operators.operatorframework.io.bundle.channels.v1=8.0.x
LABEL operators.operatorframework.io.bundle.channel.default.v1=8.0.x
LABEL operators.operatorframework.io.metrics.builder=operator-sdk-v1.28.0
LABEL operators.operatorframework.io.metrics.mediatype.v1=metrics+v1
LABEL operators.operatorframework.io.metrics.project_layout=go.kubebuilder.io/v3

# Labels for testing.
LABEL operators.operatorframework.io.test.mediatype.v1=scorecard+v1
LABEL operators.operatorframework.io.test.config.v1=tests/scorecard/

# Copy files to locations specified by labels.
COPY bundle/manifests /manifests/
COPY bundle/metadata /metadata/
COPY bundle/tests/scorecard /tests/scorecard/

LABEL name="amq8/amq-broker-rhel9-operator-bundle"
LABEL description="Red Hat AMQ Broker 8.0 Operator Bundle"
LABEL maintainer="Red Hat, Inc."
LABEL version="8.0.0"
LABEL summary="Red Hat AMQ Broker 8.0 Operator Bundle"
LABEL amq.broker.version="8.0.0.OPR.1.SR1"
LABEL com.redhat.component="amq-broker-rhel9-operator-bundle-container"
LABEL com.redhat.delivery.backport=false
LABEL com.redhat.delivery.operator.bundle=true
LABEL com.redhat.openshift.versions="v4.12"
LABEL io.k8s.display-name="Red Hat AMQ Broker 8.0 Operator Bundle"
LABEL io.openshift.tags="messaging,amq,integration,operator,golang"
