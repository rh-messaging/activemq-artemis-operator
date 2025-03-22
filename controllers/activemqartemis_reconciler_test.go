package controllers

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/RHsyseng/operator-utils/pkg/olm"
	"github.com/RHsyseng/operator-utils/pkg/resource/compare"
	brokerv1beta1 "github.com/arkmq-org/activemq-artemis-operator/api/v1beta1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/arkmq-org/activemq-artemis-operator/pkg/utils/common"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	utilpointer "k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
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
	comparator := compare.MapComparator{
		Comparator: compare.SimpleComparator(),
	}

	reconciler := &ActiveMQArtemisReconcilerImpl{
		log:            ctrl.Log.WithName("test"),
		customResource: nil,
	}

	comparator.Comparator.SetComparator(reflect.TypeOf(appsv1.StatefulSet{}), reconciler.CompareMetaAndSpec)
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

func TestComparatorMetaAndSpec(t *testing.T) {

	reconciler := &ActiveMQArtemisReconcilerImpl{
		log:            ctrl.Log.WithName("test"),
		customResource: nil,
	}

	ss0 := &appsv1.StatefulSet{}
	equal := reconciler.CompareMetaAndSpec(ss0, ss0)

	if !equal {
		t.Errorf("expect equal on same instance!")
	}

	ss1 := &appsv1.StatefulSet{}
	ss1.Annotations = map[string]string{"A": "B"}
	equal = reconciler.CompareMetaAndSpec(ss0, ss1)

	if equal {
		t.Errorf("expect not equal on differnet annotations")
	}
}

func TestGetSingleStatefulSetStatus(t *testing.T) {

	var expected int32 = int32(1)
	ss := &appsv1.StatefulSet{}
	ss.ObjectMeta.Name = "joe"
	ss.Spec.Replicas = &expected
	ss.Status.Replicas = 1
	ss.Status.ReadyReplicas = 1

	cr := &brokerv1beta1.ActiveMQArtemis{}
	statusRunning := common.GetSingleStatefulSetStatus(ss, cr)
	if statusRunning.Ready[0] != "joe-0" {
		t.Errorf("not good!, expect correct 0 ordinal" + statusRunning.Ready[0])
	}

	ss.Status.Replicas = 0
	ss.Status.ReadyReplicas = 0

	statusRunning = common.GetSingleStatefulSetStatus(ss, cr)
	if statusRunning.Stopped[0] != "joe" {
		t.Errorf("not good!, expect ss name in stopped" + statusRunning.Stopped[0])
	}

	var expectedTwo int32 = int32(2)
	ss.Spec.Replicas = &expectedTwo
	ss.Status.Replicas = 2
	ss.Status.ReadyReplicas = 1

	statusRunning = common.GetSingleStatefulSetStatus(ss, cr)
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
	name := getPropertiesResourceNsName(&cr)
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

func TestAlder32GenWithDots(t *testing.T) {

	userProps := `addressSettings.#.redeliveryMultiplier=5
    addressSettings.\"news.#\".redeliveryMultiplier=2
    addressSettings.\"order.#\".redeliveryMultiplier=3`

	res := alder32FromData([]byte(userProps))
	assert.EqualValues(t, "3264295767", res)
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

	reconciler := &ActiveMQArtemisReconcilerImpl{
		log:            ctrl.Log.WithName("test"),
		customResource: cr,
	}

	newSpec, err := reconciler.PodTemplateSpecForCR(cr, common.Namers{}, &v1.PodTemplateSpec{}, k8sClient)

	assert.NoError(t, err)
	assert.NotNil(t, newSpec)
	expectedEnv := v1.EnvVar{
		Name:  "DEBUG_ARGS",
		Value: "-Djava.security.auth.login.config=/amq/extra/secrets/test-config-jaas-config/login.config",
	}
	assert.Contains(t, newSpec.Spec.Containers[0].Env, expectedEnv)
}

func TestProcess_TemplateIncludesLabelsServiceAndSecret(t *testing.T) {

	var kindMatch string = "Secret"
	cr := &brokerv1beta1.ActiveMQArtemis{
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			DeploymentPlan: brokerv1beta1.DeploymentPlanType{
				Labels: map[string]string{"myPodKey": "myPodValue"},
			},
			ResourceTemplates: []brokerv1beta1.ResourceTemplate{
				{
					// match all
					Labels: map[string]string{"myKey": "myValue"},
				},
				{
					// match just Secrets
					Selector: &brokerv1beta1.ResourceSelector{
						Kind: &kindMatch,
					},
					Labels: map[string]string{"mySecretKey": "mySecretValue"},
				}},
		},
	}
	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log.WithName("TestProcess_TemplateIncludesLabelsServiceAndSecret"), isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	namer := MakeNamers(cr)

	newSS, err := reconciler.ProcessStatefulSet(cr, *namer, nil)
	reconciler.trackDesired(newSS)
	assert.NoError(t, err)

	reconciler.ProcessDeploymentPlan(cr, *namer, nil, nil, newSS)

	fakeClient := fake.NewClientBuilder().Build()
	err = reconciler.ProcessResources(cr, fakeClient, nil)
	assert.NoError(t, err)

	var ssFound = false
	var secretFound = false
	var serviceFound = false
	for _, resource := range common.ToResourceList(reconciler.requestedResources) {
		if ss, ok := resource.(*appsv1.StatefulSet); ok {

			newSpec := ss.Spec.Template
			assert.NoError(t, err)
			assert.NotNil(t, newSpec)

			v, ok := newSpec.Labels["myPodKey"]
			assert.True(t, ok)
			assert.Equal(t, "myPodValue", v)

			ssFound = true
		}

		if secret, ok := resource.(*v1.Secret); ok {
			assert.True(t, len(secret.Labels) >= 1)
			assert.Equal(t, secret.Labels["myKey"], "myValue")
			assert.Equal(t, secret.Labels["mySecretKey"], "mySecretValue")
			secretFound = true
		}

		if service, ok := resource.(*v1.Service); ok {
			assert.True(t, len(service.Labels) >= 1)
			assert.Equal(t, service.Labels["myKey"], "myValue")
			_, found := service.Labels["mySecretKey"]
			assert.False(t, found)
			serviceFound = true
		}
	}
	assert.True(t, ssFound)
	assert.True(t, secretFound)
	assert.True(t, serviceFound)
}

