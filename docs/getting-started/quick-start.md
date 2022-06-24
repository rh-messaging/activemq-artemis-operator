---
title: "Quick Start"
description: "One page summary of how to start a new ArtemisCloud project."
lead: "One page summary of how to start a new ArtemisCloud project."
date: 2020-11-16T13:59:39+01:00
lastmod: 2020-11-16T13:59:39+01:00
draft: false
images: []
menu:
  docs:
    parent: "getting-started"
weight: 110
toc: true
---

## Overview

At the moment these instructions have been tested against Kubernetes 1.20,
other kubernetes or OpenShift environments may require minor adjustment.

One important note about operators in general is that to get the operator
installed requires cluster-admin level privileges. Once installed, a regular
user should be able to install ActiveMQ Artemis via the provided custom
resource.

## General environment requirements

Currently the operator is tested against kubernetes v1.20.
You can install a [Minikube](https://minikube.sigs.k8s.io/docs/) or a [CodeReady Containers(CRC)](https://developers.redhat.com/products/codeready-containers/overview) to deploy the operator.

## Getting the code and build the image

To launch the operator you will need to clone the [activemq-artemis-operator](https://github.com/artemiscloud/activemq-artemis-operator) and checkout the main branch.

Follow the [building](building.md) instructions, tag, and push it into your project
namespace.

## Deploying the operator

To deploy the operator simply run

```$xslt
$ make deploy
```
or if you have built your own image, overrides the defaults

```$xslt
make OPERATOR_IMAGE_REPO=<your registry url> OPERATOR_VERSION=<your tag> deploy
```
The defaults are specified in [Makefile](../Makefile).

The operator will be deployed into namespace **activemq-artemis-operator** and watches the same namespace.

To watch all namespace, change the **WATCH_NAMESPACE** environment variable defined in config/manager/manager.yaml to be empty string before deploy the operator.

At this point you should see the activemq-artemis-operator starting up and if you check the
logs you should see something like

```$xslt
$ kubectl get pod -n activemq-artemis-operator
NAME                                                   READY   STATUS    RESTARTS   AGE
activemq-artemis-controller-manager-5ff459cd95-kn22m   1/1     Running   0          70m
```

## Deploying the broker

Now that the operator is running and listening for changes related to our crd we can deploy [one of our basic broker custom resource examples](../examples/artemis-basic-deployment.yaml)
which looks like

```$xslt
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: ex-aao
spec:
  deploymentPlan:
    size: 1
    image: placeholder
```  

Note in particular the **spec.image** which identifies the container image to use to launch the AMQ Broker. If it's empty or 'placeholder' it will get the latest default image url from config/manager/manager.yaml where a list of supported broker image are defined as environment variables.

To deploy the broker simply execute

```$xslt
kubectl create -f examples/artemis-basic-deployment.yaml -n activemq-artemis-operator
```
In a mement you should see one broker pod is created alongside the operator pod:

```$xslt
$ kubectl get pod
NAME                                                   READY   STATUS    RESTARTS   AGE
activemq-artemis-controller-manager-5ff459cd95-kn22m   1/1     Running   0          128m
ex-aao-ss-0                                            1/1     Running   0          23m
```

## Scaling

The spec.deploymentPlan.size controls how many broker pods you want to deploy to the cluster. You can change this value and apply it to a running deployment to scale up and scale down the broker pods.

For example if you want to scale up the above deployment to 2 pods, modify the size to 2:

examples/artemis-basic-deployment.yaml
```$xslt
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: ex-aao
spec:
  deploymentPlan:
    size: 2
    image: placeholder
```
and apply it:

```$xslt
kubectl apply -f examples/artemis-basic-deployment.yaml -n activemq-artemis-operator
```

and you will get 2 broker pods in the cluster

```$xslt
kubectl get pod
NAME                                                   READY   STATUS    RESTARTS   AGE
activemq-artemis-controller-manager-5ff459cd95-kn22m   1/1     Running   0          140m
ex-aao-ss-0                                            1/1     Running   0          35m
ex-aao-ss-1                                            1/1     Running   0          69s
```

You can scale down the deployment in similar manner by reducing the size and apply it again.

### Clustering

By default if broker pods are scaled to more than one then the broker pods form a broker
[cluster](https://activemq.apache.org/components/artemis/documentation/2.19.0/clusters.html), meaning connect to each other and redistribute messages using default 'ON_DEMAND' policy. 

## Undeploying the broker

To undeploy the broker we simply execute

```$xslt
$ kubectl delete -f examples/artemis-basic-deployment.yaml -n activemq-artemis-operator
activemqartemis.broker.amq.io "ex-aao" deleted
```

## Managing Queues

### Overview

Users can use the activemqartemisaddress CRD to create and remove queues/address on a running broker pod.

For example suppose you have deployed a broker pod like above, you can deploy an activemqartemisaddress resouce from the [examples dir](../examples/address-queue-create-auto-removed.yaml):

address-queue-create-auto-removed.yaml:
```$xslt
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemisAddress
metadata:
  name: ex-aaoaddress
spec:
  addressName: myAddress0
  queueName: myQueue0
  routingType: anycast
  removeFromBrokerOnDelete: true
```

and the deploy command:

```$xslt
$ kubectl create -f examples/address-queue-create-auto-removed.yaml -n activemq-artemis-operator
activemqartemisaddress.broker.amq.io/ex-aaoaddress created
```

When it is deployed it will create a queue named **myQueue0** on an address **myAddress0** with **anycast** routing type.

The **spec.removeFromBrokerOnDelete** controls how to deal with the created queue/address resources when you delete the above custom resource:

```$xslt
$ kubectl delete -f examples/address-queue-create-auto-removed.yaml -n activemq-artemis-operator
activemqartemisaddress.broker.amq.io "ex-aaoaddress" deleted
```

If **spec.removeFromBrokerOnDelete** is true, the queue/address resources will be deleted from broker.
If it is false, the queue/address created by this custome resource will be kept in broker even after the custom resource has been deleted.

## Draining messages on scale down

When a broker pod is being scaled down, a scaledown controller can be deployed aumatically to handle message migration from the scaled down broker pod to an active broker.

When the scale down controller detects the event it starts a drainer pod.
The drainer pod will contact one of the live pods in the cluster and drain the messages over to it.
After the draining is complete it shuts down itself.

The message draining only works when you enabled persistence and messageMigration on broker custome resource.
For example create a broker.yaml with the following content:

```$xslt
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: ex-aao
spec:
  deploymentPlan:
    size: 2
    image: placeholder
    persistenceEnabled: true
    messageMigration: true
```

To demonstrate the message draining first deploy the above custom resource (assuming the operator is running):

```$xslt
$ kubectl create -f broker.yaml -n activemq-artemis-operator
activemqartemis.broker.amq.io/ex-aao created
```

You shall see 2 broker pods are created.

```$xslt
$ kubectl get pod
NAME                                                   READY   STATUS    RESTARTS   AGE
activemq-artemis-controller-manager-5ff459cd95-kn22m   1/1     Running   0          3h19m
ex-aao-ss-0                                            1/1     Running   0          89s
ex-aao-ss-1                                            1/1     Running   0          53s
```

Now we'll use broker's cli tool to send some messages to each broker pod. 

First send 100 messages to broker **ex-aao-ss-0**:

```$xslt
kubectl exec ex-aao-ss-0 -- amq-broker/bin/artemis producer --user x --password y --url tcp://ex-aao-ss-0:61616 --message-count=100
Defaulted container "ex-aao-container" out of: ex-aao-container, ex-aao-container-init (init)
Connection brokerURL = tcp://ex-aao-ss-0:61616
Producer ActiveMQQueue[TEST], thread=0 Started to calculate elapsed time ...

Producer ActiveMQQueue[TEST], thread=0 Produced: 100 messages
Producer ActiveMQQueue[TEST], thread=0 Elapsed time in second : 0 s
Producer ActiveMQQueue[TEST], thread=0 Elapsed time in milli second : 409 milli seconds
```

then send another 100 messages to broker **ex-aao-ss-1**

```$xslt
kubectl exec ex-aao-ss-1 -- amq-broker/bin/artemis producer --user x --password y --url tcp://ex-aao-ss-1:61616 --message-count=100
Defaulted container "ex-aao-container" out of: ex-aao-container, ex-aao-container-init (init)
Connection brokerURL = tcp://ex-aao-ss-1:61616
Producer ActiveMQQueue[TEST], thread=0 Started to calculate elapsed time ...

Producer ActiveMQQueue[TEST], thread=0 Produced: 100 messages
Producer ActiveMQQueue[TEST], thread=0 Elapsed time in second : 0 s
Producer ActiveMQQueue[TEST], thread=0 Elapsed time in milli second : 466 milli seconds
```

Now each of the 2 brokers has 100 messages. Modify the broker.yaml to scale down to one broker

broker.yaml
```$xslt
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: ex-aao
spec:
  deploymentPlan:
    size: 1
    image: placeholder
    persistenceEnabled: true
    messageMigration: true
```
and re-apply it:
```$xslt
kubectl apply -f broker.yaml -n activemq-artemis-operator
```

The broker pods will be reduced to only one

```$xslt
$ kubectl get pod
NAME                                                   READY   STATUS    RESTARTS   AGE
activemq-artemis-controller-manager-5ff459cd95-kn22m   1/1     Running   0          3h57m
ex-aao-ss-0                                            1/1     Running   0          39m
```

Now the messages on the broker pod **ex-aao-ss-1** should have been all migrated to pod **ex-aao-ss-0**.
Use the broker cli tool again to check:

```$xslt
kubectl exec ex-aao-ss-0 -- amq-broker/bin/artemis queue stat --user x --password y --url tcp://ex-aao-ss-0:61616
Defaulted container "ex-aao-container" out of: ex-aao-container, ex-aao-container-init (init)
Connection brokerURL = tcp://ex-aao-ss-0:61616
|NAME                     |ADDRESS                  |CONSUMER_COUNT |MESSAGE_COUNT |MESSAGES_ADDED |DELIVERING_COUNT |MESSAGES_ACKED |SCHEDULED_COUNT |ROUTING_TYPE |
|$.artemis.internal.sf.my-cluster.941368e6-79c9-11ec-b4c8-0242ac11000b|$.artemis.internal.sf.my-cluster.941368e6-79c9-11ec-b4c8-0242ac11000b|0              |0             |0              |0                |0              |0               |MULTICAST    |
|DLQ                      |DLQ                      |0              |0             |0              |0                |0              |0               |ANYCAST      |
|ExpiryQueue              |ExpiryQueue              |0              |0             |0              |0                |0              |0               |ANYCAST      |
|TEST                     |TEST                     |0              |200           |200            |0                |0              |0               |ANYCAST      |
|activemq.management.91b87b03-0c70-4630-beb8-6a7919a4923a|activemq.management.91b87b03-0c70-4630-beb8-6a7919a4923a|1              |0             |0              |0                |0              |0               |MULTICAST    |
```
You can see the queue TEST has 200 messages now.


## Undeploying the operator

Run this command to undeploy the operator

```$xslt
make OPERATOR_IMAGE_REPO=<your repo> OPERATOR_VERSION=<tag> undeploy
```