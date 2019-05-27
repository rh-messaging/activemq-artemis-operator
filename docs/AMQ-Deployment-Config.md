
## Deployment Configuration for ActiveMQ Artemis broker 
 
#### Deploying an ActiveMQ Artemis broker with SSL 

#### Procedure:

1. Deploy the ActiveMQ Artemis operator in a project namespace.

2. Create a service account to be used for the ActiveMQ Artemis broker:

   ```bash
    echo '{"kind": "ServiceAccount", "apiVersion": "v1", "metadata": {"name": "amq-service-account"}}' | oc create -f -
   ```

3. The ActiveMQ Artemis broker requires a keystore, and a truststore. Follow the link for creation instructions.

   [create broker keystore and trust store ](https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/deploying_amq_broker_on_openshift_container_platform/index#configuring-ssl_broker-ocp)

4. Add SSL configuration in the custom resource file:

e.g.

   ```yaml

          sslConfig:
            secretName: amq-app-secret
            trustStoreFilename: broker.ts
            trustStorePassword: changeit
            keystoreFilename: broker.ks
            keyStorePassword: changeit
    
 ```


 
#### Deploying an ActiveMQ Artemis broker with Persistent

 Add persistent flag true in the custom resource file:
 
e.g.

```yaml

          persistent: true
    
 ```

#### Deploying an ActiveMQ Artemis broker with Clustering

Add below configuration in custom resource file:
 
e.g.

```yaml
  clusterConfig:
    clusterUserName: clusteruser
    clusterPassword: clusterpass
 ```

## Trigger a ActiveMQ Artemis deployment

Use the console to `Create Broker` or create one manually as seen below. Ensure SSL configuration is correct in the
custom resource file.

```bash
$ oc create -f deploy/crs/broker_v1alpha1_activemqartemis_cr.yaml
```

## Clean up an ActiveMQ Artemis deployment

```bash
oc delete -f deploy/crs/broker_v1alpha1_activemqartemis_cr.yaml
```