func TestProcess_TemplateIncludesLabelsSecretRegexp(t *testing.T) {

	var regexpNameMatch string = ".*-props"
	var exactNameMatch string = "-props"

	cr := &brokerv1beta1.ActiveMQArtemis{
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			ResourceTemplates: []brokerv1beta1.ResourceTemplate{
				{
					Selector: &brokerv1beta1.ResourceSelector{
						// match just -props secrets by name with regex
						Name: &regexpNameMatch,
					},
					Labels: map[string]string{"mySecretKey": "mySecretValue"},
				},
				{
					Selector: &brokerv1beta1.ResourceSelector{
						// match just -props secrets by name
						Name: &exactNameMatch,
					},
					Labels: map[string]string{"myExactSecretKey": "myExactSecretValue"},
				}},
		},
	}

	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log.WithName("TestProcess_TemplateIncludesLabelsServiceAndSecret"), isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	namer := MakeNamers(cr)

	newSS, _ := reconciler.ProcessStatefulSet(cr, *namer, nil)
	reconciler.ProcessDeploymentPlan(cr, *namer, nil, nil, newSS)

	fakeClient := fake.NewClientBuilder().Build()
	err := reconciler.ProcessResources(cr, fakeClient, nil)
	assert.NoError(t, err)

	var secretFound = false
	var serviceFound = false

	for _, resource := range common.ToResourceList(reconciler.requestedResources) {
		if secret, ok := resource.(*v1.Secret); ok {
			assert.True(t, len(secret.Labels) >= 1)
			assert.Equal(t, secret.Labels["mySecretKey"], "mySecretValue")
			assert.Equal(t, secret.Labels["myExactSecretKey"], "myExactSecretValue")
			secretFound = true
		}

		if service, ok := resource.(*v1.Service); ok {
			assert.True(t, len(service.Labels) >= 1)
			_, found := service.Labels["mySecretKey"]
			assert.False(t, found)
			_, found = service.Labels["myExactSecretKey"]
			assert.False(t, found)
			serviceFound = true
		}

	}
	assert.True(t, secretFound)
	assert.True(t, serviceFound)

}

