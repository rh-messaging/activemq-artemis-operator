#!/bin/bash

echo "Deploying cluster-wide operator, dont forget change WATCH_NAMESPACE to empty! and cluser-role-binding subjects namespace"

read -p "Please confirm that you have changed WATCH_NAMESPACE and role-binding to proper values [Y/N]: "
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]
then
  exit 1
fi

KUBE=oc

$KUBE create -f ./crds
$KUBE create -f ./service_account.yaml
$KUBE create -f ./cluster_role.yaml
$KUBE create -f ./cluster_role_binding.yaml
$KUBE create -f ./election_role.yaml
$KUBE create -f ./election_role_binding.yaml
$KUBE create -f ./operator_config.yaml
$KUBE create -f ./operator.yaml
