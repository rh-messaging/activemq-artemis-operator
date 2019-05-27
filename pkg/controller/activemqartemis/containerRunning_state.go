package activemqartemis

import (
	"context"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/utils/fsm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	statefulSetSizeUpdated          = 1 << 0
	statefulSetClusterConfigUpdated = 1 << 1
	statefulSetSSLConfigUpdated     = 1 << 2
)

type ContainerRunningState struct {
	s              fsm.State
	namespacedName types.NamespacedName
	parentFSM      *ActiveMQArtemisFSM
}

func MakeContainerRunningState(_parentFSM *ActiveMQArtemisFSM, _namespacedName types.NamespacedName) ContainerRunningState {

	rs := ContainerRunningState{
		s:              fsm.MakeState(ContainerRunning, ContainerRunningID),
		namespacedName: _namespacedName,
		parentFSM:      _parentFSM,
	}

	return rs
}

func NewContainerRunningState(_parentFSM *ActiveMQArtemisFSM, _namespacedName types.NamespacedName) *ContainerRunningState {

	rs := MakeContainerRunningState(_parentFSM, _namespacedName)

	return &rs
}

func (rs *ContainerRunningState) ID() int {
	return ContainerRunningID
}

func (rs *ContainerRunningState) Enter(previousStateID int) error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Entering ContainerRunningState")

	return nil
}

func (rs *ContainerRunningState) Update() (error, int) {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Updating ContainerRunningState")

	var err error = nil
	var nextStateID int = ContainerRunningID
	var statefulSetUpdates uint32 = 0

	currentStatefulSet := &appsv1.StatefulSet{}
	err = rs.parentFSM.r.client.Get(context.TODO(), types.NamespacedName{Name: rs.parentFSM.customResource.Name + "-ss", Namespace: rs.parentFSM.customResource.Namespace}, currentStatefulSet)
	for {
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Error(err, "Failed to get StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
			err = nil
			break
		}

		// Ensure the StatefulSet size is the same as the spec
		size := rs.parentFSM.customResource.Spec.Size
		if *currentStatefulSet.Spec.Replicas != size {
			currentStatefulSet.Spec.Replicas = &size
			statefulSetUpdates |= statefulSetSizeUpdated
		}

		if rs.clusterConfigSyncCausedUpdateOn(currentStatefulSet) {
			statefulSetUpdates |= statefulSetClusterConfigUpdated
		}

		if rs.sslConfigSyncCausedUpdateOn(currentStatefulSet) {
			statefulSetUpdates |= statefulSetSSLConfigUpdated
		}

		break
	}

	if statefulSetUpdates > 0 {
		err = rs.parentFSM.r.client.Update(context.TODO(), currentStatefulSet)
		if err != nil {
			reqLogger.Error(err, "Failed to update StatefulSet.", "Deployment.Namespace", currentStatefulSet.Namespace, "Deployment.Name", currentStatefulSet.Name)
		}
	}

	return err, nextStateID
}

// https://stackoverflow.com/questions/37334119/how-to-delete-an-element-from-a-slice-in-golang
func remove(s []corev1.EnvVar, i int) []corev1.EnvVar {
	s[i] = s[len(s)-1]
	// We do not need to put s[i] at the end, as it will be discarded anyway
	return s[:len(s)-1]
}

func (rs *ContainerRunningState) clusterConfigSyncCausedUpdateOn(currentStatefulSet *appsv1.StatefulSet) bool {

	foundClustered := false
	foundClusterUser := false
	foundClusterPassword := false

	clusteredNeedsUpdate := false
	clusterUserNeedsUpdate := false
	clusterPasswordNeedsUpdate := false

	statefulSetUpdated := false

	// TODO: Remove yuck
	// ensure password and username are valid if can't via openapi validation?
	if rs.parentFSM.customResource.Spec.ClusterConfig.ClusterPassword != "" &&
		rs.parentFSM.customResource.Spec.ClusterConfig.ClusterUserName != "" {

		envVarArray := []corev1.EnvVar{}
		// Find the existing values
		for _, v := range currentStatefulSet.Spec.Template.Spec.Containers[0].Env {
			if v.Name == "AMQ_CLUSTERED" {
				foundClustered = true
				if v.Value == "false" {
					clusteredNeedsUpdate = true
				}
			}
			if v.Name == "AMQ_CLUSTER_USER" {
				foundClusterUser = true
				if v.Value != rs.parentFSM.customResource.Spec.ClusterConfig.ClusterUserName {
					clusterUserNeedsUpdate = true
				}
			}
			if v.Name == "AMQ_CLUSTER_PASSWORD" {
				foundClusterPassword = true
				if v.Value != rs.parentFSM.customResource.Spec.ClusterConfig.ClusterPassword {
					clusterPasswordNeedsUpdate = true
				}
			}
		}

		if !foundClustered || clusteredNeedsUpdate {
			newClusteredValue := corev1.EnvVar{
				"AMQ_CLUSTERED",
				"true",
				nil,
			}
			envVarArray = append(envVarArray, newClusteredValue)
			statefulSetUpdated = true
		}

		if !foundClusterUser || clusterUserNeedsUpdate {
			newClusteredValue := corev1.EnvVar{
				"AMQ_CLUSTER_USER",
				rs.parentFSM.customResource.Spec.ClusterConfig.ClusterUserName,
				nil,
			}
			envVarArray = append(envVarArray, newClusteredValue)
			statefulSetUpdated = true
		}

		if !foundClusterPassword || clusterPasswordNeedsUpdate {
			newClusteredValue := corev1.EnvVar{
				"AMQ_CLUSTER_PASSWORD",
				rs.parentFSM.customResource.Spec.ClusterConfig.ClusterPassword,
				nil,
			}
			envVarArray = append(envVarArray, newClusteredValue)
			statefulSetUpdated = true
		}

		if statefulSetUpdated {
			envVarArrayLen := len(envVarArray)
			if envVarArrayLen > 0 {
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
				if "AMQ_CLUSTERED" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name ||
					"AMQ_CLUSTER_USER" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name ||
					"AMQ_CLUSTER_PASSWORD" == currentStatefulSet.Spec.Template.Spec.Containers[i].Env[j].Name {
					currentStatefulSet.Spec.Template.Spec.Containers[i].Env = remove(currentStatefulSet.Spec.Template.Spec.Containers[i].Env, j)
					statefulSetUpdated = true
				}
			}
		}
	}

	return statefulSetUpdated
}

