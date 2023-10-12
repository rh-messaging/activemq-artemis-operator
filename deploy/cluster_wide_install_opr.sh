#!/bin/bash

echo "Deploying cluster-wide operator"

read -p "Enter namespaces to watch (empty for all namespaces): " WATCH_NAMESPACE
if [ -z ${WATCH_NAMESPACE} ]; then
  WATCH_NAMESPACE="*"
fi

DEPLOY_PATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

if oc version; then
    KUBE_CLI=oc
else
    KUBE_CLI=kubectl
fi

$KUBE_CLI create -f $DEPLOY_PATH/crds
$KUBE_CLI create -f $DEPLOY_PATH/service_account.yaml
$KUBE_CLI create -f $DEPLOY_PATH/cluster_role.yaml
SERVICE_ACCOUNT_NS="$($KUBE_CLI get -f $DEPLOY_PATH/service_account.yaml -o jsonpath='{.metadata.namespace}')"
sed "s/namespace:.*/namespace: ${SERVICE_ACCOUNT_NS}/" $DEPLOY_PATH/cluster_role_binding.yaml | $KUBE_CLI apply -f -
$KUBE_CLI create -f $DEPLOY_PATH/election_role.yaml
$KUBE_CLI create -f $DEPLOY_PATH/election_role_binding.yaml
sed -e "/WATCH_NAMESPACE/,/- name/ { /WATCH_NAMESPACE/b; /valueFrom:/bx; /- name/b; d; :x s/valueFrom:/value: '${WATCH_NAMESPACE}'/}" $DEPLOY_PATH/operator.yaml | $KUBE_CLI apply -f -
