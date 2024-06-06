---
title: "Exchanging messages over an ssl ingress with cert-manager's generated certs"
description: "Steps to get a producer and a consummer exchanging messages over a deployed broker on kubernetes using an ingress and certificates genererated via cert-manager"
draft: false
images: []
menu:
  docs:
    parent: "tutorials"
weight: 110
toc: true
---

### Prerequisite

Before you start, you need to have access to a running Kubernetes cluster
environment. A [Minikube](https://minikube.sigs.k8s.io/docs/start/) instance
running on your laptop will do fine.

#### Start minikube with a parametrized dns domain name

```console
$ minikube start --dns-domain='demo.artemiscloud.io'
😄  minikube v1.32.0 on Fedora 39
🎉  minikube 1.33.1 is available! Download it: https://github.com/kubernetes/minikube/releases/tag/v1.33.1
💡  To disable this notice, run: 'minikube config set WantUpdateNotification false'

✨  Automatically selected the kvm2 driver. Other choices: qemu2, ssh
👍  Starting control plane node minikube in cluster minikube
🔥  Creating kvm2 VM (CPUs=2, Memory=6000MB, Disk=20000MB) ...
🐳  Preparing Kubernetes v1.28.3 on Docker 24.0.7 ...
    ▪ Generating certificates and keys ...
    ▪ Booting up control plane ...
    ▪ Configuring RBAC rules ...
🔗  Configuring bridge CNI (Container Networking Interface) ...
    ▪ Using image gcr.io/k8s-minikube/storage-provisioner:v5
🔎  Verifying Kubernetes components...
🌟  Enabled addons: storage-provisioner, default-storageclass
🏄  Done! kubectl is now configured to use "minikube" cluster and "default" namespace by default
```

#### Enable nginx and ssl passthrough for minikube

```console
$ minikube addons enable ingress
$ minikube kubectl -- patch deployment -n ingress-nginx ingress-nginx-controller --type='json' -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value":"--enable-ssl-passthrough"}]'
```

#### Make sure the domain of your cluster is resolvable

If you are running your OpenShift cluster locally, you might not be able to
resolve the urls to IPs out of the blue. Follow [this guide]({{< ref
"/docs/help/hostname_resolution" >}} "set up dnsmasq") to configure your setup.

### Deploy the operator

#### create the namespace

```console
$ kubectl create namespace send-receive-project
$ kubectl config set-context --current --namespace=send-receive-project
```

Go to the root of the operator repo and install it:

```console
$ ./deploy/install_opr.sh
```

Wait for the Operator to start (status: `running`).
```console
$ kubectl get pod --namespace send-receive-project --watch
```

### Create the chain of trust with Cert manager

#### Install cert manager

[Follow the official documentation.](https://cert-manager.io/docs/installation/)

```console
$ kubectl get pod --namespace cert-manager --watch
NAME                                       READY   STATUS              RESTARTS   AGE
cert-manager-7ddd8cdb9f-fx9mw              1/1     Running             0          13s
cert-manager-cainjector-57cd76c845-zg45d   1/1     Running             0          13s
cert-manager-webhook-cf8f9f895-kkj5r       0/1     ContainerCreating   0          13s
cert-manager-webhook-cf8f9f895-kkj5r       0/1     Running             0          17s
cert-manager-webhook-cf8f9f895-kkj5r       1/1     Running             0          21s
```

#### Create the root issuer

```console
$ kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: send-receive-root-issuer
  namespace: send-receive-project
spec:
  selfSigned: {}
EOF
```

```console
$ kubectl get issuers  -n send-receive-project -o wide send-receive-root-issuer --watch
NAME                       READY   STATUS   AGE
send-receive-root-issuer   True             45s
```

#### Create the issuer certificate

```console
$ kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: send-receive-issuer-cert
  namespace: send-receive-project
spec:
  isCA: true
  commonName: ArtemiseCloud send-receive issuer
  dnsNames:
    - issuer.demo.artemiscloud.io
  secretName: send-receive-issuer-cert-secret
  privateKey:
    algorithm: ECDSA
    size: 256
  issuerRef:
    name: send-receive-root-issuer
    kind: Issuer
EOF
```

```console
$ kubectl get certificates  -n send-receive-project --watch
NAME                       READY   SECRET                            AGE
send-receive-issuer-cert   True    send-receive-issuer-cert-secret   24s
```


#### Create the issuer

```console
$ kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: send-receive-issuer
  namespace: send-receive-project
spec:
  ca:
    secretName: send-receive-issuer-cert-secret
EOF
```

```console
$ kubectl get issuer  -n send-receive-project --watch
NAME                       READY   AGE
send-receive-issuer        True    10s
send-receive-root-issuer   True    50s
```

#### Download the issuer's CA

> Note: to execute the following commands you'll need
[jq](https://jqlang.github.io/jq/).

We need to download a certificate so that our `artermis` client is be
able to open a secure connection to the broker.

In this tutorial we configure the deployment CR so that upon starting a new
broker, `cert-manager` is using the `send-receive-issuer` to generate the
broker's certificate. The issuer is signing the generated certs with its CA:
`send-receive-issuer-cert`. In turns this means that a client using the
`send-receive-issuer-cert` in its trust store will trust any of the issuer's
generated certs.

The issuer CA can be found in the `send-receive-issuer-cert-secret ` secret:
```console
$ kubectl get secrets send-receive-issuer-cert-secret -o json | jq -r '.data."tls.crt"' | base64 -d
-----BEGIN CERTIFICATE-----
MIIClzCCAj2gAwIBAgIRAMM9CevyAwrXP/KTKZdUA40wCgYIKoZIzj0EAwIwLDEq
MCgGA1UEAxMhQXJ0ZW1pc2VDbG91ZCBzZW5kLXJlY2VpdmUgaXNzdWVyMB4XDTI0
MDUxNDEwMzI1MVoXDTI0MDgxMjEwMzI1MVowADCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAPQMxPdW+9N6rLfw7Wg8d4mSWOWKiNtSWlG2eyIXmrCnwjdo
pghihq91MjRxVCbQBsUVFc34NQ1IsOMPetWeqXjwAIXl8hvKgyKwcS644dHOKl1U
36etHZpm1Zb0lH+0yL1HSTmP0KwKr3FbqM3QfZ83KfDrmjMU3PrmENs4HwRmD9r5
3Qy7AFSJo9tMkMASsXbSKuJWCp44dRzD/mC9Mu+jsUUNafIeVznTBlSSxQUsvZhE
FvLWMIgTpV8RKijuDut/YaYmBmY0C9JKgCBbKUUA4bvIIxmdeJybYqJWDoiPzvZc
j9Z/FByaIlFFrmv+VMjEG5LZLwKbwcZYgCoRi0MCAwEAAaOBoDCBnTAOBgNVHQ8B
Af8EBAMCBaAwDAYDVR0TAQH/BAIwADAfBgNVHSMEGDAWgBRb8dir/2ibdJQUUliM
ft4ZT5WbpDBcBgNVHREBAf8EUjBQgk5zZW5kLXJlY2VpdmUtc3MtMC5zZW5kLXJl
Y2VpdmUtaGRscy1zdmMuc2VuZC1yZWNlaXZlLXByb2plY3Quc3ZjLmNsdXN0ZXIu
bG9jYWwwCgYIKoZIzj0EAwIDSAAwRQIgXNd/dVEZ+E9qFPvWg3qlwNTnPGPXO5cg
7aXMPOKK60UCIQDMt3N/FZWJKMeRZImkXob4+9iSgusly6pPdWwDr2IV8w==
-----END CERTIFICATE-----
```

Store the output in the `/tmp/IssuerCA.pem` file:
```console
$ kubectl get secrets send-receive-issuer-cert-secret -o json | jq -r '.data."tls.crt"' | base64 -d > /tmp/IssuerCA.pem
```

### Deploy the ActiveMQ Artemis Broker

#### Ingress and ssl configuration

The acceptor is configured as follows:
* `port: 62626`
* `sslEnabled: true` activates ssl
* `expose: true` and `exposeMode: ingress` are triggering an ingress creation
* `sslSecret: send-receive-sslacceptor-0-svc-ing-ptls` provides the name of the
  sercret store that'll be used to secure the connection. The `-ptls` suffix
  indicates that it will be provide by `cert-manager` so the artemis operator
  will not validate its present when the Artemis CR is deployed.
* `ingressHost:
  ing.$(ITEM_NAME).$(CR_NAME)-$(BROKER_ORDINAL).$(CR_NAMESPACE).$(INGRESS_DOMAIN)`
   customizes the url to access the `acceptor` from outside the cluster.

An annotation is used to ask `cert-manager` to produce the `sslSecret`
needed to secure the connection. It is configured as follows:
* the `selector` attaches the annotation to the `acceptor`'s ingress by its name
* `cert-manager.io/issuer: send-receive-issuer` links to the previously created
  issuer, `cert-manager` will use it to generate the secrets.
* `spec.hosts` list the domains for which the certificate applies, it has to
  match  the url of the ingress set in the `acceptor`.
* `spec.secretName` matches the `sslSecret` value of the `acceptor` and must be
  set to the name of the ingress + `-ptls`. Here:
  `send-receive-sslacceptor-0-svc-ing-ptls`.

#### broker configuration

Two queues are added to exchange messages on. These are configured via the broker
properties. Two queues are setup, one called `APP.JOBS` that is of type
`ANYCAST` and one called `APP.COMMANDS` that is of type `MULTICAST`.

#### Apply the configuration and start the broker

```console
$ kubectl apply -f - <<'EOF'
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: send-receive
  namespace: send-receive-project
spec:
  ingressDomain: demo.artemiscloud.io
  acceptors:
    - name: sslacceptor
      port: 62626
      sslEnabled: true
      expose: true
      exposeMode: ingress
      sslSecret: send-receive-sslacceptor-0-svc-ing-ptls
      ingressHost: ing.$(ITEM_NAME).$(CR_NAME)-$(BROKER_ORDINAL).$(CR_NAMESPACE).$(INGRESS_DOMAIN)
  resourceTemplates:
    - selector:
        kind: Ingress
        name: send-receive-sslacceptor-0-svc-ing
      annotations:
        cert-manager.io/issuer: send-receive-issuer
      patch:
        kind: Ingress
        spec:
          tls:
            - hosts:
                - ing.sslacceptor.send-receive-0.send-receive-project.demo.artemiscloud.io
              secretName: send-receive-sslacceptor-0-svc-ing-ptls
  brokerProperties:
    - addressConfigurations."APP.JOBS".routingTypes=ANYCAST
    - addressConfigurations."APP.JOBS".queueConfigs."APP.JOBS".routingType=ANYCAST
    - addressConfigurations."APP.COMMANDS".routingTypes=MULTICAST
EOF
```

Wait for the Broker to be ready:

```console
$ kubectl get ActivemqArtemis  --watch
NAME           READY   AGE
send-receive   True    51s
```

Check that the ingress is available and has an IP address:

```console
$ kubectl get ingress --show-labels
NAME                                 CLASS   HOSTS                                                                     ADDRESS         PORTS     AGE   LABELS
send-receive-sslacceptor-0-svc-ing   nginx   ing.sslacceptor.send-receive-0.send-receive-project.demo.artemiscloud.io  192.168.39.68   80, 443   18s   ActiveMQArtemis=send-receive,application=send-receive-app,statefulset.kubernetes.io/pod-name=send-receive-ss-0

```

### Exchange messages between a producer and a consumer

#### Download Artemis

Download the [latest
release](https://activemq.apache.org/components/artemis/download/) of ActiveMQ
Artemis, decompress the tarball and locate the artemis executable.

The client needs a bit more information than just the raw URL of the ingress to
open a working connection:

* `sslEnabled` is set to `true`
* `trustStorePath` is set to the issuer CA downloaded at the previous Steps
* `trustStoreType` is set to `PEM`
* `useTopologyForLoadBalancing` is set to `false` Leaving the value to true
  would lead to some errors where the client will try to open a direct line of
  communication with the broker without going through the ingress. The ingress
  will provide a round robin load balancing when multiple brokers can be
  reached.

##### Test the connection

```console
$ ./artemis check queue --name TEST --produce 10 --browse 10 --consume 10 --url 'tcp://ing.sslacceptor.send-receive-0.send-receive-project.demo.artemiscloud.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false' --verbose
Executing org.apache.activemq.artemis.cli.commands.check.QueueCheck check queue --name TEST --produce 10 --browse 10 --consume 10 --url tcp://send-receive-ss-0.send-receive-hdls-svc.send-receive-project.svc.demo.artemiscloud.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM --verbose 
Home::/home/tlavocat/dev/activemq-artemis/artemis-distribution/target/apache-artemis-2.34.0-SNAPSHOT-bin/apache-artemis-2.34.0-SNAPSHOT, Instance::null
Connection brokerURL = tcp://ing.sslacceptor.send-receive-0.send-receive-project.demo.artemiscloud.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM
Running QueueCheck
Checking that a producer can send 10 messages to the queue TEST ... success
Checking that a consumer can browse 10 messages from the queue TEST ... success
Checking that a consumer can consume 10 messages from the queue TEST ... success
Checks run: 3, Failures: 0, Errors: 0, Skipped: 0, Time elapsed: 6.402 sec - QueueCheck
```

#### ANYCAST

For this use case, run first the producer, then the consumer.

```console
$ ./artemis producer --destination queue://APP.JOBS --url "tcp://ing.sslacceptor.send-receive-0.send-receive-project.demo.artemiscloud.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false"
Producer ActiveMQQueue[APP.JOBS], thread=0 Started to calculate elapsed time ...

Producer ActiveMQQueue[APP.JOBS], thread=0 Produced: 1000 messages
Producer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in second : 9 s
Producer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in milli second : 9104 milli seconds
```

```console
$ ./artemis consumer --destination queue://APP.JOBS --url "tcp://ing.sslacceptor.send-receive-0.send-receive-project.demo.artemiscloud.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false"
Connection brokerURL = tcp://ing.sslacceptor.send-receive-0.send-receive-project.demo.artemiscloud.io:443?sslEnabled=true&trustStorePath=/home/tlavocat/IssuerCA.pem&trustStoreType=PEM
Consumer:: filter = null
Consumer ActiveMQQueue[APP.JOBS], thread=0 wait until 1000 messages are consumed
Received 1000
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumed: 1000 messages
Consumer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in second : 0 s
Consumer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in milli second : 116 milli seconds
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumed: 1000 messages
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumer thread finished
```

#### MULTICAST

For this use case, run first the consumer(s), then the producer.
[More details there](https://activemq.apache.org/components/artemis/documentation/2.0.0/address-model.html).

1. in `n` other terminal(s) connect `n` consumer(s):

```console
$ ./artemis consumer --destination topic://APP.COMMANDS --url "tcp://ing.sslacceptor.send-receive-0.send-receive-project.demo.artemiscloud.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false"
Consumer:: filter = null
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 wait until 1000 messages are consumed
Received 1000
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumed: 1000 messages
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in second : 8 s
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in milli second : 8434 milli seconds
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumed: 1000 messages
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumer thread finished
```

2. connect the producer to start broadcasting messages.

```console
$ ./artemis producer --destination topic://APP.COMMANDS --url "tcp://ing.sslacceptor.send-receive-0.send-receive-project.demo.artemiscloud.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false"
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Started to calculate elapsed time ...

Producer ActiveMQTopic[APP.COMMANDS], thread=0 Produced: 1000 messages
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in second : 0 s
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in milli second : 653 milli seconds
```

