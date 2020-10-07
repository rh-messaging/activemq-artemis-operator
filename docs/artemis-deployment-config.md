
## Deployment Configuration for AMQ Broker broker 
 
#### Deploying an AMQ Broker broker with Persistent

 Add persistent flag true in the custom resource file:
 
e.g.

```yaml
          persistenceEnabled: true
    
 ```

## Trigger a AMQ Broker deployment

Use the console to `Create Broker` or create one manually as seen below. Ensure SSL configuration is correct in the
custom resource file.

```bash
$ oc create -f deploy/crs/broker_activemqartemis_cr.yaml
```

## Clean up an AMQ Broker deployment

```bash
oc delete -f deploy/crs/broker_activemqartemis_cr.yaml
```

