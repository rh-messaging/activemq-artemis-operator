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

# write a single yaml.
# it takes one parameter:
# 1 array that contains total lines of the yaml
function writeFile() {
  array_name=$1[@]
  lines=("${!array_name}")

  case $resource_kind in

    CustomResourceDefinition)
      if [[ ${resource_name} =~ (activemqartemises) ]]; then
        file_path="$crdsdir/broker_activemqartemis_crd.yaml"
      elif [[ ${resource_name} =~ (activemqartemissecurities) ]]; then
        file_path="$crdsdir/broker_activemqartemissecurity_crd.yaml"
      elif [[ ${resource_name} =~ (activemqartemisaddresses) ]]; then
        file_path="$crdsdir/broker_activemqartemisaddress_crd.yaml"
      elif [[ ${resource_name} =~ (activemqartemisscaledowns) ]]; then
        file_path="$crdsdir/broker_activemqartemisscaledown_crd.yaml"
      else
        file_path="$crdsdir/${resource_name}.yaml"
      fi
      ;;

    Deployment)
      file_path="$destdir/operator.yaml"
      ;;

    ClusterRoleBinding)
      file_path="$destdir/cluster_role_binding.yaml"
      ;;

    ClusterRole)
      file_path="$destdir/cluster_role.yaml"
      ;;

    ServiceAccount)
      file_path="$destdir/service_account.yaml"
      ;;

    ConfigMap)
      file_path="$destdir/operator_config.yaml"
      ;;

    Role)
      file_path="$destdir/election_role.yaml"
      ;;

    RoleBinding)
      file_path="$destdir/election_role_binding.yaml"
      ;;

    Namespace)
      file_path="skip_this_file"
      ;;
    
    *)
      file_path="$destdir/${resource_kind}_${resource_name}.yaml"
      ;;
    esac

  if [[ ${file_path} != "skip_this_file" ]]; then
    rm -rf ${file_path}
    for value in "${file[@]}"
      do
        printf "%s\n" "${value}" >> ${file_path}
      done
  fi
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

