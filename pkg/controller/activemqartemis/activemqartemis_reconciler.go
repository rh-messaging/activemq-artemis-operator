package activemqartemis

import (
	"fmt"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v1alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/secrets"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha1"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/selectors"

	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/environments"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/volumes"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
)

const (
	statefulSetSizeUpdated          = 1 << 0
	statefulSetClusterConfigUpdated = 1 << 1
	statefulSetSSLConfigUpdated     = 1 << 2
	statefulSetImageUpdated         = 1 << 3
	statefulSetPersistentUpdated    = 1 << 4
	statefulSetAioUpdated           = 1 << 5
	statefulSetCommonConfigUpdated  = 1 << 6
	statefulSetRequireLoginUpdated  = 1 << 7
	statefulSetRoleUpdated          = 1 << 8
)

type ActiveMQArtemisReconciler struct {
	statefulSetUpdates uint32
}

type ActiveMQArtemisIReconciler interface {
	Process(customResource *brokerv2alpha1.ActiveMQArtemis, client client.Client, currentStatefulSet *appsv1.StatefulSet) uint32
	ProcessDeploymentPlan(customResource *brokerv2alpha1.ActiveMQArtemis, client client.Client, currentStatefulSet *appsv1.StatefulSet) uint32
	ProcessAcceptors(customResource *brokerv2alpha1.ActiveMQArtemis, client client.Client, currentStatefulSet *appsv1.StatefulSet)
	ProcessConnectors(customResource *brokerv2alpha1.ActiveMQArtemis, client client.Client, currentStatefulSet *appsv1.StatefulSet)
}

func (reconciler *ActiveMQArtemisReconciler) Process(customResource *brokerv2alpha1.ActiveMQArtemis, client client.Client, currentStatefulSet *appsv1.StatefulSet) uint32 {

	statefulSetUpdates := reconciler.ProcessDeploymentPlan(customResource, client, currentStatefulSet)
	reconciler.ProcessAcceptors(customResource, client, currentStatefulSet)
	reconciler.ProcessConnectors(customResource, client, currentStatefulSet)

	return statefulSetUpdates
}

func (reconciler *ActiveMQArtemisReconciler) SyncMessageMigration(customResource *brokerv2alpha1.ActiveMQArtemis, r *ReconcileActiveMQArtemis) {

	var err error = nil
	var retrieveError error = nil

	namespacedName := types.NamespacedName{
		Name:      customResource.Name,
		Namespace: customResource.Namespace,
	}

	scaledown := &v1alpha1.ActiveMQArtemisScaledown{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ActiveMQArtemisScaledown",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:    selectors.LabelBuilder.Labels(),
			Name:      customResource.Name,
			Namespace: customResource.Namespace,
		},
		Spec: v1alpha1.ActiveMQArtemisScaledownSpec{
			MasterURL:  "",
			Kubeconfig: "",
			Namespace:  customResource.Namespace,
			LocalOnly:  true,
		},
		Status: v1alpha1.ActiveMQArtemisScaledownStatus{},
	}

	if customResource.Spec.DeploymentPlan.MessageMigration {
		if err = resources.Retrieve(customResource, namespacedName, r.client, scaledown); err != nil {
			// err means not found so create
			if retrieveError = resources.Create(customResource, r.client, r.scheme, scaledown); retrieveError == nil {
			}
		}
	} else {
		if err = resources.Retrieve(customResource, namespacedName, r.client, scaledown); err == nil {
			// err means not found so create
			if retrieveError = resources.Delete(customResource, r.client, scaledown); retrieveError == nil {
			}
		}
	}
}

