package controllers

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/RHsyseng/operator-utils/pkg/resource/compare"
	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHexShaHashOfMap(t *testing.T) {

	nilOne := hexShaHashOfMap(nil)
	nilTwo := hexShaHashOfMap(nil)

	if nilOne != nilTwo {
		t.Errorf("HexShaHashOfMap(nil) = %v, want %v", nilOne, nilTwo)
	}

	props := []string{"a=a", "b=b"}

	propsOriginal := hexShaHashOfMap(props)

	// modify
	props = append(props, "c=c")

	propsModified := hexShaHashOfMap(props)

	if propsOriginal == propsModified {
		t.Errorf("HexShaHashOfMap(props mod) = %v, want %v", propsOriginal, propsModified)
	}

	// revert, drop the last entry b/c they are ordered
	props = props[:2]

	if propsOriginal != hexShaHashOfMap(props) {
		t.Errorf("HexShaHashOfMap(props) with revert = %v, want %v", propsOriginal, hexShaHashOfMap(props))
	}

	// modify further, drop first entry
	props = props[:1]

	if propsOriginal == hexShaHashOfMap(props) {
		t.Errorf("HexShaHashOfMap(props) with just a = %v, want %v", propsOriginal, hexShaHashOfMap(props))
	}

}

func TestMapComparatorForStatefulSet(t *testing.T) {

	ss := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{Kind: "StatefulSet", APIVersion: "apps/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "ss",
			GenerateName:               "",
			Namespace:                  "a",
			SelfLink:                   "",
			UID:                        "",
			ResourceVersion:            "1",
			Generation:                 0,
			CreationTimestamp:          metav1.Time{},
			DeletionTimestamp:          &metav1.Time{},
			DeletionGracePeriodSeconds: new(int64),
			Labels:                     nil,
			Annotations:                nil,
			OwnerReferences:            []metav1.OwnerReference{},
			Finalizers:                 []string{},
			ClusterName:                "",
			ManagedFields:              []metav1.ManagedFieldsEntry{},
		},
		Spec:   appsv1.StatefulSetSpec{},
		Status: appsv1.StatefulSetStatus{},
	}

	ssMod := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{Kind: "StatefulSet", APIVersion: "apps/v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "ss",
			GenerateName:               "",
			Namespace:                  "a",
			SelfLink:                   "",
			UID:                        "",
			ResourceVersion:            "1",
			Generation:                 0,
			CreationTimestamp:          metav1.Time{},
			DeletionTimestamp:          &metav1.Time{},
			DeletionGracePeriodSeconds: new(int64),
			Labels:                     nil,
			Annotations:                nil,
			OwnerReferences:            []metav1.OwnerReference{},
			Finalizers:                 []string{},
			ClusterName:                "",
			ManagedFields:              []metav1.ManagedFieldsEntry{},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:             new(int32),
			Selector:             &metav1.LabelSelector{},
			Template:             v1.PodTemplateSpec{},
			VolumeClaimTemplates: []v1.PersistentVolumeClaim{},
			ServiceName:          "ssMod",
			PodManagementPolicy:  "",
			UpdateStrategy:       appsv1.StatefulSetUpdateStrategy{},
			RevisionHistoryLimit: new(int32),
			MinReadySeconds:      0,
		},
		Status: appsv1.StatefulSetStatus{},
	}

	ss0 := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{Kind: "StatefulSet", APIVersion: "apps/v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "ss0",
			GenerateName:               "",
			Namespace:                  "a",
			SelfLink:                   "",
			UID:                        "",
			ResourceVersion:            "1",
			Generation:                 0,
			CreationTimestamp:          metav1.Time{},
			DeletionTimestamp:          &metav1.Time{},
			DeletionGracePeriodSeconds: new(int64),
			Labels:                     nil,
			Annotations:                nil,
			OwnerReferences:            []metav1.OwnerReference{},
			Finalizers:                 []string{},
			ClusterName:                "",
			ManagedFields:              []metav1.ManagedFieldsEntry{},
		},
		Spec:   appsv1.StatefulSetSpec{},
		Status: appsv1.StatefulSetStatus{},
	}

	var requestedResources []client.Object

	requestedResources = append(requestedResources, ss0)

	requestedResources = append(requestedResources, ssMod)

	deployed := make(map[reflect.Type][]client.Object)
	var deployedSets []client.Object
	deployedSets = append(deployedSets, ss)

	ssType := reflect.ValueOf(ss).Elem().Type()
	deployed[ssType] = deployedSets

	requested := compare.NewMapBuilder().Add(requestedResources...).ResourceMap()
	comparator := compare.NewMapComparator()

	comparator.Comparator.SetDefaultComparator(semanticEquals)
	deltas := comparator.Compare(deployed, requested)

	for resourceType, delta := range deltas {
		t.Log("", "instances of ", resourceType, "Will create ", len(delta.Added), "update ", len(delta.Updated), "and delete", len(delta.Removed))

		for index := range delta.Added {
			resourceToAdd := delta.Added[index]
			t.Log("", "instances of ", resourceType, "Will add", resourceToAdd)
		}

		for index := range delta.Updated {
			resourceToUpdate := delta.Updated[index]
			t.Log("", "instances of ", resourceType, "Will update", resourceToUpdate)

		}

		for index := range delta.Removed {
			resourceToRemove := delta.Removed[index]
			t.Log("", "instances of ", resourceType, "Will remove", resourceToRemove)

		}
	}
	if len(deltas[ssType].Added) != 1 {
		t.Errorf("expect new addition to appear!")

	}
	if (len(deltas[ssType].Updated)) != 1 {
		t.Errorf("not good!, expect difference on ss to be respected as an update")
	}
}

