---
title: "Building"
description: "Building ArtemisCloud.io"
lead: "Building ArtemisCloud.io"
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

# Building the operator

## General environment requirements

### A kubernetes cluster

Currently the operator is tested against kubernetes v1.20.
You can install a [Minikube](https://minikube.sigs.k8s.io/docs/) or a [CodeReady Containers(CRC)](https://developers.redhat.com/products/codeready-containers/overview) to deploy the operator.

### Docker

Current version being used is v20.10. Checkout [this page](https://docs.docker.com/get-docker/) for help on installing docker on your specific operating system.

### Go v1.16

Install Go version v1.16 following [this guide](https://go.dev/doc/install).

### operator-sdk v1.15.0

Install [operator-sdk](https://sdk.operatorframework.io/) following [this guide](https://sdk.operatorframework.io/docs/installation/).

## Get the code

```$xslt
git clone https://github.com/artemiscloud/activemq-artemis-operator
cd activemq-artemis-operator
git checkout main
```

## Building the code locally

```$xslt
make
```
or
```$xslt
make build
```

## Building the operator image

There are 2 variables you may need to override in order to push the images to your preferred registry.

```$xslt
OPERATOR_IMAGE_REPO (your preferred image registry name, for example quay.io/hgao/operator
```
and
```$xslt
OPERATOR_VERSION (the image's tag, for example v1.1)
```

Now build the image passing the variables

```$xslt
make OPERATOR_IMAGE_REPO=<your repo> OPERATOR_VERSION=<tag> docker-build
```

If finished sucessfully it will print the image url in the end. The image url is like

```$xslt
${OPERATOR_IMAGE_REPO}:${TAG}
```

## Push the image to registry

```$xslt
docker push ${OPERATOR_IMAGE_REPO}:${TAG}
```
or use the make target **docker-push**
```$xslt
make OPERATOR_IMAGE_REPO=<your repo> OPERATOR_VERSION=<tag> docker-push
```

Now follow the [quickstart](quickstart.md) to deploy the operator.
