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

```{"stage":"init", "id":"minikube_start"}
minikube start --profile tutorialtester
minikube profile tutorialtester
```
```shell tutorial_tester
* [tutorialtester] minikube v1.32.0 on Fedora 40
* Automatically selected the docker driver. Other choices: kvm2, qemu2, ssh
* Using Docker driver with root privileges
* Starting control plane node tutorialtester in cluster tutorialtester
* Pulling base image ...
* Creating docker container (CPUs=2, Memory=15900MB) ...
* Preparing Kubernetes v1.28.3 on Docker 24.0.7 ...
  - Generating certificates and keys ...
  - Booting up control plane ...
  - Configuring RBAC rules ...
* Configuring bridge CNI (Container Networking Interface) ...
  - Using image gcr.io/k8s-minikube/storage-provisioner:v5
* Verifying Kubernetes components...
* Enabled addons: storage-provisioner, default-storageclass
* Done! kubectl is now configured to use "tutorialtester" cluster and "default" namespace by default
* minikube profile was successfully set to tutorialtester
```

#### Enable nginx and ssl passthrough for minikube

```{"stage":"init"}
minikube addons enable ingress
minikube kubectl -- patch deployment -n ingress-nginx ingress-nginx-controller --type='json' -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value":"--enable-ssl-passthrough"}]'
```
```shell tutorial_tester
* ingress is an addon maintained by Kubernetes. For any concerns contact minikube on GitHub.
You can view the list of minikube maintainers at: https://github.com/kubernetes/minikube/blob/master/OWNERS
  - Using image registry.k8s.io/ingress-nginx/kube-webhook-certgen:v20231011-8b53cabe0
  - Using image registry.k8s.io/ingress-nginx/kube-webhook-certgen:v20231011-8b53cabe0
  - Using image registry.k8s.io/ingress-nginx/controller:v1.9.4
* Verifying ingress addon...
* The 'ingress' addon is enabled
deployment.apps/ingress-nginx-controller patched
```

#### Make sure the domain of your cluster is resolvable

If you are running your OpenShift cluster locally, you might not be able to
resolve the urls to IPs out of the blue. Follow [this guide](../help/hostname_resolution.md) to configure your setup.

This tutorial will follow the simple /etc/hosts approach, but feel free to use
the most appropriate one for you.

### Deploy the operator

#### create the namespace

```{"stage":"init"}
kubectl create namespace send-receive-project
kubectl config set-context --current --namespace=send-receive-project
```
```shell tutorial_tester
namespace/send-receive-project created
Context "tutorialtester" modified.
```

Go to the root of the operator repo and install it:

```{"stage":"init", "rootdir":"$operator"}
./deploy/install_opr.sh
```
```shell tutorial_tester
Deploying operator to watch single namespace
Client Version: 4.15.0-0.okd-2024-01-27-070424
Kustomize Version: v5.0.4-0.20230601165947-6ce0bf390ce3
Kubernetes Version: v1.28.3
customresourcedefinition.apiextensions.k8s.io/activemqartemises.broker.amq.io created
customresourcedefinition.apiextensions.k8s.io/activemqartemisaddresses.broker.amq.io created
customresourcedefinition.apiextensions.k8s.io/activemqartemisscaledowns.broker.amq.io created
customresourcedefinition.apiextensions.k8s.io/activemqartemissecurities.broker.amq.io created
serviceaccount/activemq-artemis-controller-manager created
role.rbac.authorization.k8s.io/activemq-artemis-operator-role created
rolebinding.rbac.authorization.k8s.io/activemq-artemis-operator-rolebinding created
role.rbac.authorization.k8s.io/activemq-artemis-leader-election-role created
rolebinding.rbac.authorization.k8s.io/activemq-artemis-leader-election-rolebinding created
deployment.apps/activemq-artemis-controller-manager created
```

Wait for the Operator to start (status: `running`).

