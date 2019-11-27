# Overview

At the moment these instructions have been tested against OpenShift 3.11,
other kubernetes or OpenShift environments may require minor adjustment.

One important note about operators in general is that to get the operator
installed requires cluster-admin level privileges. Once installed, a regular
user should be able to install ActiveMQ Artemis via the provided custom
resource.

# Quick Start

## Getting the code

To launch the operator you will need to clone the [activemq-artemis-operator](https://github.com/rh-messaging/activemq-artemis-operator)
and checkout the 0.10.0 tag as per

```$xslt
git clone https://github.com/rh-messaging/activemq-artemis-operator
git checkout 0.10.0
```

## Deploying the operator

In the activemq-artemis-operator/deploy directory you should see [operator.yaml](https://github.com/rh-messaging/activemq-artemis-operator/blob/0.10.0/deploy/operator.yaml)
within which you will want to update the [spec.containers.image](https://github.com/rh-messaging/activemq-artemis-operator/blob/0.10.0/deploy/operator.yaml#L18-L19)
with the correct location for the activemq-artemis-operator container image that you either pulled or [built](building.md).

As per the [operator-framework/operator-sdk](https://github.com/operator-framework/operator-sdk) Quick Start we first
prepare the project namespace to receive the operator:

```$xslt
# Setup Service Account
$ kubectl create -f deploy/service_account.yaml
# Setup RBAC
$ kubectl create -f deploy/role.yaml
$ kubectl create -f deploy/role_binding.yaml
# Setup the ActiveMQArtemis CRD
$ kubectl create -f deploy/crds/broker_v2alpha1_activemqartemis_crd.yaml
# Setup the ActiveMQArtemisAddress CRD
$ kubectl create -f deploy/crds/broker_v2alpha1_activemqartemisaddress_crd.yaml
```

Note that the CRDs should be installed before the operator starts, if not the operator will log messages informing
you to do so.

Now build the source according to the [building](building.md) instructions, tag, and push it into your project
namespace. Then update the deploy/operator.yaml to reference the built image and once done you can deploy the
built activemq-artemis-operator via:

```$xslt
$ kubectl create -f deploy/operator.yaml
```

At this point in your project namespace you should see the activemq-artemis-operator starting up and if you check the
logs you should see something like

```$xslt
Executing entrypoint
exec /activemq-artemis-operator/activemq-artemis-operator
{"level":"info","ts":1553619035.0707157,"logger":"cmd","msg":"Go Version: go1.11.5"}
{"level":"info","ts":1553619035.0708382,"logger":"cmd","msg":"Go OS/Arch: linux/amd64"}
{"level":"info","ts":1553619035.070858,"logger":"cmd","msg":"Version of operator-sdk: v0.5.0"}
{"level":"info","ts":1553619035.0715153,"logger":"leader","msg":"Trying to become the leader."}
{"level":"info","ts":1553619035.3516414,"logger":"leader","msg":"No pre-existing lock was found."}
{"level":"info","ts":1553619035.363638,"logger":"leader","msg":"Became the leader."}
{"level":"info","ts":1553619035.526576,"logger":"cmd","msg":"Registering Components."}
{"level":"info","ts":1553619035.5271027,"logger":"kubebuilder.controller","msg":"Starting EventSource","controller":"activemqartemis-controller","source":"kind source: /, Kind="}
{"level":"info","ts":1553619035.5276275,"logger":"kubebuilder.controller","msg":"Starting EventSource","controller":"activemqartemis-controller","source":"kind source: /, Kind="}
{"level":"info","ts":1553619035.527945,"logger":"kubebuilder.controller","msg":"Starting EventSource","controller":"activemqartemis-controller","source":"kind source: /, Kind="}
{"level":"info","ts":1553619035.5283449,"logger":"kubebuilder.controller","msg":"Starting EventSource","controller":"activemqartemisaddress-controller","source":"kind source: /, Kind="}
{"level":"info","ts":1553619035.5286813,"logger":"kubebuilder.controller","msg":"Starting EventSource","controller":"activemqartemisaddress-controller","source":"kind source: /, Kind="}
{"level":"info","ts":1553619035.729491,"logger":"metrics","msg":"Metrics Service object already exists","name":"activemq-artemis-operator"}
{"level":"info","ts":1553619035.7295585,"logger":"cmd","msg":"Starting the Cmd."}
{"level":"info","ts":1553619035.8302743,"logger":"kubebuilder.controller","msg":"Starting Controller","controller":"activemqartemisaddress-controller"}
{"level":"info","ts":1553619035.830541,"logger":"kubebuilder.controller","msg":"Starting Controller","controller":"activemqartemis-controller"}
{"level":"info","ts":1553619035.9306898,"logger":"kubebuilder.controller","msg":"Starting workers","controller":"activemqartemisaddress-controller","worker count":1}
{"level":"info","ts":1553619035.9311671,"logger":"kubebuilder.controller","msg":"Starting workers","controller":"activemqartemis-controller","worker count":1}
```

## Deploying the broker

Now that the operator is running and listening for changes related to our crd we can deploy our basic broker custom
resource instance for 'ex-aao' from [broker_v2alpha1_activemqartemis_cr.yaml](https://github.com/rh-messaging/activemq-artemis-operator/blob/0.10.0/deploy/crs/broker_v2alpha1_activemqartemis_cr.yaml)
which looks like

```$xslt
apiVersion: broker.amq.io/v2alpha1
kind: ActiveMQArtemis
metadata:
  name: ex-aao
spec:
  # Add fields here
  size: 4
  image: registry.redhat.io/amq7/amq-broker:7.5
```  

Note in particular the [spec.image:](https://github.com/rh-messaging/activemq-artemis-operator/blob/0.10.0/deploy/crs/broker_v2alpha1_activemqartemis_cr.yaml#L8)
which identifies the container image to use to launch the AMQ Broker. Ignore the size as its unused at the moment.

To deploy the broker we simply execute

```$xslt
kubectl create -f deploy/crs/broker_v2alpha1_activemqartemis_cr.yaml
```

at which point you should now see the statefulset 'ex-aao-ss' under Overview in the web console.

```$xslt
activemqartemis.broker.amq.io "ex-aao" created
```

If you expand the ex-aao-ss statefulset you should see that it has 1 pod running along with two services
under 'Networking'; hs which is short for headless service, and ping. The headless service provides access to all protocol
ports that the broker is listening on. The ping service is a service utilized by the brokers for discovery and allows them
to form a cluster properly within the OpenShift environment.

At the moment external access is limited in that the nodePort services, console-jolokia, and mux-protocol, acts as
load balancers meaning traffic sent to them gets round robined by default. To have at least basic functionality at
the moment the [session affinity](https://kubernetes.io/docs/concepts/services-networking/service/) is set to be
sticky to the client ip so that as you navigate through the console each click doesn't go to a separate broker.
This is an area that requires further development.

### headless-service

The headless service internally exposes all broker ports: 
- 1883: MQTT
- 5672: AMQP
- 8161: Web Console / Jolokia
- 61613: STOMP
- 61616: All protocols as above 

### ping

The ping service, used internally by the brokers for clustering themselves, internally exposes the 8888 port.

### ex-aao-console-jolokia-service

The broker hosts its own web console at port 8161 and this service provides external access to it via a 
[NodePort](https://kubernetes.io/docs/concepts/services-networking/service/#nodeport)
that provides direct access to the web console. Note that the actual port number chosen is, by default, in the range
of 30000-32767 so you will want to click in here to get the actual port number needed to access the console.

Once you have the port number using http://any.ocp.node.ip:consoleNodePortNumber should allow you to access the running
brokers web console.

### ex-aao-mux-protocol-service

The broker has a multiplexed protocol port at 61616 supporting all protocols; AMQP, CORE, HornetQ, MQTT, OpenWire and STOMP.
This broker port is exposed via a [NodePort](https://kubernetes.io/docs/concepts/services-networking/service/#nodeport)
as is the web console. 

To produce or consume messages to the broker from outside the [OCP SDN](https://docs.openshift.com/container-platform/3.11/admin_guide/sdn_troubleshooting.html)
you should be able to produce or consume messages to protocol://://any.ocp.node.ip:muxProtocolNodePortNumber.

## Scaling

The ex-aao-ss-0 pod should now also be visible via the OpenShift console via the Applications -> Pods
sub menu, or by drilling down through the statefulset. The number of running pods can be adjusted via the 'oc scale' 
command as per:

```$xslt
oc scale --replicas 0 statefulset ex-aao-ss
```

which would scale down the number of pods to zero. You can also scale the number to greater than one whereby the broker
pods will form a broker cluster with default settings: 

```$xslt
oc scale --replicas 2 statefulset ex-aao-ss
```

One important caveat here is that upon scaledown the persistent
volume claim remains associated with the stable name of the broker pod that no longer exists. While that particular broker
pod is down the data in its journal files remains safe, but will only become accessible again once that particular broker
pod ordinal is up.

### Accessing more than one broker externally

An OpenShift specific solution to this problem is to [enable wildcard routing](https://docs.openshift.com/container-platform/3.11/install_config/router/default_haproxy_router.html#using-wildcard-routes)
on the openshift router and then use a wildcarded route on the headless service to access each broker pod individually
by DNS name. An example of such a wild card enabled route that has been used during development is:

```$xslt
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  annotations:
  name: wildconsole-jolokia
  namespace: abo-2
spec:
  host: star.console-jolokia-abo-2.apps-ocp311.kieley.ca
  port:
    targetPort: console-jolokia
  to:
    kind: Service
    name: hs
    weight: 100
  wildcardPolicy: Subdomain
```

In particular the wildcardPolicy of Subdomain and the host: which has star as the prefix. As well note that both the
namespace and host in the example are specific to the environment in which it was tested.

When successfully deployedand viewed through the OpenShift web console the 'star' will be seen as '*', indicating
wildcard routing for everything '.console-jolokia-abo-2.apps-ocp311.kieley.ca'. Note also the specific target port
of 'console-jolokia' which is port 8161 in the headless service which is the port the web console and jolokia are
served from.

### Usage

```$xslt
[rkieley@i7t450s ~]$ curl  http://admin:admin@ex-aao-ss-0.console-jolokia-abo-2.apps-ocp311.kieley.ca/console/jolokia/read/org.apache.activemq.artemis:broker=\"amq-broker\"/NodeID | jq
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current 
                                 Dload  Upload   Total   Spent    Left  Speed                                                                                                                
100   191    0   191    0     0   4897      0 --:--:-- --:--:-- --:--:--  4897 
{                                                                                                   
  "request": {                                                                
    "mbean": "org.apache.activemq.artemis:broker=\"amq-broker\"",                                                                                                                                
    "attribute": "NodeID",                                                     
    "type": "read"                                                                                          
  },                                                                                                                              
  "value": "e0709987-4cc3-11e9-b53b-0a580a810099",                                               
  "timestamp": 1553533881,                                                                                             
  "status": 200                                                                                                     
}                                                                                                            
[rkieley@i7t450s ~]$ curl  http://admin:admin@ex-aao-ss-1.console-jolokia-abo-2.apps-ocp311.kieley.ca/console/jolokia/read/org.apache.activemq.artemis:broker=\"amq-broker\"/NodeID | jq
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current                              
                                 Dload  Upload   Total   Spent    Left  Speed                                          
100   191    0   191    0     0   5617      0 --:--:-- --:--:-- --:--:--  5617                                                                                                                                                                 
{                                                                               
  "request": {                                                                                                                   
    "mbean": "org.apache.activemq.artemis:broker=\"amq-broker\"",                                
    "attribute": "NodeID",                                        
    "type": "read"                                                                                  
  },                                                                                                        
  "value": "958a8ea9-4cc3-11e9-b153-0a580a820085",                                                                                                                                                
  "timestamp": 1553533884,                                                                                             
  "status": 200                                                                                                       
}
```

To access those DNS names properly via a browser such as Firefox you may need to set the proxy to be the
openshift-router IP and port if you receive a 503.

### Clustering

If broker pods are scaled to more than two then the broker pods form a broker
[cluster](https://activemq.apache.org/artemis/docs/2.6.0/clusters.html), meaning connect to each other and
redistribute messages 'ON_DEMAND'. With two or more broker pods running you can check the broker log of each
broker and you should see something similar to the following message regarding cluster formation:

broker 0

>
> 2019-03-22 16:59:46,406 INFO [org.apache.activemq.artemis.core.server] AMQ221027: Bridge ClusterConnectionBridge@6f13fb88 [name=$.artemis.internal.sf.my-cluster.e0709987-4cc3-11e9-b53b-0a580a810099, queue=QueueImpl[name=$.artemis.internal.sf.my-cluster.e0709987-4cc3-11e9-b53b-0a580a810099, postOffice=PostOfficeImpl [server=ActiveMQServerImpl::serverUUID=958a8ea9-4cc3-11e9-b153-0a580a820085], temp=false]@6478aa targetConnector=ServerLocatorImpl (identity=(Cluster-connection-bridge::ClusterConnectionBridge@6f13fb88 [name=$.artemis.internal.sf.my-cluster.e0709987-4cc3-11e9-b53b-0a580a810099, queue=QueueImpl[name=$.artemis.internal.sf.my-cluster.e0709987-4cc3-11e9-b53b-0a580a810099, postOffice=PostOfficeImpl [server=ActiveMQServerImpl::serverUUID=958a8ea9-4cc3-11e9-b153-0a580a820085], temp=false]@6478aa targetConnector=ServerLocatorImpl [initialConnectors=[TransportConfiguration(name=artemis, factory=org-apache-activemq-artemis-core-remoting-impl-netty-NettyConnectorFactory) ?port=61616&host=ex-aao-ss-1-hs-abo-2-svc-cluster-local], discoveryGroupConfiguration=null]]::ClusterConnectionImpl@1533123860[nodeUUID=958a8ea9-4cc3-11e9-b153-0a580a820085, connector=TransportConfiguration(name=artemis, factory=org-apache-activemq-artemis-core-remoting-impl-netty-NettyConnectorFactory) ?port=61616&host=ex-aao-ss-0-hs-abo-2-svc-cluster-local, address=, server=ActiveMQServerImpl::serverUUID=958a8ea9-4cc3-11e9-b153-0a580a820085])) [initialConnectors=[TransportConfiguration(name=artemis, factory=org-apache-activemq-artemis-core-remoting-impl-netty-NettyConnectorFactory) ?port=61616&host=ex-aao-ss-1-hs-abo-2-svc-cluster-local], discoveryGroupConfiguration=null]] is connected

broker 1
>
> 2019-03-22 16:59:46,668 INFO [org.apache.activemq.artemis.core.server] AMQ221027: Bridge ClusterConnectionBridge@dcfadf [name=$.artemis.internal.sf.my-cluster.958a8ea9-4cc3-11e9-b153-0a580a820085, queue=QueueImpl[name=$.artemis.internal.sf.my-cluster.958a8ea9-4cc3-11e9-b153-0a580a820085, postOffice=PostOfficeImpl [server=ActiveMQServerImpl::serverUUID=e0709987-4cc3-11e9-b53b-0a580a810099], temp=false]@268b2b2c targetConnector=ServerLocatorImpl (identity=(Cluster-connection-bridge::ClusterConnectionBridge@dcfadf [name=$.artemis.internal.sf.my-cluster.958a8ea9-4cc3-11e9-b153-0a580a820085, queue=QueueImpl[name=$.artemis.internal.sf.my-cluster.958a8ea9-4cc3-11e9-b153-0a580a820085, postOffice=PostOfficeImpl [server=ActiveMQServerImpl::serverUUID=e0709987-4cc3-11e9-b53b-0a580a810099], temp=false]@268b2b2c targetConnector=ServerLocatorImpl [initialConnectors=[TransportConfiguration(name=artemis, factory=org-apache-activemq-artemis-core-remoting-impl-netty-NettyConnectorFactory) ?port=61616&host=ex-aao-ss-0-hs-abo-2-svc-cluster-local], discoveryGroupConfiguration=null]]::ClusterConnectionImpl@104716441[nodeUUID=e0709987-4cc3-11e9-b53b-0a580a810099, connector=TransportConfiguration(name=artemis, factory=org-apache-activemq-artemis-core-remoting-impl-netty-NettyConnectorFactory) ?port=61616&host=ex-aao-ss-1-hs-abo-2-svc-cluster-local, address=, server=ActiveMQServerImpl::serverUUID=e0709987-4cc3-11e9-b53b-0a580a810099])) [initialConnectors=[TransportConfiguration(name=artemis, factory=org-apache-activemq-artemis-core-remoting-impl-netty-NettyConnectorFactory) ?port=61616&host=ex-aao-ss-0-hs-abo-2-svc-cluster-local], discoveryGroupConfiguration=null]] is connected

## Undeploying the broker

To undeploy the broker we simply execute

```$xslt
kubectl delete -f deploy/crs/broker_v2alpha1_activemqartemis_cr.yaml
```

at which point you should no longer see the statefulset 'ex-aao-ss' under Overview in the web console.
On the command line you should see:

```$xslt
activemqartemis.broker.amq.io "ex-aao" deleted
```

## Managing Queues

### Overview

Very basic, non-robust, functionality for adding and removing queues via custom resource definitions has been added. Of interest are two
additional yaml files, [broker_v2alpha1_activemqartemisaddress_crd.yaml](https://github.com/rh-messaging/activemq-artemis-operator/blob/master/deploy/crds/broker_v2alpha1_activemqartemisaddress_crd.yaml)
which provides the custom resource definition for an ActiveMQArtemisAddress and an example implementation of a custom
resource based on this crd, [broker_v2alpha1_activemqartemisaddress_cr.yaml](https://github.com/rh-messaging/activemq-artemis-operator/blob/master/deploy/crs/broker_v2alpha1_activemqartemisaddress_cr.yaml)

In the implemented custom resource you will note the following of interest:

```$xslt
spec:
  # Add fields here
  addressName: myAddress0
  queueName: myQueue0
  routingType: anycast
```

Note that for the moment in this initial implementation each of the three fields; addressName, queueName,
and routingType are required as per the [crd](https://github.com/rh-messaging/activemq-artemis-operator/blob/master/deploy/crds/broker_v2alpha1_activemqartemisaddress_crd.yaml#L35-L39).
This will possibly be relaxed in the future when the feature is more mature.

### Deploying the ActiveMQArtemisAddress crd

To deploy the crd itself, prior to starting the operator execute:

```$xslt
kubectl create -f deploy/crds/broker_v2alpha1_activemqartemisaddress_crd.yaml
```

### Creating a queue across the running broker cluster

To actually deploy an ActiveMQArtemisAddress successfully at this point requires the broker cluster to be deployed
via the operator FIRST. With the running broker cluster you can utilize the included custom resource example to
create an address on every RUNNING broker via:

```$xslt
kubectl create -f deploy/crs/broker_v2alpha1_activemqartemisaddress_cr.yaml
```

This will create an address 'myAddress0' with an 'anycast' routed queue named 'myQueue0' on every RUNNING broker. NOTE
that if at a later point you 'oc scale' the broker cluster up the newly added broker won't have this address added by
the operator. This requires further development.

### Deleting a queue across the running broker cluster

Deleting a queue deployed via an ActiveMQArtemisAddress custom resource is straightforward:

```$xslt
kubectl delete -f deploy/crs/broker_v2alpha1_activemqartemisaddress_cr.yaml
```

## Draining messages on scale down

A scaledown controller can be deployed with the operator that supports message draining on scale down.
The custom scale down resource definition is in `deploy/crds/broker_v2alpha1_activemqartemisscaledown_crd.yaml`.

When a broker pod is scaled down, the scale down controller detects the event and started a drainer pod.
The drainer pod will contact one of the live pods in the cluster and drain the messages over to it.
After the draining is complete it shuts down itself.

To demonstrate, following the steps below (assuming that minikube is used).

* Deploy related CRDs:

```$xslt
# Setup Service Account
$ kubectl create -f deploy/service_account.yaml
# Setup RBAC
$ kubectl create -f deploy/role.yaml
$ kubectl create -f deploy/role_binding.yaml
# Setup the ActiveMQArtemis CRD
$ kubectl create -f deploy/crds/broker_v2alpha1_activemqartemis_crd.yaml
# Setup the ActiveMQArtemisScaledown CRD
$ kubectl create -f deploy/crds/broker_v2alpha1_activemqartemisscaledown_crd.yaml
```

* Deploy the operator

```$xslt
$ kubectl create -f deploy/operator.yaml
```

* Deploy the clustered broker

```$xslt
kubectl create -f deploy/crds/broker_v2alpha1_activemqartemis_drainpod.yaml
```

* Once the broker pod is up, scale it to 2.

```$xslt
kubectl scale statefulset ex-aao-ss --replicas 2
```

* Now if you list the pods using `kubectl get pod` you will see 3 pods
are running. The output is like:

```$xslt
activemq-artemis-operator-8566d9bf58-9g25l   1/1     Running   0          3m38s
ex-aao-ss-0                 1/1     Running   0          112s
ex-aao-ss-1                 1/1     Running   0          8s
```

* Log into each pod and send some messages to each broker.

On pod ex-aao-ss-0 (suppose it's cluster IP is 172.17.0.6) run the following command

```$xslt
/opt/amq-broker/bin/artemis producer --url tcp://172.17.0.6:61616 --user admin --password admin
```
On pod ex-aao-ss-1 (suppose it's cluster IP is 172.17.0.7) run the following command

```$xslt
/opt/amq-broker/bin/artemis producer --url tcp://172.17.0.7:61616 --user admin --password admin
```

* Checking each broker has a queue named TEST and each has 1000 messages added.

* Now scale down the cluster from 2 to 1

```$xslt
kubectl scale statefulset ex-aao-ss --replicas 1
```

* Observe that the pod ex-aao-ss-1 is shutdown and soon after a new drainer pod
of the same name is started by the controller. This drainer pod will shutdown itself once the 
messages have been drained to the other pod (namely ex-aao-ss-0).

* Wait until the drainer pos is shutdown. Then check on the message count on TEST queue at 
pod ex-aao-ss-0 and you will see the the messages at the queue is 2000, which
means the drainer pod has done its work.










