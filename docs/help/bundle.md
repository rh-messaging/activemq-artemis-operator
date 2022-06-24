---
title: "Bundle"
description: "Bundle ArtemisCloud.io"
lead: "Bundle ArtemisCloud.io"
date: 2020-10-06T08:49:31+00:00
lastmod: 2020-10-06T08:49:31+00:00
draft: false
images: []
menu:
  docs:
    parent: "help"
weight: 630
toc: true
---

# Bunding A Bundle and Deploy it into the Operator Lifecycle Manager(OLM)

## About the Operator Lifecycle Manager (OLM)

The [Operator Lifecycle Manager](https://olm.operatorframework.io/) can help users to install and manage operators.
The ArtemisCloud operator can be built into a bundle image and installed into OLM.

## Building

### Creating the bundle's manifests/metadata

Before you build the bundle image generate the manifests and metadata:

```$xslt
make IMAGE_TAG_BASE=<bundle image registry> OPERATOR_IMAGE_REPO=<operator image registry> OPERATOR_VERSION=<operator tag> bundle
```
You'll get some warnings like

```$xslt
WARN[0001] ClusterServiceVersion validation: [OperationFailed] provided API should have an example annotation 
```
which can be ignored. It is because the samples in the CSV only have current version.

### Building the bundle image:

```$xslt
make IMAGE_TAG_BASE=<bundle image registry> bundle-build
```
The result image tag takes the form like
```$xslt
${IMAGE_TAG_BASE}-bundle:v0.0.1
```
Note: the version v0.0.1 is defined by VERSION variable in the [Makefile](../Makefile)

To push the built bundle image

```$xslt
make IMAGE_TAG_BASE=<bundle image registry> bundle-push
```

### Building the catalog image

Now with the bundle image in place, build the catalog(index) iamge:

```$xslt
make IMAGE_TAG_BASE=<bundle image registry> catalog-build
```
The result image tag takes the form like
```$xslt
${IMAGE_TAG_BASE}-index:v0.0.1
```

To push the catalog image to repo:

```$xslt
make IMAGE_TAG_BASE=<bundle image registry> catalog-push
```

## Installing operator via OLM (Minikube)

### Install olm (If olm is not installed already)

Make sure the Minikube is up and running.

Use the [operator-sdk tool](https://sdk.operatorframework.io/):

```$xslt
operator-sdk olm install
```
It will deploy the latest olm into Minikube.

### Create a catalog source (e.g. catalog-source.yaml):

```
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: artemis-index
  namespace: operators
spec:
  sourceType: grpc
  image: quay.io/hgao/operator-catalog:v0.0.1
  displayName: ArtemisCloud Index
  publisher: ArtemisCloud
  updateStrategy:
    registryPoll:
      interval: 10m
```

and deploy it:

```$xslt
$ kubectl create -f catalog-source.yaml
```
In a moment you will see the index image is up and running in namespace **operators**:

```$xslt
[a]$ kubectl get pod -n operators
NAME                  READY   STATUS    RESTARTS   AGE
artemis-index-bzh75   1/1     Running   0          42s
```

### Creating a subscription (e.g. subscription.yaml)

```
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: my-subscription
  namespace: operators
spec:
  channel: upstream
  name: activemq-artemis-operator
  source: artemis-index
  sourceNamespace: operators
  installPlanApproval: Automatic
```

and deploy it:
```$xslt
  $ kubectl create -f subscription.yaml
```
An operator will be installed into **operators** namespace.

```$xslt
$ kubectl get pod
NAME                                                              READY   STATUS      RESTARTS   AGE
9365c56f188be1738a1fabddb5a408a693d8c1f2d7275514556644e52ejpdpj   0/1     Completed   0          2m20s
activemq-artemis-controller-manager-84d58db649-tkt89              1/1     Running     0          117s
artemis-index-frpn4                                               1/1     Running     0          3m35s
```