func semanticEquals(a client.Object, b client.Object) bool {
	return equality.Semantic.DeepEqual(a, b)
}

func TestGetSingleStatefulSetStatus(t *testing.T) {

	var expected int32 = int32(1)
	ss := &appsv1.StatefulSet{}
	ss.ObjectMeta.Name = "joe"
	ss.Spec.Replicas = &expected
	ss.Status.Replicas = 1
	ss.Status.ReadyReplicas = 1

	cr := &brokerv1beta1.ActiveMQArtemis{}
	statusRunning := getSingleStatefulSetStatus(ss, cr)
	if statusRunning.Ready[0] != "joe-0" {
		t.Errorf("not good!, expect correct 0 ordinal" + statusRunning.Ready[0])
	}

	ss.Status.Replicas = 0
	ss.Status.ReadyReplicas = 0

	statusRunning = getSingleStatefulSetStatus(ss, cr)
	if statusRunning.Stopped[0] != "joe" {
		t.Errorf("not good!, expect ss name in stopped" + statusRunning.Stopped[0])
	}

	var expectedTwo int32 = int32(2)
	ss.Spec.Replicas = &expectedTwo
	ss.Status.Replicas = 2
	ss.Status.ReadyReplicas = 1

	statusRunning = getSingleStatefulSetStatus(ss, cr)
	if statusRunning.Ready[0] != "joe-0" {
		t.Errorf("not good!, expect correct 0 ordinal ready" + statusRunning.Ready[0])
	}

	if statusRunning.Starting[0] != "joe-1" {
		t.Errorf("not good!, expect ss name in starting" + statusRunning.Stopped[0])
	}

	if cr.Status.DeploymentPlanSize != 2 {
		t.Errorf("not good!, status not updated")
	}
}

func TestGetConfigAppliedConfigMapName(t *testing.T) {
	cr := brokerv1beta1.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test",
		},
	}
	name := getConfigAppliedConfigMapName(&cr)
	assert.Equal(t, "test-ns", name.Namespace)
	assert.Equal(t, "test-props", name.Name)
}

func TestExtractSha(t *testing.T) {
	json := `{"configuration": {"properties": {"a_status.properties": {"alder32": "123456"}}}}`
	status, err := unmarshallStatus(json)
	assert.NoError(t, err)
	sha, err := extractSha(status, "a_status.properties")
	assert.Equal(t, "123456", sha)
	assert.NoError(t, err)

	json = `{"configuration": {"properties": {"a_status.properties": {}}}}`
	status, err = unmarshallStatus(json)
	assert.NoError(t, err)
	sha, err = extractSha(status, "a_status.properties")
	assert.Empty(t, sha)
	assert.NoError(t, err)

	json = `you shall fail`
	status, err = unmarshallStatus(json)
	assert.Error(t, err)
	sha, err = extractSha(status, "a_status.properties")
	assert.Empty(t, sha)
	assert.Error(t, err)
}

func extractSha(status brokerStatus, name string) (string, error) {
	current, present := status.BrokerConfigStatus.PropertiesStatus[name]
	if !present {
		return "", errors.New("not present")
	} else {
		return current.Alder32, nil
	}
}

