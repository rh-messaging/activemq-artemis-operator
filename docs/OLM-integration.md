

## Building the operator

In the activemq-artemis-operator directory issue the following command: 

```bash
make
```

## Upload to a container registry

e.g.

```bash
docker push localhost.localdomain:5000/<repo>/activemq-artemis-operator:<version>
```

## Deploy to OpenShift 4 using OLM

To install this operator on OpenShift 4 for end-to-end testing, make sure you have access to a quay.io account to create an application repository. Follow the [authentication](https://github.com/operator-framework/operator-courier/#authentication) instructions for Operator Courier to obtain an account token. This token is in the form of "basic XXXXXXXXX" and both words are required for the command.

Push the operator bundle to your quay application repository as follows:

```bash
operator-courier push deploy/catalog_resources/courier/bundle_dir/0.4.0 <quay.io account> <application repo name> <version> "basic YWhhbWVlZDpIYW1lZWRAMTIz" "basic XXXXXXXXX"
```

If pushing to another quay repository, replace with your username or other repot name. 

for example : 

```bash
operator-courier push deploy/catalog_resources/courier/bundle_dir/0.4.0 ahameed amqoperator 0.4.0 "basic YWhhbWVlZDpIYW1lZWRAMTIz"
```


Also note that the push command does not overwrite an existing repository, and it needs to be deleted before a new version can be built and uploaded. Once the bundle has been uploaded, create an [Operator Source](https://github.com/operator-framework/community-operators/blob/master/docs/testing-operators.md#linking-the-quay-application-repository-to-your-openshift-40-cluster) to load your operator bundle in OpenShift.

```bash
oc create -f deploy/catalog_resources/courier/activemq-artemis-operatorsource.yaml 
```

Remember to replace _registryNamespace_ with your quay namespace. The name, display name and publisher of the operator are the only other attributes that may be modified.

It will take a few minutes for the operator to become visible under the _OperatorHub_ section of the OpenShift console _Catalog_. It can be easily found by filtering the provider type to _Custom_.



## Deploy to OpenShift 3.11+ using OLM

As cluster-admin and an OCP 3.11+ cluster with OLM installed, issue the following command:

```bash
# If using the default OLM namespace "operator-lifecycle-manager"
./scripts/catalog-redhat.sh

# If using a different namespace for OLM
./scripts/catalog-redhat.sh <namespace>

configmap/activemq-artemis-resources created
catalogsource.operators.coreos.com/activemq-artemis-resources created


```

This will create a new `CatalogSource` and `ConfigMap`, allowing the OLM Catalog to see this Operator's `ClusterServiceVersion`.

### Trigger a ActiveMQ Artemis deployment

Use the OLM console to subscribe to the `ActiveMQ Artemis` Operator Catalog Source within your namespace. Once subscribed, use the console to `Create Broker` or create one manually as seen below.

```bash
$ oc create -f deploy/crds/broker_v1alpha1_activemqartemis_cr.yaml
```

### Clean up a ActiveMQ Artemis deployment

```bash
oc delete -f deploy/crds/broker_v1alpha1_activemqartemis_cr.yaml
```

