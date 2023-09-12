/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jolokia_client

import (
	"context"
	"strconv"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	ss "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	mgmt "github.com/artemiscloud/activemq-artemis-operator/pkg/utils/artemis"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type JkInfo struct {
	Artemis *mgmt.Artemis
	IP      string
	Ordinal string
}

func GetBrokers(resource types.NamespacedName, ssInfos []ss.StatefulSetInfo, client rtclient.Client) []*JkInfo {
	reqLogger := ctrl.Log.WithValues("Request.Namespace", resource.Namespace, "Request.Name", resource.Name)

	var artemisArray []*JkInfo = nil

	for n, info := range ssInfos {
		reqLogger.Info("Now retrieve ss", "order", n, "ssName", info.NamespacedName)
		statefulset, err := ss.RetrieveStatefulSet(info.NamespacedName.Name, info.NamespacedName, info.Labels, client)
		if nil != err {
			reqLogger.Error(err, "error retriving ss")
			reqLogger.Info("Statefulset: " + info.NamespacedName.Name + " not found")
		} else {
			reqLogger.Info("Statefulset: " + info.NamespacedName.Name + " found")
			pod := &corev1.Pod{}
			podNamespacedName := types.NamespacedName{
				Name:      statefulset.Name + "-0",
				Namespace: resource.Namespace,
			}

			// For each of the replicas
			var i int = 0
			var replicas int = int(*statefulset.Spec.Replicas)
			reqLogger.Info("finding pods in ss", "replicas", replicas)
			for i = 0; i < replicas; i++ {
				s := statefulset.Name + "-" + strconv.Itoa(i)
				podNamespacedName.Name = s
				reqLogger.Info("Trying finding pod " + s)
				if err = client.Get(context.TODO(), podNamespacedName, pod); err != nil {
					if errors.IsNotFound(err) {
						reqLogger.Error(err, "Pod IsNotFound", "Namespace", resource.Namespace, "Name", resource.Name)
					} else {
						reqLogger.Error(err, "Pod lookup error", "Namespace", resource.Namespace, "Name", resource.Name)
					}
				} else {
					reqLogger.Info("Pod found", "Namespace", resource.Namespace, "Name", resource.Name)
					containers := pod.Spec.Containers //get env from this
					jolokiaSecretName := resource.Name + "-jolokia-secret"

					jolokiaUser, jolokiaPassword, jolokiaProtocol := resolveJolokiaRequestParams(resource.Namespace, client, client.Scheme(), jolokiaSecretName, &containers, podNamespacedName, statefulset, info.Labels)

					reqLogger.Info("New Jolokia with ", "User: ", jolokiaUser, "Protocol: ", jolokiaProtocol, "broker ip", pod.Status.PodIP)
					artemis := mgmt.GetArtemis(pod.Status.PodIP, "8161", "amq-broker", jolokiaUser, jolokiaPassword, jolokiaProtocol)
					jkInfo := JkInfo{
						Artemis: artemis,
						IP:      pod.Status.PodIP,
						Ordinal: strconv.Itoa(i),
					}
					artemisArray = append(artemisArray, &jkInfo)
				}
			}
		}
	}

	reqLogger.Info("Finally we gathered some mgmt array", "size", len(artemisArray))
	return artemisArray
}

func resolveJolokiaRequestParams(namespace string,
	client rtclient.Client,
	scheme *runtime.Scheme,
	jolokiaSecretName string,
	containers *[]corev1.Container,
	podNamespacedName types.NamespacedName,
	statefulset *appsv1.StatefulSet,
	labels map[string]string) (string, string, string) {

	var jolokiaUser string
	var jolokiaPassword string
	var jolokiaProtocol string

	userDefined := false
	jolokiaUserFromSecret := secrets.GetValueFromSecret(namespace, jolokiaSecretName, "jolokiaUser", labels, client, scheme, nil)
	if jolokiaUserFromSecret != nil {
		userDefined = true
		jolokiaUser = *jolokiaUserFromSecret
	}
	if userDefined {
		jolokiaPasswordFromSecret := secrets.GetValueFromSecret(namespace, jolokiaSecretName, "jolokiaPassword", labels, client, scheme, nil)
		if jolokiaPasswordFromSecret != nil {
			jolokiaPassword = *jolokiaPasswordFromSecret
		}
	}
	if len(*containers) == 1 {
		envVars := (*containers)[0].Env
		for _, oneVar := range envVars {
			if !userDefined && oneVar.Name == "AMQ_USER" {
				jolokiaUser = getEnvVarValue(&oneVar, &podNamespacedName, statefulset, client, labels)
			}
			if !userDefined && oneVar.Name == "AMQ_PASSWORD" {
				jolokiaPassword = getEnvVarValue(&oneVar, &podNamespacedName, statefulset, client, labels)
			}
			if oneVar.Name == "AMQ_CONSOLE_ARGS" {
				jolokiaProtocol = getEnvVarValue(&oneVar, &podNamespacedName, statefulset, client, labels)
			}
			if jolokiaUser != "" && jolokiaPassword != "" && jolokiaProtocol != "" {
				break
			}
		}
	}

	if jolokiaProtocol == "" {
		jolokiaProtocol = "http"
	} else {
		jolokiaProtocol = "https"
	}

	return jolokiaUser, jolokiaPassword, jolokiaProtocol
}

func getEnvVarValue(envVar *corev1.EnvVar, namespace *types.NamespacedName, statefulset *appsv1.StatefulSet, client rtclient.Client, labels map[string]string) string {
	var result string
	if envVar.Value == "" {
		result = getEnvVarValueFromSecret(envVar.Name, envVar.ValueFrom, namespace, statefulset, client, labels)
	} else {
		result = envVar.Value
	}
	return result
}

func getEnvVarValueFromSecret(envName string, varSource *corev1.EnvVarSource, namespace *types.NamespacedName, statefulset *appsv1.StatefulSet, client rtclient.Client, labels map[string]string) string {

	reqLogger := ctrl.Log.WithValues("Namespace", namespace.Name, "StatefulSet", statefulset.Name)

	var result string = ""

	secretName := varSource.SecretKeyRef.LocalObjectReference.Name
	secretKey := varSource.SecretKeyRef.Key

	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: statefulset.Namespace,
	}
	// Attempt to retrieve the secret
	stringDataMap := map[string]string{
		envName: "",
	}
	theSecret := secrets.NewSecret(namespacedName, secretName, stringDataMap, labels)
	var err error = nil
	if err = resources.Retrieve(namespacedName, client, theSecret); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Secret IsNotFound.", "Secret Name", secretName, "Key", secretKey)
		}
	} else {
		elem, ok := theSecret.Data[envName]
		if ok {
			result = string(elem)
		}
	}
	return result
}
