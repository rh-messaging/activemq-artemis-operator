#!/bin/sh

if [[ -z ${1} ]]; then
    CATALOG_NS="operator-lifecycle-manager"
else
    CATALOG_NS=${1}
fi

CSV=`cat deploy/catalog_resources/redhat/activemq-artemis-operator.v1.0.0.clusterserviceversion.yaml | sed -e 's/^/          /' | sed '0,/ /{s/          /        - /}'`
CRD=`cat deploy/crds/broker_v1alpha1_activemqartemis_crd.yaml | sed -e 's/^/          /' | sed '0,/ /{s/          /        - /}'`
CRDActivemqartemisaddress=`cat deploy/crds/broker_v1alpha1_activemqartemisaddress_crd.yaml | sed -e 's/^/          /' | sed '0,/ /{s/          /        - /}'`
PKG=`cat deploy/catalog_resources/redhat/activemq-artemis.package.yaml | sed -e 's/^/          /' | sed '0,/ /{s/          /        - /}'`

cat << EOF > deploy/catalog_resources/redhat/catalog-source.yaml
apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: ConfigMap
    metadata:
      name: activemq-resources
      namespace: ${CATALOG_NS}
    data:
      clusterServiceVersions: |
${CSV}
      customResourceDefinitions: |
${CRD}
${CRDActivemqartemisaddress}
      packages: >
${PKG}

  - apiVersion: operators.coreos.com/v1alpha1
    kind: CatalogSource
    metadata:
      name: activemq-resources
      namespace: ${CATALOG_NS}
    spec:
      configMap: activemq-resources
      displayName: ActiveMQ Artemis Operator
      publisher: Red Hat
      sourceType: internal
    status:
      configMapReference:
        name: activemq-resources
        namespace: ${CATALOG_NS}
EOF