func (reconciler *ActiveMQArtemisReconciler) ProcessDeploymentPlan(customResource *brokerv2alpha1.ActiveMQArtemis, client client.Client, currentStatefulSet *appsv1.StatefulSet) uint32 {

	deploymentPlan := &customResource.Spec.DeploymentPlan

	// Ensure the StatefulSet size is the same as the spec
	if *currentStatefulSet.Spec.Replicas != deploymentPlan.Size {
		currentStatefulSet.Spec.Replicas = &deploymentPlan.Size
		reconciler.statefulSetUpdates |= statefulSetSizeUpdated
	}

	if clusterConfigSyncCausedUpdateOn(customResource, client, currentStatefulSet) {
		reconciler.statefulSetUpdates |= statefulSetClusterConfigUpdated
	}

	if sslConfigSyncCausedUpdateOn(deploymentPlan, currentStatefulSet) {
		reconciler.statefulSetUpdates |= statefulSetSSLConfigUpdated
	}

	if imageSyncCausedUpdateOn(deploymentPlan, currentStatefulSet) {
		reconciler.statefulSetUpdates |= statefulSetImageUpdated
	}

	if aioSyncCausedUpdateOn(deploymentPlan, currentStatefulSet) {
		reconciler.statefulSetUpdates |= statefulSetAioUpdated
	}

	if persistentSyncCausedUpdateOn(deploymentPlan, currentStatefulSet) {
		reconciler.statefulSetUpdates |= statefulSetPersistentUpdated
	}

	if commonConfigSyncCausedUpdateOn(customResource, client, currentStatefulSet) {
		reconciler.statefulSetUpdates |= statefulSetCommonConfigUpdated
	}

	if updatedEnvVar := environments.BoolSyncCausedUpdateOn(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_REQUIRE_LOGIN", deploymentPlan.RequireLogin); updatedEnvVar != nil {
		environments.Update(currentStatefulSet.Spec.Template.Spec.Containers, updatedEnvVar)
		reconciler.statefulSetUpdates |= statefulSetRequireLoginUpdated
	}

	if updatedEnvVar := environments.StringSyncCausedUpdateOn(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_ROLE", deploymentPlan.Role); updatedEnvVar != nil {
		environments.Update(currentStatefulSet.Spec.Template.Spec.Containers, updatedEnvVar)
		reconciler.statefulSetUpdates |= statefulSetRoleUpdated
	}

	return reconciler.statefulSetUpdates
}

func (reconciler *ActiveMQArtemisReconciler) ProcessAcceptors(customResource *brokerv2alpha1.ActiveMQArtemis, client client.Client, currentStatefulSet *appsv1.StatefulSet) {

	ensureCOREOn61616Exists := customResource.Spec.DeploymentPlan.MessageMigration || customResource.Spec.DeploymentPlan.Clustered
	acceptorEntry := ""
	defaultArgs := "tcpSendBufferSize=1048576;tcpReceiveBufferSize=1048576;useEpoll=true;amqpCredits=1000;amqpMinCredits=300"

	var portIncrement int32 = 10
	var currentPortIncrement int32 = 0
	var port61616InUse bool = false
	for _, acceptor := range customResource.Spec.Acceptors {
		if 0 == acceptor.Port {
			acceptor.Port = 61626 + currentPortIncrement
			currentPortIncrement += portIncrement
		}
		if "" == acceptor.Protocols ||
			"all" == strings.ToLower(acceptor.Protocols) {
			acceptor.Protocols = "AMQP,CORE,HORNETQ,MQTT,OPENWIRE,STOMP"
		}
		acceptorEntry = acceptorEntry + "<acceptor name=\"" + acceptor.Name + "\">"
		acceptorEntry = acceptorEntry + "tcp:" + "\\/\\/" + "ACCEPTOR_IP:"
		acceptorEntry = acceptorEntry + fmt.Sprintf("%d", acceptor.Port)
		acceptorEntry = acceptorEntry + "?protocols=" + strings.ToUpper(acceptor.Protocols)
		// TODO: Evaluate more dynamic messageMigration
		if 61616 == acceptor.Port {
			port61616InUse = true
		}
		if ensureCOREOn61616Exists &&
			(61616 == acceptor.Port) &&
			!strings.Contains(acceptor.Protocols, "CORE") {
			acceptorEntry = acceptorEntry + ",CORE"
		}
		if acceptor.SSLEnabled {
			secretName := customResource.Name + "-" + acceptor.Name + "-secret"
			if "" != acceptor.SSLSecret {
				secretName = acceptor.SSLSecret
			}
			acceptorEntry = acceptorEntry + ";" + generateSSLArguments(customResource, client, secretName)
		}
		acceptorEntry = acceptorEntry + ";" + defaultArgs
		// TODO: SSL
		acceptorEntry = acceptorEntry + "<\\/acceptor>"
	}
	// TODO: Evaluate more dynamic messageMigration
	if ensureCOREOn61616Exists && !port61616InUse {
		acceptorEntry = acceptorEntry + "<acceptor name=\"" + "scaleDown" + "\">"
		acceptorEntry = acceptorEntry + "tcp:" + "\\/\\/" + "ACCEPTOR_IP:"
		acceptorEntry = acceptorEntry + fmt.Sprintf("%d", 61616)
		acceptorEntry = acceptorEntry + "?protocols=" + "CORE"
		acceptorEntry = acceptorEntry + ";" + defaultArgs
		// TODO: SSL
		acceptorEntry = acceptorEntry + "<\\/acceptor>"
	}

	if amqAcceptorsEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_ACCEPTORS"); nil != amqAcceptorsEnvVar {
		if 0 == strings.Compare(amqAcceptorsEnvVar.Value, acceptorEntry) {
			return
		}
	}

	environments.Delete(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_ACCEPTORS")
	if "" != acceptorEntry {
		acceptorsEnvVar := &corev1.EnvVar{
			Name:      "AMQ_ACCEPTORS",
			Value:     acceptorEntry,
			ValueFrom: nil,
		}
		environments.Create(currentStatefulSet.Spec.Template.Spec.Containers, acceptorsEnvVar)
	}
}