func TestAlder32Gen(t *testing.T) {

	userProps := `admin=admin
			tom=tom
			peter=peter`

	res := alder32FromData([]byte(userProps))
	assert.True(t, strings.Contains(res, "2905476010"))
}

func TestAlder32GenSpace(t *testing.T) {

	userProps := `admin = joe`

	res := alder32FromData([]byte(userProps))
	assert.True(t, strings.Contains(res, "295568261"))
}

func TestAlder32GenWithEmptyLine(t *testing.T) {

	userProps := `
			admin=admin
			tom=tom
			peter=peter`

	res := alder32FromData([]byte(userProps))
	assert.True(t, strings.Contains(res, "2905476010"))
}

func TestAlder32GenWithSpace(t *testing.T) {

	userProps := `addressesSettings.#.redeliveryMultiplier=2.3
	addressesSettings.#.redeliveryCollisionAvoidanceFactor=1.2
	addressesSettings.Some\ value\ with\ space.redeliveryCollisionAvoidanceFactor=1.2`

	res := alder32FromData([]byte(userProps))
	assert.EqualValues(t, "2211202255", res)
}

func TestAlder32GenBrokerProps(t *testing.T) {

	propsString := "# generated by crd\n#\nconnectionRouters.autoShard.keyType=CLIENT_ID\nconnectionRouters.autoShard.localTargetFilter=NULL|${STATEFUL_SET_ORDINAL}|-${STATEFUL_SET_ORDINAL}\nconnectionRouters.autoShard.policyConfiguration=CONSISTENT_HASH_MODULO\nconnectionRouters.autoShard.policyConfiguration.properties.MODULO=2\nacceptorConfigurations.tcp.params.router=autoShard\naddressesSettings.\"LB.#\".defaultAddressRoutingType=ANYCAST\n"

	res := alder32FromData([]byte(propsString))
	assert.True(t, strings.Contains(res, "1897435425"))
}

func TestAlder32RolesProps(t *testing.T) {

	propsStringWithLeadingWhiteSpaceBeforeComment := `
	# rbac
    control-plane=control-plane,control-plane-0,control-plane-1
    consumers=c1,c2,c3,c4
    producers=p
	! exclimation mark comment to strip with leading ws
! as start of line to strip
     # partitioned consumer roles for connectionRouter
shard-consumers-broker-0=c1,c2
shard-consumers-broker-1=c3,c4

     		# should resolve to NULL in absence of this
shard-control-plane=control-plane,control-plane-0,control-plane-1
shard-producers=p`

	propsStringCommentsStripped := `
control-plane=control-plane,control-plane-0,control-plane-1
consumers=c1,c2,c3,c4
producers=p
shard-consumers-broker-0=c1,c2
shard-consumers-broker-1=c3,c4
shard-control-plane=control-plane,control-plane-0,control-plane-1
shard-producers=p`

	res := alder32FromData([]byte(propsStringWithLeadingWhiteSpaceBeforeComment))

	expected := alder32FromData([]byte(propsStringCommentsStripped))

	assert.Equal(t, res, expected)
}

func TestAlder32PropsWithFF(t *testing.T) {

	propsStringWithLeadingWhiteSpaceBeforeComment := "\n\t\f# with form feed\nproducers=p"

	propsStringCommentsStripped := "producers=p"

	res := alder32FromData([]byte(propsStringWithLeadingWhiteSpaceBeforeComment))

	expected := alder32FromData([]byte(propsStringCommentsStripped))

	assert.Equal(t, res, expected)
}