```{"stage":"init"}
kubectl wait pod --all --for=condition=Ready --namespace=send-receive-project --timeout=600s
```
```shell tutorial_tester
pod/activemq-artemis-controller-manager-55b8c479df-st8zz condition met
```

### Create the chain of trust with Cert manager

#### Install cert manager

[Follow the official documentation.](https://cert-manager.io/docs/installation/)

```{"stage":"cert-manager"}
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.15.1/cert-manager.yaml
```
```shell tutorial_tester
namespace/cert-manager created
customresourcedefinition.apiextensions.k8s.io/certificaterequests.cert-manager.io created
customresourcedefinition.apiextensions.k8s.io/certificates.cert-manager.io created
customresourcedefinition.apiextensions.k8s.io/challenges.acme.cert-manager.io created
customresourcedefinition.apiextensions.k8s.io/clusterissuers.cert-manager.io created
customresourcedefinition.apiextensions.k8s.io/issuers.cert-manager.io created
customresourcedefinition.apiextensions.k8s.io/orders.acme.cert-manager.io created
serviceaccount/cert-manager-cainjector created
serviceaccount/cert-manager created
serviceaccount/cert-manager-webhook created
clusterrole.rbac.authorization.k8s.io/cert-manager-cainjector created
clusterrole.rbac.authorization.k8s.io/cert-manager-controller-issuers created
clusterrole.rbac.authorization.k8s.io/cert-manager-controller-clusterissuers created
clusterrole.rbac.authorization.k8s.io/cert-manager-controller-certificates created
clusterrole.rbac.authorization.k8s.io/cert-manager-controller-orders created
clusterrole.rbac.authorization.k8s.io/cert-manager-controller-challenges created
clusterrole.rbac.authorization.k8s.io/cert-manager-controller-ingress-shim created
clusterrole.rbac.authorization.k8s.io/cert-manager-cluster-view created
clusterrole.rbac.authorization.k8s.io/cert-manager-view created
clusterrole.rbac.authorization.k8s.io/cert-manager-edit created
clusterrole.rbac.authorization.k8s.io/cert-manager-controller-approve:cert-manager-io created
clusterrole.rbac.authorization.k8s.io/cert-manager-controller-certificatesigningrequests created
clusterrole.rbac.authorization.k8s.io/cert-manager-webhook:subjectaccessreviews created
clusterrolebinding.rbac.authorization.k8s.io/cert-manager-cainjector created
clusterrolebinding.rbac.authorization.k8s.io/cert-manager-controller-issuers created
clusterrolebinding.rbac.authorization.k8s.io/cert-manager-controller-clusterissuers created
clusterrolebinding.rbac.authorization.k8s.io/cert-manager-controller-certificates created
clusterrolebinding.rbac.authorization.k8s.io/cert-manager-controller-orders created
clusterrolebinding.rbac.authorization.k8s.io/cert-manager-controller-challenges created
clusterrolebinding.rbac.authorization.k8s.io/cert-manager-controller-ingress-shim created
clusterrolebinding.rbac.authorization.k8s.io/cert-manager-controller-approve:cert-manager-io created
clusterrolebinding.rbac.authorization.k8s.io/cert-manager-controller-certificatesigningrequests created
clusterrolebinding.rbac.authorization.k8s.io/cert-manager-webhook:subjectaccessreviews created
role.rbac.authorization.k8s.io/cert-manager-cainjector:leaderelection created
role.rbac.authorization.k8s.io/cert-manager:leaderelection created
role.rbac.authorization.k8s.io/cert-manager-webhook:dynamic-serving created
rolebinding.rbac.authorization.k8s.io/cert-manager-cainjector:leaderelection created
rolebinding.rbac.authorization.k8s.io/cert-manager:leaderelection created
rolebinding.rbac.authorization.k8s.io/cert-manager-webhook:dynamic-serving created
service/cert-manager created
service/cert-manager-webhook created
deployment.apps/cert-manager-cainjector created
deployment.apps/cert-manager created
deployment.apps/cert-manager-webhook created
mutatingwebhookconfiguration.admissionregistration.k8s.io/cert-manager-webhook created
validatingwebhookconfiguration.admissionregistration.k8s.io/cert-manager-webhook created
```

```{"stage":"cert-manager"}
kubectl wait pod --all --for=condition=Ready --namespace=cert-manager --timeout=240s
```
```shell tutorial_tester
pod/cert-manager-5798486f6b-4m822 condition met
pod/cert-manager-cainjector-7666685ff5-6wd64 condition met
pod/cert-manager-webhook-5f594df789-2nrjq condition met
```

#### Create the root issuer

```bash {"stage":"cert-manager", "HereTag":"EOF", "runtime":"bash", "label":"create root issuer"}
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: send-receive-root-issuer
  namespace: send-receive-project
spec:
  selfSigned: {}
EOF
```
```shell tutorial_tester
issuer.cert-manager.io/send-receive-root-issuer created
```

```{"stage":"cert-manager"}
kubectl wait issuer send-receive-root-issuer  --for=condition=Ready --namespace=send-receive-project
```
```shell tutorial_tester
issuer.cert-manager.io/send-receive-root-issuer condition met
```

#### Create the issuer certificate

```bash {"stage":"etc", "runtime":"bash", "label":"get the cluster ip"}
export CLUSTER_IP=$(minikube ip --profile tutorialtester)
```

```bash {"stage":"cert-manager", "HereTag":"EOF", "runtime":"bash", "label":"create issuer certificate"}
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: send-receive-issuer-cert
  namespace: send-receive-project
spec:
  isCA: true
  commonName: ArtemiseCloud send-receive issuer
  dnsNames:
    - ${CLUSTER_IP}.nip.io
  secretName: send-receive-issuer-cert-secret
  privateKey:
    algorithm: ECDSA
    size: 256
  issuerRef:
    name: send-receive-root-issuer
    kind: Issuer
EOF
```
```shell tutorial_tester
certificate.cert-manager.io/send-receive-issuer-cert created
```

```{"stage":"cert-manager"}
kubectl wait certificate send-receive-issuer-cert --for=condition=Ready --namespace=send-receive-project
```
```shell tutorial_tester
certificate.cert-manager.io/send-receive-issuer-cert condition met
```

#### Create the issuer

```bash {"stage":"cert-manager", "HereTag":"EOF", "runtime":"bash", "label":"create issuer"}
kubectl apply -f - <<EOF
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
```shell tutorial_tester
issuer.cert-manager.io/send-receive-issuer created
```

```{"stage":"cert-manager"}
kubectl wait issuer send-receive-issuer --for=condition=Ready --namespace=send-receive-project
```
```shell tutorial_tester
issuer.cert-manager.io/send-receive-issuer condition met
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

Store the output in the `/tmp/IssuerCA.pem` file:
```bash {"stage":"cert-manager", "runtime":"bash", "label":"dowload certificate"}
kubectl get secrets send-receive-issuer-cert-secret -o json | jq -r '.data."tls.crt"' | base64 -d > /tmp/IssuerCA.pem
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

Get minikube's ip

```bash {"stage":"deploy", "HereTag":"EOF", "runtime":"bash", "label":"deploy the broker"}
INGRESS_HOST='ing.$(ITEM_NAME).$(CR_NAME)-$(BROKER_ORDINAL).$(CR_NAMESPACE).$(INGRESS_DOMAIN)'
cat <<EOF > deploy.yml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: send-receive
  namespace: send-receive-project
spec:
  ingressDomain: ${CLUSTER_IP}.nip.io
  acceptors:
    - name: sslacceptor
      port: 62626
      sslEnabled: true
      expose: true
      exposeMode: ingress
      sslSecret: send-receive-sslacceptor-0-svc-ing-ptls
      ingressHost: ${INGRESS_HOST}
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
                - ing.sslacceptor.send-receive-0.send-receive-project.${CLUSTER_IP}.nip.io
              secretName: send-receive-sslacceptor-0-svc-ing-ptls
  brokerProperties:
    - addressConfigurations."APP.JOBS".routingTypes=ANYCAST
    - addressConfigurations."APP.JOBS".queueConfigs."APP.JOBS".routingType=ANYCAST
    - addressConfigurations."APP.COMMANDS".routingTypes=MULTICAST
EOF
kubectl apply -f deploy.yml
```
```shell tutorial_tester
activemqartemis.broker.amq.io/send-receive created
```

Wait for the Broker to be ready:

```{"stage":"deploy"}
kubectl wait ActiveMQArtemis send-receive --for=condition=Ready --namespace=send-receive-project --timeout=240s
```
```shell tutorial_tester
activemqartemis.broker.amq.io/send-receive condition met
```

Check that the ingress is available and has an IP address:

```{"stage":"deploy"}
kubectl get ingress --show-labels
```
```shell tutorial_tester
NAME                                 CLASS   HOSTS                                                                     ADDRESS   PORTS     AGE   LABELS
send-receive-sslacceptor-0-svc-ing   nginx   ing.sslacceptor.send-receive-0.send-receive-project.192.168.49.2.nip.io             80, 443   41s   ActiveMQArtemis=send-receive,application=send-receive-app,statefulset.kubernetes.io/pod-name=send-receive-ss-0
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

```bash {"stage":"test_setup", "rootdir":"$tmpdir.1", "runtime":"bash", "label":"download artemis"}
wget --quiet https://archive.apache.org/dist/activemq/activemq-artemis/2.36.0/apache-artemis-2.36.0-bin.tar.gz
tar -zxf apache-artemis-2.36.0-bin.tar.gz apache-artemis-2.36.0/
```

#### Figure out the broker endpoint

Recover the ingress url

```bash {"stage":"test_setup", "runtime":"bash", "label":"get the ingress host"}
export INGRESS_URL=$(kubectl get ingress send-receive-sslacceptor-0-svc-ing -o json | jq -r '.spec.rules[] | .host')
```

Craft the broker url for artemis

```bash {"stage":"test_setup", "runtime":"bash", "label":"compute the broker url"}
export BROKER_URL="tcp://${INGRESS_URL}:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false"
```

##### Test the connection

```bash {"stage":"test0", "rootdir":"$tmpdir.1/apache-artemis-2.36.0/bin/", "runtime":"bash", "label":"test connection"}
./artemis check queue --name TEST --produce 10 --browse 10 --consume 10 --url ${BROKER_URL} --verbose
```
```shell tutorial_tester
Executing org.apache.activemq.artemis.cli.commands.check.QueueCheck check queue --name TEST --produce 10 --browse 10 --consume 10 --url tcp://ing.sslacceptor.send-receive-0.send-receive-project.192.168.49.2.nip.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false --verbose 
Home::/tmp/1757791309/apache-artemis-2.36.0, Instance::null
Connection brokerURL = tcp://ing.sslacceptor.send-receive-0.send-receive-project.192.168.49.2.nip.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false
Running QueueCheck
Checking that a producer can send 10 messages to the queue TEST ... success
Checking that a consumer can browse 10 messages from the queue TEST ... success
Checking that a consumer can consume 10 messages from the queue TEST ... success
Checks run: 3, Failures: 0, Errors: 0, Skipped: 0, Time elapsed: 0.423 sec - QueueCheck
```

#### ANYCAST

For this use case, run first the producer, then the consumer.

```bash {"stage":"test1", "rootdir":"$tmpdir.1/apache-artemis-2.36.0/bin/", "runtime":"bash", "label":"anycast: produce 1000 messages"}
./artemis producer --destination queue://APP.JOBS --url ${BROKER_URL}
```
```shell tutorial_tester
Connection brokerURL = tcp://ing.sslacceptor.send-receive-0.send-receive-project.192.168.49.2.nip.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false
Producer ActiveMQQueue[APP.JOBS], thread=0 Started to calculate elapsed time ...

Producer ActiveMQQueue[APP.JOBS], thread=0 Produced: 1000 messages
Producer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in second : 9 s
Producer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in milli second : 9376 milli seconds
```

```bash {"stage":"test1", "rootdir":"$tmpdir.1/apache-artemis-2.36.0/bin/", "runtime":"bash", "label":"anycast: consume 1000 messages"}
./artemis consumer --destination queue://APP.JOBS --url ${BROKER_URL}
```
```shell tutorial_tester
Connection brokerURL = tcp://ing.sslacceptor.send-receive-0.send-receive-project.192.168.49.2.nip.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false
Consumer:: filter = null
Consumer ActiveMQQueue[APP.JOBS], thread=0 wait until 1000 messages are consumed
Received 1000
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumed: 1000 messages
Consumer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in second : 0 s
Consumer ActiveMQQueue[APP.JOBS], thread=0 Elapsed time in milli second : 120 milli seconds
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumed: 1000 messages
Consumer ActiveMQQueue[APP.JOBS], thread=0 Consumer thread finished
```

#### MULTICAST

For this use case, run first the consumer(s), then the producer.
[More details there](https://activemq.apache.org/components/artemis/documentation/2.0.0/address-model.html).

1. in `n` other terminal(s) connect `n` consumer(s):

```bash {"stage":"test2", "rootdir":"$tmpdir.1/apache-artemis-2.36.0/bin/", "parallel": true, "runtime":"bash", "env":["BROKER_URL"], "label":"multicast: consume 1000 messages"}
./artemis consumer --destination topic://APP.COMMANDS --url ${BROKER_URL}
```
```shell tutorial_tester
Connection brokerURL = tcp://ing.sslacceptor.send-receive-0.send-receive-project.192.168.49.2.nip.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false
Consumer:: filter = null
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 wait until 1000 messages are consumed
Received 1000
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumed: 1000 messages
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in second : 5 s
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in milli second : 5331 milli seconds
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumed: 1000 messages
Consumer ActiveMQTopic[APP.COMMANDS], thread=0 Consumer thread finished
```

2. connect the producer to start broadcasting messages.

```bash {"stage":"test2", "rootdir":"$tmpdir.1/apache-artemis-2.36.0/bin/", "parallel": true, "runtime":"bash", "env":["BROKER_URL"], "label":"multicast: produce 1000 messages"}
sleep 5s
./artemis producer --destination topic://APP.COMMANDS --url ${BROKER_URL}
```
```shell tutorial_tester
Connection brokerURL = tcp://ing.sslacceptor.send-receive-0.send-receive-project.192.168.49.2.nip.io:443?sslEnabled=true&trustStorePath=/tmp/IssuerCA.pem&trustStoreType=PEM&useTopologyForLoadBalancing=false
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Started to calculate elapsed time ...

Producer ActiveMQTopic[APP.COMMANDS], thread=0 Produced: 1000 messages
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in second : 0 s
Producer ActiveMQTopic[APP.COMMANDS], thread=0 Elapsed time in milli second : 550 milli seconds
```

### cleanup

To leave a pristine environment after executing this tutorial you can simply,
delete the minikube cluster and clean the `/etc/hosts` file.

```{"stage":"teardown", "requires":"init/minikube_start"}
minikube delete --profile tutorialtester
```
```shell tutorial_tester
* Deleting "tutorialtester" in docker ...
* Deleting container "tutorialtester" ...
* Removing /home/tlavocat/.minikube/machines/tutorialtester ...
* Removed all traces of the "tutorialtester" cluster.
```