func generateSSLArguments(customResource *brokerv2alpha1.ActiveMQArtemis, client client.Client, secretName string) string {

	sslArguments := "sslEnabled=true"
	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: customResource.Namespace,
	}
	stringDataMap := map[string]string{}
	userPasswordSecret := secrets.NewSecret(customResource, secretName, stringDataMap)

	keyStorePassword := "password"
	keyStorePath := "\\/etc\\/" + secretName + "-volume\\/broker.ks"
	trustStorePassword := "password"
	trustStorePath := "\\/etc\\/" + secretName + "-volume\\/client.ts"
	if err := resources.Retrieve(customResource, namespacedName, client, userPasswordSecret); err == nil {
		if "" != string(userPasswordSecret.Data["keyStorePassword"]) {
			keyStorePassword = string(userPasswordSecret.Data["keyStorePassword"])
		}
		if "" != string(userPasswordSecret.Data["keyStorePath"]) {
			keyStorePath = string(userPasswordSecret.Data["keyStorePath"])
		}
		if "" != string(userPasswordSecret.Data["trustStorePassword"]) {
			trustStorePassword = string(userPasswordSecret.Data["trustStorePassword"])
		}
		if "" != string(userPasswordSecret.Data["trustStorePath"]) {
			trustStorePath = string(userPasswordSecret.Data["trustStorePath"])
		}
	}
	sslArguments = sslArguments + ";" + "keyStorePath=" + keyStorePath
	sslArguments = sslArguments + ";" + "keyStorePassword=" + keyStorePassword
	sslArguments = sslArguments + ";" + "trustStorePath=" + trustStorePath
	sslArguments = sslArguments + ";" + "trustStorePassword=" + trustStorePassword

	return sslArguments
}

func (reconciler *ActiveMQArtemisReconciler) ProcessConnectors(customResource *brokerv2alpha1.ActiveMQArtemis, client client.Client, currentStatefulSet *appsv1.StatefulSet) {

	connectorEntry := ""
	connectors := customResource.Spec.Connectors

	for _, connector := range connectors {
		if connector.Type == "" {
			connector.Type = "tcp"
		}
		connectorEntry = connectorEntry + "<connector name=\"" + connector.Name + "\">"
		connectorEntry = connectorEntry + strings.ToLower(connector.Type) + ":\\/\\/" + strings.ToLower(connector.Host) + ":"
		connectorEntry = connectorEntry + fmt.Sprintf("%d", connector.Port)
		// TODO: SSL
		if connector.SSLEnabled {
			secretName := customResource.Name + "-" + connector.Name + "-secret"
			if "" != connector.SSLSecret {
				secretName = connector.SSLSecret
			}
			connectorEntry = connectorEntry + ";" + generateSSLArguments(customResource, client, secretName)
		}
		connectorEntry = connectorEntry + "<\\/connector>"
	}

	if amqConnectorsEnvVar := environments.Retrieve(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_CONNECTORS"); amqConnectorsEnvVar != nil {
		if 0 == strings.Compare(amqConnectorsEnvVar.Value, connectorEntry) {
			return
		}
	}

	environments.Delete(currentStatefulSet.Spec.Template.Spec.Containers, "AMQ_CONNECTORS")
	if "" != connectorEntry {
		connectorsEnvVar := &corev1.EnvVar{
			Name:      "AMQ_CONNECTORS",
			Value:     connectorEntry,
			ValueFrom: nil,
		}
		environments.Create(currentStatefulSet.Spec.Template.Spec.Containers, connectorsEnvVar)
	}
}