func TestProcess_TemplateDuplicateKeyReplacesOk(t *testing.T) {

	cr := &brokerv1beta1.ActiveMQArtemis{
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			ResourceTemplates: []brokerv1beta1.ResourceTemplate{
				{
					Labels: map[string]string{"mySecretKey": "mySecretValueWillBeReplacedByDuplicate"},
				},
				{
					Labels: map[string]string{"mySecretKey": "mySecretValue"},
				}},
		},
	}

	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log.WithName("TestProcess_TemplateDuplicateKeyReplacesOk"), isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	namer := MakeNamers(cr)

	newSS, _ := reconciler.ProcessStatefulSet(cr, *namer, nil)
	reconciler.ProcessDeploymentPlan(cr, *namer, nil, nil, newSS)

	fakeClient := fake.NewClientBuilder().Build()
	err := reconciler.ProcessResources(cr, fakeClient, nil)
	assert.NoError(t, err)

	var secretFound = false
	for _, resource := range common.ToResourceList(reconciler.requestedResources) {
		if secret, ok := resource.(*v1.Secret); ok {
			assert.True(t, len(secret.Labels) >= 1)
			assert.Equal(t, secret.Labels["mySecretKey"], "mySecretValue")
			secretFound = true
		}
	}
	assert.True(t, secretFound)
}

func Test_Respect_existing_JAVA_OPTS_properties_def(t *testing.T) {

	cr := &brokerv1beta1.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{Name: "cr"},
		Spec:       brokerv1beta1.ActiveMQArtemisSpec{},
	}

	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log.WithName("Test_Respect_existing_JAVA_OPTS_properties_def"), isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	namer := MakeNamers(cr)

	existingSS := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: namer.SsNameBuilder.Name()},
		Spec: appsv1.StatefulSetSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{
						{
							Name: cr.Name + "-container-init",
							Env: []v1.EnvVar{
								{
									Name:  javaOptsEnvVarName,
									Value: "a",
								},
							},
						},
					},
				},
			},
		},
	}
	reconciler.deployed = make(map[reflect.Type][]client.Object)
	reconciler.addToDeployed(reflect.TypeOf(appsv1.StatefulSet{}), &existingSS)
	newSS, _ := reconciler.ProcessStatefulSet(cr, *namer, nil)

	var index = -1
	for i, env := range newSS.Spec.Template.Spec.InitContainers[0].Env {
		if env.Name == javaOptsEnvVarName {
			index = i
			break
		}
	}
	assert.True(t, index != -1)
	assert.True(t, strings.Contains(newSS.Spec.Template.Spec.InitContainers[0].Env[index].Value, "properties"))
}

