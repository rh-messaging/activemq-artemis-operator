---
title: "Exchanging messages over an ssl ingress"  
description: "Steps to get a producer and a consummer exchanging messages over a deployed broker on kubernetes using an ingress"
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

#### Enable nginx and ssl passthrough for minikube

```console
$ minikube addons enable ingress
$ minikube kubectl -- patch deployment -n ingress-nginx ingress-nginx-controller --type='json' -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value":"--enable-ssl-passthrough"}]'
```
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
$ kubectl get pod --namespace send-receive-project --watch
```

### Deploy the ActiveMQ Artemis Broker

For this tutorial we need to:

* have a broker that is able to listen to any network interface. For that we
  setup an `acceptor` that will be listening on every interfaces on port
  `62626`.
* have the ssl protocol configured for the `acceptor`
* have queues to exchange messages on. These are configured by the broker
  properties. Two queues are setup, one called `APP.JOBS` that is of type
  `ANYCAST` and one called `APP.COMMANDS` that is of type `MULTICAST`.

#### Create the certs

We'll take some inspiration from the [ssl broker
setup](https://github.com/artemiscloud/send-receive-project/blob/main/docs/tutorials/ssl_broker_setup.md)
to configure the certificates.

> [!NOTE]
> In this tutorial:
> * The password used for the certificates is `000000`.
> * The secret name is `send-receive-sslacceptor-secret` composed from the broker
>   name `send-receive` and the acceptor name `sselacceptor`

```console
$ keytool -genkeypair -alias artemis -keyalg RSA -keysize 2048 -storetype PKCS12 -keystore broker.ks -validity 3000
$ keytool -export -alias artemis -file broker.cert -keystore broker.ks
$ keytool -import -v -trustcacerts -alias artemis -file broker.cert -keystore client.ts
$ kubectl create secret generic send-receive-sslacceptor-secret --from-file=broker.ks --from-file=client.ts --from-literal=keyStorePassword='000000' --from-literal=trustStorePassword='000000' -n send-receive-project
```

#### Start the broker

```console
$ kubectl apply -f - <<EOF
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: send-receive
  namespace: send-receive-project
spec:
  ingressDomain: localresolvconf.com
  acceptors:
    - name: sslacceptor
      port: 62626
      expose: true
      sslEnabled: true
      sslSecret: send-receive-sslacceptor-secret
  brokerProperties:
    - addressConfigurations."APP.JOBS".routingTypes=ANYCAST
    - addressConfigurations."APP.JOBS".queueConfigs."APP.JOBS".routingType=ANYCAST
    - addressConfigurations."APP.COMMANDS".routingTypes=MULTICAST
EOF
```

Wait for the Broker to be ready:

```console
kubectl get ActivemqArtemis  --watch
NAME           READY   AGE
send-receive   False   14s
send-receive   True    51s
send-receive   True    51s
```

#### Create a route to access the ingress:

Check for the ingress availability:

```console
$ kubectl get ingress --show-labels
NAME                                 CLASS   HOSTS                                                                              ADDRESS         PORTS     AGE   LABELS
send-receive-sslacceptor-0-svc-ing   nginx   send-receive-sslacceptor-0-svc-ing-send-receive-project.localresolvconf.com   192.168.39.57   80, 443   18h   ActiveMQArtemis=send-receive,application=send-receive-app,statefulset.kubernetes.io/pod-name=send-receive-ss-0
```

> [!NOTE]
> The ingress hostname is resolving to
> `send-receive-sslacceptor-0-svc-ing-send-receive-project.localresolvconf.com`.
> This is because we have specified in the broker `spec` an `ingressDomain`.
> This field must be populated for the operation to succeed.

For instance, you can add a new entry in your `/etc/hosts` to make this domain
point to the associated address, in our example: `192.168.39.57`

```console
$ cat /etc/hosts
[...]
192.168.39.57 send-receive-sslacceptor-0-svc-ing-send-receive-project.localresolvconf.com
```

### Exchanging messages between a producer and a consumer

Download the [latest
release](https://activemq.apache.org/components/artemis/download/) of ActiveMQ
Artemis, decompress the tarball and locate the artemis executable.

The `artemis` will need to point to the https endopint generated in earlier with
a couple of parameters set:
* `sslEnabled` = `true`
* `verifyHost` = `false`
* `trustStorePath` = `/some/path/broker.ks`
* `trustStorePassword` = `000000`

To use the consumer and the producer you'll need to give the path to the
`broker.ks` file you've created earlier. In the following commands the file is
located to `/home/tlavocat/dev/send-receive-project/broker.ks`.

#### ANYCAST

For this use case, run first the producer, then the consumer.

```console
$ ./artemis producer --destination queue://APP.JOBS --url "tcp://send-receive-sslacceptor-0-svc-ing-send-receive-project.localresolvconf.com:443?sslEnabled=true&verifyHost=false&trustStorePath=/home/tlavocat/dev/send-receive-project/broker.ks&trustStorePassword=000000"
Connection brokerURL = tcp://send-receive-sslacceptor-0-svc-ing-send-receive-project.localresolvconf.com:443?sslEnabled=true&verifyHost=false&trustStorePath=/home/tlavocat/dev/send-receive-project/broker.ks&trustStorePassword=000000
Producer ActiveMQQueue[APP.JOBS], thread=0 Started to calculate elapsed time ...