// https://stackoverflow.com/questions/37334119/how-to-delete-an-element-from-a-slice-in-golang
func remove(s []corev1.EnvVar, i int) []corev1.EnvVar {
	s[i] = s[len(s)-1]
	// We do not need to put s[i] at the end, as it will be discarded anyway
	return s[:len(s)-1]
}

func clusterConfigSyncCausedUpdateOn(customResource *brokerv2alpha1.ActiveMQArtemis, client client.Client, currentStatefulSet *appsv1.StatefulSet) bool {

	foundClustered := false
	foundClusterUser := false
	foundClusterPassword := false

	clusteredNeedsUpdate := false
	clusterUserNeedsUpdate := false
	clusterPasswordNeedsUpdate := false

	statefulSetUpdated := false
	deploymentPlan := customResource.Spec.DeploymentPlan
	secretName := "amq-credentials-secret"
	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: currentStatefulSet.Namespace,
	}
	stringDataMap := map[string]string{}
	secret := secrets.NewSecret(customResource, secretName, stringDataMap)
	err := resources.Retrieve(customResource, namespacedName, client, secret)

	clusterUserEnvVarSource := &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secretName,
			},
			Key:      "clusterUser",
			Optional: nil,
		},
	}

	clusterPasswordEnvVarSource := &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secretName,
			},
			Key:      "clusterPassword",
			Optional: nil,
		},
	}

	// TODO: Remove yuck
	// ensure password and username are valid if can't via openapi validation?
	envVarArray := []corev1.EnvVar{}
	// Find the existing values
	for _, v := range currentStatefulSet.Spec.Template.Spec.Containers[0].Env {
		if v.Name == "AMQ_CLUSTERED" {
			foundClustered = true
			boolValue, _ := strconv.ParseBool(v.Value)
			if boolValue != deploymentPlan.Clustered {
				clusteredNeedsUpdate = true
			}
		}
		if v.Name == "AMQ_CLUSTER_USER" {
			foundClusterUser = true
			if err == nil {
				secretClusterUser := string(secret.Data[v.ValueFrom.SecretKeyRef.Key])
				if "" != secretClusterUser &&
					"" != deploymentPlan.ClusterUser {
					if 0 != strings.Compare(secretClusterUser, deploymentPlan.ClusterUser) {
						clusterUserNeedsUpdate = true
					}
				}
			}
		}
		if v.Name == "AMQ_CLUSTER_PASSWORD" {
			foundClusterPassword = true
			if err == nil {
				secretClusterPassword := string(secret.Data[v.ValueFrom.SecretKeyRef.Key])
				if "" != secretClusterPassword &&
					"" != deploymentPlan.ClusterPassword {
					if 0 != strings.Compare(secretClusterPassword, deploymentPlan.ClusterPassword) {
						clusterPasswordNeedsUpdate = true
					}
				}
			}
		}
	}

	if !foundClustered || clusteredNeedsUpdate {
		newClusteredValue := corev1.EnvVar{
			"AMQ_CLUSTERED",
			strconv.FormatBool(deploymentPlan.Clustered),
			nil,
		}
		envVarArray = append(envVarArray, newClusteredValue)
		statefulSetUpdated = true
	}

	if !foundClusterUser || clusterUserNeedsUpdate {
		newClusteredValue := corev1.EnvVar{
			"AMQ_CLUSTER_USER",
			"",
			clusterUserEnvVarSource,
		}
		envVarArray = append(envVarArray, newClusteredValue)
		statefulSetUpdated = true
	}

	if !foundClusterPassword || clusterPasswordNeedsUpdate {
		newClusteredValue := corev1.EnvVar{
			"AMQ_CLUSTER_PASSWORD",
			"",
			clusterPasswordEnvVarSource, //nil,
		}
		envVarArray = append(envVarArray, newClusteredValue)
		statefulSetUpdated = true
	}

	if statefulSetUpdated {
		envVarArrayLen := len(envVarArray)
		if envVarArrayLen > 0 {
			for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
				for j := len(currentStatefulSet.Spec.Template.Spec.Containers[i].Env) - 1; j >= 0; j-- {
					if ("AMQ_CLUSTERED" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && clusteredNeedsUpdate) ||
						("AMQ_CLUSTER_USER" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && clusterUserNeedsUpdate) ||
						("AMQ_CLUSTER_PASSWORD" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && clusterPasswordNeedsUpdate) {
						currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
					}
				}
			}

			containerArrayLen := len(currentStatefulSet.Spec.Template.Spec.Containers)
			for i := 0; i < containerArrayLen; i++ {
				for j := 0; j < envVarArrayLen; j++ {
					currentStatefulSet.Spec.Template.Spec.Containers[i].Env = append(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, envVarArray[j])
				}
			}
		}
	}

	return statefulSetUpdated
}

