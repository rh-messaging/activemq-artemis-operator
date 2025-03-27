---
title: "Operator"
description: "Operator arkmq-org.io"
lead: "Operator arkmq-org.io"
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

## Overview of the arkmq-org Operator Custom Resource Definitions

In general, a Custom Resource Definition (CRD) is a schema of configuration items that you can modify for a custom Kubernetes 
object deployed with an Operator. By creating a corresponding Custom Resource (CR) instance, you can specify values for 
configuration items in the CRD. If you are an Operator developer, what you expose through a CRD essentially becomes the 
API for how a deployed object is configured and used. You can directly access the CRD through regular HTTP curl commands, 
because the CRD gets exposed automatically through Kubernetes.

The following CRD's are available for the Operator and can be found in the Operator Repository under *config/crd/bases/*

| CRD                 | Description                                                    |           Name            | Shortname  |
| :---                | :----:                                                         | :----:                    | :---:      |
| **Main broker CRD** | Create and configure a broker deployment                       |     activemqartemises     |     aa     |
| **Address CRD**     | Create addresses and queues for a broker deployment            | activemqartemisaddresses  |    aaa     |
| **Scaledown CRD**   | Creates a Scaledown Controller for message migration           | activemqartemisscaledowns |    aad     |
| **Security CRD**    | Configure the security and authentication method of the Broker | activemqartemissecurities |    aas     |

### Additional resources

To learn how to install the ActiveMQ Artemis Operator (and all included CRDs) using:

