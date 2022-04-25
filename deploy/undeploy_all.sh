#!/bin/bash

echo "Undeploy everything..."

KUBE=oc

$KUBE delete -f ./crds
$KUBE delete -f ./service_account.yaml
$KUBE delete -f ./role.yaml
$KUBE delete -f ./role_binding.yaml
$KUBE delete -f ./cluster_role.yaml
$KUBE delete -f ./cluster_role_binding.yaml
$KUBE delete -f ./election_role.yaml
$KUBE delete -f ./election_role_binding.yaml
$KUBE delete -f ./operator_config.yaml
$KUBE delete -f ./operator.yaml