func sslConfigSyncCausedUpdateOn(deploymentPlan *brokerv2alpha1.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

	foundKeystore := false
	foundKeystorePassword := false
	foundKeystoreTruststoreDir := false
	foundTruststore := false
	foundTruststorePassword := false

	keystoreNeedsUpdate := false
	keystorePasswordNeedsUpdate := false
	keystoreTruststoreDirNeedsUpdate := false
	truststoreNeedsUpdate := false
	truststorePasswordNeedsUpdate := false

	statefulSetUpdated := false

	// TODO: Remove yuck
	// ensure password and username are valid if can't via openapi validation?
	//if customResource.Spec.SSLConfig.KeyStorePassword != "" &&
	//	customResource.Spec.SSLConfig.KeystoreFilename != "" {
	if false { // "TODO-FIX-REPLACE"

		envVarArray := []corev1.EnvVar{}
		// Find the existing values
		for _, v := range currentStatefulSet.Spec.Template.Spec.Containers[0].Env {
			if v.Name == "AMQ_KEYSTORE" {
				foundKeystore = true
				//if v.Value != customResource.Spec.SSLConfig.KeystoreFilename {
				if false { // "TODO-FIX-REPLACE"
					keystoreNeedsUpdate = true
				}
			}
			if v.Name == "AMQ_KEYSTORE_PASSWORD" {
				foundKeystorePassword = true
				//if v.Value != customResource.Spec.SSLConfig.KeyStorePassword {
				if false { // "TODO-FIX-REPLACE"
					keystorePasswordNeedsUpdate = true
				}
			}
			if v.Name == "AMQ_KEYSTORE_TRUSTSTORE_DIR" {
				foundKeystoreTruststoreDir = true
				if v.Value != "/etc/amq-secret-volume" {
					keystoreTruststoreDirNeedsUpdate = true
				}
			}
		}

		if !foundKeystore || keystoreNeedsUpdate {
			newClusteredValue := corev1.EnvVar{
				"AMQ_KEYSTORE",
				"TODO-FIX-REPLACE", //customResource.Spec.SSLConfig.KeystoreFilename,
				nil,
			}
			envVarArray = append(envVarArray, newClusteredValue)
			statefulSetUpdated = true
		}

		if !foundKeystorePassword || keystorePasswordNeedsUpdate {
			newClusteredValue := corev1.EnvVar{
				"AMQ_KEYSTORE_PASSWORD",
				"TODO-FIX-REPLACE", //customResource.Spec.SSLConfig.KeyStorePassword,
				nil,
			}
			envVarArray = append(envVarArray, newClusteredValue)
			statefulSetUpdated = true
		}

		if !foundKeystoreTruststoreDir || keystoreTruststoreDirNeedsUpdate {
			newClusteredValue := corev1.EnvVar{
				"AMQ_KEYSTORE_TRUSTSTORE_DIR",
				"/etc/amq-secret-volume",
				nil,
			}
			envVarArray = append(envVarArray, newClusteredValue)
			statefulSetUpdated = true
		}

		if statefulSetUpdated {
			envVarArrayLen := len(envVarArray)
			if envVarArrayLen > 0 {
				for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
					for j := len(currentStatefulSet.Spec.Template.Spec.Containers[i].Env) - 1; j >= 0; j-- {
						if ("AMQ_KEYSTORE" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && keystoreNeedsUpdate) ||
							("AMQ_KEYSTORE_PASSWORD" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && keystorePasswordNeedsUpdate) ||
							("AMQ_KEYSTORE_TRUSTSTORE_DIR" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && keystoreTruststoreDirNeedsUpdate) {
							currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
						}
					}
				}

				containerArrayLen := len(currentStatefulSet.Spec.Template.Spec.Containers)
				for i := 0; i < containerArrayLen; i++ {
					for j := 0; j < envVarArrayLen; j++ {
						currentStatefulSet.Spec.Template.Spec.Containers[i].Env = append(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, envVarArray[j])
					}
				}
			}
		}
	} else {
		for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
			for j := len(currentStatefulSet.Spec.Template.Spec.Containers[i].Env) - 1; j >= 0; j-- {
				if "AMQ_KEYSTORE" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name ||
					"AMQ_KEYSTORE_PASSWORD" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name ||
					"AMQ_KEYSTORE_TRUSTSTORE_DIR" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name {
					currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
					statefulSetUpdated = true
				}
			}
		}
	}

	//if customResource.Spec.SSLConfig.TrustStorePassword != "" &&
	//	customResource.Spec.SSLConfig.TrustStoreFilename != "" {
	if false { // "TODO-FIX-REPLACE"
		envVarArray := []corev1.EnvVar{}
		// Find the existing values
		for _, v := range currentStatefulSet.Spec.Template.Spec.Containers[0].Env {
			if v.Name == "AMQ_TRUSTSTORE" {
				foundTruststore = true
				//if v.Value != customResource.Spec.SSLConfig.TrustStoreFilename {
				if false { // "TODO-FIX-REPLACE"
					truststoreNeedsUpdate = true
				}
			}
			if v.Name == "AMQ_TRUSTSTORE_PASSWORD" {
				foundTruststorePassword = true
				//if v.Value != customResource.Spec.SSLConfig.TrustStorePassword {
				if false { // "TODO-FIX-REPLACE"
					truststorePasswordNeedsUpdate = true
				}
			}
		}

		if !foundTruststore || truststoreNeedsUpdate {
			newClusteredValue := corev1.EnvVar{
				"AMQ_TRUSTSTORE",
				"TODO-FIX-REPLACE", //customResource.Spec.SSLConfig.TrustStoreFilename,
				nil,
			}
			envVarArray = append(envVarArray, newClusteredValue)
			statefulSetUpdated = true
		}

		if !foundTruststorePassword || truststorePasswordNeedsUpdate {
			newClusteredValue := corev1.EnvVar{
				"AMQ_TRUSTSTORE_PASSWORD",
				"TODO-FIX-REPLACE", //customResource.Spec.SSLConfig.TrustStorePassword,
				nil,
			}
			envVarArray = append(envVarArray, newClusteredValue)
			statefulSetUpdated = true
		}

		if statefulSetUpdated {
			envVarArrayLen := len(envVarArray)
			if envVarArrayLen > 0 {
				for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
					for j := len(currentStatefulSet.Spec.Template.Spec.Containers[i].Env) - 1; j >= 0; j-- {
						if ("AMQ_TRUSTSTORE" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && truststoreNeedsUpdate) ||
							("AMQ_TRUSTSTORE_PASSWORD" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && truststorePasswordNeedsUpdate) {
							currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
						}
					}
				}

				containerArrayLen := len(currentStatefulSet.Spec.Template.Spec.Containers)
				for i := 0; i < containerArrayLen; i++ {
					for j := 0; j < envVarArrayLen; j++ {
						currentStatefulSet.Spec.Template.Spec.Containers[i].Env = append(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, envVarArray[j])
					}
				}
			}
		}
	} else {
		for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
			for j := len(currentStatefulSet.Spec.Template.Spec.Containers[i].Env) - 1; j >= 0; j-- {
				if "AMQ_TRUSTSTORE" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name ||
					"AMQ_TRUSTSTORE_PASSWORD" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name {
					currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
					statefulSetUpdated = true
				}
			}
		}
	}

	if statefulSetUpdated {
		sslConfigSyncEnsureSecretVolumeMountExists(deploymentPlan, currentStatefulSet)
	}

	return statefulSetUpdated
}