The Kubernetes CLI, see [Installing the Operator](#installing-the-operator-using-the-cli)

For complete configuration references to use when creating CR instances based on the main broker and address CRDs, see:

Broker Custom Resource configuration reference 

Address Custom Resource configuration reference

Sample Custom Reference can be found in the Operator Repository under the *deploy/crs* directory

### Operator deployment notes

This section describes some important considerations when planning an Operator-based deployment

Deploying the Custom Resource Definitions (CRDs) that accompany the ActiveMQ Artemis Operator requires cluster administrator 
privileges for your Kubernetes cluster. When the Operator is deployed, non-administrator users can create broker instances 
via corresponding Custom Resources (CRs). To enable regular users to deploy CRs, the cluster administrator must first assign 
roles and permissions to the CRDs. For more information, see Creating cluster roles for Custom Resource Definitions in the 
Kubernetes documentation.

When you update your cluster with the CRDs for the latest Operator version, this update affects all projects in the cluster. 
Any broker Pods deployed from previous versions of the Operator might become unable to update their status. To fix this issue 
for an affected project, you must also upgrade that project to use the latest version of the Operator.

You cannot create more than one broker deployment in a given Kubernetes project by deploying multiple broker Custom Resource (CR) 
instances. However, when you have created a broker deployment in a project, you can deploy multiple CR instances for addresses.

If you intend to deploy brokers with persistent storage and do not have container-native storage in your Kubernetes cluster, 
you need to manually provision Persistent Volumes (PVs) and ensure that these are available to be claimed by the Operator. 
For example, if you want to create a cluster of two brokers with persistent storage (that is, by setting persistenceEnabled=true 
in your CR), you need to have two persistent volumes available. By default, each broker instance requires storage of 2 GiB.

If you specify persistenceEnabled=false in your CR, the deployed brokers uses ephemeral storage. Ephemeral storage means 
that every time you restart the broker Pods, any existing data is lost.

For more information about provisioning persistent storage in Kubernetes, see [Understanding persistent storage](https://docs.openshift.com/container-platform/4.1/storage/understanding-persistent-storage.html)

## Installing the Operator using the CLI

This section shows how to use the Kubernetes command-line interface (CLI) to deploy the latest version of 
the Operator for arkmq-org in your Kubernetes project.

If you intend to deploy brokers with persistent storage and do not have container-native storage in your Kubernetes cluster, 
you need to manually provision Persistent Volumes (PVs) and ensure that they are available to be claimed by the Operator. 
For example, if you want to create a cluster of two brokers with persistent storage (that is, by setting persistenceEnabled=true 
in your Custom Resource), you need to have two PVs available. By default, each broker instance requires storage of 2 GiB.

If you specify persistenceEnabled=false in your Custom Resource, the deployed brokers uses ephemeral storage. Ephemeral 
storage means that that every time you restart the broker Pods, any existing data is lost.

## Configuring logging for the Operator

This section describes how to configure logging for the operator.

The operator image is using zap logger for logging. You can set the zap log level editing the container args defined in the operator deployment, i.e.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    control-plane: controller-manager
  name: activemq-artemis-controller-manager
  spec:
      containers:
      - args:
        - --zap-log-level=error
...
```

You can also edit the operator deployment using the Kubernetes dashboard or the Kubernetes command-line tool, for example

```shell script
$ sed 's/--zap-log-level=debug/--zap-log-level=error/' deploy/operator.yaml | kubectl apply -f -
```

However if you install the operator from OperatorHub you don't have control over the resources which are deployed by olm
framework. In that case if you want to change log options for the operator you need to edit the Subscription yaml
from the OperatorHub after the operator is installed. The Subscription spec has a config option that allows you to 
pass environment variables into operator container. To configure the log level add/edit the config option as shown
in below example:

```yaml
kind: Subscription
metadata:
  name: operator
spec:
  config:
    env:
    - name: ARGS
      value: "--zap-log-level=debug"
```

Note: The env var name must be **ARGS** and the value is **--zap-log-level={level}** where {level} must
be one of **debug**, **info** and **error**. Any other values will be ignored.

After editing the Subscription yaml as such, save it and the operator will restart with the given log level.

### Getting the Operator code

This procedure shows how to access and prepare the code you need to install the latest version of the Operator for arkmq-org .

Download the latest version of the Operator from https://github.com/arkmq-org/activemq-artemis-operator/tags

When the download has completed, move the archive to your chosen installation directory.
```shell script
$ mkdir ~/broker/operator
$ mv activemq-artemis-operator-0.18.1.zip ~/broker/operator
```

In your chosen installation directory, extract the contents of the archive. For example:

```shell script
$ cd ~/broker/operator
$ unzip activemq-artemis-operator-0.18.1.zip
```



### Preparing the kubernetes Environment

Switch to the directory that was created when you extracted the archive. For example:

```shell script
$ cd activemq-artemis-operator
```

Specify the project in which you want to install the Operator. You can create a new project or switch to an existing one.

Create a new namespace:

```shell script
$ kubectl create namespace  <project-name>
```

Or, switch to an existing namespace:

```shell script
$ kubectl config set-context $(kubectl config current-context) --namespace= <project-name>
```

Specify a service account to use with the Operator.

In the deploy directory of the Operator archive that you extracted, open the service_account.yaml file.

Ensure that the kind element is set to ServiceAccount.

In the metadata section, assign a custom name to the service account, or use the default name. The default name is activemq-artemis-operator.

Create the service account in your project.

```shell script
$ kubectl create -f deploy/service_account.yaml
```

Specify a role name for the Operator.

Open the role.yaml file. This file specifies the resources that the Operator can use and modify.

Ensure that the kind element is set to Role.

In the metadata section, assign a custom name to the role, or use the default name. The default name is activemq-artemis-operator.

Create the role in your project.

```shell script
$ kubectl create -f deploy/role.yaml
```

Specify a role binding for the Operator. The role binding binds the previously-created service account to the Operator role, 
based on the names you specified.

Open the role_binding.yaml file. Ensure that the name values for ServiceAccount and Role match those specified in the 
service_account.yaml and role.yaml files. For example:
```yaml
metadata:
    name: activemq-artemis-operator
subjects:
    kind: ServiceAccount
    name: activemq-artemis-operator
roleRef:
    kind: Role
    name: activemq-artemis-operator
````

Create the role binding in your project.

```shell script
$ kubectl create -f deploy/role_binding.yaml
```

In the procedure that follows, you deploy the Operator in your project.

### Deploy The Operator

Switch to the directory that was created when you previously extracted the Operator installation archive. For example:

```shell script
$ cd ~/broker/operator/amq-broker-operator--ocp-install-examples
```

Deploy the CRDs that are included with the Operator. You must install the CRDs in your Kubernetes cluster before deploying and starting the Operator.

Deploy the main broker CRD.

```shell script
$ kubectl create -f deploy/crds/broker_activemqartemis_crd.yaml
```

Deploy the address CRD.

```shell script
$ kubectl create -f deploy/crds/broker_activemqartemisaddress_crd.yaml
```

Deploy the scaledown controller CRD.

```shell script
$ kubectl create -f deploy/crds/broker_activemqartemisscaledown_crd.yaml
```

In the deploy directory of the Operator archive that you downloaded and extracted, open the operator.yaml file. Ensure that the value of the spec.containers.image property is set to the latest Operator image for ActiveMQ Artemis , as shown below.
```yaml
spec:
    template:
        spec:
            containers:
                image: quay.io/arkmq-org/activemq-artemis-operator:latest
```

Deploy the Operator.

```shell script
$ kubectl create -f deploy/operator.yaml
```

In your Kubernetes project, the Operator starts in a new Pod.

In the Kubernetes web console, the information on the Events tab of the Operator Pod confirms that Kubernetes has deployed 
the Operator image that you specified, has assigned a new container to a node in your Kubernetes cluster, and has started 
the new container.

In addition, if you click the Logs tab within the Pod, the output should include lines resembling the following:

```shell script
...
{"level":"info","ts":1553619035.8302743,"logger":"kubebuilder.controller","msg":"Starting Controller","controller":"activemqartemisaddress-controller"}
{"level":"info","ts":1553619035.830541,"logger":"kubebuilder.controller","msg":"Starting Controller","controller":"activemqartemis-controller"}
{"level":"info","ts":1553619035.9306898,"logger":"kubebuilder.controller","msg":"Starting workers","controller":"activemqartemisaddress-controller","worker count":1}
{"level":"info","ts":1553619035.9311671,"logger":"kubebuilder.controller","msg":"Starting workers","controller":"activemqartemis-controller","worker count":1}
```

The preceding output confirms that the newly-deployed Operator is communicating with Kubernetes, that the controllers for 
the broker and addressing are running, and that these controllers have started some workers.

It is recommended that you deploy only a single instance of the ActiveMQ Artemis Operator in a given Kubernetes project. 
Setting the replicas element of your Operator deployment to a value greater than 1, or deploying the Operator more than 
once in the same project is not recommended.

## Creating Operator-based broker deployments

### Deploying a basic broker instance

The following procedure shows how to use a Custom Resource (CR) instance to create a basic broker deployment.

    NOTE: You cannot create more than one broker deployment in a given Kubernetes project by deploying multiple Custom 
    Resource (CR) instances. However, when you have created a broker deployment in a project, you can deploy multiple CR 
    instances for addresses.

Prerequisites

1. You must have already installed the arkmq-org Operator.

2. To use the Kubernetes command-line interface (CLI) to install the ActiveMQ Artemis Operator, see [Installing the Operator](#installing-the-operator-using-the-cli).

When you have successfully installed the Operator, the Operator is running and listening for changes related to your CRs. 
This example procedure shows how to use a CR instance to deploy a basic broker in your project.

1. Start configuring a Custom Resource (CR) instance for the broker deployment.

Using the Kubernetes command-line interface switch to the namespace you are using for your project

```shell script
$ kubectl config set-context $(kubectl config current-context) --namespace= <project-name>
```

Open the sample CR file called broker_activemqartemis_v1beta1_cr.yaml that is included in the deploy/crs directory of the Operator
installation archive that you downloaded and extracted. For a basic broker deployment, the configuration might resemble 
that shown below. This configuration is the default content of the broker_activemqartemis_cr.yaml sample CR.

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: ex-aao
spec:
  deploymentPlan:
    image: placeholder
    size: 2
    ...
```

Observe that the sample CR uses a naming convention of **ex-aao**. This naming convention denotes that the CR is an example 
resource for the arkmq-org (based on the ActiveMQ Artemis project) Operator. When you deploy this sample CR, the resulting 
Stateful Set uses the name **ex-aao-ss**. Furthermore, broker Pods in the deployment are directly based on the Stateful Set name, 
for example, **ex-aao-ss-0**, **ex-aao-ss-1**, and so on. 

The size value specifies the number of brokers to deploy. The default value of 2 specifies a clustered broker deployment 
of two brokers. However, to deploy a single broker instance, change the value to 1.

The image value specifies the container image to use to launch the broker. It uses the **placeholder** key to identify that the operator should choose the latest supported broker image.

Save the CR file.

Switch to the namespace in which you are creating the broker deployment.

```shell script
$ kubectl config set-context $(kubectl config current-context) --namespace= <project-name>
```

Create the CR.

```shell script
$ kubectl create -f <path/to/custom-resource-instance>.yaml
```

In the Kubernetes web console you will see a new Stateful Set called **ex-aao-ss**.

Click the **ex-aao-ss** Stateful Set. You see that there is one Pod, corresponding to the single broker that you defined in the CR.

Within the Stateful Set, click the pod link and you should see the status of the pod as running. Click on the logs link 
in the top right corner to see the broker’s output.

To test that the broker is running normally, access a shell on the broker Pod to send some test messages.

Using the Kubernetes web console:

Click Pods on the left menu

Click the ex-aao-ss Pod.

In the top righthand corner, click the link to exec into pod

Using the Kubernetes command-line interface:

Get the Pod names and internal IP addresses for your project.

```shell script
$ kubectl get pods -o wide



NAME                          STATUS   IP
amq-broker-operator-54d996c   Running  10.129.2.14
ex-aao-ss-0                   Running  10.129.2.15
```

Access the shell for the broker Pod.

```shell script
$ kubectl exec --stdin --tty ex-aao-ss-0 -- /bin/bash
```

From the shell, use the artemis command to send some test messages. Specify the internal IP address of the broker Pod in the URL. For example:

```shell script
$ ./amq-broker/bin/artemis producer --url tcp://10.129.2.15:61616 --destination queue://demoQueue
```

The preceding command automatically creates a queue called demoQueue on the broker and sends a default quantity of 1000 messages to the queue.

You should see output that resembles the following:

```shell
Connection brokerURL = tcp://10.129.2.15:61616
Producer ActiveMQQueue[demoQueue], thread=0 Started to calculate elapsed time ...

Producer ActiveMQQueue[demoQueue], thread=0 Produced: 1000 messages
Producer ActiveMQQueue[demoQueue], thread=0 Elapsed time in second : 3 s
Producer ActiveMQQueue[demoQueue], thread=0 Elapsed time in milli second : 3492 milli seconds
```

For a complete configuration reference for the main broker Custom Resource (CR), see Broker Custom Resource configuration reference.

### Deploying clustered brokers
If there are two or more broker Pods running in your project, the Pods automatically form a broker cluster. A clustered configuration enables brokers to connect to each other and redistribute messages as needed, for load balancing.

The following procedure shows you how to deploy clustered brokers. By default, the brokers in this deployment use on demand load balancing, meaning that brokers will forward messages only to other brokers that have matching consumers.

Prerequisites

1. A basic broker instance is already deployed. See Deploying a basic broker instance.


Open the CR file that you used for your basic broker deployment.

For a clustered deployment, ensure that the value of deploymentPlan.size is 2 or greater. For example:
```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: ex-aao
spec:
    deploymentPlan:
        size: 4
        ...
```

Save the modified CR file.

Switch to projects namespace:

```shell script
$ kubectl config set-context $(kubectl config current-context) --namespace= <project-name>
```

At the command line, apply the change:

```shell script
$ kubectl apply -f <path/to/custom-resource-instance>.yaml
```

In the Kubernetes web console, additional broker Pods starts in your project, according to the number specified in your CR. 
By default, the brokers running in the project are clustered.

Open the Logs tab of each Pod. The logs show that Kubernetes has established a cluster connection bridge on each broker. 
Specifically, the log output includes a line like the following:

```shell script
targetConnector=ServerLocatorImpl (identity=(Cluster-connection-bridge::ClusterConnectionBridge@6f13fb88
```

### Applying Custom Resource changes to running broker deployments
The following are some important things to note about applying Custom Resource (CR) changes to running broker deployments:

1. You cannot dynamically update the **persistenceEnabled** attribute in your CR. To change this attribute, scale your cluster 
down to zero brokers. Delete the existing CR. Then, recreate and redeploy the CR with your changes, also specifying a deployment size.

2. The value of the **deploymentPlan.size** attribute in your CR overrides any change you make to size of your broker deployment 
via the **kubectl scale** command. For example, suppose you use **kubectl** scale to change the size of a deployment from three brokers to two, 
but the value of **deploymentPlan.size** in your CR is still 3. In this case, Kubernetes initially scales the deployment down to two brokers. 
However, when the scaledown operation is complete, the Operator restores the deployment to three brokers, as specified in the CR.

3. As described in [Deploying the Operator using the CLI](#deploying-the-operator-using-the-cli), if you create a broker deployment with persistent storage (that is, by setting persistenceEnabled=true in your CR), you might need to provision Persistent Volumes (PVs) for the arkmq-org Operator to claim for your broker Pods. If you scale down the size of your broker deployment, the Operator releases any PVs that it previously claimed for the broker Pods that are now shut down. However, if you remove your broker deployment by deleting your CR, arkmq-org Operator does not release Persistent Volume Claims (PVCs) for any broker Pods that are still in the deployment when you remove it. In addition, these unreleased PVs are unavailable to any new deployment. In this case, you need to manually release the volumes. For more information, see Releasing volumes in the Kubernetes documentation.

4. During an active scaling event, any further changes that you apply are queued by the Operator and executed only when scaling is complete. For example, suppose that you scale the size of your deployment down from four brokers to one. Then, while scaledown is taking place, you also change the values of the broker administrator user name and password. In this case, the Operator queues the user name and password changes until the deployment is running with one active broker.

5. all CR changes – apart from changing the size of your deployment, or changing the value of the expose attribute for acceptors, connectors, or the console – cause existing brokers to be restarted. If you have multiple brokers in your deployment, only one broker restarts at a time.


## Configuring Scheduling, Preemption and Eviction


### Liveness, Readiness and Startup Probes

The Liveness, Readiness Startup Probes are used by Kubernetes to detect when the Broker is started and to check it is still alive. 
For full documentation on this topic refer to the [Configure Liveness, Readiness and Startup Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/) 
chapter in the Kubernetes documentation.

#### The Liveness probe

The Liveness probe is configured in the Artemis CR something like:

```yaml
spec:
  deploymentPlan:
    size: 1
    image: placeholder
    livenessProbe:
      initialDelaySeconds: 5
      periodSeconds: 5
```

If no Liveness probe is configured or the handler itself is missing from a configured Liveness Probe then  the Operator 
will create a default TCP Probe that will check the liveness of the broker by connecting to the web Server port, the default config is:

```yaml
spec:
  deploymentPlan:
    livenessProbe:
      tcpSocket:
        port: 8161
      initialDelaySeconds: 30,
      timeoutSeconds:      5,
```

##### Using the Artemis Health Check

you can also use the Artemis Health Checker to check that the broker is running, something like:

```yaml
spec:
  deploymentPlan:
    livenessProbe:
      exec:
        command:
        - /home/jboss/amq-broker/bin/artemis 
        - check 
        - node 
        - --silent
        - --user
        - $AMQ_USER
        - --password
        - $AMQ_PASSWORD
      initialDelaySeconds: 30,
      timeoutSeconds:

```

By default this uses the URI of the acceptor configured with the name **artemis**. Since this is not configured by default
it will need configuring in the broker CR. Alternatively configure the acceptor used by passing the **--acceptor** 
argument on the artemis check command.


    NOTE: $AMQ_USER and $AMQ_PASSWORD are environment variables that are configured by the Operator
    
You can also check the status of the broker by producing and consuming a message:

```yaml
spec:
  deploymentPlan:
    livenessProbe:
      exec:
        command:
          - /home/jboss/amq-broker/bin/artemis
          - check
          - queue
          - --name
          - livenessqueue
          - --produce
          - "1"
          - --consume
          - "1"
          - --silent
          - --user
          - $AMQ_USER
          - --password
          - $AMQ_PASSWORD
      initialDelaySeconds: 30,
      timeoutSeconds:
```

The liveness queue must exist and be deployed the broker and be of type anycast with acceptable configuration, something like:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemisAddress
metadata:
  name: livenessqueue
  namespace: activemq-artemis-operator
spec:
  addressName: livenessqueue
  queueConfiguration:
    purgeOnNoConsumers: false
    maxConsumers: -1
    durable: true
    enabled: true
  queueName: livenessqueue
  routingType: anycast
```

    NOTE: The livenessqueue queue above should should only be used by the livness probe.

#### The Readiness Probe

As with the Liveness Probe the Readiness probe has a default probe if not configured. Unlike the readiness probe this is 
a script that is shipped in the Kubernetes Image, this can be found [here](https://github.com/arkmq-org/activemq-artemis-broker-kubernetes-image/blob/main/modules/activemq-artemis-launch/added/readinessProbe.sh)

The script will try to establish a tcp connection to each port configured in the broker.xml.  

#### The Startup Probe

The Startup Probe has no default probe if not configured. The Startup Probe is configured in the Artemis CR something like:

```yaml
spec:
  deploymentPlan:
    startupProbe:
      exec:
        command:
            - /bin/bash
            - '-c'
            - /opt/amq/bin/artemis
            - 'check'
            - 'node'
            - '--up'
            - '--url'
            - 'tcp://$HOSTNAME:61616'
      initialDelaySeconds: 5
      periodSeconds: 5
```

###  Tolerations

It is possible to configure tolerations on the deployed broker image . An example of a toleration would be something like:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
  deploymentPlan:
    size: 1
    tolerations:
      - key: "example-key"
        operator: "Exists"
        effect: "NoSchedule"
```

The use of Taints and Tolerations is outside the scope of this document, for full documentation see the [Kubernetes Documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)

### Affinity

It is possible to configure Affinity for the container pods, An example of this would be:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
  deploymentPlan:
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
            - matchExpressions:
                - key: disktype
                  operator: In
                  values:
                    - ssd
```

Affinity is outside the scope of this document, for full documentation see the [Kubernetes Documentation](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/)

### Node Selectors

It is possible to configure Node Selectors for the container pods, An example of this would be:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
  deploymentPlan:
    nodeSelector:
      location: "production"
```

Node Selectors are outside the scope of this document, for full documentation see the [Kubernetes Documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/)

### Priority Class

It is possible to configure PriorityClassName for the container pods, An example of this would be:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
  resourceTemplates:
    - selector:
        kind: StatefulSet
      patch:
        spec:
          template:
            spec:
              priorityClassName: high-priority
```

Pod Priority is outside the scope of this document, for full documentation see the [Kubernetes Documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/)

## Configuring Labels and Annotations

### Labels

Labels can be added to the pods by defining them like so:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
  deploymentPlan:
    labels:
      location: "production"
      partition: "customerA"
```

Labels are outside the scope of this document, for full documentation see the [Kubernetes Documentation](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/)

### Annotations

Annotations can be added to the pods by defining them like so:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
  deploymentPlan:
    annotations:
      sidecar.istio.io/inject: "true"
      promethes-prop: "somevalue"
```

### Custom Labels and Annotations on supporting resources; Services, Ingress, Secrets etc.

It is possible to configure  ResourceTemplate(s) for resources that are managed by the operator.
The TemplateType contains Labels and Annotations with an optional Selector. If the selector is empty
the template matches all resources. Othewise it can be used to restrict what is matched.
Note: the relevant variables supported by [`ingressHost`](https://github.com/arkmq-org/activemq-artemis-operator/issues/614) in the CRD can be referenced in keys and values for both labels and annotations.
In the following example, the annotation "someKey=someValue" is added to all Services

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
  resourceTemplates:
   - selector:
       kind: "Service"
     annotations:
       someKey: "somevalue"
```

## Custom Modifications via a Strategic Merge Patch

Occasionally it is necessary to make customisations to the spec of a managed resource. The `resourceTemplate.patch` attribute can be used to apply such customisations. The `patch` is appled by the operator using a [strategic merge](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/#notes-on-the-strategic-merge-patch) before submitting to the api server.
In the following example, a custom security context is added to the internal broker container of the managed StatefulSet by patching just the required attribute. Note: `name` is the mergeKey, it must match that of the managed container with the CR.Name prefix:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
spec:
  resourceTemplates:
  - selector:
      kind: "StatefulSet"
    patch:
      kind: "StatefulSet"
      spec:
        template:
          spec:
            containers:
            - name: "broker-container"
              securityContext:
                runAsNonRoot: true
```

## Setting  Environment Variables

As an advanced option, you can set environment variables for containers using a CR.
For example, to have the JDK output what it sees as 'the system', provide a relevant JDK_JAVA_OPTIONS key in the env attribute.

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
  deploymentPlan:
    size: 1
    image: placeholder
  env:
    - name: JDK_JAVA_OPTIONS
      value: -XshowSettings:system

```
---
**NOTE**

You are configuring an array of [envVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#envvar-v1-core) which is a very powerfull concept. Proceed with care, taking due respect to any environment the operator may set and depend on. For full documentation see the [Kubernetes Documentation](https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-container/)

There are a few well-known environment variables that are used by the operator internally to configure brokers, as shown below

* JAVA_ARGS_APPEND
* JAVA_OPTS
* DEBUG_ARGS

If you want to add some values to any of them, make sure you define it in **spec.env** using `value` field. If you use `valueFrom` for the env var you will get validation error condition in your CR's status.

If you really need `valueFrom` to define the values for the above env vars you can use a different var and then reference it in the internal var's `value` field.

For example:

```yaml
  env:
    - name: ENV_FROM_X
      valueFrom:
        secretKeyRef:
          key: JAVA_ARGS_APPEND
          name: amq-broker-dev-java-args-append
    - name: JAVA_ARGS_APPEND
      value: $(ENV_FROM_X)
```
Reference: [define-interdependent-environment-variables](https://kubernetes.io/docs/tasks/inject-data-application/define-interdependent-environment-variables/)

## Configuring brokerProperties

The CRD brokerProperties attribute allows the direct configuration of the Artemis internal configuration Bean of a broker via key value pairs. It is usefull to override or augment elements of the CR, or to configure broker features that are not exposed via CRD attributes. In cases where the init container is used to augment xml configuration, broker properties can provide an in CR alternative. As a general 'bag of configration' it is very powerful but it must be treated with due respect to all other sources of configuration. For details of what can be configured see the [Artemis configuraton documentation](https://activemq.apache.org/components/artemis/documentation/latest/configuration-index.html#broker-properties).

The format is an array of strings of the form `key=value` where the key identifies a (potentially nested) property of the configuration bean.
Note: the array of strings ends up in a java properties file, where the following list of characters are significant: (space)`' '`, (colon)`':'`, (equals)`'='`. If these need to be present in your keys or values, they need to be escaped with a leading back slash `'\'`.

The CR Status contains a Condition reflecting the application of the brokerProperties volume mount projection.
For advanced use cases, with a broker version >= 2.27.1, it is possible to use a `broker-N.` prefix to provide configuration to a specific instance(0-N) of your deployment plan.

For example, to provide explicit config for the amount of memory messages will consume in a broker, overriding the defaults from container and JVM heap limits, you could use:

```yaml
...
spec:
  ...
  brokerProperties:
    - globalMaxSize=512m
```

## Providing additional brokerProperties configuration from a secret
In order to provide a way to split or orgainse these properties by file or by secret, an extra mount can be used to provide a secret that will be treated as an additional source of broker properties configuration.

Using an **extraMounts** secret with a suffix "-bp" will cause the operator to auto mount the secret and make the broker aware of it's location. In addition the CR.Status.Condition[BrokerPropertiesApplied] will reflect the content of this secret.

Broker properties are applied in order, starting with the CR.brokerProperties and then with the "-bp" auto mounts in turn. Keys (or property files) from secrets are applied in alphabetical order and the supported formats are text and JSON.

To configure the global max size with text brokerProperties configuration from a "-bp" secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: config-bp
stringData:
  globalMem.properties: |
    globalMaxSize=512M
```

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: ex-aao
spec:
  deploymentPlan:
    extraMounts:
      secrets:
      - "config-bp"
```

To configure the global max size with JSON brokerProperties configuration from a "-bp" secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: config-bp
stringData:
  globalMem.json: |
    {"globalMaxSize":"512M"}
```

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: ex-aao
spec:
  deploymentPlan:
    extraMounts:
      secrets:
      - "config-bp"
```

To configure a specific broker instance in a "-bp" secret, use `broker-N` as the prefix for a key in the secret data. For example:

Create two -bp secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: config-1-bp
stringData:
  journal1.properties: |
    journalFileSize=12345
  broker-0.globalMem.properties: |
    globalMaxSize=512M
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: config-2-bp
stringData:
  journal2.properties: |
    journalMinFiles=3
  broker-1.globalMem.properties: |
    globalMaxSize=12M
```

and add the above secrets to extraMounts in the CR:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: ex-aao
spec:
  deploymentPlan:
    size: 2
    extraMounts:
      secrets:
      - "config-1-bp"
      - "config-2-bp"
```
When the CR is deployed the broker in pod 0 broker will get `globalMaxSize=512M` and pod 1 broker will get `globalMaxSize=12M`. While both will get properties from `journal1.properties` of secret **config-1-bp** and `journal2.properties` from secret **config-2-bp**.

## Replace ActiveMQArtemisAddress and ActiveMQArtemisSecurity CRDs with broker properties
The ActiveMQArtemisAddress and ActiveMQArtemisSecurity CRDs are deprecated in favour of the configuration via broker properties. It is possible to replace the use of the activemqartemisaddresses CRD and much of the activemqartemissecurities CRD with configuration via broker properties.

## Configuring Logging for Brokers

By default the operator deploys a broker with a default logging configuration that comes with the [Artemis container image]
(https://github.com/arkmq-org/activemq-artemis-broker-kubernetes-image). Broker logs its messages to console only.

Users can change the broker logging configuration by providing their own in a configmap or secret. The name of the configmap
or secret must have the suffix **-logging-config**. There must a key **logging.properties** and the value must of the full content of the logging configuration. (The broker is using slf4j with
log4j2 binding so the content should be log4j2's configuration in Java's properties file format).

Then you need to give the name of the configmap or secret in the broker custom resource via **extraMounts**. For example

`for configmap`
```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
spec:
  deploymentPlan:
    size: 1
    image: placeholder
    extraMounts:
      configMaps:
      - "my-logging-config"
```
`for secret`
```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
spec:
  deploymentPlan:
    size: 1
    image: placeholder
    extraMounts:
      secrets:
      - "my-logging-config"
```

## Configuring JAAS for Brokers

An entire JAAS configuration file (login.config) can be supplied via a secret with a `-jaas-config` postfix in the spec.deploymentPlan.extraMounts.secrets field. This file will be referenced from
the jaas config system property (`java.security.auth.login.config`) and it will override anything configured via artemis create or via the ArtemisSecurityCR. For full details of how to configure JAAS for the broker refer to the [JAAS Security manager documentation](https://activemq.apache.org/components/artemis/documentation/latest/security.html#JAAS_Security_Manager).
Note: care must be taken to respect the any configured admin user such that the operator can still access the jolokia endpoint of the broker. The simplest way to do that is to reference the existing PropertiesLoginModule configuration files in your login.config.
For example, here we have two instances of the PropertiesLoginModule, one that references the the default credentials from `/home/jboss/amq-broker/etc` and one that has user suplied values from the secret. `reload=true` will ensure that the properties are reloaded if the secret changes. The `login.config` key in your secret called `<...>-jaas-config`, would have the following as the value:

```
		// a full login.config with the default activemq realm
		activemq {

				// ensure the operator can connect to the broker by referencing the existing properties config
				org.apache.activemq.artemis.spi.core.security.jaas.PropertiesLoginModule sufficient
					org.apache.activemq.jaas.properties.user="artemis-users.properties"
					org.apache.activemq.jaas.properties.role="artemis-roles.properties"
					baseDir="/home/jboss/amq-broker/etc";

				// a custom LoginModule that will reload from this secret
				org.apache.activemq.artemis.spi.core.security.jaas.PropertiesLoginModule sufficient
					reload=true
					org.apache.activemq.jaas.properties.user="users.properties"
					org.apache.activemq.jaas.properties.role="roles.properties";

				// add any other supported LoginModule here
		};
```

There would be corresponding keys for users.properties and roles.properties, they are picked up from the same mount point as your login.config by default.

With the possiblity of configuring arbritary jaas login modules directly, the ArtemisSecurityCR ActiveMQArtemisSecuritySpec.LoginModules and ActiveMQArtemisSecuritySpec.SecurityDomains fields are deprecated.

## restricted mode (experimental)
The CR supports a boolean restricted attribute. For single pod broker deployments this provides an empty broker that is configured through brokerProperties. The broker is secured with PKI, there are no passwords. Cert manager can be used to create the necessary PKI secrets.  The end result is a minimal broker deployment; an embedded broker with an mtls endpoint for the jolokia jvm agent and RBAC that allows just the operator to check the broker status. There is no init container, no jetty and no xml.

## operator PKI
In order for the operator to be able to use mtls to connect to the broker operand it needs a client certificate and a trust bundle listing the trusted CAs. The user needs to provide these two secrets in the operator namespace; cert manager can be used to create and populate both. If CRs use the restricted flag, these secrets are a prerequisit.
The default operator cert secret name is `activemq-artemis-manager-cert` and the default operator trust bundle secret name is `activemq-artemis-manager-ca`. 
If either of these secrets need to be named differently, an enviroment variable can provide the alternative name using key ACTIVEMQ_ARTEMIS_MANAGER_CERT_SECRET_NAME or ACTIVEMQ_ARTEMIS_MANAGER_CA_SECRET_NAME.

## Locking down a broker deployment

Often when verificiation is complete it is desirable to lock down the broker images and prevent auto upgrades, which will result in a roll out of images and a restart of your broker.
The key enabler here is the **image** and **initImage** fields. When a fully qualified SHA uri is provided, the operator will only deploy that exact version.
Note: both the image and initImage fields must be set because they have an implicit dependency on each other.

The second enabler is the **version** field. The version field can restrict the matching versions selected by the operator using a ``major<.minor><.patch>`` format. When the Version field is empty, the operator will choose the latest version. When a major version is specified, only minor or patch version of that major will be chosen. An exact match can be configuired as **2.28.0**.
The operator supports a level of indirection when resolving versions, there are env vars that map version to image uris. If these change, via an operator redeployment, then locking down via a version may not be sufficient. In that case, the image and initImage fields will be necessary.

The operator will validate the a CR specifies both image and initImage or a Version. It will also validate that a speficied version matches the internal list of supported versions.
The CR Status sub resource will contain feedback via the Valid Condition if validation fails.

## Disabling reconcile with the `arkmq.org/block-reconcile` annotation

In cases where a rollout of the stateful set is necessitated via a new feature or bug fix but not immediately desirable, potentially because of the necessary broker restart, it is possible to block the reconcile of a CR. Applying the `arkmq.org/block-reconcile` boolean annotation to a CR will indicate that the operator should not reconcile the CR. The CR status will reflect the blocked state via an additional `ReconcileBlocked` Condition. Once the annotation is removed or set to false on the CR, reconcile will resume.


## Enable broker's metrics plugin

The ActiveMQ Artemis Broker comes with a metrics plugin to expose metrics data. The metrics data can be collected by tools such as Prometheus and visualized by tools such as Grafana.
By default, the metrics plugin is disabled.
To instruct the Operator to enable metrics for each broker Pod in a deployment, you must set the value of the `deploymentPlan.enableMetricsPlugin` property to true in the Custom Resource (CR) instance used to create the deployment.
In addition, you need to expose the console, for example

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: artemis-with-metrics
spec:
  console:
    expose: true
  deploymentPlan:
    enableMetricsPlugin: true
```

### Enable JVM metrics

JVM memory metrics are enabled by default. Use the `spec.brokerProperties` field to enable JVM GC and threads metrics, for further details see the following example:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: artemis-with-metrics
spec:
  console:
    expose: true
  deploymentPlan:
    enableMetricsPlugin: true
  brokerProperties:
    - "metricsConfiguration.jvmGc=true"
    - "metricsConfiguration.jvmMemory=true"
    - "metricsConfiguration.jvmThread=true"
```

### Monitor broker metrics by using Prometheus

The operator will expose a port named **console-jolokia** for the Prometheus to monitor. The following
is a sample Prometheus ServiceMonitor resource

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: artemis-with-metrics-monitor
  labels:
    team: prometheus
spec:
  selector:
    matchLabels:
      application: artemis-with-metrics-app
  endpoints:
  - port: console-jolokia
```
For a complete example please refer to this [arkmq-org example](https://github.com/arkmq-org/arkmq-org-examples/tree/main/operator/prometheus).

## Enabling Operator Metrics

The operator exposes a port called **http-metrics** for Prometheus to monitor.
The following is a sample Prometheus PodMonitor resource

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: activemq-artemis-controller-manager
spec:
  selector:
    matchLabels:
      control-plane: controller-manager
  podMetricsEndpoints:
  - port: http-metrics
```

## Configuring PodDisruptionBudget for broker deployment

The ActiveMQArtemis custom resource offers a PodDisruptionBudget option
for the broker pods deployed by the operator. When it is specified the operator
will deploy a PodDisruptionBudget for the broker deployment.

For example

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
spec:
  deploymentPlan:
    size: 2
    image: placeholder
    podDisruptionBudget:
      minAvailable: 1
```

When deploying the above custom resource the operator will create a PodDisruptionBudget
object with the **minAvailable** set to 1. The operator also sets the proper selector
so that the PodDisruptionBudget matches the broker statefulset.

## Configuring TopologySpreadConstraints for broker deployment

The ActiveMQArtemis custom resource offers a TopologySpreadConstraints option
for the broker pods deployed by the operator. When it is specified the operator
will deploy a TopologySpreadConstraints for the broker deployment.

For example

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
  deploymentPlan:
    topologySpreadConstraints:
    - maxSkew: 2
      topologyKey: topology.kubernetes.io/zone
      whenUnsatisfiable: DoNotSchedule
      labelSelector:
        matchExpressions:
        - key: app
          operator: In
          values:
          - test-app
```

When deploying the above custom resource the operator will spread matching pods among the given topology

### Container SecurityContext

The ActiveMQArtemis custom resource offers a container level SecurityContext option for the broker that holds security configuration that will be applied to the containers.

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: broker
  namespace: activemq-artemis-operator
spec:
  deploymentPlan:
    containerSecurityContext:
      runAsNonRoot: true
```

## Configuring Jolokia Access

The operator uses jolokia endpoints to get broker status and also create queue/address resources using the address CRs.

To gain access to jolokia the operator need to have proper credentials (username/password).

By default, the operator retrieves the username and password from the broker container's environment variables `AMQ_USER` and `AMQ_PASSWORD`. These environment variables are populated with values from the secret **[broker CR name]-credentials-secret**, using the `AMQ_USER` and `AMQ_PASSWORD` keys.

If you specify adminUser and adminPassword in the broker CR, these values will be written to the auto-generated secret **[broker CR name]-credentials-secret**, using the `AMQ_USER` and `AMQ_PASSWORD` keys.

Alternatively, you can provide your own secret named **[broker CR name]-credentials-secret**, which must contain four entries: `AMQ_USER` / `AMQ_PASSWORD` for administrative access, and `AMQ_CLUSTER_USER` / `AMQ_CLUSTER_PASSWORD` for intra-cluster communication.

```yaml
apiVersion: v1
metadata:
  name: amq-credentials-secret
  namespace: default
kind: Secret
type: Opaque
stringData:
  AMQ_USER: admin
  AMQ_PASSWORD: adminPassword
  AMQ_CLUSTER_USER: cluster
  AMQ_CLUSTER_PASSWORD: clusterPassword
```

However when you use security CRs, jass login module configs, or init container to configure security login modules,
The above adminUser and adminPassword may be overridden and jolokia client in the operator won't be able to get the correct credentials to connect to the broker. In that case the user should provide a secret called **[broker cr name]-jolokia-secret**, in which you put 2 entries for username and password for jolokia credential to use. The 2 entries should have keys named **jolokiaUser** and **jolokiaPassword** respectively, the value for **jolokiaUser** is the user name and the value for **jolokiaPassword** is the password.

For example when you have a broker cr named **amq** like this:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: amq
  namespace: default
spec:
  requireLogin: true
  deploymentPlan:
    size: 1
```

And you use init container to configure security to have a username **alice** with password **password1** for jolokia access. To enable operator to use client to have access jolokia, create a secret named **amq-jolokia-secret** in the same namespace, like this:

```yaml
apiVersion: v1
metadata:
  name: amq-jolokia-secret
  namespace: default
kind: Secret
type: Opaque
stringData:
  jolokiaUser: alice
  jolokiaPassword: password1
```

## Configuring Additional Volumes to the Broker

### Attaching extra volumes shared by all broker pods

When you have some existing volumes to be used by brokers, you can use **spec.deploymentPlan.extraVolumes** and **spec.deploymentPlan.extraVolumeMounts** to configure a broker custom resource to mount those volumes. For example if you have a PersistentVolumeClaim type volume called **my-pvc** and it has 1G capacity with some useful data in it:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 1Gi
```
You can configure a broker CR to use it:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: artemis-broker
spec:
  deploymentPlan:
    size: 2
    extraVolumes:
      - name: mydata
        persistentVolumeClaim:
          claimName: my-pvc
    extraVolumeMounts:
      - name: mydata
        mountPath: /opt/mydata
```
When deploying the above CR, the PVC volume will be mounted to path **/opt/mydata** in the broker container of both broker pods. The **extraVolumeMounts** is optional. If not specified a default mountPath is given based on the type of the volume, following the pattern:

/amq/extra/volumes/<volume.name>

For example if you configure to attach a PersistentVolumeClaim type volume called `mydata`, the default mount path is **/amq/extra/volumes/mydata**.

### Attaching extra persistent volume claims to each broker

The operator also supports configuration for each of the brokers of a custom resource to have a separate persistent volume. To do this you need configure the CR using **spec.extraVolumeClaimTemplates** in your CR. For example:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: artemis-broker
spec:
  deploymentPlan:
    extraVolumeClaimTemplates:
    - metadata:
        name: mydata
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 10Mi
    size: 2
    extraVolumeMounts:
      - name: mydata
        mountPath: /opt/mydata
```
The **extraVolumeClaimTemplates** is a list of PVC specs. The key is the **pvc name base** of the PVC. The value is of type [persistentVolumeClaimSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#persistentvolumeclaimspec-v1-core)

When deploying the above CR, the operator will append the external PVC to the statefulset's PersistentVolumeClaimTemplate field. When the statesulset rolls out the pods it will mount matching PVCs to each pod.

Note for each pod the PVC's name must follow the pattern `<volumeName>-<statefulset-name>-<ordinal>`.
For the above CR the matching PVC names are **mydata-artemis-broker-ss-0** for pod0 and **mydata-artemis-broker-ss-1** for pod1 respectively. You can configure an optional VolumeMount for each PVC under **extraVolumeMounts**. If not specified the default mount path is **/opt/<volumeName>/data**.

For complete configruation options please take a look at the api definitions of [broker CRD](../../api/v1beta1/activemqartemis_types.go).

## Using cert-manager and trust-manager configure brokers

Note: this feature currently is experimental. Feedback is welcomed.

[cert-manager](https://cert-manager.io/) adds certificates and certificate issuers as resource types in Kubernetes clusters, and simplifies the process of obtaining, renewing and using those certificates.

The operator provides options in the custom resource that utilizes cert-manager x509 certificates to configure SSL/TLS transports for brokers. It also works with [trust-manager](https://github.com/cert-manager/trust-manager) to distribute trust CA bundles.

Before configuring a broker you need to have the certificates and bundles ready. The bundle target secret key must end with `.pem`. In the following example a self-signed isser is used as a root CA.

Step 1 - create the root self-signed issuer

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: root-issuer
spec:
  selfSigned: {}
```

Step 2 - create the root Certificate

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: root-ca
  namespace: cert-manager
spec:
  isCA: true
  commonName: "arkmq-org.io.root"
  secretName: root-ca-secret
  subject:
    organizations:
    - "www.arkmq-org.io"
  issuerRef:
    name: root-issuer
    kind: ClusterIssuer
```

Step 3 - create a ca issuer that is used to issue broker certificates signed by the root CA

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: broker-cert-issuer
spec:
  ca:
    secretName: root-ca-secret
```

Step 4 - create a broker certificate signed by the root CA

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: server-cert
spec:
  isCA: false
  commonName: "arkmq-org.io"
  dnsNames:
    - "artemis-broker-ss-0"
    - "artemis-broker-ss-0.artemis-broker-hdls-svc.default.svc.cluster.local"
  secretName: server-cert-secret
  subject:
    organizations:
    - "www.arkmq-org.io"
  issuerRef:
    name: broker-cert-issuer
    kind: ClusterIssuer
```

Step 5 - create the ca bundle from the root CA using trust-manager and setting a target secret key that ends with `.pem`

```yaml
apiVersion: trust.cert-manager.io/v1alpha1
kind: Bundle
metadata:
  name: ca-bundle
spec:
  sources:
  - useDefaultCAs: false
  - secret:
      name: "root-ca-secret"
      key: "tls.crt"
  target:
    secret:
      key: "trust-bundle.pem"
```

### Configuring SSL/TLS for management console

Once you have the certificate and ca bundle ready you can configure the management console of the broker to used it:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: artemis-broker
spec:
  console:
    expose: true
    sslEnabled: true
    sslSecret: server-cert-secret
    trustSecret: ca-bundle
  deploymentPlan:
    size: 1
```

The above broker cr configures a broker that has a SSL/TLS secured management console whose keystore and truststore are generated from certificate stored in secret `server-cert-secret`.

### Configuring SSL/TLS for acceptors and connectors

With the certificate ready you can configure an acceptor and/or connector of the broker to use it:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: artemis-broker
spec:
  acceptors:
    - name: new-acceptor
      protocols: all
      port: 62666
      sslEnabled: true
      needClientAuth: true
      expose: true
      sslSecret: server-cert-secret
      trustSecret: ca-bundle
  deploymentPlan:
    size: 1
```

The above broker cr configures a broker that has a SSL/TLS secured acceptor called `new-acceptor` whose keystore and truststore are generated from secret `server-cert-secret` that is from the certificate resource.

You can configure a connector with ssl parameters from a certificate in like manner, for example the following yaml configures a connector called `new-connector` with the certificated above mentioned:

```yaml
apiVersion: broker.amq.io/v1beta1
kind: ActiveMQArtemis
metadata:
  name: artemis-broker
spec:
  connectors:
    - name: new-connector
      host: artemis-broker-ss-0
      port: 62666
      sslEnabled: true
      expose: true
      sslSecret: server-cert-secret
      trustSecret: ca-bundle
  deploymentPlan:
    size: 1
```

For details on how to use cert-manager to manage your certificates please refer to its [documentation](https://cert-manager.io/docs/).
