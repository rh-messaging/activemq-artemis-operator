

## Building the operator

In the activemq-artemis-operator directory issue the following command: 

```bash
make
```

## Upload to a container registry

e.g.

```bash
docker push quay.io/<repo>/activemq-artemis-operator:<version>
```

## Deploy to OpenShift using OLM

As cluster-admin and an OCP 3.11+ cluster with OLM installed, issue the following command:

```bash
# If using the default OLM namespace "operator-lifecycle-manager"
./hack/catalog-redhat.sh

# If using a different namespace for OLM
./hack/catalog-redhat.sh <namespace>

configmap/activemq-resources created
catalogsource.operators.coreos.com/activemq-resources created


```

This will create a new `CatalogSource` and `ConfigMap`, allowing the OLM Catalog to see this Operator's `ClusterServiceVersion`.

### Trigger a Broker AMQ Artemis deployment

Use the OLM console to subscribe to the `Broker AMQ Artemis` Operator Catalog Source within your namespace. Once subscribed, use the console to `Create Broker` or create one manually as seen below.

```bash
$ oc create -f deploy/crds/broker_v1alpha1_activemqartemis_cr.yaml
```

### Clean up a Broker AMQ Artemis deployment

```bash
oc delete -f deploy/crds/broker_v1alpha1_activemqartemis_cr.yaml
```

