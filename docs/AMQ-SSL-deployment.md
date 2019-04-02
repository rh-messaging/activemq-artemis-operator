

## Deploying a AMQ broker with SSL

### Procedure:


1. Use the OLM to deploy operator in a project

2. Create a service account to be used for the AMQ Broker  

   ```bash
    echo '{"kind": "ServiceAccount", "apiVersion": "v1", "metadata": {"name": "amq-service-account"}}' | oc create -f -
   ```
3. Add the view role to the service account

    ```bash
     oc policy add-role-to-user view system:serviceaccount:<namespace>:amq-service-account
    ```

4. AMQ Broker requires a broker keystore, a client keystore, and a client truststore, follow the link to creat.

   [create broker keystore and trust store ](https://access.redhat.com/documentation/en-us/red_hat_amq/7.2/html-single/deploying_amq_broker_on_openshift_container_platform/index#configuring-ssl_broker-ocp)

5. Add SSL configuration in CR file 

e.g.

   ```yaml

        sslEnabled: true
          sslConfig:
            secretName: amq-app-secret
            trustStoreFilename: broker.ts
            trustStorePassword: changeit
            keystoreFilename: broker.ks
            keyStorePassword: changeit
    
 ```


### Trigger a ActiveMQ Artemis deployment

use the console to `Create Broker` or create one manually as seen below, make sure added or add SSL config in CR file.

```bash
$ oc create -f deploy/crds/broker_v1alpha1_activemqartemis_cr.yaml
```

### Clean up a ActiveMQ Artemis deployment

```bash
oc delete -f deploy/crds/broker_v1alpha1_activemqartemis_cr.yaml
```