func (rs *ContainerRunningState) sslConfigSyncCausedUpdateOn(currentStatefulSet *appsv1.StatefulSet) bool {

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
	if rs.parentFSM.customResource.Spec.SSLConfig.KeyStorePassword != "" &&
		rs.parentFSM.customResource.Spec.SSLConfig.KeystoreFilename != "" {

		envVarArray := []corev1.EnvVar{}
		// Find the existing values
		for _, v := range currentStatefulSet.Spec.Template.Spec.Containers[0].Env {
			if v.Name == "AMQ_KEYSTORE" {
				foundKeystore = true
				if v.Value != rs.parentFSM.customResource.Spec.SSLConfig.KeystoreFilename {
					keystoreNeedsUpdate = true
				}
			}
			if v.Name == "AMQ_KEYSTORE_PASSWORD" {
				foundKeystorePassword = true
				if v.Value != rs.parentFSM.customResource.Spec.SSLConfig.KeyStorePassword {
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
				rs.parentFSM.customResource.Spec.SSLConfig.KeystoreFilename,
				nil,
			}
			envVarArray = append(envVarArray, newClusteredValue)
			statefulSetUpdated = true
		}

		if !foundKeystorePassword || keystorePasswordNeedsUpdate {
			newClusteredValue := corev1.EnvVar{
				"AMQ_KEYSTORE_PASSWORD",
				rs.parentFSM.customResource.Spec.SSLConfig.KeyStorePassword,
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

	if rs.parentFSM.customResource.Spec.SSLConfig.TrustStorePassword != "" &&
		rs.parentFSM.customResource.Spec.SSLConfig.TrustStoreFilename != "" {

		envVarArray := []corev1.EnvVar{}
		// Find the existing values
		for _, v := range currentStatefulSet.Spec.Template.Spec.Containers[0].Env {
			if v.Name == "AMQ_TRUSTSTORE" {
				foundTruststore = true
				if v.Value != rs.parentFSM.customResource.Spec.SSLConfig.TrustStoreFilename {
					truststoreNeedsUpdate = true
				}
			}
			if v.Name == "AMQ_TRUSTSTORE_PASSWORD" {
				foundTruststorePassword = true
				if v.Value != rs.parentFSM.customResource.Spec.SSLConfig.TrustStorePassword {
					truststorePasswordNeedsUpdate = true
				}
			}
		}

		if !foundTruststore || truststoreNeedsUpdate {
			newClusteredValue := corev1.EnvVar{
				"AMQ_TRUSTSTORE",
				rs.parentFSM.customResource.Spec.SSLConfig.TrustStoreFilename,
				nil,
			}
			envVarArray = append(envVarArray, newClusteredValue)
			statefulSetUpdated = true
		}

		if !foundTruststorePassword || truststorePasswordNeedsUpdate {
			newClusteredValue := corev1.EnvVar{
				"AMQ_TRUSTSTORE_PASSWORD",
				rs.parentFSM.customResource.Spec.SSLConfig.TrustStorePassword,
				nil,
			}
			envVarArray = append(envVarArray, newClusteredValue)
			statefulSetUpdated = true
		}

		if statefulSetUpdated {
			envVarArrayLen := len(envVarArray)
			if envVarArrayLen > 0 {
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
		rs.sslConfigSyncEnsureSecretVolumeMountExists(currentStatefulSet)

		if (rs.parentFSM.customResource.Spec.SSLConfig.KeyStorePassword != "" &&
			rs.parentFSM.customResource.Spec.SSLConfig.KeystoreFilename != "") ||
			(rs.parentFSM.customResource.Spec.SSLConfig.TrustStorePassword != "" &&
				rs.parentFSM.customResource.Spec.SSLConfig.TrustStoreFilename != "") {
			for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
				currentStatefulSet.Spec.Template.Spec.Containers[i].LivenessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
			}
		} else {
			for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Containers); i++ {
				currentStatefulSet.Spec.Template.Spec.Containers[i].LivenessProbe.HTTPGet.Scheme = corev1.URISchemeHTTP
			}
		}
	}

	return statefulSetUpdated
}

func (rs *ContainerRunningState) sslConfigSyncEnsureSecretVolumeMountExists(currentStatefulSet *appsv1.StatefulSet) {

	secretVolumeExists := false
	secretVolumeMountExists := false

	for i := 0; i < len(currentStatefulSet.Spec.Template.Spec.Volumes); i++ {
		if currentStatefulSet.Spec.Template.Spec.Volumes[i].Name == rs.parentFSM.customResource.Spec.SSLConfig.SecretName {
			secretVolumeExists = true
			break
		}
	}
	if !secretVolumeExists {
		volume := corev1.Volume{
			Name: "broker-secret-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: rs.parentFSM.customResource.Spec.SSLConfig.SecretName,
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

func (rs *ContainerRunningState) Exit() error {

	// Log where we are and what we're doing
	reqLogger := log.WithValues("ActiveMQArtemis Name", rs.parentFSM.customResource.Name)
	reqLogger.Info("Exiting ContainerRunningState")

	return nil
}
