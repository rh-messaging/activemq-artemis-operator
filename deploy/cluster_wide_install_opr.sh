#!/bin/bash

echo "Deploying cluster-wide operator, dont forget change WATCH_NAMESPACE to empty! and cluser-role-binding subjects namespace"
KUBE=oc

$KUBE create -f ./crds
$KUBE create -f ./service_account.yaml
$KUBE create -f ./cluster_role.yaml
$KUBE create -f ./cluster_role_binding.yaml
$KUBE create -f ./election_role.yaml
$KUBE create -f ./election_role_binding.yaml
$KUBE create -f ./operator_config.yaml
$KUBE create -f ./operator.yaml