func TestExtractErrors(t *testing.T) {

	json := "{\"configuration\":{\"properties\":{\"broker.properties\":{\"alder32\":\"1\"},\"system\":{\"alder32\":\"1\"}}},\"server\":{\"jaas\":{\"properties\":{\"artemis-users.properties\":{\"reloadTime\":\"1669744377685\",\"Alder32\":\"955331033\"},\"artemis-roles.properties\":{\"reloadTime\":\"1669744377685\",\"Alder32\":\"701302135\"}}},\"state\":\"STARTED\",\"version\":\"2.27.0\",\"nodeId\":\"a644c0c6-700e-11ed-9d4f-0a580ad90188\",\"identity\":null,\"uptime\":\"33.176 seconds\"}}"
	status, err := unmarshallStatus(json)
	assert.NoError(t, err)
	sha, err := extractSha(status, "broker.properties")
	assert.Equal(t, "1", sha)
	assert.NoError(t, err)

	json = `{"configuration": {
			"properties": {
				"a_status.properties": {
					"alder32": "110827957",
					"cr:alder32": "1f4004ae",
					"errors": []
				},
				"broker.properties": {
					"alder32": "524289198",
					"errors": [
						{
							"value": "notValid=bla",
							"reason": "No accessor method descriptor for: notValid on: class org.apache.activemq.artemis.core.config.impl.FileConfiguration"
						}
					]
				}
			}
		}
	}`
	status, err = unmarshallStatus(json)
	assert.NoError(t, err)
	appplyErrors := status.BrokerConfigStatus.PropertiesStatus["broker.properties"].ApplyErrors
	assert.True(t, len(appplyErrors) > 0)

	marshalledErrorsStr := marshallApplyErrors(appplyErrors)
	assert.True(t, strings.Contains(marshalledErrorsStr, "bla"))

}

func TestGetJaasConfigExtraMountPath(t *testing.T) {
	cr := &brokerv1beta1.ActiveMQArtemis{
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			DeploymentPlan: brokerv1beta1.DeploymentPlanType{
				ExtraMounts: brokerv1beta1.ExtraMountsType{
					ConfigMaps: []string{
						"some-cm",
					},
					Secrets: []string{
						"test-config-jaas-config",
						"other",
					},
				},
			},
		},
	}
	path, found := getJaasConfigExtraMountPath(cr)
	assert.Equal(t, path, "/amq/extra/secrets/test-config-jaas-config/login.config")
	assert.True(t, found)

	cr = &brokerv1beta1.ActiveMQArtemis{
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			DeploymentPlan: brokerv1beta1.DeploymentPlanType{
				ExtraMounts: brokerv1beta1.ExtraMountsType{
					ConfigMaps: []string{
						"test-config",
						"some-cm",
					},
					Secrets: []string{
						"test-config-jaas-config",
						"other-secret",
					},
				},
			},
		},
	}
	path, found = getJaasConfigExtraMountPath(cr)
	assert.Equal(t, path, "/amq/extra/secrets/test-config-jaas-config/login.config")
	assert.True(t, found)
}

func TestGetJaasConfigExtraMountPathNotPresent(t *testing.T) {
	cr := &brokerv1beta1.ActiveMQArtemis{
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			DeploymentPlan: brokerv1beta1.DeploymentPlanType{
				ExtraMounts: brokerv1beta1.ExtraMountsType{
					ConfigMaps: []string{
						"test-config",
					},
				},
			},
		},
	}
	path, found := getJaasConfigExtraMountPath(cr)
	assert.Empty(t, path)
	assert.False(t, found)
}

func TestNewPodTemplateSpecForCR_IncludesDebugArgs(t *testing.T) {
	// client := fake.NewClientBuilder().Build()
	reconciler := &ActiveMQArtemisReconcilerImpl{}

	cr := &brokerv1beta1.ActiveMQArtemis{
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			DeploymentPlan: brokerv1beta1.DeploymentPlanType{
				ExtraMounts: brokerv1beta1.ExtraMountsType{
					ConfigMaps: []string{
						"some-cm",
					},
					Secrets: []string{
						"test-config-jaas-config",
						"other",
					},
				},
			},
		},
	}

	newSpec, err := reconciler.NewPodTemplateSpecForCR(cr, Namers{}, &v1.PodTemplateSpec{}, k8sClient)

	assert.NoError(t, err)
	assert.NotNil(t, newSpec)
	expectedEnv := v1.EnvVar{
		Name:  "DEBUG_ARGS",
		Value: "-Djava.security.auth.login.config=/amq/extra/secrets/test-config-jaas-config/login.config",
	}
	assert.Contains(t, newSpec.Spec.Containers[0].Env, expectedEnv)
}

