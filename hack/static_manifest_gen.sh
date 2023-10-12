#!/bin/bash

# service accounts, role/rolebinding, operator resources
mkdir -p $1
# crds
mkdir -p $1/crds

destdir=$1
crdsdir=$1/crds
file=()
resource_kind=""
resource_name=""

OPERATOR_NAMESPACE=${2}
echo "OPERATOR_NAMESPACE:${OPERATOR_NAMESPACE}"

# write a single yaml.
# it takes one parameter:
# 1 array that contains total lines of the yaml
function writeFile() {
  array_name=$1[@]
  lines=("${!array_name}")

  case $resource_kind in

    CustomResourceDefinition)
      if [[ ${resource_name} =~ (activemqartemises) ]]; then
        createFile "$crdsdir/broker_activemqartemis_crd.yaml"
      elif [[ ${resource_name} =~ (activemqartemissecurities) ]]; then
        createFile "$crdsdir/broker_activemqartemissecurity_crd.yaml"
      elif [[ ${resource_name} =~ (activemqartemisaddresses) ]]; then
        createFile "$crdsdir/broker_activemqartemisaddress_crd.yaml"
      elif [[ ${resource_name} =~ (activemqartemisscaledowns) ]]; then
        createFile "$crdsdir/broker_activemqartemisscaledown_crd.yaml"
      else
        createFile "$crdsdir/${resource_name}.yaml"
      fi
      ;;

    Deployment)
      createFile "$destdir/operator.yaml"
      ;;

    Role)
      if [[ ${resource_name} =~ (operator) ]]; then
        createFile "$destdir/role.yaml"
        createFile "$destdir/cluster_role.yaml"
        sed -i 's/kind: Role/kind: ClusterRole/' \
          "$destdir/cluster_role.yaml"
      elif [[ ${resource_name} =~ (leader-election) ]]; then
        createFile "$destdir/election_role.yaml"
      else
        createFile "$crdsdir/${resource_name}.yaml"
      fi
      ;;

    RoleBinding)
      if [[ ${resource_name} =~ (operator) ]]; then
        createFile "$destdir/role_binding.yaml"
        createFile "$destdir/cluster_role_binding.yaml"
        sed -i -e 's/kind: Role/kind: ClusterRole/' \
          -e 's/kind: RoleBinding/kind: ClusterRoleBinding/' \
          "$destdir/cluster_role_binding.yaml"
        echo "  namespace: ${OPERATOR_NAMESPACE}" >> \
          "$destdir/cluster_role_binding.yaml"
      elif [[ ${resource_name} =~ (leader-election) ]]; then
        createFile "$destdir/election_role_binding.yaml"
      else
        createFile "$crdsdir/${resource_name}.yaml"
      fi
      ;;

    ServiceAccount)
      createFile "$destdir/service_account.yaml"
      ;;

    Namespace)
      echo "Skipping ${resource_kind}:${resource_name}"
      ;;
    
    *)
      createFile "$destdir/${resource_kind}_${resource_name}.yaml"
      ;;
    esac
}

function createFile() {
  echo "Writing $1"
  rm -rf $1
  for value in "${file[@]}"; do
    printf "%s\n" "${value}" >> $1
  done
}

function beginFile() {
  if [[ ${#file[@]} > 0 ]]
  then
    # it gathered a single file now
    writeFile file
  fi
  file=()
}

function appendFile() {
  file+=("$1")
}

IFS=''
while read -r line
do
  if [[ $line =~ ^---$ ]]
  then
    beginFile
    resource_kind=""
    resource_name=""
    meta_start="false"
  else
    to_append="true"

    if [[ $resource_kind == "" && $line =~ ^(kind: ) ]]
    then
      resource_kind=$(echo $line | cut -b 7-)
    fi

    if [[ $line =~ ^(metadata:) ]]
    then
      meta_start="true"
    elif [[ ! $line =~ ^("  ") ]]; then
      meta_start="false"
    elif [[ ${meta_start} == "true" ]]; then
      if [[ $line =~ ^(  name: ) ]]; then
        resource_name=$(echo $line | cut -b 9-)
      elif [[ $line =~ ^(  namespace: ) ]]; then
        to_append="false"
      fi
    fi

    if [[ $resource_kind == "RoleBinding" ]]; then
      if [[ $line =~ ^(subjects:) ]]; then
        rb_subjects="true"
      elif [[ ${rb_subjects} == "true" ]]; then
        if [[ $line =~ ^(  namespace: ) ]]; then
          to_append="false"
        fi
      fi
    fi

    if [[ $to_append == "true" ]]; then
      appendFile "$line"
    fi
  fi
done
# check the last one
beginFile

