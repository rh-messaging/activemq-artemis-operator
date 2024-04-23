---
title: "Exchanging messages using port forwarding"  
description: "Steps to get a producer and a consummer exchanging messages over a deployed broker on OpenShift using port forwwarding"
draft: false
images: []
menu:
  docs:
    parent: "send-receive"
weight: 110
toc: true
---

### Prerequisite

Before you start, you need to have access to a running Kubernetes cluster
environment. A [Minikube](https://minikube.sigs.k8s.io/docs/start/) instance
running on your laptop will do fine.

### Deploy the operator

#### create the namespace

```console
$ kubectl create namespace send-receive-project
$ kubectl config set-context --current --namespace=send-receive-project
```

Go to the root of the operator repo and install it:

```console
$ cd send-receive-project
$ ./deploy/install_opr.sh
```

Wait for the Operator to start (status: `running`).
```console
$ kubectl get pod --namespace send-receive-project
```

### Deploying the ActiveMQ Artemis Broker

For this tutorial we need to:

* have a broker that is able to listen to any network interface. For that we
  setup an `acceptor` that will be listening on every interfaces on port
  `62626`.
* have queues to exchange messages on. These are configured by the broker
  properties. Two queues are setup, one called `APP.JOBS` that is of type
  `ANYCAST` and one called `APP.COMMANDS` that is of type `MULTICAST`.

```console
$ kubectl apply -f - <<EOF
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: default
  namespace: send-receive-project
spec:
  acceptors:
    - bindToAllInterfaces: true
      name: acceptAll
      port: 62626
  brokerProperties:
    - addressConfigurations."APP.JOBS".routingTypes=ANYCAST
    - addressConfigurations."APP.JOBS".queueConfigs."APP.JOBS".routingType=ANYCAST
    - addressConfigurations."APP.COMMANDS".routingTypes=MULTICAST
EOF
```

Wait for the Broker to be ready:

```console
$ kubectl get ActivemqArtemis -o custom-columns="NAME:.metadata.name,READY:.status.conditions[?(@.type=='Ready')].status" -n send-receive-project
NAME           READY
send-receive   True
```

### Forwarding ports

```console
$ kubectl port-forward default-ss-0 62626 -n send-receive-project
Forwarding from 127.0.0.1:62626 -> 62626
Forwarding from [::1]:62626 -> 62626
```

### Exchanging messages between a producer and a consumer

Download the [latest
release](https://activemq.apache.org/components/artemis/download/) of ActiveMQ
Artemis, decompress the tarball and locate the artemis executable.

#### ANYCAST

For this use case, run first the producer, then the consumer.

```console
$ ./artemis producer --destination APP.JOBS  --url tcp://localhost:62626

Connection brokerURL = tcp://localhost:62626
Producer ActiveMQQueue[APP.JOBS], thread=0 Started to calculate elapsed time ...
Producer ActiveMQQueue[APP.JOBS], thread=0 Produced: 1000 messages
Producer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in second : 8 s
Producer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in milli second : 8711 milli seconds
```

```console
$ ./artemis consumer --destination APP.JOBS  --url tcp://localhost:62626

Connection brokerURL = tcp://localhost:62626
Consumer:: filter = null
Consumer ActiveMQQueue[APP.JOBS], thread=0 wait until 1000 messages are consumed
Received 1000
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumed: 1000 messages
Consumer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in second : 0 s
Consumer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in milli second : 61 milli seconds
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumed: 1000 messages
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumer thread finished
```

#### MULTICAST

For this use case, run first the consumer(s), then the producer.
More details there
https://activemq.apache.org/components/artemis/documentation/2.0.0/address-model.html.

In `n` other terminal(s) connect `n` consumer(s):

```console
$ ./artemis consumer --destination topic://APP.COMMANDS  --url tcp://localhost:62626

Connection brokerURL = tcp://localhost:62626
Consumer:: filter = null
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 wait until 1000 messages are consumed
Received 1000
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumed: 1000 messages
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in second : 14 s
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in milli second : 14934 milli seconds
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumed: 1000 messages
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumer thread finished
```

Then connect the producer

```console
$ ./artemis producer --destination topic://APP.COMMANDS  --url tcp://localhost:62626
Connection brokerURL = tcp://localhost:62626
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Started to calculate elapsed time ...

Producer ActiveMQTopic[APP.COMMANDS], thread=0 Produced: 1000 messages
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in second : 0 s
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in milli second : 889 milli seconds
```