func TestProcess_TemplateKeyValue(t *testing.T) {

	var kindMatch string = "Service"
	var matchOrdinalServices string = ".+-[0-9]+-svc"
	var matchGvForIngress string = "networking.k8s.io/v1"
	cr := &brokerv1beta1.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{Name: "cr"},
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			ResourceTemplates: []brokerv1beta1.ResourceTemplate{
				{
					// match all
					Labels: map[string]string{"myKey": "myValue-$(CR_NAME)"},
				},
				{
					// match Acceptor Services with Ordinals
					Selector: &brokerv1beta1.ResourceSelector{
						Kind: &kindMatch,
						Name: &matchOrdinalServices,
					},
					Labels: map[string]string{"myKey-$(CR_NAME)": "myValue-$(BROKER_ORDINAL)"},
				},
				{
					// match Ingress
					Selector: &brokerv1beta1.ResourceSelector{
						APIGroup: &matchGvForIngress,
					},
					Annotations: map[string]string{"myIngressKey-$(CR_NAME)": "myValue-$(BROKER_ORDINAL)"},
				},
			},
			Acceptors: []brokerv1beta1.AcceptorType{{
				Name:   "aa",
				Port:   563,
				Expose: true,
			}},
		},
	}

	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log.WithName("test"), isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	namer := MakeNamers(cr)

	newSS, _ := reconciler.ProcessStatefulSet(cr, *namer, nil)
	reconciler.trackDesired(newSS)

	err := routev1.AddToScheme(scheme.Scheme)
	assert.NoError(t, err)

	fakeClient := fake.NewClientBuilder().Build()
	reconciler.ProcessAcceptorsAndConnectors(cr, *namer,
		fakeClient, nil, newSS)

	err = reconciler.ProcessResources(cr, fakeClient, nil)
	assert.NoError(t, err)

	var secretFound = false
	var serviceFound = false
	var ssFound = false
	for _, resource := range common.ToResourceList(reconciler.requestedResources) {
		if ss, ok := resource.(*appsv1.StatefulSet); ok {

			v, ok := ss.Labels["myKey"]
			assert.True(t, ok)
			assert.Equal(t, "myValue-cr", v)
			ssFound = true
		}

		if secret, ok := resource.(*v1.Secret); ok {
			assert.True(t, len(secret.Labels) >= 1)
			assert.Equal(t, secret.Labels["myKey"], "myValue-cr", resource.GetName())
			secretFound = true
		}

		if service, ok := resource.(*v1.Service); ok {
			assert.True(t, len(service.Labels) >= 1)
			assert.Equal(t, service.Labels["myKey"], "myValue-cr", resource.GetName())

			if strings.Contains(service.GetName(), "-0-") {
				assert.Equal(t, service.Labels["myKey-cr"], "myValue-0", resource.GetName())
				serviceFound = true
			}
		}

		if ingress, ok := resource.(*netv1.Ingress); ok {
			assert.True(t, len(ingress.Annotations) >= 1)
			assert.Equal(t, ingress.Annotations["myIngressKey-cr"], "myValue-0", resource.GetName())
		}

	}
	assert.True(t, ssFound)
	assert.True(t, secretFound)
	assert.True(t, serviceFound)
}

func TestProcess_TemplateCustomAttributeIngress(t *testing.T) {

	var matchGvForIngress string = "networking.k8s.io/v1"
	var ingressClassVal = "SomeClass"
	cr := &brokerv1beta1.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{Name: "cr"},
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			ResourceTemplates: []brokerv1beta1.ResourceTemplate{
				{
					// match Ingress
					Selector: &brokerv1beta1.ResourceSelector{
						APIGroup: &matchGvForIngress,
					},
					Annotations: map[string]string{"myIngressKey-$(CR_NAME)": "myValue-$(BROKER_ORDINAL)"},
					Patch: &unstructured.Unstructured{Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"ingressClassName": ingressClassVal,
						},
					},
					},
				},
			},
			Acceptors: []brokerv1beta1.AcceptorType{{
				Name:       "aa",
				Port:       563,
				Expose:     true,
				SSLEnabled: false,
				ExposeMode: &brokerv1beta1.ExposeModes.Ingress,
			}},
		},
	}

	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log.WithName("test"), isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	namer := MakeNamers(cr)

	newSS, _ := reconciler.ProcessStatefulSet(cr, *namer, nil)
	reconciler.trackDesired(newSS)

	fakeClient := fake.NewClientBuilder().Build()
	reconciler.ProcessAcceptorsAndConnectors(cr, *namer,
		fakeClient, nil, newSS)

	err := reconciler.ProcessResources(cr, fakeClient, nil)
	assert.NoError(t, err)

	var ingressOk = false
	for _, resource := range common.ToResourceList(reconciler.requestedResources) {

		if ingress, ok := resource.(*netv1.Ingress); ok {
			assert.True(t, len(ingress.Annotations) >= 1)
			assert.Equal(t, ingress.Annotations["myIngressKey-cr"], "myValue-0", resource.GetName())
			assert.NotNil(t, ingress.Spec.IngressClassName)
			assert.Equal(t, *ingress.Spec.IngressClassName, ingressClassVal)
			ingressOk = true
		}
	}
	assert.True(t, ingressOk)
}