func sslConfigSyncEnsureSecretVolumeMountExists(deploymentPlan *brokerv2alpha1.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) {

	secretVolumeExists := false
	secretVolumeMountExists := false

	for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Volumes); i++ {
		//if currentStatefulSet.Spec.Template.Spec.Volumes[i].Name == customResource.Spec.SSLConfig.SecretName {
		if false { // "TODO-FIX-REPLACE"
			secretVolumeExists = true
			break
		}
	}
	if !secretVolumeExists {
		volume := corev1.Volume{
			Name: "broker-secret-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "TODO-FIX-REPLACE", //customResource.Spec.SSLConfig.SecretName,
				},
			},
		}

		currentStatefulSet.Spec.Template.Spec.Volumes = append(currentStatefulSet.Spec.Template.Spec.Volumes, volume)
	}

	for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
		for j := 0; j < len(currentStatefulSet.Spec.Template.Spec.Containers[i].VolumeMounts); j++ {
			if currentStatefulSet.Spec.Template.Spec.Containers[i].VolumeMounts[j].Name == "broker-secret-volume" {
				secretVolumeMountExists = true
				break
			}
		}
		if !secretVolumeMountExists {
			volumeMount := corev1.VolumeMount{
				Name:      "broker-secret-volume",
				MountPath: "/etc/amq-secret-volume",
				ReadOnly:  true,
			}
			currentStatefulSet.Spec.Template.Spec.Containers[i].VolumeMounts = append(currentStatefulSet.Spec.Template.Spec.Containers[i].VolumeMounts, volumeMount)
		}
	}
}

