#!/bin/bash

echo "Deploying operator to watch single namespace"

KUBE=oc

$KUBE create -f ./crds
$KUBE create -f ./service_account.yaml
$KUBE create -f ./role.yaml
$KUBE create -f ./role_binding.yaml
$KUBE create -f ./election_role.yaml
$KUBE create -f ./election_role_binding.yaml
$KUBE create -f ./operator_config.yaml
$KUBE create -f ./operator.yaml