func TestNewPodTemplateSpecForCR_AppendsDebugArgs(t *testing.T) {
	// client := fake.NewClientBuilder().Build()
	reconciler := &ActiveMQArtemisReconcilerImpl{}

	cr := &brokerv1beta1.ActiveMQArtemis{
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			Env: []v1.EnvVar{
				{
					Name:  "DEBUG_ARGS",
					Value: "-Dtest.arg=foo",
				},
			},
			DeploymentPlan: brokerv1beta1.DeploymentPlanType{
				ExtraMounts: brokerv1beta1.ExtraMountsType{
					ConfigMaps: []string{
						"some-cm",
					},
					Secrets: []string{
						"test-config-jaas-config",
						"other",
					},
				},
			},
		},
	}

	newSpec, err := reconciler.NewPodTemplateSpecForCR(cr, Namers{}, &v1.PodTemplateSpec{}, k8sClient)

	assert.NoError(t, err)
	assert.NotNil(t, newSpec)
	expectedEnv := v1.EnvVar{
		Name:  "DEBUG_ARGS",
		Value: "-Dtest.arg=foo -Djava.security.auth.login.config=/amq/extra/secrets/test-config-jaas-config/login.config",
	}
	assert.Contains(t, newSpec.Spec.Containers[0].Env, expectedEnv)
}

func TestLoginConfigSyntaxCheck(t *testing.T) {
	good := map[string][]byte{
		"simple": []byte(`a {
		SampleLoginModule Required  a=b b=d;
		SampleLoginModule Optional;
		SampleLoginModule requisite;
		SampleLoginModule sufficient;
	   };`),
		"quotex": []byte(` aaa {
		SampleLoginModule Required  a=b b=d;
		SampleLoginModule Required
		   base=2
		   option=x;
	   };`),
		"comments": []byte(` aaa {
		// a good comment
		/* and another */
		/* and line */
		SampleLoginModule Required  a=b b=d;
		// more comments
		SampleLoginModule Required
		   base=2
		   option=x; 
	   };`),

		"comments_multiline": []byte(` aaa {
		/* and multi 
		line */
		SampleLoginModule Required  a=b b=d;

		/* more 
		comments */

	   };`),

		"comment_at_end_of_line": []byte(` aaa {
		// a good comment
		SampleLoginModule Required  a=b b=d; // again
		SampleLoginModule Required
		   base=2
		   option=x; // and another comment
	   };`),

		"twoRealm": []byte(` aa 
		{
		SampleLoginModule Required  a=b b=d;
		SampleLoginModule Required
		   base=2
		   option="x";
	   };
	   
	     bb {
		   SampleLoginModule Required
		   base=2
		   option="${x}";
	 }  ;`),

		"full": []byte(`
	 // a full login.config
	 activemq {
		 org.apache.activemq.artemis.spi.core.security.jaas.PropertiesLoginModule required
			 reload=true
			 debug=true
			 org.apache.activemq.jaas.properties.user="users.properties"
			 org.apache.activemq.jaas.properties.role="roles.properties";
	 };

	 console {

		 // ensure the operator can connect to the mgmt console by referencing the existing properties config
		 // operatorAuth = plain
		 // hawtio.realm = console
		 org.apache.activemq.artemis.spi.core.security.jaas.PropertiesLoginModule required
			 reload=true
			 debug=true
			 org.apache.activemq.jaas.properties.user="artemis-users.properties"
			 org.apache.activemq.jaas.properties.role="artemis-roles.properties"
			 baseDir="/home/jboss/amq-broker/etc";

	 };`),
	}

	for k, v := range good {
		assert.True(t, MatchBytesAgainsLoginConfigRegexp(v), "for key "+k)
	}

	bad := map[string][]byte{
		"twoRealm-missingSemiBetweenRealms": []byte(` aa 
		{
		SampleLoginModule Required  a=b b=d;
		SampleLoginModule Required
		   base=2
		   option="x";
	   } // missing semi - and comments! // may have to strip comments as a first step of validation
	   
	     bb {
		   SampleLoginModule Required
		   base=2
		   option="${x}";
	 }  ;`),
		"no_flags": []byte(`aa 
	 {
	     SampleLoginModule a=b b=d;
	 };`),

		"no_semi_on_module": []byte(`aa 
	 {
	     SampleLoginModule sufficient a=b
	 };`),

		"no_semi_at_end": []byte(`aa 
	 {
	     SampleLoginModule sufficient;
	 }`),
		"no_value_for_key": []byte(`aa 
	 {
	     SampleLoginModule sufficient a=;
	 };`),
		"no_key for value": []byte(`aa 
	 {
	     SampleLoginModule sufficient =a;
	 };`),
	}

	for k, v := range bad {
		assert.False(t, MatchBytesAgainsLoginConfigRegexp(v), "for key "+k)
	}

}