func aioSyncCausedUpdateOn(deploymentPlan *brokerv2alpha1.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

	foundAio := false
	foundNio := false
	var extraArgs string = ""
	extraArgsNeedsUpdate := false

	// Find the existing values
	for _, v := range currentStatefulSet.Spec.Template.Spec.Containers[0].Env {
		if v.Name == "AMQ_EXTRA_ARGS" {
			if strings.Index(v.Value, "--aio") > -1 {
				foundAio = true
			}
			if strings.Index(v.Value, "--nio") > -1 {
				foundNio = true
			}
			extraArgs = v.Value
			break
		}
	}

	if "aio" == strings.ToLower(deploymentPlan.JournalType) && foundNio {
		extraArgs = strings.Replace(extraArgs, "--nio", "--aio", 1)
		extraArgsNeedsUpdate = true
	}

	if !("aio" == strings.ToLower(deploymentPlan.JournalType)) && foundAio {
		extraArgs = strings.Replace(extraArgs, "--aio", "--nio", 1)
		extraArgsNeedsUpdate = true
	}

	newExtraArgsValue := corev1.EnvVar{}
	if extraArgsNeedsUpdate {
		newExtraArgsValue = corev1.EnvVar{
			"AMQ_EXTRA_ARGS",
			extraArgs,
			nil,
		}

		containerArrayLen := len(currentStatefulSet.Spec.Template.Spec.Containers)
		for i := 0; i < containerArrayLen; i++ {
			//for j := 0; j < envVarArrayLen; j++ {
			for j := 0; j < len(currentStatefulSet.Spec.Template.Spec.Containers[i].Env); j++ {
				if "AMQ_EXTRA_ARGS" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name {
					currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
					currentStatefulSet.Spec.Template.Spec.Containers[i].Env = append(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, newExtraArgsValue)
					break
				}
			}
		}
	}

	return extraArgsNeedsUpdate
}

func persistentSyncCausedUpdateOn(deploymentPlan *brokerv2alpha1.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

	foundDataDir := false
	foundDataDirLogging := false

	dataDirNeedsUpdate := false
	dataDirLoggingNeedsUpdate := false

	statefulSetUpdated := false

	// TODO: Remove yuck
	// ensure password and username are valid if can't via openapi validation?
	if deploymentPlan.PersistenceEnabled {

		envVarArray := []corev1.EnvVar{}
		// Find the existing values
		for _, v := range currentStatefulSet.Spec.Template.Spec.Containers[0].Env {
			if v.Name == "AMQ_DATA_DIR" {
				foundDataDir = true
				if v.Value == "false" {
					dataDirNeedsUpdate = true
				}
			}
			if v.Name == "AMQ_DATA_DIR_LOGGING" {
				foundDataDirLogging = true
				if v.Value != "true" {
					dataDirLoggingNeedsUpdate = true
				}
			}
		}

		if !foundDataDir || dataDirNeedsUpdate {
			newDataDirValue := corev1.EnvVar{
				"AMQ_DATA_DIR",
				volumes.GLOBAL_DATA_PATH,
				nil,
			}
			envVarArray = append(envVarArray, newDataDirValue)
			statefulSetUpdated = true
		}

		if !foundDataDirLogging || dataDirLoggingNeedsUpdate {
			newDataDirLoggingValue := corev1.EnvVar{
				"AMQ_DATA_DIR_LOGGING",
				"true",
				nil,
			}
			envVarArray = append(envVarArray, newDataDirLoggingValue)
			statefulSetUpdated = true
		}

		if statefulSetUpdated {
			envVarArrayLen := len(envVarArray)
			if envVarArrayLen > 0 {
				for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
					for j := len(currentStatefulSet.Spec.Template.Spec.Containers[i].Env) - 1; j >= 0; j-- {
						if ("AMQ_DATA_DIR" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && dataDirNeedsUpdate) ||
							("AMQ_DATA_DIR_LOGGING" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && dataDirLoggingNeedsUpdate) {
							currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
						}
					}
				}

				containerArrayLen := len(currentStatefulSet.Spec.Template.Spec.Containers)
				for i := 0; i < containerArrayLen; i++ {
					for j := 0; j < envVarArrayLen; j++ {
						currentStatefulSet.Spec.Template.Spec.Containers[i].Env = append(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, envVarArray[j])
					}
				}
			}
		}
	} else {

		for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
			for j := len(currentStatefulSet.Spec.Template.Spec.Containers[i].Env) - 1; j >= 0; j-- {
				if "AMQ_DATA_DIR" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name ||
					"AMQ_DATA_DIR_LOGGING" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name {
					currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
					statefulSetUpdated = true
				}
			}
		}
	}

	return statefulSetUpdated
}