Producer ActiveMQQueue[APP.JOBS], thread=0 Produced: 1000 messages
Producer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in second : 7 s
Producer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in milli second : 7800 milli seconds
```

```console
$ ./artemis consumer --destination queue://APP.JOBS --url "tcp://send-receive-sslacceptor-0-svc-ing-send-receive-project.localresolvconf.com:443?sslEnabled=true&verifyHost=false&trustStorePath=/home/tlavocat/dev/send-receive-project/broker.ks&trustStorePassword=000000"
Connection brokerURL = tcp://send-receive-sslacceptor-0-svc-ing-send-receive-project.localresolvconf.com:443?sslEnabled=true&verifyHost=false&trustStorePath=/home/tlavocat/dev/send-receive-project/broker.ks&trustStorePassword=000000
Consumer:: filter = null
Consumer ActiveMQQueue[APP.JOBS], thread=0 wait until 1000 messages are consumed
Received 1000
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumed: 1000 messages
Consumer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in second : 0 s
Consumer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in milli second : 619 milli seconds
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumed: 1000 messages
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumer thread finished
```

#### MULTICAST

For this use case, run first the consumer(s), then the producer.
More details there
https://activemq.apache.org/components/artemis/documentation/2.0.0/address-model.html.

In `n` other terminal(s) connect `n` consumer(s):

```console
$ ./artemis consumer --destination topic://APP.COMMANDS --url "tcp://send-receive-sslacceptor-0-svc-ing-send-receive-project.localresolvconf.com:443?sslEnabled=true&verifyHost=false&trustStorePath=/home/tlavocat/dev/send-receive-project/broker.ks&trustStorePassword=000000"
Connection brokerURL = tcp://send-receive-sslacceptor-0-svc-ing-send-receive-project.localresolvconf.com:443?sslEnabled=true&verifyHost=false&trustStorePath=/home/tlavocat/dev/send-receive-project/broker.ks&trustStorePassword=000000
Consumer:: filter = null
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 wait until 1000 messages are consumed
Received 1000
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumed: 1000 messages
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in second : 19 s
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in milli second : 19499 milli seconds
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumed: 1000 messages
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumer thread finished
```

Then connect the producer to start broadcasting messages.

```console
$ ./artemis producer --destination topic://APP.COMMANDS --url "tcp://send-receive-sslacceptor-0-svc-ing-send-receive-project.localresolvconf.com:443?sslEnabled=true&verifyHost=false&trustStorePath=/home/tlavocat/dev/send-receive-project/broker.ks&trustStorePassword=000000"
Connection brokerURL = tcp://send-receive-sslacceptor-0-svc-ing-send-receive-project.localresolvconf.com:443?sslEnabled=true&verifyHost=false&trustStorePath=/home/tlavocat/dev/send-receive-project/broker.ks&trustStorePassword=000000
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Started to calculate elapsed time ...

Producer ActiveMQTopic[APP.COMMANDS], thread=0 Produced: 1000 messages
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in second : 1 s
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in milli second : 1081 milli seconds
```