func TestProcess_TemplateCustomAttributeMisSpellingIngress(t *testing.T) {

	var matchGvForIngress string = "networking.k8s.io/v1"
	var ingressClassVal = "SomeClass"
	cr := &brokerv1beta1.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{Name: "cr"},
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			ResourceTemplates: []brokerv1beta1.ResourceTemplate{
				{
					// match Ingress
					Selector: &brokerv1beta1.ResourceSelector{
						APIGroup: &matchGvForIngress,
					},
					Annotations: map[string]string{"myIngressKey-$(CR_NAME)": "myValue-$(BROKER_ORDINAL)"},
					Patch: &unstructured.Unstructured{Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"ingressClazzName": ingressClassVal, // wrong attribute
						},
					},
					},
				},
			},
			Acceptors: []brokerv1beta1.AcceptorType{{
				Name:       "aa",
				Port:       563,
				Expose:     true,
				SSLEnabled: false,
				ExposeMode: &brokerv1beta1.ExposeModes.Ingress,
			}},
		},
	}

	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log.WithName("test"), isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	namer := MakeNamers(cr)
	newSS, err := reconciler.ProcessStatefulSet(cr, *namer, nil)
	assert.NoError(t, err)

	fakeClient := fake.NewClientBuilder().Build()
	reconciler.ProcessAcceptorsAndConnectors(cr, *namer,
		fakeClient, nil, newSS)

	err = reconciler.ProcessResources(cr, fakeClient, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Clazz")
}

func TestProcess_TemplateCustomAttributeContainerSecurityContext(t *testing.T) {

	var kindMatchSs string = "StatefulSet"

	cr := &brokerv1beta1.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{Name: "cr"},
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			ResourceTemplates: []brokerv1beta1.ResourceTemplate{
				{
					Selector: &brokerv1beta1.ResourceSelector{
						Kind: &kindMatchSs,
					},
					Patch: &unstructured.Unstructured{Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name": "cr-container", // merge on name key
											"securityContext": map[string]interface{}{
												"runAsNonRoot": true,
											},
										},
									},
								},
							},
						},
					},
					},
				},
			},
		},
	}

	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log.WithName("test"), isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	namer := MakeNamers(cr)

	newSS, _ := reconciler.ProcessStatefulSet(cr, *namer, nil)
	reconciler.trackDesired(newSS)

	fakeClient := fake.NewClientBuilder().Build()
	err := reconciler.ProcessResources(cr, fakeClient, nil)
	assert.NoError(t, err)

	var runAsRootOk = false
	for _, resource := range common.ToResourceList(reconciler.requestedResources) {

		if ss, ok := resource.(*appsv1.StatefulSet); ok {
			assert.NotNil(t, ss.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot)
			assert.True(t, *ss.Spec.Template.Spec.Containers[0].SecurityContext.RunAsNonRoot)
			assert.NotEqual(t, "", ss.Spec.Template.Spec.Containers[0].Image)
			runAsRootOk = true
		}

	}
	assert.True(t, runAsRootOk)
}

func TestProcess_TemplateCustomAttributePriorityClassName(t *testing.T) {

	var kindMatchSs string = "StatefulSet"

	cr := &brokerv1beta1.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{Name: "cr"},
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			ResourceTemplates: []brokerv1beta1.ResourceTemplate{
				{
					Selector: &brokerv1beta1.ResourceSelector{
						Kind: &kindMatchSs,
					},
					Patch: &unstructured.Unstructured{Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"priorityClassName": "high-priority",
								},
							},
						},
					},
					},
				},
			},
		},
	}

	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log.WithName("test"), isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	namer := MakeNamers(cr)

	newSS, _ := reconciler.ProcessStatefulSet(cr, *namer, nil)
	reconciler.trackDesired(newSS)

	fakeClient := fake.NewClientBuilder().Build()
	err := reconciler.ProcessResources(cr, fakeClient, nil)
	assert.NoError(t, err)

	var priorityClassNameOk = false
	for _, resource := range common.ToResourceList(reconciler.requestedResources) {

		if ss, ok := resource.(*appsv1.StatefulSet); ok {
			assert.Equal(t, ss.Spec.Template.Spec.PriorityClassName, "high-priority")
			priorityClassNameOk = true
		}

	}
	assert.True(t, priorityClassNameOk)
}