func imageSyncCausedUpdateOn(deploymentPlan *brokerv2alpha1.DeploymentPlanType, currentStatefulSet *appsv1.StatefulSet) bool {

	// At implementation time only one container
	if strings.Compare(currentStatefulSet.Spec.Template.Spec.Containers[0].Image, deploymentPlan.Image) != 0 {
		containerArrayLen := len(currentStatefulSet.Spec.Template.Spec.Containers)
		for i := 0; i < containerArrayLen; i++ {
			currentStatefulSet.Spec.Template.Spec.Containers[i].Image = deploymentPlan.Image
		}
		return true
	}

	return false
}

func commonConfigSyncCausedUpdateOn(customResource *brokerv2alpha1.ActiveMQArtemis, client client.Client, currentStatefulSet *appsv1.StatefulSet) bool {

	foundCommonUser := false
	foundCommonPassword := false

	commonUserNeedsUpdate := false
	commonPasswordNeedsUpdate := false

	statefulSetUpdated := false
	deploymentPlan := customResource.Spec.DeploymentPlan
	secretName := "amq-app-secret"
	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: currentStatefulSet.Namespace,
	}
	stringDataMap := map[string]string{}
	secret := secrets.NewSecret(customResource, secretName, stringDataMap)
	err := resources.Retrieve(customResource, namespacedName, client, secret)

	// TODO: Remove yuck
	envVarArray := []corev1.EnvVar{}
	// Find the existing values
	for _, v := range currentStatefulSet.Spec.Template.Spec.Containers[0].Env {
		if v.Name == "AMQ_USER" {
			foundCommonUser = true
			if err == nil {
				secretCommonUser := string(secret.Data[v.ValueFrom.SecretKeyRef.Key])
				if "" != secretCommonUser &&
					"" != deploymentPlan.User {
					if 0 != strings.Compare(secretCommonUser, deploymentPlan.User) {
						commonUserNeedsUpdate = true
					}
				}
			}
		}
		if v.Name == "AMQ_PASSWORD" {
			foundCommonPassword = true
			if err == nil {
				secretCommonPassword := string(secret.Data[v.ValueFrom.SecretKeyRef.Key])
				if "" != secretCommonPassword &&
					"" != deploymentPlan.Password {
					if 0 != strings.Compare(secretCommonPassword, deploymentPlan.Password) {
						commonPasswordNeedsUpdate = true
					}
				}
			}
		}
	}

	if !foundCommonUser || commonUserNeedsUpdate {
		newCommonedValue := corev1.EnvVar{
			"AMQ_USER",
			deploymentPlan.User,
			nil,
		}
		envVarArray = append(envVarArray, newCommonedValue)
		statefulSetUpdated = true
	}

	if !foundCommonPassword || commonPasswordNeedsUpdate {
		newCommonedValue := corev1.EnvVar{
			"AMQ_PASSWORD",
			deploymentPlan.Password,
			nil,
		}
		envVarArray = append(envVarArray, newCommonedValue)
		statefulSetUpdated = true
	}

	if statefulSetUpdated {
		envVarArrayLen := len(envVarArray)
		if envVarArrayLen > 0 {
			for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
				for j := len(currentStatefulSet.Spec.Template.Spec.Containers[i].Env) - 1; j >= 0; j-- {
					if ("AMQ_USER" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && commonUserNeedsUpdate) ||
						("AMQ_PASSWORD" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name && commonPasswordNeedsUpdate) {
						currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
					}
				}
			}

			containerArrayLen := len(currentStatefulSet.Spec.Template.Spec.Containers)
			for i := 0; i < containerArrayLen; i++ {
				for j := 0; j < envVarArrayLen; j++ {
					currentStatefulSet.Spec.Template.Spec.Containers[i].Env = append(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, envVarArray[j])
				}
			}
		}
	}

	return statefulSetUpdated
}