func TestNewPodTemplateSpecForCR_AppendsDebugArgs(t *testing.T) {

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

	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log.WithName("test"), isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	newSpec, err := reconciler.PodTemplateSpecForCR(cr, common.Namers{}, &v1.PodTemplateSpec{}, k8sClient)

	assert.NoError(t, err)
	assert.NotNil(t, newSpec)
	expectedEnv := v1.EnvVar{
		Name:  "DEBUG_ARGS",
		Value: "-Dtest.arg=foo -Djava.security.auth.login.config=/amq/extra/secrets/test-config-jaas-config/login.config",
	}
	assert.Contains(t, newSpec.Spec.Containers[0].Env, expectedEnv)
}

func TestNewPodTemplateSpecForCR_IncludesImagePullSecret(t *testing.T) {

	cr := &brokerv1beta1.ActiveMQArtemis{
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			DeploymentPlan: brokerv1beta1.DeploymentPlanType{
				ImagePullSecrets: []v1.LocalObjectReference{
					{
						Name: "testPullSecret",
					},
				},
			},
		},
	}
	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log, isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	newSpec, err := reconciler.PodTemplateSpecForCR(cr, common.Namers{}, &v1.PodTemplateSpec{}, k8sClient)
	assert.NoError(t, err)
	assert.NotNil(t, newSpec)
	expectedPullSecret := []v1.LocalObjectReference{
		{
			Name: "testPullSecret",
		},
	}
	assert.Equal(t, newSpec.Spec.ImagePullSecrets, expectedPullSecret)
}

func TestNewPodTemplateSpecForCR_IncludesTopologySpreadConstraints(t *testing.T) {
	matchLabels := make(map[string]string)
	matchLabels["my-label"] = "my-value"

	mySelector := &metav1.LabelSelector{
		MatchLabels: matchLabels,
	}

	cr := &brokerv1beta1.ActiveMQArtemis{
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			DeploymentPlan: brokerv1beta1.DeploymentPlanType{
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{
					{
						MaxSkew:           int32(1),
						TopologyKey:       string("topology.kubernetes.io/zone"),
						WhenUnsatisfiable: v1.ScheduleAnyway,
						LabelSelector:     mySelector,
					},
				},
			},
		},
	}
	outer := NewActiveMQArtemisReconciler(&NillCluster{}, ctrl.Log, isOpenshift)
	reconciler := NewActiveMQArtemisReconcilerImpl(cr, outer)

	newSpec, err := reconciler.PodTemplateSpecForCR(cr, common.Namers{}, &v1.PodTemplateSpec{}, k8sClient)
	assert.NoError(t, err)
	assert.NotNil(t, newSpec)
	expectedTopologySpreadConstraints := []v1.TopologySpreadConstraint{
		{
			MaxSkew:           int32(1),
			TopologyKey:       string("topology.kubernetes.io/zone"),
			WhenUnsatisfiable: v1.ScheduleAnyway,
			LabelSelector:     mySelector,
		},
	}
	assert.Equal(t, newSpec.Spec.TopologySpreadConstraints, expectedTopologySpreadConstraints)
}

func TestNewPodTemplateSpecForCR_IncludesContainerSecurityContext(t *testing.T) {
	containerSecurityContext := &v1.SecurityContext{RunAsNonRoot: utilpointer.Bool(false)}

	cr := &brokerv1beta1.ActiveMQArtemis{
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			DeploymentPlan: brokerv1beta1.DeploymentPlanType{
				ContainerSecurityContext: containerSecurityContext,
			},
		},
	}

	reconciler := &ActiveMQArtemisReconcilerImpl{
		log:            ctrl.Log.WithName("test"),
		customResource: cr,
	}

	newSpec, err := reconciler.PodTemplateSpecForCR(cr, common.Namers{}, &v1.PodTemplateSpec{}, k8sClient)

	assert.NoError(t, err)
	assert.NotNil(t, newSpec)
	expectedSecurityContext := &v1.SecurityContext{RunAsNonRoot: utilpointer.Bool(false)}

	assert.Equal(t, newSpec.Spec.Containers[0].SecurityContext, expectedSecurityContext)
	assert.Equal(t, newSpec.Spec.InitContainers[0].SecurityContext, expectedSecurityContext)
}

func TestLoginConfigSyntaxCheck(t *testing.T) {
	good := map[string][]byte{
		"simple": []byte(`a {
		SampleLoginModule Required  a=b b=d;
		SampleLoginModule Optional;
		SampleLoginModule requisite;
		SampleLoginModule sufficient;
	   };`),
		"equalsSpace": []byte(`a {
		SampleLoginModule Required  a = b b= d c = 4;
		SampleLoginModule Optional
		a =2
		b= 3
		c = 4
		;
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

		"full-ldap-quoted-val": []byte(`
	 activemq	{ org.apache.activemq.artemis.spi.core.security.jaas.LDAPLoginModule sufficient 
		debug=true 
		initialContextFactory=com.sun.jndi.ldap.LdapCtxFactory 
		connectionURL="ldap://blabla"
		connectionTimeout="5000"
		connectionProtocol="simple"
		readTimeout="5000"
		authentication="simple"
		userBase="DC=aa,DC=aaa,DC=aaaaa,DC=aa,DC=aa"
		userSearchMatching="(&(objectCategory=user)(SAMAccountName=\\{0}))"
		roleBase="DC=aa,DC=aaa,DC=aaaaa,DC=aa,DC=aa"
		roleName=sAMAccountName
		roleSearchMatching="(&(objectCategory=group)(groupType:1.1.111.111111.1.1.111:=1111111111)(member:1.1.111.111111.1.1.1111:=\{0})(sAMAccountName=AAA AAA AAA*))"
		referral=follow;	
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

		"dual_munged_opt": []byte(`aa 
	 {
	     SampleLoginModule sufficientRequired a=b;
	 };`),

		"dual_or_opt": []byte(`aa 
	 {
	     SampleLoginModule Sufficient|Required a=b;
	 };`),

		"dual_space_opt": []byte(`aa 
	 {
	     SampleLoginModule Sufficient Required a=b;
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

func TestStatusMarshall(t *testing.T) {

	Status := brokerv1beta1.ActiveMQArtemisStatus{
		Conditions: []metav1.Condition{},
		PodStatus: olm.DeploymentStatus{
			Ready:    []string{},
			Starting: []string{},
			Stopped:  []string{},
		},
		DeploymentPlanSize: 0,
		ScaleLabelSelector: "",
		ExternalConfigs:    []brokerv1beta1.ExternalConfigStatus{},
		Version:            brokerv1beta1.VersionStatus{},
		Upgrade:            brokerv1beta1.UpgradeStatus{},
	}
	v, err := json.Marshal(Status)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(v), ":false"))

}

func TestGetBrokerHost(t *testing.T) {
	cr := brokerv1beta1.ActiveMQArtemis{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "test",
		},
		Spec: brokerv1beta1.ActiveMQArtemisSpec{
			IngressDomain: "my-domain.com",
		},
	}

	var ingressHost string
	specIngressHost := "$(CR_NAME)-$(CR_NAMESPACE)-$(ITEM_NAME)-$(BROKER_ORDINAL)-$(RES_TYPE).$(INGRESS_DOMAIN)"

	ingressHost = formatTemplatedString(&cr, specIngressHost, "0", "my-acceptor", "ing")
	assert.Equal(t, "test-test-ns-my-acceptor-0-ing.my-domain.com", ingressHost)

	ingressHost = formatTemplatedString(&cr, specIngressHost, "1", "my-connector", "rte")
	assert.Equal(t, "test-test-ns-my-connector-1-rte.my-domain.com", ingressHost)

	ingressHost = formatTemplatedString(&cr, specIngressHost, "2", "my-console", "abc")
	assert.Equal(t, "test-test-ns-my-console-2-abc.my-domain.com", ingressHost)
}

func TestParseBrokerPropertyWithOrdinal(t *testing.T) {
	var matches []string

	matches = ParseBrokerPropertyWithOrdinal("broker-0.maxDiskUsage")
	assert.Equal(t, 3, len(matches))
	assert.Equal(t, "broker-0.maxDiskUsage", matches[0])
	assert.Equal(t, "broker-0", matches[1])
	assert.Equal(t, "maxDiskUsage", matches[2])

	matches = ParseBrokerPropertyWithOrdinal("broker-999.maxDiskUsage=97")
	assert.Equal(t, 3, len(matches))
	assert.Equal(t, "broker-999.maxDiskUsage=97", matches[0])
	assert.Equal(t, "broker-999", matches[1])
	assert.Equal(t, "maxDiskUsage=97", matches[2])

	matches = ParseBrokerPropertyWithOrdinal("maxDiskUsage=97")
	assert.Equal(t, 0, len(matches))

	matches = ParseBrokerPropertyWithOrdinal("a.broker-0.maxDiskUsage")
	assert.Equal(t, 0, len(matches))

	matches = ParseBrokerPropertyWithOrdinal("broker-0-maxDiskUsage")
	assert.Equal(t, 0, len(matches))

	matches = ParseBrokerPropertyWithOrdinal("broker-a.maxDiskUsage")
	assert.Equal(t, 0, len(matches))
}

func TestBrokerPropertiesData(t *testing.T) {

	data := BrokerPropertiesData([]string{
		"maxDiskUsage=97",
		"minDiskFree=5",
	})

	assert.Equal(t, 1, len(data))

	assert.True(t, strings.Contains(data[BrokerPropertiesName], "maxDiskUsage=97"))
	assert.True(t, strings.Contains(data[BrokerPropertiesName], "minDiskFree=5"))
}

func TestBrokerPropertiesDataWithOrdinal(t *testing.T) {

	data := BrokerPropertiesData([]string{
		"broker-0.maxDiskUsage=98",
		"broker-0.minDiskFree=6",
		"broker-999.maxDiskUsage=99",
		"broker-999.minDiskFree=7",
	})

	assert.Equal(t, 3, len(data))

	assert.False(t, strings.Contains(data[BrokerPropertiesName], "maxDiskUsage"))
	assert.False(t, strings.Contains(data[BrokerPropertiesName], "minDiskFree"))

	broker0BrokerPropertiesName := "broker-0" + OrdinalPrefixSep + BrokerPropertiesName
	assert.True(t, strings.Contains(data[broker0BrokerPropertiesName], "maxDiskUsage=98"))
	assert.True(t, strings.Contains(data[broker0BrokerPropertiesName], "minDiskFree=6"))

	broker999BrokerPropertiesName := "broker-999" + OrdinalPrefixSep + BrokerPropertiesName
	assert.True(t, strings.Contains(data[broker999BrokerPropertiesName], "maxDiskUsage=99"))
	assert.True(t, strings.Contains(data[broker999BrokerPropertiesName], "minDiskFree=7"))
}

func TestBrokerPropertiesDataWithAndWithoutOrdinal(t *testing.T) {

	data := BrokerPropertiesData([]string{
		"maxDiskUsage=97",
		"minDiskFree=5",
		"broker-0.maxDiskUsage=98",
		"broker-0.minDiskFree=6",
		"broker-999.maxDiskUsage=99",
		"broker-999.minDiskFree=7",
	})

	assert.Equal(t, 3, len(data))

	assert.True(t, strings.Contains(data[BrokerPropertiesName], "maxDiskUsage=97"))
	assert.True(t, strings.Contains(data[BrokerPropertiesName], "minDiskFree=5"))

	broker0BrokerPropertiesName := "broker-0" + OrdinalPrefixSep + BrokerPropertiesName
	assert.True(t, strings.Contains(data[broker0BrokerPropertiesName], "maxDiskUsage=98"))
	assert.True(t, strings.Contains(data[broker0BrokerPropertiesName], "minDiskFree=6"))

	broker999BrokerPropertiesName := "broker-999" + OrdinalPrefixSep + BrokerPropertiesName
	assert.True(t, strings.Contains(data[broker999BrokerPropertiesName], "maxDiskUsage=99"))
	assert.True(t, strings.Contains(data[broker999BrokerPropertiesName], "minDiskFree=7"))
}
